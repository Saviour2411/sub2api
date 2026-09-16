#!/usr/bin/env python3
"""隔离真实服务的混合协议切流回归；只向本地模拟上游生成请求。

使用当前构建的 sub2api 进程、真实 Nginx/Redis/PostgreSQL，不访问生产数据。
这里验证应用协议与排空，不替代 deploy.py 的生产主机/资源/首次迁移门禁。
"""
import argparse
import base64
import concurrent.futures
import hashlib
import http.server
import http.client
import json
import os
from pathlib import Path
import socket
import ssl
import struct
import subprocess
import tempfile
import threading
import time
import urllib.error
import urllib.request
import uuid

ROOT = Path(__file__).resolve().parents[2]


def command(*args, **kwargs):
    return subprocess.check_output(args, text=True, stderr=subprocess.STDOUT, **kwargs).strip()


def eventually(fn, seconds=30):
    deadline = time.monotonic()+seconds
    last = None
    while time.monotonic() < deadline:
        try:
            result = fn()
            if result:
                return result
        except (OSError, ValueError, subprocess.CalledProcessError) as error:
            last = error
        time.sleep(0.05)
    raise AssertionError(f"等待条件超时: {last}")


def read_exact(stream, n):
    data = b""
    while len(data) < n:
        chunk = stream.read(n-len(data))
        if not chunk:
            raise EOFError("WS 帧被截断")
        data += chunk
    return data


def read_frame(stream):
    first, second = read_exact(stream, 2)
    length = second & 127
    if length == 126:
        length = struct.unpack("!H", read_exact(stream, 2))[0]
    elif length == 127:
        length = struct.unpack("!Q", read_exact(stream, 8))[0]
    mask = read_exact(stream, 4) if second & 128 else None
    body = read_exact(stream, length)
    if mask:
        body = bytes(b ^ mask[i % 4] for i, b in enumerate(body))
    return first, body


def write_frame(stream, data, mask=False, opcode=1):
    body = data if isinstance(data, bytes) else json.dumps(data).encode()
    n = len(body)
    header = bytes([128 | opcode, (128 if mask else 0) | (n if n < 126 else 126 if n <= 65535 else 127)])
    if n >= 126:
        header += struct.pack("!H" if n <= 65535 else "!Q", n)
    if mask:
        key = os.urandom(4)
        header += key
        body = bytes(b ^ key[i % 4] for i, b in enumerate(body))
    stream.write(header+body)
    stream.flush()


