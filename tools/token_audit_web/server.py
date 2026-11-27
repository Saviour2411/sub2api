"""登录、受限任务队列和测试包进程管理。"""
from __future__ import annotations

import argparse
import hashlib
import hmac
from http.cookies import SimpleCookie
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import ipaddress
import json
import mimetypes
import os
from pathlib import Path
import queue
import re
import secrets
import shutil
import signal
import socket
import subprocess
import sys
import threading
import time
from urllib.parse import urlsplit

from token_audit.protocol import MessagesClient, endpoint_url, redact, utc_now
from token_audit.freshness import catalog

TOOLS = Path(__file__).resolve().parents[1]
STATIC = Path(__file__).resolve().parent / "static"
FILES = {"report.md", "summary.json", "events.jsonl", "rounds.csv", "signature-cases.csv", "concurrency.csv", "plan.json"}
TESTS = ("conversation", "fingerprint", "signature_freshness", "signature_relay", "signature", "concurrency")
DEFAULTS = {"model": "claude-opus-5-5", "freshness_models": "claude-opus-5-5,claude-sonnet-4-6",
            "tests": ["conversation", "fingerprint", "signature_freshness"], "concurrency": 200,
            "timeout": 180, "signature_samples": 2, "freshness_samples": 2, "freshness_interval": 3,
            "load_max_tokens": 2048, "auth": "x-api-key", "peer_auth": "x-api-key", "require_awsb": False}


def save_json(path, value):
    tmp = path.with_suffix(".pending")
    tmp.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    tmp.replace(path)


def public_url(value, enforce=True):
    url = endpoint_url(value)
    parsed = urlsplit(url)
    if len(url) > 2048 or any(ord(c) < 33 for c in url) or "\\" in url:
        raise ValueError("渠道地址格式无效")
    if enforce:
        addresses = socket.getaddrinfo(parsed.hostname, parsed.port or (443 if parsed.scheme == "https" else 80), type=socket.SOCK_STREAM)
        if not addresses or any(not ipaddress.ip_address(a[4][0]).is_global or ipaddress.ip_address(a[4][0]).is_multicast for a in addresses):
            raise ValueError("仅支持公网HTTP或HTTPS渠道地址")
    return url


def normalize(data, enforce=True):
    if not isinstance(data, dict):
        raise ValueError("请求必须是JSON对象")
    config = {**DEFAULTS, **{k: data[k] for k in DEFAULTS if k in data}}
    for name in ("url", "key", "peer_url", "peer_key", "label"):
        value = data.get(name, "")
        if not isinstance(value, str):
            raise ValueError(name + "必须为字符串")
        config[name] = value.strip()
    if not config["url"]:
        raise ValueError("请填写渠道URL")
    config["url"] = public_url(config["url"], enforce)
    if (not isinstance(config["tests"], list) or not config["tests"] or len(config["tests"]) != len(set(config["tests"]))
            or any(t not in TESTS for t in config["tests"])):
        raise ValueError("请选择有效且不重复的测试项")
    config["tests"] = [t for t in TESTS if t in config["tests"]]
    for name in ("key", "peer_key"):
        value = config[name]
        if len(value) > 512 or (value and any(ord(c) < 33 or ord(c) > 126 for c in value)):
            raise ValueError("Key必须是无空白的ASCII文本")
    if not config["key"]:
        raise ValueError("请填写渠道Key")
    if config["peer_url"]:
        if "signature_relay" not in config["tests"] or not config["peer_key"]:
            raise ValueError("对端需要选择工具续接并填写对端Key")
        config["peer_url"] = public_url(config["peer_url"], enforce)
    elif config["peer_key"]:
        raise ValueError("请填写续接对端URL")
    for name in ("auth", "peer_auth"):
        if config[name] not in ("x-api-key", "bearer"):
            raise ValueError("认证方式无效")
    if type(config["require_awsb"]) is not bool or (config["require_awsb"] and "signature_freshness" not in config["tests"]):
        raise ValueError("AWSB标记要求启用现签测试")
    for name, low, high in (("concurrency", 1, 200), ("timeout", 10, 300), ("signature_samples", 2, 5),
                            ("freshness_samples", 2, 5), ("freshness_interval", 1, 60), ("load_max_tokens", 128, 8192)):
        if type(config[name]) is not int or not low <= config[name] <= high:
            raise ValueError(name + "范围为" + str(low) + "至" + str(high))
    if not isinstance(config["model"], str) or not re.fullmatch(r"[A-Za-z0-9._:/-]{1,150}", config["model"]):
        raise ValueError("模型名格式无效")
    if not isinstance(config["freshness_models"], str):
        raise ValueError("现签模型格式无效")
    models = [m.strip() for m in config["freshness_models"].split(",")]
    if not 1 <= len(models) <= 4 or len(set(models)) != len(models) or any(not re.fullmatch(r"[A-Za-z0-9._:/-]{1,150}", m) for m in models):
        raise ValueError("现签模型需为1至4个不重复模型")
    config["freshness_models"] = ",".join(models)
    if len(config["label"]) > 80:
        raise ValueError("渠道名称最多80字")
    config["label"] = config["label"] or urlsplit(config["url"]).netloc
    if redact({k: v for k, v in config.items() if k not in ("key", "peer_key")}, [config["key"], config["peer_key"]]) != {k: v for k, v in config.items() if k not in ("key", "peer_key")}:
        raise ValueError("地址、名称及其他字段不得包含Key")
    return config


