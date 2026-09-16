#!/usr/bin/env python3
"""使用隔离 Docker/Redis 验证强制停止和按实例清理，不调用真实模型。"""
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import time
import uuid
from unittest.mock import patch

sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location("state_tests", Path(__file__).with_name("blue-green-test.py"))
state_tests = importlib.util.module_from_spec(spec)
spec.loader.exec_module(state_tests)


def run(*args, **kwargs):
    return subprocess.run(args, check=True, text=True, capture_output=True, **kwargs)


def main():
    case = state_tests.BlueGreenTests()
    case.setUp()
    deployment = case.prepare_forced_retirement()
    prefix = "sub2api-force-test-"+uuid.uuid4().hex[:10]
    redis_name, old_name = prefix+"-redis", prefix+"-old"
    containers = []
    try:
        data = deployment.directory/"redis-data"
        data.mkdir()
        run("docker","run","-d","--name",redis_name,"--mount",f"type=bind,source={data},target=/data",
            "redis:8.4-alpine","redis-server","--save","","--appendonly","no")
        containers.append(redis_name)
        for attempt in range(100):
            response = subprocess.run(("docker","exec",redis_name,"redis-cli","--raw","PING"),text=True,capture_output=True)
            if response.returncode == 0 and response.stdout.strip() == "PONG":
                break
            time.sleep(0.1)
        else:
            raise AssertionError("隔离 Redis 未就绪")
        seed = "for index=1,600 do redis.call('SET','lifecycle:affinity:1:1:session:old'..index,'old'); redis.call('SET','lifecycle:affinity:1:1:response:active'..index,'active') end; redis.call('SET','lifecycle:live:old','old'); redis.call('SET','lifecycle:known:old','known'); redis.call('SET','unrelated:key','old'); return 1"
        run("docker","exec",redis_name,"redis-cli","EVAL",seed,"0")
        identity = run("docker","run","-d","--name",old_name,"alpine:3.23","sh","-c",
                       "trap 'echo received-TERM' TERM; echo ready; while :; do sleep 1; done").stdout.strip()
        containers.append(old_name)
        for attempt in range(100):
            if "ready" in run("docker","logs",identity).stdout:
                break
            time.sleep(0.1)
        deployment.state["slots"]["blue"]["container_id"] = identity
        original_run, original_inspect = deployment.run, deployment.inspect

        def execute(*args, check=True):
            if args[:3] == ("docker","exec","sub2api-redis"):
                return subprocess.run((*args[:2],redis_name,*args[3:]),check=check,text=True,capture_output=True)
            if args[:2] in (("docker","stop"),("docker","logs"),("docker","rm")) and args[-1] == identity:
                return subprocess.run(args,check=check,text=True,capture_output=True)
            return original_run(*args,check=check)

        def inspect(name):
            if name != "sub2api-blue":
                return original_inspect(name)
            result = subprocess.run(("docker","inspect",identity),text=True,capture_output=True)
            return json.loads(result.stdout)[0] if result.returncode == 0 else None

        deployment.run, deployment.inspect = execute, inspect
        with patch.object(state_tests.bg,"FORCED_STOP_GRACE",1):
            deployment.drain(3600)
        result = deployment.state["forced_retire_result"]
        assert deployment.state["phase"] == "stable" and deployment.state["active"] == "green"
        assert result["exit_code"] == 137 and result["usage_loss_unknown"] and result["logs_verified"]
        assert result["affinity_removed"] == 600
        assert inspect("sub2api-blue") is None
        remaining = run("docker","exec",redis_name,"redis-cli","--raw","DBSIZE").stdout.strip()
        assert remaining == "602", remaining
        assert run("docker","exec",redis_name,"redis-cli","--raw","GET","unrelated:key").stdout.strip() == "old"
        assert run("docker","exec",redis_name,"redis-cli","--raw","GET","lifecycle:known:old").stdout.strip() == "known"
        assert run("docker","exec",redis_name,"redis-cli","--raw","GET","lifecycle:affinity:1:1:response:active1").stdout.strip() == "active"
        print("隔离强制退役通过：实际Docker退出137、600条旧归属清理、活动及其他实例数据保留")
    finally:
        for container in reversed(containers):
            subprocess.run(("docker","rm","-f",container),capture_output=True)
        case.doCleanups()


if __name__ == "__main__":
    main()