class Upstream(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    holds = {}
    seen = {}
    lock = threading.Lock()

    def log_message(self, *args):
        pass

    def events(self, payload):
        name = payload.get("input", "")
        if not isinstance(name, str):
            name = json.dumps(name)
        with self.lock:
            self.seen[name] = self.seen.get(name, 0)+1
        response = {"id": "resp_"+hashlib.sha256(name.encode()).hexdigest()[:24], "object": "response", "model": "gpt-5.1", "status": "completed", "output": [{"type":"message", "role":"assistant", "content":[{"type":"output_text", "text":"ok"}]}], "usage":{"input_tokens":7, "output_tokens":3, "total_tokens":10}}
        yield {"type":"response.created", "response":dict(response, status="in_progress", usage=None)}
        yield {"type":"response.output_text.delta", "delta":"ok"}
        hold = self.holds.get(name)
        if hold:
            hold[0].set()
            assert hold[1].wait(90), "测试没有释放上游流"
        yield {"type":"response.completed", "response":response}

    def do_POST(self):
        payload = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        if not payload.get("stream"):
            # 账号创建时的能力探测不是客户生成，也不计入逐笔用量基线。
            body = json.dumps({"id":"probe", "object":"response", "status":"completed", "output":[{"type":"function_call", "name":"probe_ping", "call_id":"probe", "arguments":"{\"ok\":true}"}]}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Connection", "close")
        self.send_header("X-Request-Id", "mock-"+uuid.uuid4().hex)
        self.end_headers()
        self.close_connection = True
        try:
            for event in self.events(payload):
                self.wfile.write(("data: "+json.dumps(event)+"\n\n").encode())
                self.wfile.flush()
        except (BrokenPipeError, ConnectionResetError):
            pass

    def do_GET(self):
        if self.headers.get("Upgrade", "").lower() != "websocket":
            self.send_error(404)
            return
        accept = base64.b64encode(hashlib.sha1((self.headers["Sec-WebSocket-Key"]+"258EAFA5-E914-47DA-95CA-C5AB0DC85B11").encode()).digest()).decode()
        self.close_connection = True
        self.send_response(101)
        self.send_header("Upgrade", "websocket")
        self.send_header("Connection", "Upgrade")
        self.send_header("Sec-WebSocket-Accept", accept)
        self.end_headers()
        try:
            while True:
                first, body = read_frame(self.rfile)
                opcode = first & 15
                if opcode == 8:
                    write_frame(self.wfile, body, opcode=8)
                    return
                if opcode == 9:
                    write_frame(self.wfile, body, opcode=10)
                    continue
                assert opcode == 1
                for event in self.events(json.loads(body)):
                    write_frame(self.wfile, event)
        except (EOFError, OSError):
            pass


class ClientWS:
    def __init__(self, port, key, session):
        self.session = session
        self.conn = ssl._create_unverified_context().wrap_socket(socket.create_connection(("127.0.0.1", port), timeout=20), server_hostname="localhost")
        self.stream = self.conn.makefile("rwb", buffering=0)
        nonce = base64.b64encode(os.urandom(16)).decode()
        self.stream.write((f"GET /v1/responses HTTP/1.1\r\nHost: localhost\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: {nonce}\r\nSec-WebSocket-Version: 13\r\nAuthorization: Bearer {key}\r\nsession_id: {session}\r\n\r\n").encode())
        first = self.stream.readline()
        assert b"101" in first, first
        while self.stream.readline() != b"\r\n":
            pass

    def turn(self, name, previous=None):
        payload = {"type":"response.create", "model":"gpt-5.1", "input":name, "prompt_cache_key":self.session, "stream":True}
        if previous:
            payload["previous_response_id"] = previous
        write_frame(self.stream, payload, mask=True)
        message = b""
        while True:
            first, body = read_frame(self.stream)
            opcode = first & 15
            if opcode == 9:
                write_frame(self.stream, body, mask=True, opcode=10)
                continue
            if opcode == 10:
                continue
            assert opcode in (0, 1), body
            if opcode == 1:
                message = b""
            message += body
            if not first & 128:
                continue
            event = json.loads(message)
            assert event.get("type") != "error", event
            if event.get("type") == "response.completed":
                return event["response"]["id"]

    def close(self):
        try:
            write_frame(self.stream, struct.pack("!H", 1000), mask=True, opcode=8)
            read_frame(self.stream)
        finally:
            self.stream.close()
            self.conn.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", default=str(ROOT/".cache/bluegreen-server"))
    parser.add_argument("--legacy-binary", help="使用实际 v0.1.234 构建验证首次续接")
    parser.add_argument("--cycles", type=int, default=20)
    args = parser.parse_args()
    cache = ROOT/".cache/tmp"
    cache.mkdir(parents=True, exist_ok=True)
    folder = Path(tempfile.mkdtemp(prefix="bg-", dir=cache))
    tag = "sub2api-test-"+uuid.uuid4().hex[:10]
    (folder/"test-id").write_text(tag)
    containers, processes, logs, samples = [], {}, [], []
    upstream = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Upstream)
    threading.Thread(target=upstream.serve_forever, daemon=True).start()
    print("隔离测试目录:", folder, flush=True)
    ports = {}
    for slot in ("blue", "green"):
        with socket.socket() as sock:
            sock.bind(("127.0.0.1", 0))
            ports[slot] = sock.getsockname()[1]
    data = folder/"data"
    data.mkdir()

    def docker(image, name, port, *options):
        full = tag+"-"+name
        command("docker", "run", "--detach", "--name", full, "--label", "sub2api.bluegreen-test="+tag, "-p", f"127.0.0.1::{port}", *options, image)
        containers.append(full)
        return int(command("docker", "port", full, str(port)+"/tcp").rsplit(":", 1)[1])

    def pg(query):
        return command("docker", "exec", tag+"-pg", "psql", "-U", "test", "-d", "test", "-Atc", query)

    def control(slot, action="state"):
        cmd = ["curl", "-fsS", "--max-time", "5", "--unix-socket", str(folder/(slot+".sock"))]
        if action in ("activate", "drain", "retire"):
            cmd += ["-X", "POST"]
        result = command(*cmd, "http://local/"+action)
        return json.loads(result) if action == "state" else True

    def api(path, payload=None, token=None):
        req = urllib.request.Request(f"http://127.0.0.1:{ports['blue']}"+path, data=json.dumps(payload).encode() if payload is not None else None, headers={"Content-Type":"application/json", **({"Authorization":"Bearer "+token} if token else {})})
        try:
            opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
            with opener.open(req, timeout=20) as response:
                return json.load(response)["data"]
        except urllib.error.HTTPError as error:
            raise AssertionError(error.read().decode()) from error

    def start(slot, legacy=False, old=False):
        log = (folder/(slot+"-"+uuid.uuid4().hex[:5]+".log")).open("w")
        logs.append(log)
        env = dict(os.environ, AUTO_SETUP="true", DATA_DIR=str(data), SERVER_HOST="0.0.0.0", SERVER_PORT=str(ports[slot]), DATABASE_HOST="127.0.0.1", DATABASE_PORT=str(pg_port), DATABASE_USER="test", DATABASE_PASSWORD="test-only-password", DATABASE_DBNAME="test", DATABASE_SSLMODE="disable", DATABASE_MAX_OPEN_CONNS="20", DATABASE_MAX_IDLE_CONNS="5", REDIS_HOST="127.0.0.1", REDIS_PORT=str(redis_port), ADMIN_EMAIL="test@example.test", ADMIN_PASSWORD="test-only-Password-123!", JWT_SECRET="test-only-jwt-secret-012345678901234567890123456789", TOTP_ENCRYPTION_KEY="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", GOMEMLIMIT="512MiB", LIFECYCLE_MODE="standby", LIFECYCLE_SOCKET=str(folder/(slot+".sock")), GATEWAY_OPENAI_WS_ENABLED="true", GATEWAY_OPENAI_WS_APIKEY_ENABLED="true", GATEWAY_OPENAI_WS_RESPONSES_WEBSOCKETS_V2="true", GATEWAY_OPENAI_WS_MODE_ROUTER_V2_ENABLED="true", GATEWAY_OPENAI_WS_STICKY_SESSION_TTL_SECONDS="5", GATEWAY_OPENAI_WS_STICKY_RESPONSE_ID_TTL_SECONDS="5", TZ="UTC")
        env["LIFECYCLE_LEGACY"] = str(legacy).lower()
        if legacy or old:
            env["GATEWAY_OPENAI_WS_MAX_INGRESS_CONNECTIONS_PER_API_KEY"] = "2"
            # 首次迁移保留旧版，不测试过期；避免启动时间消耗测试专用的 5 秒 TTL。
            env["GATEWAY_OPENAI_WS_STICKY_SESSION_TTL_SECONDS"] = "60"
            env["GATEWAY_OPENAI_WS_STICKY_RESPONSE_ID_TTL_SECONDS"] = "60"
        process = subprocess.Popen([str(Path(args.legacy_binary if old else args.binary).resolve())], cwd=folder, env=env, stdout=log, stderr=subprocess.STDOUT)
        processes[slot] = process
        def started():
            assert process.poll() is None, f"候选启动失败，详见 {log.name}"
            if old:
                return json.loads(command("curl","-fsS","--max-time","2",f"http://127.0.0.1:{ports[slot]}/health"))["status"] == "ok"
            return control(slot)["state"] == "standby"
        eventually(started, 90)
        if old:
            return
        control(slot, "check")
        assert "event: complete" in command("curl", "-fsS", "--unix-socket", str(folder/(slot+".sock")), "http://local/synthetic")

    legacy_proxy = ""

    def switch(slot):
        control(slot, "activate")
        (folder/"upstream.conf").write_text(f"upstream app {{ server host.docker.internal:{ports[slot]}; }}\n"+legacy_proxy)
        command("docker", "exec", tag+"-nginx", "nginx", "-t")
        begin = time.monotonic()
        command("docker", "exec", tag+"-nginx", "nginx", "-s", "reload")
        expected = control(slot)["instance_id"]
        eventually(lambda: all(expected in command("curl", "-kfsS", "--max-time", "2", "--http2", "-D", "-", f"https://127.0.0.1:{port}/readyz") for port in (proxy_port, direct_port)), 5)
        return time.monotonic()-begin

    def stream(name, session=None, previous=None):
        payload = {"model":"gpt-5.1", "input":name, "stream":True}
        if previous:
            payload["previous_response_id"] = previous
        headers = ["-H", "Authorization: Bearer "+api_key, "-H", "Content-Type: application/json"]
        if session:
            headers += ["-H", "session_id: "+session]
        try:
            result = command("curl", "-ksS", "--fail-with-body", "--http2", "--max-time", "75", "-w", "\n%{http_version}", *headers, "--data", json.dumps(payload), f"https://127.0.0.1:{proxy_port}/v1/responses")
        except subprocess.CalledProcessError as error:
            raise AssertionError(f"流式请求 {name} 失败: {error.output}") from error
        assert result.endswith("\n2"), "没有使用 HTTP/2"
        assert '"response.completed"' in result, result[:1000]
        return result

    try:
        pg_port = docker("postgres:18.1-alpine3.23", "pg", 5432, "-e", "POSTGRES_USER=test", "-e", "POSTGRES_DB=test", "-e", "POSTGRES_PASSWORD=test-only-password")
        redis_port = docker("redis:8.4-alpine", "redis", 6379)
        eventually(lambda: pg("SELECT 1") == "1")
        start("blue")
        control("blue", "activate")
        pg("UPDATE users SET balance=1000, concurrency=100")
        token = api("/api/v1/auth/login", {"email":"test@example.test", "password":"test-only-Password-123!"})["access_token"]
        # 仅给隔离测试库中的虚构管理员建立测试确认，不代表任何生产用户作出承诺。
        api("/api/v1/admin/compliance/accept", {"phrase":"我已阅读、理解并同意 Sub2API 部署与运营合规承诺", "language":"zh"}, token)
        group = api("/api/v1/admin/groups", {"name":"bluegreen-test", "platform":"openai", "rate_multiplier":1, "subscription_type":"standard"}, token)
        api("/api/v1/admin/accounts", {"name":"local-mock-only", "platform":"openai", "type":"apikey", "credentials":{"api_key":"mock-only", "base_url":f"http://127.0.0.1:{upstream.server_port}"}, "extra":{"openai_passthrough":True, "openai_apikey_responses_websockets_v2_enabled":True, "openai_apikey_responses_websockets_v2_mode":"passthrough"}, "concurrency":100, "priority":1, "group_ids":[group["id"]], "upstream_billing_probe_enabled":False}, token)
        api_key = api("/api/v1/keys", {"name":"bluegreen-test", "group_id":group["id"]}, token)["key"]
        command("openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", str(folder/"tls.key"), "-out", str(folder/"tls.crt"), "-days", "1", "-subj", "/CN=localhost")
        (folder/"upstream.conf").write_text(f"upstream app {{ server host.docker.internal:{ports['blue']}; }}\n")
        (folder/"nginx.conf").write_text('''events {}\nhttp { include /test/upstream.conf; map $http_upgrade $upgrade_connection { default upgrade; '' close; } server { listen 8443 ssl; listen 8444 ssl; http2 on; ssl_certificate /test/tls.crt; ssl_certificate_key /test/tls.key; location / { proxy_pass http://app; proxy_http_version 1.1; proxy_set_header Upgrade $http_upgrade; proxy_set_header Connection $upgrade_connection; proxy_buffering off; proxy_request_buffering off; proxy_read_timeout 90s; proxy_next_upstream off; } } }\n''')
        proxy_port = docker("nginx:1.28-alpine", "nginx", 8443, "-p", "127.0.0.1::8444", "--add-host", "host.docker.internal:host-gateway", "-v", str(folder)+":/test:rw", "-v", str(folder/"nginx.conf")+":/etc/nginx/nginx.conf:ro")
        direct_port = int(command("docker", "port", tag+"-nginx", "8444/tcp").rsplit(":", 1)[1])
        for entrance_port in (proxy_port,direct_port):
            eventually(lambda: command("curl","-kfsS","--noproxy","*","--http2","--max-time","3",
                                       f"https://127.0.0.1:{entrance_port}/health"))
        stream("warmup")
        health_stop, health_errors, health_count = threading.Event(), [], [0]
        def health_traffic():
            context = ssl._create_unverified_context()
            while not health_stop.is_set():
                connection = http.client.HTTPSConnection("127.0.0.1", proxy_port, context=context, timeout=3)
                started, stage = time.monotonic(), "connect"
                try:
                    connection.request("GET", "/health", headers={"Connection":"close"})
                    stage = "headers"
                    response = connection.getresponse()
                    assert response.status == 200
                    stage = "body"
                    response.read()
                    health_count[0] += 1
                except Exception as error:
                    health_errors.append({"error":str(error), "stage":stage, "elapsed":time.monotonic()-started, "at":time.time()})
                finally:
                    connection.close()
                health_stop.wait(0.01)
        health_thread = threading.Thread(target=health_traffic, daemon=True)
        health_thread.start()
        active = "blue"
        for index in range(args.cycles):
            candidate = "green" if active == "blue" else "blue"
            start(candidate)
            session = f"cycle-{index}"
            ws = ClientWS(proxy_port, api_key, session)
            ws.turn(session+"-ws-before")
            hold_name = session+"-long"
            Upstream.holds[hold_name] = (threading.Event(), threading.Event())
            detached_name = session+"-disconnected"
            Upstream.holds[detached_name] = (threading.Event(), threading.Event())
            detached = subprocess.Popen(["curl", "-kfsS", "--http2", "--max-time", "70", "-H", "Authorization: Bearer "+api_key, "-H", "Content-Type: application/json", "--data", json.dumps({"model":"gpt-5.1", "input":detached_name, "stream":True}), f"https://127.0.0.1:{direct_port}/v1/responses"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            assert Upstream.holds[detached_name][0].wait(20), "断连测试未开始生成"
            detached.terminate()
            detached.wait(timeout=5)
            with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
                long_stream = pool.submit(stream, hold_name)
                assert Upstream.holds[hold_name][0].wait(20), "长流未抵达模拟上游"
                try:
                    latency = switch(candidate)
                    control(active, "drain")
                    drain_started = time.monotonic()
                    snapshot = control(active)
                    assert snapshot["state"] == "draining" and snapshot["work"].get("ws_connections", 0) > 0 and snapshot["work"].get("sse", 0) > 0, snapshot
                    stream(session+"-new-request")
                    ws.turn(session+"-ws-after")
                    # 新入口建立的同会话 WS 必须转发到旧实例，不能重复调度和计费。
                    forwarded = ClientWS(direct_port, api_key, session)
                    forwarded.turn(session+"-ws-forwarded")
                    same_owner = control(active)["work"].get("ws_connections", 0) >= 2
                    forwarded.close()
                    ws.close()
                    Upstream.holds[hold_name][1].set()
                    Upstream.holds[detached_name][1].set()
                    long_stream.result(timeout=20)
                    assert same_owner, "同会话跨入口没有留在拥有者"
                finally:
                    Upstream.holds[hold_name][1].set()
                    Upstream.holds[detached_name][1].set()
            eventually(lambda: control(active)["state"] == "drained", 30)
            control(active, "retire")
            assert processes[active].wait(timeout=30) == 0
            samples.append({"cycle":index+1, "switch_seconds":latency, "drain_seconds":time.monotonic()-drain_started})
            print(json.dumps(samples[-1]), flush=True)
            active = candidate
        expected = sum(Upstream.seen.values())
        eventually(lambda: int(pg("SELECT count(*) FROM usage_logs")) == expected)
        assert all(count == 1 for count in Upstream.seen.values()), Upstream.seen
        assert pg("SELECT count(*) FROM usage_logs WHERE input_tokens<>7 OR output_tokens<>3 OR total_cost<=0") == "0"
        health_stop.set()
        health_thread.join(timeout=5)
        assert not health_errors, health_errors
        assert health_count[0] > 0
        control(active, "drain")
        eventually(lambda: control(active)["state"] == "drained", 30)
        control(active, "retire")
        assert processes[active].wait(timeout=30) == 0
        assert max(sample["switch_seconds"] for sample in samples) < 2, samples
        legacy_result = None
        if args.legacy_binary:
            start("blue",old=True)
            legacy_proxy = (f"server {{ listen unix:/test/legacy.sock; location / {{ proxy_pass http://host.docker.internal:{ports['blue']}; "
                            "proxy_http_version 1.1; proxy_set_header Upgrade $http_upgrade; proxy_set_header Connection $upgrade_connection; "
                            "proxy_buffering off; proxy_request_buffering off; proxy_next_upstream off; proxy_read_timeout 90s; } }\n")
            (folder/"upstream.conf").write_text(f"upstream app {{ server host.docker.internal:{ports['blue']}; }}\n"+legacy_proxy)
            command("docker","exec",tag+"-nginx","nginx","-s","reload")
            eventually(lambda: (folder/"legacy.sock").exists())
            ws = ClientWS(proxy_port,api_key,"legacy-session")
            previous = ws.turn("legacy-before")
            # v0.1.234 只给 HTTP 响应登记 HTTP 续接身份，不能用旧 WS 响应冒充 HTTP 归属。
            http_before = stream("legacy-http-before")
            http_previous = next(json.loads(line[6:])["response"]["id"] for line in http_before.splitlines()
                                 if line.startswith("data: ") and json.loads(line[6:]).get("type") == "response.completed")
            hold = "legacy-long-stream"
            Upstream.holds[hold] = (threading.Event(),threading.Event())
            with concurrent.futures.ThreadPoolExecutor(max_workers=1) as pool:
                pending_stream = pool.submit(stream,hold)
                assert Upstream.holds[hold][0].wait(20)
                start("green",legacy=True)
                latency = switch("green")
                previous = ws.turn("legacy-still-connected",previous=previous)
                forwarded = ClientWS(direct_port,api_key,"legacy-session")
                forwarded.turn("legacy-forwarded",previous=previous)
                # 上限 2：仍活着的旧 WS + 新入口续接只能占两份，不能中转侧再多占一份。
                leases = command("docker","exec",tag+"-redis","redis-cli","--scan","--pattern","concurrency:openai_ws_ingress:api_key:*").splitlines()
                assert len(leases) == 1
                assert command("docker","exec",tag+"-redis","redis-cli","ZCARD",leases[0]) == "2"
                stream("legacy-response-continuation",previous=http_previous)
                stream("first-new-instance")
                forwarded.close()
                ws.close()
                Upstream.holds[hold][1].set()
                pending_stream.result(timeout=20)
            assert processes["blue"].poll() is None, "旧版必须保留，不伪装已排空"
            expected = sum(Upstream.seen.values())
            eventually(lambda: int(pg("SELECT count(*) FROM usage_logs")) == expected)
            assert all(n == 1 for n in Upstream.seen.values())
            assert pg("SELECT count(*) FROM usage_logs WHERE input_tokens<>7 OR output_tokens<>3 OR total_cost<=0") == "0"
            legacy_result = {"switch_seconds":latency,"old_retained":True,"ingress_lease_count":2,"usage_rows":expected}
            print("实际旧版首次迁移协议验证："+json.dumps(legacy_result),flush=True)
        report = {"sha":command("git", "rev-parse", "HEAD", cwd=ROOT), "cycles":args.cycles, "upstream_requests":expected, "usage_rows":expected, "ordinary_requests":health_count[0], "ordinary_errors":health_errors, "samples":samples, "legacy":legacy_result}
        (ROOT/".analysis_tmp").mkdir(exist_ok=True)
        (ROOT/".analysis_tmp/bluegreen-protocol-result.json").write_text(json.dumps(report, indent=2)+"\n")
        print("真实应用协议切流和逐笔用量回归通过", flush=True)
    finally:
        if "health_stop" in locals():
            health_stop.set()
            health_thread.join(timeout=5)
        for started, release in Upstream.holds.values():
            release.set()
        for process in processes.values():
            if process.poll() is None:
                # 仅隔离测试的失败清理；绝不用于生产发布或宣称排空成功。
                process.kill()
                process.wait()
        for container in reversed(containers):
            if container.endswith("-nginx"):
                (folder/"nginx.log").write_text(command("docker", "logs", container))
            command("docker", "rm", "--force", "--volumes", container)
        upstream.shutdown()
        for log in logs:
            log.close()
        # 失败日志保留在项目内，便于 CI 上传与复现；无生产密钥。


if __name__ == "__main__":
    main()