def command(config, output, dry=False):
    args = [sys.executable, "-B", "-u", str(TOOLS / "probe_token_audit.py"), "--url", config["url"], "--model", config["model"],
            "--tests", ",".join(config["tests"]), "--output", str(output), "--load-origin", "服务器2网页服务"]
    for name in ("auth", "concurrency", "timeout", "signature_samples", "freshness_models", "freshness_samples", "freshness_interval", "load_max_tokens"):
        args.extend(["--" + name.replace("_", "-"), str(config[name])])
    if config["require_awsb"]:
        args.append("--require-awsb")
    if config["peer_url"]:
        args.extend(["--peer-url", config["peer_url"], "--peer-auth", config["peer_auth"]])
    if dry:
        args.append("--dry-run")
    return args


def environment(config, public_only):
    env = {k: v for k, v in os.environ.items() if not k.startswith("TOKEN_AUDIT_")}
    env.update(TOKEN_AUDIT_API_KEY=config["key"], TOKEN_AUDIT_PEER_KEY=config["peer_key"],
               TOKEN_AUDIT_PUBLIC_ONLY="1" if public_only else "0", PYTHONIOENCODING="utf-8", PYTHONUTF8="1", PYTHONDONTWRITEBYTECODE="1")
    return env


class Jobs:
    def __init__(self, root, public_only=True):
        self.root = Path(root).resolve()
        self.root.mkdir(parents=True, exist_ok=True)
        self.public_only, self.lock = public_only, threading.RLock()
        self.jobs, self.pending, self.queue = {}, {}, queue.Queue()
        self.active, self.process = None, None
        for path in self.root.glob("*/job.json"):
            try:
                job = json.loads(path.read_text(encoding="utf-8"))
                if job["state"] in ("queued", "running", "cancelling"):
                    job.update(state="interrupted", finished_at=utc_now(), error="服务重启，任务未自动重试")
                    save_json(path, job)
                self.jobs[job["id"]] = job
            except (ValueError, KeyError, OSError):
                continue
        threading.Thread(target=self.worker, daemon=True).start()

    def plan(self, config):
        output = self.root / ("plan-" + secrets.token_hex(8)) / "result"
        result = subprocess.run(command(config, output, True), cwd=self.root,
            env=environment(config, self.public_only), capture_output=True, text=True, encoding="utf-8", timeout=15)
        if result.returncode != 0:
            raise ValueError(redact(result.stderr[-3000:], [config["key"], config["peer_key"]]))
        return json.loads(result.stdout)

    def persist(self, job):
        save_json(self.root / job["id"] / "job.json", job)

    def add(self, config):
        plan = self.plan(config)
        with self.lock:
            if sum(j["state"] in ("queued", "running", "cancelling") for j in self.jobs.values()) >= 5:
                raise ValueError("队列已满，最多5个待完成任务")
            if len(self.jobs) >= 200:
                raise ValueError("已保存200个任务，请先删除不需要的历史任务")
            if shutil.disk_usage(self.root).free < 512 * 1024 * 1024:
                raise ValueError("剩余磁盘不足512MiB，请先清理历史任务")
            job_id = secrets.token_hex(12)
            (self.root / job_id).mkdir()
            job = {"id": job_id, "created_at": utc_now(), "state": "queued", "label": config["label"],
                   "config": {k: v for k, v in config.items() if k not in ("key", "peer_key")}, "plan": plan}
            self.jobs[job_id], self.pending[job_id] = job, config
            self.persist(job)
            self.queue.put(job_id)
            return dict(job)

    def cancel(self, job_id):
        with self.lock:
            job = self.jobs[job_id]
            if job["state"] == "queued":
                self.pending.pop(job_id, None)
                job.update(state="cancelled", finished_at=utc_now())
            elif job["state"] in ("running", "cancelling"):
                job["state"] = "cancelling"
                if self.active == job_id and self.process:
                    self.process.terminate()
            self.persist(job)

    def delete(self, job_id):
        with self.lock:
            job = self.jobs[job_id]
            if job["state"] in ("queued", "running", "cancelling"):
                raise ValueError("请先取消任务")
            target = (self.root / job_id).resolve()
            if target.parent != self.root or not re.fullmatch(r"[a-f0-9]{24}", job_id):
                raise ValueError("任务路径无效")
            shutil.rmtree(target)
            del self.jobs[job_id]

    def worker(self):
        while True:
            job_id = self.queue.get()
            config = None
            try:
                with self.lock:
                    config = self.pending.pop(job_id, None)
                    if config is None:
                        continue
                    job = self.jobs[job_id]
                    job.update(state="running", started_at=utc_now())
                    self.active = job_id
                    self.persist(job)
                    folder = self.root / job_id
                    self.process = subprocess.Popen(command(config, folder / "result"), cwd=self.root,
                        env=environment(config, self.public_only), stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                        text=True, encoding="utf-8", errors="replace", bufsize=1)
                deadline = time.monotonic() + 7200
                watchdog = threading.Timer(7200, self.process.kill)
                watchdog.daemon = True
                watchdog.start()
                try:
                    with (folder / "execution.log").open("w", encoding="utf-8") as log:
                        for line in self.process.stdout:
                            if log.tell() < 2 * 1024 * 1024:
                                log.write(redact(line, [config["key"], config["peer_key"]]))
                                log.flush()
                    code = self.process.wait()
                finally:
                    watchdog.cancel()
                    self.process.stdout.close()
                with self.lock:
                    if job["state"] == "cancelling":
                        state = "cancelled"
                    elif time.monotonic() >= deadline:
                        state = "failed"
                        job["error"] = "超过2小时任务上限，已停止"
                    else:
                        state = "finished" if (folder / "result" / "summary.json").exists() else "failed"
                    job.update(state=state, exit_code=code, finished_at=utc_now())
                    self.persist(job)
            except Exception as exc:
                with self.lock:
                    if job_id in self.jobs:
                        self.jobs[job_id].update(state="failed", error=redact(str(exc), [config.get("key", ""), config.get("peer_key", "")] if config else []), finished_at=utc_now())
                        self.persist(self.jobs[job_id])
            finally:
                config = None
                with self.lock:
                    self.active, self.process = None, None
                self.queue.task_done()

    def detail(self, job_id):
        with self.lock:
            job = dict(self.jobs[job_id])
        folder = self.root / job_id
        log = folder / "execution.log"
        if log.exists():
            with log.open("rb") as handle:
                handle.seek(max(0, log.stat().st_size - 64000))
                job["log"] = handle.read().decode("utf-8", "replace")
        summary = folder / "result" / "summary.json"
        if summary.exists() and job["state"] not in ("running", "cancelling"):
            job["summary"] = json.loads(summary.read_text(encoding="utf-8"))
            # 网页列表保留诊断字段，完整提示词和签名仅通过报告下载获取。
            for row in job["summary"]["rows"]:
                row.pop("request", None)
                result = row["result"]
                for key in ("assistant_content", "text", "error_excerpt"):
                    result.pop(key, None)
        events = folder / "result" / "events.jsonl"
        completed = 0
        if events.exists():
            with events.open(encoding="utf-8") as handle:
                for line in handle:
                    try:
                        event = json.loads(line)
                    except ValueError:
                        continue
                    if event.get("kind") in ("target_result", "concurrency_result"):
                        completed += 1
        job["completed_requests"] = len(job["summary"]["rows"]) if job.get("summary") else completed
        job["files"] = [name for name in sorted(FILES) if (folder / "result" / name).is_file()]
        return job


