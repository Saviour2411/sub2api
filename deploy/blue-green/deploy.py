#!/usr/bin/env python3
"""双槽发布执行器。支持首次初始化；旧版无退役凭据时保留，不强停。"""
import argparse
import copy
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import time
import threading

SLOTS = {"blue": 18080, "green": 18082}
LEGACY_SHA = "cfd9afa871def9ebd457bb95fb4c2e8b4b02b34f"
NGINX_ROOT = Path("/etc/nginx")
VHOSTS = {"api": ("api.saviour.cc.cd", 2503), "direct": ("direct.saviour.cc.cd", 443)}


def probe_tls(host):
    # 回环访问绕过 CDN，显式信任 Nginx 已配置的 Origin 证书；仍校验域名、有效期及证书。
    origin = ["--cacert", "/root/cert/saviour.cc.cd/saviour.cc.cd.pem"] if host == VHOSTS["api"][0] else []
    return ["--noproxy", "*", *origin]


class Refused(RuntimeError):
    pass


def require(condition, message):
    if not condition:
        raise Refused(message)


def atomic_write(path, content):
    path = Path(path)
    temporary = path.with_name(path.name + ".next")
    with temporary.open("w", encoding="utf-8") as stream:
        os.chmod(temporary, 0o600)
        stream.write(content)
        stream.flush()
        os.fsync(stream.fileno())
    os.replace(temporary, path)
    fd = os.open(path.parent, os.O_RDONLY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def bytes_value(value):
    match = re.fullmatch(r"([0-9]+)(KiB|MiB|GiB|B)?", str(value))
    require(match is not None, "GOMEMLIMIT 必须是显式字节数，不接受 off 或比例预算")
    return int(match[1]) * {None: 1, "B": 1, "KiB": 1024, "MiB": 1024**2, "GiB": 1024**3}[match[2]]


def require_matching_runtime(slot, expected, actual_entries):
    actual = dict(item.split("=", 1) for item in actual_entries if "=" in item)
    for name in ("DATABASE_MAX_OPEN_CONNS", "GOMEMLIMIT", "JWT_SECRET", "TOTP_ENCRYPTION_KEY"):
        require(actual.get(name) == expected.get(name), f"{slot} 的实际运行参数 {name} 与批准清单不一致；不按新清单推算旧实例资源")


def same_release(left, right):
    # 重跑时 CI run/attempt 可能变化；恢复身份只由不可变镜像、提交及迁移基线决定。
    return all(left.get(key) == right.get(key) for key in
               ("sha","image","compatible_from","migration_class","rollback_compatible"))


def resource_check(sample, config):
    free = sample["maximum"] - sample["reserved"] - sample["used"]
    db_min = int(config.get("database_free_min",32))
    mem_min = int(config.get("memory_free_min_bytes",1024**3))
    require(db_min > 0 and mem_min > 0, "实际余量阈值必须为正，不关闭资源保护")
    require(free >= db_min, f"数据库实际可用连接 {free} 低于发布余量 {db_min}，保留当前流量")
    require(sample["memory_available"] >= mem_min, "实际可用内存不足，保留当前流量")


class Deployment:
    def __init__(self, directory):
        self.directory = Path(directory).resolve()
        self.root = self.directory / "blue-green"
        self.state_path = self.root / "state.json"
        self.config = None
        if self.state_path.is_file():
            self.config = json.loads((self.root / "config.json").read_text())
            self.state = json.loads(self.state_path.read_text())
        else:
            self.state = self.legacy_baseline()
        require(self.state.get("schema") == 1 and self.state.get("active") in SLOTS, "部署状态格式无效")

    def machine_id(self):
        return Path("/etc/machine-id").read_text().strip()

    def container_name(self, slot):
        return "sub2api" if self.state["slots"].get(slot, {}).get("legacy") else "sub2api-"+slot

    def legacy_baseline(self):
        require(str(self.directory) == "/root/proj/sub2api/deploy", "首次初始化只允许固定生产目录")
        require(not self.inspect("sub2api-blue") and not self.inspect("sub2api-green"), "状态缺失但槽位容器已存在，拒绝猜测活动实例")
        old = self.inspect("sub2api")
        require(old and old["State"]["Running"], "找不到运行中的旧单实例")
        require(old["Config"].get("Labels", {}).get("org.opencontainers.image.revision") == LEGACY_SHA, "未知旧版不能自动套用 v0.1.234 迁移路径")
        require(old["NetworkSettings"]["Ports"].get("8080/tcp") == [{"HostIp":"127.0.0.1", "HostPort":"18080"}], "旧应用端口不匹配")
        self.mount("sub2api", self.directory/"data", "/app/data")
        return {"schema":1, "generation":0, "active":"blue", "phase":"stable", "slots":{"blue":{
            "legacy":True, "sha":LEGACY_SHA, "version":"0.1.234", "image":old["Image"],
            "container_id":old["Id"], "instance_id":"legacy-v0.1.234"}}}

    def initialize(self, release):
        # 只在固定 SHA 门禁和精确无迁移变化证据已完成后写入首次部署状态。
        require(release.get("compatible_from") == LEGACY_SHA and release.get("migration_class") == "none", "首次迁移必须无数据库变化")
        require(release.get("ci_sha") == release.get("sha") and release.get("ci") == "success" and release.get("security") == "success", "首次迁移门禁未通过")
        require(re.fullmatch(r"[a-f0-9]{40}", release.get("sha", "")) and re.fullmatch(r"[A-Za-z0-9._/:-]+@sha256:[a-f0-9]{64}", release.get("image", "")), "首次迁移缺少固定版本")
        source = self.directory/"docker-compose.yml"
        require(source.read_bytes() == (self.directory/"docker-compose.sub2api.yml").read_bytes(), "活动 Compose 不一致")
        full = self.run("nginx", "-T").stdout
        require("worker_shutdown_timeout" not in full, "Nginx 不得强制终止旧 worker")
        require("/etc/nginx/conf.d/*.conf" in full, "Nginx 未加载 conf.d")
        for host,port in VHOSTS.values():
            self.run("curl", "--fail", "--silent", "--max-time", "5", "--resolve", f"{host}:{port}:127.0.0.1", *probe_tls(host), f"https://{host}:{port}/health")
        journal = self.root/"bootstrap.json"
        if journal.exists():
            saved = json.loads(journal.read_text())
            require(saved["state"] == self.state, "初始化期间旧容器身份发生变化")
            self.config = saved["config"]
        else:
            vhosts = {kind:str(((NGINX_ROOT/"sites-enabled")/("sub2api-"+host)).resolve()) for kind,(host,_) in VHOSTS.items()}
            originals = {}
            for kind, path in vhosts.items():
                text = Path(path).read_text()
                host, port = VHOSTS[kind]
                require(Path(path).is_relative_to(NGINX_ROOT) and "server_name "+host+";" in text and re.search(r"listen\s+"+str(port)+r"\s+ssl", text), "代理入口不匹配")
                require(text.count("proxy_pass http://127.0.0.1:18080;") == 1 and "proxy_pass http://sub2api_active;" not in text, "不能自动改写未知代理配置")
                originals[path] = text
            self.config = {"machine_id":self.machine_id(), "compose_sha256":hashlib.sha256(source.read_bytes()).hexdigest(),
                "vhosts":vhosts, "original_vhosts":originals, "upstream_file":str(NGINX_ROOT/"conf.d/sub2api-blue-green.conf"),
                "probes":[{"url":f"https://{host}:{port}/readyz", "resolve":f"{host}:{port}:127.0.0.1"} for host,port in VHOSTS.values()],
                "database_free_min":32, "memory_free_min_bytes":1024**3}
            require(not Path(self.config["upstream_file"]).exists(), "蓝绿代理文件已存在但状态缺失，拒绝覆盖")
            atomic_write(journal, json.dumps({"state":self.state,"config":self.config}, ensure_ascii=False, indent=2))
        require(self.config["compose_sha256"] == hashlib.sha256(source.read_bytes()).hexdigest(), "初始化期间 Compose 已变更")
        (self.root/"run").mkdir(exist_ok=True, mode=0o700)
        os.chown(self.root/"run",1000,1000)
        os.chmod(self.root/"run",0o700)
        # 先将两入口转换成共用 upstream，但仍指向旧应用；reload 不终止现存连接。
        for path, original in self.config["original_vhosts"].items():
            transformed = self.transform_vhost(original)
            require(Path(path).read_text() in (original, transformed), "初始化期间代理配置被其他操作修改")
            atomic_write(path, transformed)
        atomic_write(self.config["upstream_file"], self.proxy_config("blue"))
        try:
            self.run("nginx", "-t")
            self.run("nginx", "-s", "reload")
        except Exception:
            for path,original in self.config["original_vhosts"].items():
                atomic_write(path, original)
            Path(self.config["upstream_file"]).unlink(missing_ok=True)
            self.run("nginx", "-t")
            self.run("nginx", "-s", "reload")
            raise
        atomic_write(self.root/"config.json", json.dumps(self.config, ensure_ascii=False, indent=2))
        self.save(bootstrap_completed_at=time.time())

    def transform_vhost(self, original):
        return original.replace("proxy_pass http://127.0.0.1:18080;", "proxy_pass http://sub2api_active;\n        add_header X-Sub2api-Slot $sub2api_slot always;")

    def proxy_config(self, slot):
        text = "# API/direct 共用，由蓝绿发布器管理\nupstream sub2api_active { server 127.0.0.1:%d; }\n" % SLOTS[slot]
        text += "map $upstream_addr $sub2api_slot { default unknown; 127.0.0.1:18080 blue; 127.0.0.1:18082 green; }\n"
        if any(s.get("legacy") for s in self.state["slots"].values()):
            # 固定 Unix 私有通道，仅应用 uid 可穿过 0700 父目录；旧实例仍自行鉴权和计费。
            text += ("server { listen unix:%s; server_name legacy; client_max_body_size 256m; client_header_buffer_size 16k; large_client_header_buffers 8 32k; location / {\n"
                     "proxy_pass http://127.0.0.1:18080; proxy_http_version 1.1;\n"
                     "proxy_set_header Host $http_host; proxy_set_header X-Real-IP $http_x_real_ip;\n"
                     "proxy_set_header X-Forwarded-For $http_x_forwarded_for; proxy_set_header X-Forwarded-Proto https;\n"
                     "proxy_set_header Upgrade $http_upgrade; proxy_set_header Connection $connection_upgrade;\n"
                     "proxy_request_buffering off; proxy_buffering off; proxy_next_upstream off;\n"
                     "proxy_read_timeout 3600s; proxy_send_timeout 3600s; } }\n") % (self.root/"run/legacy.sock")
        return text

    def resources(self):
        sql = "SELECT json_build_object('maximum',current_setting('max_connections')::int,'reserved',current_setting('superuser_reserved_connections')::int+coalesce(current_setting('reserved_connections',true),'0')::int,'used',(SELECT count(*) FROM pg_stat_activity WHERE backend_type='client backend'))"
        result = self.run("docker", "exec", "sub2api-postgres", "sh", "-c", 'exec psql -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-sub2api}" -Atc "$1"', "psql", sql)
        connections = json.loads(result.stdout)
        meminfo = {line.split(':')[0]:int(line.split()[1])*1024 for line in Path("/proc/meminfo").read_text().splitlines()}
        sample = {**connections, "memory_available":meminfo["MemAvailable"], "memory_total":meminfo["MemTotal"]}
        resource_check(sample, self.config)
        self.save(resources=sample)
        return sample

    def run(self, *args, check=True):
        result = subprocess.run(args, cwd=self.directory, text=True, capture_output=True, check=False)
        if check and result.returncode:
            # 命令可能包含环境密钥，不将完整 argv/stdout 写入日志。
            raise Refused(f"{args[0]} 操作失败，退出码 {result.returncode}: {result.stderr[-1200:]}")
        return result

    def save(self, **changes):
        self.state.update(changes)
        self.state["updated_at"] = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
        atomic_write(self.state_path, json.dumps(self.state, ensure_ascii=False, indent=2) + "\n")

    def control(self, slot, action="state"):
        info = self.state["slots"].get(slot, {})
        if info.get("legacy"):
            require(action in ("state", "check"), "旧版没有管理操作，禁止假造排空或退役")
            live = self.inspect("sub2api")
            require(live and live["Id"] == info["container_id"] and live["State"]["Running"], "旧版容器身份/状态变化")
            self.run("curl", "--fail", "--silent", "--max-time", "5", "http://127.0.0.1:18080/health")
            return {"instance_id":info["instance_id"], "state":"active", "legacy":True, "sealed":False}
        command = ["curl", "--fail", "--silent", "--show-error", "--max-time", "5", "--unix-socket", str(self.root / "run" / (slot + ".sock"))]
        if action in ("activate", "drain", "retire"):
            command += ["-X", "POST"]
        output = self.run(*command, "http://localhost/" + action).stdout
        return json.loads(output) if action == "state" else output

    def inspect(self, container):
        result = self.run("docker", "inspect", container, check=False)
        return json.loads(result.stdout)[0] if result.returncode == 0 else None

    def mount(self, container, source, destination):
        data = self.inspect(container)
        require(data is not None, f"缺少现有容器 {container}")
        actual = [m for m in data["Mounts"] if m["Destination"] == destination]
        require(len(actual) == 1 and actual[0]["Type"] == "bind" and actual[0]["Source"] == str(source), f"{container} 数据挂载不匹配")

    def preflight(self, release, needs_capacity=True):
        require(re.fullmatch(r"[a-f0-9]{40}", release["sha"]), "发布 SHA 无效")
        require(re.fullmatch(r"[a-zA-Z0-9._/:-]+@sha256:[a-f0-9]{64}", release["image"]), "必须使用固定镜像摘要")
        require(release.get("ci_sha") == release["sha"] and release.get("ci") == "success" and release.get("security") == "success", "固定提交的 CI/Security Scan 未通过")
        active = self.state["slots"][self.state["active"]]
        require(release.get("compatible_from") == active["sha"], "未提供针对当前活动提交的迁移兼容审查")
        require(release.get("migration_class") in ("none", "expand"), "破坏性或未知迁移禁止普通蓝绿发布")
        require(self.machine_id() == self.config["machine_id"], "目标机器指纹不匹配")
        source = self.directory / "docker-compose.yml"
        require(source.read_bytes() == (self.directory / "docker-compose.sub2api.yml").read_bytes(), "活动 Compose 与二开清单不一致")
        require(hashlib.sha256(source.read_bytes()).hexdigest() == self.config["compose_sha256"], "Compose 变更未经首次迁移配置审查")
        self.mount("sub2api-postgres", self.directory / "postgres_data", "/var/lib/postgresql/data")
        self.mount("sub2api-redis", self.directory / "redis_data", "/data")
        for slot in self.state["slots"]:
            self.mount(self.container_name(slot), self.directory / "data", "/app/data")
        # 两个入口必须引用同一个 upstream，切流只有一个原子替换点。
        require(set(self.config["vhosts"]) == {"api", "direct"}, "必须核验 API 与 direct 两个入口")
        for path in self.config["vhosts"].values():
            contents = Path(path).read_text()
            require("proxy_pass http://sub2api_active;" in contents, "入口尚未迁移到统一 upstream")
        full_nginx = self.run("nginx", "-T").stdout
        require("worker_shutdown_timeout" not in full_nginx, "不能配置强制终止旧 Nginx worker 的超时")
        compose = json.loads(self.run("docker", "compose", "-f", str(source), "config", "--format", "json").stdout)
        service = compose["services"]["sub2api"]
        env = service["environment"]
        require(len(env.get("JWT_SECRET", "")) >= 32 and re.fullmatch(r"[a-fA-F0-9]{64}",env.get("TOTP_ENCRYPTION_KEY", "")), "蓝绿必须显式配置共享 JWT/TOTP 密钥；不自动改写生产参数")
        for slot in self.state["slots"]:
            running = self.inspect(self.container_name(slot))
            require_matching_runtime(slot, env, running["Config"]["Env"])
        # 上限不等于预留：保持现有服务上限，仅对实际占用和发布余量做硬检查。
        sample = self.resources()
        if 2*int(env["DATABASE_MAX_OPEN_CONNS"]) > sample["maximum"] or 2*bytes_value(env["GOMEMLIMIT"]) > sample["memory_total"]:
            print("::warning::双实例配置上限超配；按实际资源余量发布，不降低正常服务上限", file=sys.stderr)
        return compose

    def render(self, slot, release, compose):
        spec = copy.deepcopy(compose["services"]["sub2api"])
        spec.pop("depends_on", None)
        spec["image"] = release["image"]
        spec["container_name"] = "sub2api-" + slot
        spec["restart"] = "on-failure"
        spec["ports"] = [{"target": 8080, "published": str(SLOTS[slot]), "host_ip": "127.0.0.1", "protocol": "tcp"}]
        spec["environment"].update({"LIFECYCLE_MODE": "standby", "LIFECYCLE_SOCKET": "/run/sub2api/"+slot+".sock", "LOG_OUTPUT_FILE_PATH": "/app/instance-logs/sub2api.log", "LIFECYCLE_LEGACY": str(any(s.get("legacy") for s in self.state["slots"].values())).lower()})
        for name in ("run", "logs/"+slot):
            path = self.root/name
            path.mkdir(parents=True, exist_ok=True, mode=0o700)
            # 项目 Dockerfile 固定使用 uid/gid 1000；只管理新槽的私有目录。
            os.chown(path,1000,1000)
            os.chmod(path,0o700)
        spec.setdefault("volumes", []).extend([
            {"type": "bind", "source": str(self.root/"run"), "target": "/run/sub2api"},
            {"type": "bind", "source": str(self.root/"logs"/slot), "target": "/app/instance-logs"},
        ])
        networks = {key: {"external": True, "name": value["name"]} for key,value in compose["networks"].items()}
        path = self.root / (slot + ".json")
        atomic_write(path, json.dumps({"services": {slot: spec}, "networks": networks}, indent=2))
        return path

    def probe(self, expected):
        for probe in self.config["probes"]:
            result = self.run("curl", "--fail", "--silent", "--show-error", "--max-time", "5", "--http1.1", "-H", "Connection: close", "--resolve", probe["resolve"], *probe_tls(probe["resolve"].split(":",1)[0]), "-D", "-", (probe["url"].replace("/readyz", "/health") if expected == "legacy-v0.1.234" else probe["url"]), check=False)
            header = "x-sub2api-slot: blue" if expected == "legacy-v0.1.234" else "x-sub2api-instance: " + expected
            if result.returncode != 0 or header.lower() not in result.stdout.lower():
                return False
        return len(self.config["probes"]) == 2

    def candidate_probe(self, slot, expected):
        # Unix 管理接口健康不代表 Docker 端口映射可达；切流前验证实际接入端口。
        result = self.run("curl", "--fail", "--silent", "--show-error", "--max-time", "3", "-D", "-", "http://127.0.0.1:%d/readyz" % SLOTS[slot], check=False)
        return result.returncode == 0 and ("x-sub2api-instance: "+expected).lower() in result.stdout.lower()

    def nginx_workers(self):
        # PID 与启动 tick 一同记录，避免重跑时误把 PID 复用当作旧 worker。
        result = {}
        for entry in Path("/proc").iterdir():
            if entry.name.isdigit():
                try:
                    if b"nginx: worker process" in (entry / "cmdline").read_bytes():
                        result[entry.name] = (entry / "stat").read_text().rsplit(")", 1)[1].split()[19]
                except (FileNotFoundError, ProcessLookupError):
                    pass
        return result

    def old_workers_present(self):
        current = self.nginx_workers()
        return any(current.get(pid) == tick for pid,tick in self.state.get("old_workers",{}).items())

    def switch(self, slot):
        instance = self.control(slot)
        expected = self.state["slots"][slot]["instance_id"]
        require(instance["instance_id"] == expected and instance["state"] in ("standby", "active"), "候选实例身份/状态发生变化")
        self.resources()
        self.control(slot, "activate")
        require(self.candidate_probe(slot, expected), "候选实际接入端口未就绪，保持原有流量")
        proxy = Path(self.config["upstream_file"])
        if self.state["phase"] != "switching":
            self.save(phase="switching", previous_proxy=proxy.read_text(), old_workers=self.nginx_workers())
        switch_started = time.monotonic()
        if not self.probe(expected):
            atomic_write(proxy, self.proxy_config(slot))
            try:
                workers = dict(self.state.get("old_workers",{})); workers.update(self.nginx_workers())
                self.save(old_workers=workers)
                self.run("nginx", "-t")
                self.run("nginx", "-s", "reload")
                deadline = time.monotonic()+10
                while not self.probe(expected):
                    require(time.monotonic() < deadline, "切流尚未确认；保留两实例，禁止排空或销毁")
                    time.sleep(0.1)
            except Exception:
                # 不盲目销毁候选；配置回退失败同样保留状态供重跑处理。
                previous = self.control(self.state["active"])
                require(previous["state"] == "active", "旧实例不再可激活；保留两实例，不盲目回切")
                self.control(self.state["active"],"check")
                atomic_write(proxy, self.state["previous_proxy"])
                self.run("nginx", "-t")
                self.run("nginx", "-s", "reload")
                raise
        old = self.state["active"]
        self.save(active=slot, pending=old, phase="switched", switched_at=time.time(), switch_seconds=time.monotonic()-switch_started, verified_instance=expected)

    def rollback(self, window=900):
        # 只能回到尚未封闭接入的保留实例；不是重新创建旧镜像。
        if self.state.get("rollback"):
            operation = self.state["rollback"]
            if self.state["phase"] != "rolling_back":
                self.drain(window)
                return
        else:
            target = self.state.get("pending")
            require(target and self.state["phase"] in ("switched", "draining", "drain_pending"), "没有可安全回滚的保留实例")
            release = self.state.get("release", {})
            require(release.get("rollback_compatible") is True and release.get("compatible_from") == self.state["slots"][target]["sha"], "缺少精确版本的数据库回滚兼容审查")
            operation = {"from": self.state["active"], "to": target}
            self.save(rollback=operation, phase="rolling_back", old_workers=self.nginx_workers())
        target = operation["to"]
        previous = operation["from"]
        expected = self.state["slots"][target]["instance_id"]
        instance = self.control(target)
        require(instance["instance_id"] == expected and not instance.get("sealed") and instance["state"] in ("active", "draining", "drained"), "旧实例不可恢复；保留当前流量")
        container = self.inspect(self.container_name(target))
        require(container and container["Id"] == self.state["slots"][target]["container_id"], "回滚容器身份不匹配")
        self.control(target, "check")
        if not self.state["slots"][target].get("legacy"):
            self.control(target, "activate")
            require(self.candidate_probe(target, expected), "旧实例实际接入端口不可用，保持当前流量")
        if not self.probe(expected):
            proxy = Path(self.config["upstream_file"])
            atomic_write(proxy, self.proxy_config(target))
            workers = dict(self.state.get("old_workers", {})); workers.update(self.nginx_workers())
            self.save(old_workers=workers)
            self.run("nginx", "-t")
            self.run("nginx", "-s", "reload")
            deadline = time.monotonic()+10
            while not self.probe(expected):
                require(time.monotonic() < deadline, "回滚切流未确认，保留两个实例")
                time.sleep(0.1)
        self.save(active=target, pending=previous, phase="draining", rollback_at=time.time(), verified_instance=expected)
        self.drain(window)

    def finish_retirement(self, old, expected):
        receipt_path = self.root/"run"/(old+".sock.retired")
        require(receipt_path.is_file(), "退役凭据缺失；不推断旧实例已排空")
        receipt = json.loads(receipt_path.read_text())
        require(receipt.get("instance_id") == expected and receipt.get("sealed") is True, "退役凭据与旧实例不匹配")
        container = self.inspect("sub2api-"+old)
        if container is not None:
            require(container["Id"] == self.state["slots"][old]["container_id"], "容器身份改变，拒绝停止")
            # 正常退役退出码为零，on-failure 不会重启它。
            self.run("docker", "stop", "--timeout", "-1", "sub2api-"+old)
            self.run("docker", "rm", "sub2api-"+old)
        slots = dict(self.state["slots"]); del slots[old]
        self.save(slots=slots, pending=None, phase="stable", drained_at=time.time(), last_error=None)

    def drain(self, window):
        old = self.state.get("pending")
        if not old:
            return True
        if self.state["slots"][old].get("legacy"):
            self.control(old, "check")
            self.save(phase="drain_pending", pending_reason="legacy_unverifiable", legacy_retained=True)
            print("::warning::首次切流完成，旧版仍保留供既有连接/会话续接；不能证明排空，不自动停容器或复用槽位", file=sys.stderr)
            return False
        expected = self.state["slots"][old]["instance_id"]
        if self.state["phase"] == "retiring":
            self.finish_retirement(old,expected)
            return True
        deadline = time.monotonic()+window
        while True:
            state = self.control(old)
            require(state["instance_id"] == expected, "旧实例身份改变，拒绝退役")
            if state["state"] == "active":
                self.control(old, "drain")
                self.save(phase="draining")
                continue
            elif state["state"] == "drained" and not self.old_workers_present():
                # retire 在应用内部原子封闭新工作；docker stop 只允许在此之后。
                self.save(phase="retiring")
                try:
                    self.control(old,"retire")
                except Refused:
                    # 即使响应丢失，也必须先核验应用落盘的封闭接入凭据。
                    pass
                self.finish_retirement(old,expected)
                return True
            if time.monotonic() >= deadline:
                self.save(phase="drain_pending")
                print("::warning::旧实例/旧代理 worker 尚未排空，保留容器；禁止复用槽位", file=sys.stderr)
                return False
            time.sleep(1)

    def deploy(self, release, window=900):
        if self.config is None:
            self.initialize(release)
        require(not self.state.get("rollback") or self.state["phase"] == "stable", "回滚未完成，先用 --rollback 恢复，不改变当前流量")
        # 未排空槽优先处理；即使本次发布不同镜像，也不允许覆盖正在服务的旧实例。
        if self.state.get("pending"):
            same = same_release(self.state.get("release", {}), release)
            if not self.drain(window if same else 0):
                require(same, "旧槽仍有工作，本次部署未改变当前流量")
                return
        if self.state["slots"][self.state["active"]]["sha"] == release["sha"] and self.state["phase"] == "stable":
            require(self.state["slots"][self.state["active"]]["image"] == release["image"], "同一提交的镜像摘要发生变化")
            return
        active = self.state["active"]
        slot = "green" if active == "blue" else "blue"
        current_release = self.state.get("release", {})
        require(self.state["phase"] == "stable" or same_release(current_release, release), "另一个发布未完成，必须先恢复同一发布")
        existing = self.inspect("sub2api-"+slot)
        compose = self.preflight(release, needs_capacity=existing is None)
        if self.state["phase"] == "stable":
            require(existing is None and slot not in self.state["slots"], "候选槽被占用")
            self.save(phase="preparing", generation=self.state.get("generation",0)+1, release=release, rollback=None)
        if existing is None:
            path = self.render(slot, release, compose)
            self.run("docker", "pull", release["image"])
            image = json.loads(self.run("docker", "image", "inspect", release["image"]).stdout)[0]
            require(image["Config"].get("Labels",{}).get("org.opencontainers.image.revision") == release["sha"], "镜像 revision 与已验收提交不一致")
            self.run("docker", "compose", "-p", "sub2api-bg-"+slot, "-f", str(path), "up", "-d", "--no-build", "--no-deps", slot)
        else:
            require(existing["Config"]["Image"] == release["image"], "槽位镜像与发布不一致；拒绝重建")
        deadline = time.monotonic()+180
        while True:
            try:
                candidate = self.control(slot)
                if candidate["state"] in ("standby", "active"):
                    self.control(slot,"check")
                    synthetic = self.control(slot,"synthetic")
                    require("event: complete\ndata: ok" in synthetic, "合成流未完整结束")
                    break
            except Refused:
                if time.monotonic() >= deadline:
                    raise
            require(time.monotonic() < deadline, "候选未就绪，未切流")
            time.sleep(1)
        slots = dict(self.state["slots"])
        known = slots.get(slot)
        require(not known or known["instance_id"] == candidate["instance_id"], "候选意外重启，停止自动恢复")
        slots[slot] = dict(container_id=self.inspect("sub2api-"+slot)["Id"], instance_id=candidate["instance_id"], image=release["image"], sha=release["sha"], version=candidate["version"])
        self.save(slots=slots)
        self.switch(slot)
        self.drain(window)


class HealthObserver:
    """无付费调用的双入口连续探针；错误如实记录，不重放客户请求。"""
    def __init__(self):
        self.stop = threading.Event()
        self.results = {name:{"requests":0,"errors":[]} for name in VHOSTS}
        self.thread = threading.Thread(target=self.observe, daemon=True)

    def observe(self):
        while not self.stop.is_set():
            for name,(host,port) in VHOSTS.items():
                result = subprocess.run(["curl","--silent","--show-error","--max-time","3","--http2","--resolve",f"{host}:{port}:127.0.0.1",*probe_tls(host),"-o","/dev/null","-w","%{http_code}",f"https://{host}:{port}/health"],capture_output=True,text=True)
                sample = self.results[name]
                sample["requests"] += 1
                if result.returncode or result.stdout != "200":
                    sample["errors"].append({"at":time.time(),"curl_exit":result.returncode,"status":result.stdout})
            self.stop.wait(0.2)

    def finish(self):
        self.stop.set()
        self.thread.join()
        return self.results


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--directory", required=True)
    action = parser.add_mutually_exclusive_group(required=True)
    action.add_argument("--release", help="固定 SHA、digest、CI 及迁移兼容性验收记录")
    action.add_argument("--snapshot", action="store_true", help="只读已有状态或旧单实例基线")
    action.add_argument("--rollback", action="store_true", help="回切尚未退役的保留实例并排空新实例")
    parser.add_argument("--window", type=int, default=900, help="排空观察秒数；超时保留旧实例")
    args = parser.parse_args()
    root = Path(args.directory).resolve()/"blue-green"
    if args.snapshot:
        print(json.dumps(Deployment(args.directory).state, ensure_ascii=False))
        return
    require(str(Path(args.directory).resolve()) == "/root/proj/sub2api/deploy", "发布目标目录不匹配")
    root.mkdir(exist_ok=True, mode=0o700)
    with (root/"deploy.lock").open("a") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as error:
            raise Refused("已有服务器发布正在执行，不自动取消它") from error
        deployment = Deployment(args.directory)
        observer = HealthObserver()
        observer.thread.start()
        try:
            require(args.window >= 0, "观察窗口不能为负数")
            if args.rollback:
                deployment.rollback(args.window)
            else:
                deployment.deploy(json.loads(Path(args.release).read_text()), args.window)
        except Exception as error:
            if (root/"config.json").is_file():
                deployment.save(last_error=str(error))
            raise
        finally:
            atomic_write(root/"observation.json", json.dumps(observer.finish(), ensure_ascii=False, indent=2))


if __name__ == "__main__":
    try:
        main()
    except (Refused, OSError, ValueError, KeyError) as error:
        print("发布安全停止："+str(error), file=sys.stderr)
        sys.exit(1)
