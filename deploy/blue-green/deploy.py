#!/usr/bin/env python3
"""双槽发布执行器。只接管已经另行批准迁移的环境；不自动接管旧单实例。"""
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

SLOTS = {"blue": 18080, "green": 18082}


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


class Deployment:
    def __init__(self, directory):
        self.directory = Path(directory).resolve()
        self.root = self.directory / "blue-green"
        self.state_path = self.root / "state.json"
        require(self.state_path.is_file(), "尚未完成首次迁移审批；拒绝接管或停止 v0.1.234 单实例")
        self.config = json.loads((self.root / "config.json").read_text())
        self.state = json.loads(self.state_path.read_text())
        require(self.state.get("schema") == 1 and self.state.get("active") in SLOTS, "部署状态格式无效")

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
        require(Path("/etc/machine-id").read_text().strip() == self.config["machine_id"], "目标机器指纹不匹配")
        source = self.directory / "docker-compose.yml"
        require(source.read_bytes() == (self.directory / "docker-compose.sub2api.yml").read_bytes(), "活动 Compose 与二开清单不一致")
        require(hashlib.sha256(source.read_bytes()).hexdigest() == self.config["compose_sha256"], "Compose 变更未经首次迁移配置审查")
        self.mount("sub2api-postgres", self.directory / "postgres_data", "/var/lib/postgresql/data")
        self.mount("sub2api-redis", self.directory / "redis_data", "/data")
        for slot in self.state["slots"]:
            self.mount("sub2api-" + slot, self.directory / "data", "/app/data")
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
            running = self.inspect("sub2api-"+slot)
            require_matching_runtime(slot, env, running["Config"]["Env"])
        pool = int(env["DATABASE_MAX_OPEN_CONNS"])
        database_reserve = int(self.config["database_reserve"])
        require(database_reserve >= 0, "数据库维护预留不能为负数")
        pg = compose["services"]["postgres"]["environment"]
        capacity = int(self.run("docker", "exec", "sub2api-postgres", "psql", "-U", pg.get("POSTGRES_USER", "postgres"), "-d", pg.get("POSTGRES_DB", "sub2api"), "-Atc", "SHOW max_connections").stdout.strip())
        require(pool > 0 and 2 * pool + database_reserve <= capacity, "双实例数据库连接池加维护预留超过容量")
        memory = bytes_value(env["GOMEMLIMIT"])
        meminfo = dict((line.split(':')[0], int(line.split()[1])*1024) for line in Path("/proc/meminfo").read_text().splitlines())
        reserve = int(self.config["memory_reserve_bytes"])
        require(memory > 0 and reserve > 0, "应用内存与数据库/系统维护预留必须为正数")
        require(2 * memory + reserve <= meminfo["MemTotal"], "双实例内存总预算不足；不自动修改生产参数")
        if needs_capacity:
            require(memory + reserve <= meminfo["MemAvailable"], "候选启动所需的可用内存不足")
        return compose

    def render(self, slot, release, compose):
        spec = copy.deepcopy(compose["services"]["sub2api"])
        spec.pop("depends_on", None)
        spec["image"] = release["image"]
        spec["container_name"] = "sub2api-" + slot
        spec["restart"] = "on-failure"
        spec["ports"] = [{"target": 8080, "published": str(SLOTS[slot]), "host_ip": "127.0.0.1", "protocol": "tcp"}]
        spec["environment"].update({"LIFECYCLE_MODE": "standby", "LIFECYCLE_SOCKET": "/run/sub2api/"+slot+".sock", "LOG_OUTPUT_FILE_PATH": "/app/instance-logs/sub2api.log"})
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
            result = self.run("curl", "--fail", "--silent", "--show-error", "--max-time", "5", "--http1.1", "-H", "Connection: close", "--resolve", probe["resolve"], "-D", "-", probe["url"], check=False)
            if result.returncode != 0 or ("x-sub2api-instance: " + expected).lower() not in result.stdout.lower():
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
        self.control(slot, "activate")
        require(self.candidate_probe(slot, expected), "候选实际接入端口未就绪，保持原有流量")
        proxy = Path(self.config["upstream_file"])
        if self.state["phase"] != "switching":
            self.save(phase="switching", previous_proxy=proxy.read_text(), old_workers=self.nginx_workers())
        if not self.probe(expected):
            atomic_write(proxy, "# 由蓝绿部署器原子更新；API/direct 共用\nupstream sub2api_active { server 127.0.0.1:%d; }\n" % SLOTS[slot])
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
        self.save(active=slot, pending=old, phase="switched", switched_at=time.time(), verified_instance=expected)

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
        container = self.inspect("sub2api-"+target)
        require(container and container["Id"] == self.state["slots"][target]["container_id"], "回滚容器身份不匹配")
        self.control(target, "check")
        self.control(target, "activate")
        require(self.candidate_probe(target, expected), "旧实例实际接入端口不可用，保持当前流量")
        if not self.probe(expected):
            proxy = Path(self.config["upstream_file"])
            atomic_write(proxy, "upstream sub2api_active { server 127.0.0.1:%d; }\n" % SLOTS[target])
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
        require(not self.state.get("rollback") or self.state["phase"] == "stable", "回滚未完成，先用 --rollback 恢复，不改变当前流量")
        # 未排空槽优先处理；即使本次发布不同镜像，也不允许覆盖正在服务的旧实例。
        if self.state.get("pending"):
            same_release = self.state.get("release") == release
            if not self.drain(window if same_release else 0):
                require(same_release, "旧槽仍有工作，本次部署未改变当前流量")
                return
        if self.state["slots"][self.state["active"]]["sha"] == release["sha"] and self.state["phase"] == "stable":
            require(self.state["slots"][self.state["active"]]["image"] == release["image"], "同一提交的镜像摘要发生变化")
            return
        active = self.state["active"]
        slot = "green" if active == "blue" else "blue"
        current_release = self.state.get("release", {})
        require(self.state["phase"] == "stable" or current_release == release, "另一个发布未完成，必须先恢复同一发布")
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


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--directory", required=True)
    action = parser.add_mutually_exclusive_group(required=True)
    action.add_argument("--release", help="固定 SHA、digest、CI 及迁移兼容性验收记录")
    action.add_argument("--rollback", action="store_true", help="回切尚未退役的保留实例并排空新实例")
    parser.add_argument("--window", type=int, default=900, help="排空观察秒数；超时保留旧实例")
    args = parser.parse_args()
    root = Path(args.directory).resolve()/"blue-green"
    require(root.is_dir(), "首次迁移尚未审定，拒绝自动初始化生产")
    with (root/"deploy.lock").open("a") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as error:
            raise Refused("已有服务器发布正在执行，不自动取消它") from error
        deployment = Deployment(args.directory)
        try:
            require(args.window >= 0, "观察窗口不能为负数")
            if args.rollback:
                deployment.rollback(args.window)
            else:
                deployment.deploy(json.loads(Path(args.release).read_text()), args.window)
        except Exception as error:
            deployment.save(last_error=str(error))
            raise


if __name__ == "__main__":
    try:
        main()
    except (Refused, OSError, ValueError, KeyError) as error:
        print("发布安全停止："+str(error), file=sys.stderr)
        sys.exit(1)