class App(ThreadingHTTPServer):
    daemon_threads = True
    request_queue_size = 64

    def __init__(self, address, jobs, auth, origin):
        super().__init__(address, Handler)
        self.jobs, self.auth, self.origin = jobs, auth, origin.rstrip("/")
        self.sessions, self.failures = {}, {}
        self.session_lock = threading.Lock()


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, *_):
        pass

    def reply(self, status, data, content_type="application/json; charset=utf-8", headers=None):
        raw = json.dumps(data, ensure_ascii=False).encode("utf-8") if content_type.startswith("application/json") else data
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(raw)))
        self.send_header("Cache-Control", "no-store")
        self.send_header("X-Content-Type-Options", "nosniff")
        self.send_header("Referrer-Policy", "no-referrer")
        self.send_header("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
        for key, value in (headers or {}).items():
            self.send_header(key, value)
        self.end_headers()
        self.wfile.write(raw)

    def body(self):
        if self.headers.get("Content-Type", "").split(";", 1)[0] != "application/json":
            raise ValueError("请求需为application/json")
        if self.headers.get("Transfer-Encoding"):
            raise ValueError("不支持分块请求体")
        size = int(self.headers.get("Content-Length", "0"))
        if not 0 < size <= 32768:
            raise ValueError("请求体长度无效")
        self.connection.settimeout(15)
        value = json.loads(self.rfile.read(size))
        if not isinstance(value, dict):
            raise ValueError("请求体必须为对象")
        return value

    def session(self):
        cookie = SimpleCookie()
        try:
            cookie.load(self.headers.get("Cookie", ""))
        except Exception:
            return None
        token = cookie.get("audit_session")
        token = token.value if token else ""
        with self.server.session_lock:
            self.server.sessions = {k: v for k, v in self.server.sessions.items() if v > time.time()}
            return token if self.server.sessions.get(token, 0) > time.time() else None

    def dispatch(self, method):
        path = urlsplit(self.path).path
        if self.headers.get("Host") != urlsplit(self.server.origin).netloc:
            return self.reply(400, {"error": "Host无效"})
        if method == "POST" and self.headers.get("Origin") != self.server.origin:
            return self.reply(403, {"error": "请求来源无效"})
        if method == "GET" and path == "/health":
            return self.reply(200, {"status": "ok"})
        if method == "GET" and (path == "/" or path.startswith("/static/")):
            relative = "index.html" if path == "/" else path.removeprefix("/static/")
            target = (STATIC / relative).resolve()
            if not target.is_relative_to(STATIC.resolve()) or not target.is_file() or target.is_symlink():
                return self.reply(404, {"error": "文件不存在"})
            mime = mimetypes.guess_type(str(target))[0] or "application/octet-stream"
            return self.reply(200, target.read_bytes(), mime + ("; charset=utf-8" if mime.startswith("text/") or mime == "application/javascript" else ""))
        if method == "POST" and path == "/api/login":
            data = self.body()
            password = data.get("password", "")
            if not isinstance(password, str) or len(password) > 256:
                raise ValueError("口令格式无效")
            ip = self.headers.get("X-Real-IP", self.client_address[0])
            now = time.time()
            with self.server.session_lock:
                self.server.failures = {k: v for k, v in self.server.failures.items() if v[-1] > now - 600}
                attempts = [v for v in self.server.failures.get(ip, []) if v > now - 600]
                if len(attempts) >= 8 or sum(len(v) for v in self.server.failures.values()) >= 100:
                    return self.reply(429, {"error": "尝试次数过多，请10分钟后重试"})
                auth = self.server.auth
                digest = hashlib.pbkdf2_hmac("sha256", password.encode(), bytes.fromhex(auth["salt"]), auth["iterations"]).hex()
                if not hmac.compare_digest(digest, auth["hash"]):
                    self.server.failures[ip] = attempts + [now]
                    return self.reply(401, {"error": "访问口令错误"})
                token = secrets.token_urlsafe(32)
                self.server.sessions[token] = now + 43200
            suffix = "; Secure" if self.server.origin.startswith("https:") else ""
            return self.reply(200, {"ok": True}, headers={"Set-Cookie": "audit_session=" + token + "; HttpOnly; SameSite=Strict; Path=/; Max-Age=43200" + suffix})
        token = self.session()
        if not token:
            return self.reply(401, {"error": "请先登录"})
        if method == "GET" and path == "/api/me":
            return self.reply(200, {"defaults": DEFAULTS, "tests": TESTS, "origin": "服务器2"})
        if method == "POST" and path == "/api/logout":
            self.body()
            with self.server.session_lock:
                self.server.sessions.pop(token, None)
            return self.reply(200, {"ok": True}, headers={"Set-Cookie": "audit_session=; HttpOnly; SameSite=Strict; Path=/; Max-Age=0"})
        if path == "/api/jobs" and method == "GET":
            with self.server.jobs.lock:
                jobs = [dict(j) for j in self.server.jobs.jobs.values()]
            return self.reply(200, {"jobs": sorted(jobs, key=lambda j: j["created_at"], reverse=True)})
        if path in ("/api/jobs", "/api/plan", "/api/models") and method == "POST":
            config = normalize(self.body(), self.server.jobs.public_only)
            if path == "/api/models":
                result = catalog(MessagesClient(config["url"], config["key"], 20, config["auth"]))
                return self.reply(200, redact(result, [config["key"], config["peer_key"]]))
            if path == "/api/plan":
                return self.reply(200, self.server.jobs.plan(config))
            return self.reply(201, self.server.jobs.add(config))
        match = re.fullmatch(r"/api/jobs/([a-f0-9]{24})(?:/(cancel|delete|files/([^/]+)))?", path)
        if match:
            job_id, action, filename = match.groups()
            if job_id not in self.server.jobs.jobs:
                return self.reply(404, {"error": "任务不存在"})
            if method == "GET" and not action:
                return self.reply(200, self.server.jobs.detail(job_id))
            if method == "POST" and action in ("cancel", "delete"):
                self.body()
                getattr(self.server.jobs, action)(job_id)
                return self.reply(200, {"ok": True})
            if method == "GET" and filename in FILES:
                target = self.server.jobs.root / job_id / "result" / filename
                if target.is_file() and self.server.jobs.jobs[job_id]["state"] not in ("running", "cancelling"):
                    return self.reply(200, target.read_bytes(), "application/octet-stream", {"Content-Disposition": 'attachment; filename="' + filename + '"'})
        return self.reply(404, {"error": "接口不存在"})

    def handle_method(self, method):
        try:
            self.dispatch(method)
        except (ValueError, TypeError, KeyError, OSError, subprocess.SubprocessError) as exc:
            self.close_connection = True
            try:
                self.reply(400, {"error": "请求未完成：" + str(exc)[:300]})
            except (BrokenPipeError, ConnectionResetError):
                pass

    def do_GET(self):
        self.handle_method("GET")

    def do_POST(self):
        self.handle_method("POST")


def main():
    parser = argparse.ArgumentParser(description="渠道检测网页服务")
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=18766)
    parser.add_argument("--data-root", type=Path, required=True)
    parser.add_argument("--auth-file", type=Path, required=True)
    parser.add_argument("--origin", required=True)
    args = parser.parse_args()
    os.environ["TOKEN_AUDIT_PUBLIC_ONLY"] = "1"
    jobs = Jobs(args.data_root)
    auth = json.loads(args.auth_file.read_text(encoding="utf-8"))
    app = App((args.host, args.port), jobs, auth, args.origin)
    def stop(*_):
        with jobs.lock:
            if jobs.process:
                jobs.process.terminate()
        raise SystemExit(0)
    signal.signal(signal.SIGTERM, stop)
    try:
        app.serve_forever()
    finally:
        app.server_close()


if __name__ == "__main__":
    main()
