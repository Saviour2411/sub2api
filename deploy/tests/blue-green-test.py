#!/usr/bin/env python3
"""发布状态机单元测试；不把模拟切换当作真实 Nginx/应用验收。"""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location("bluegreen", ROOT/"deploy/blue-green/deploy.py")
bg = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bg)


class FakeDeployment(bg.Deployment):
    def __init__(self, directory):
        super().__init__(directory)
        self.calls = []
        self.fail = None
        self.busy = set()
        self.routed = self.state["active"]
        self.states = {slot: {"state":"active", "instance_id":entry["instance_id"], "version":"test"} for slot,entry in self.state["slots"].items()}
        self.containers = {"sub2api-"+slot: {"Id":entry["container_id"], "Config":{"Image":entry["image"]}} for slot,entry in self.state["slots"].items()}

    def run(self, *args, check=True):
        self.calls.append(args)
        if self.fail and self.fail == args[:len(self.fail)]:
            raise bg.Refused("注入失败")
        if args[:3] == ("docker","image","inspect"):
            output = json.dumps([{"Config":{"Labels":{"org.opencontainers.image.revision": self.state["release"]["sha"]}}}])
        elif args == ("nginx","-T"):
            output = "include /etc/nginx/conf.d/*.conf;"
        else:
            output = ""
        if args[:2] == ("docker","compose"):
            slot = args[-1]
            self.containers["sub2api-"+slot] = {"Id":"container-"+self.state["release"]["sha"],"Config":{"Image":self.state["release"]["image"]}}
            self.states[slot] = {"state":"standby","instance_id":"instance-"+self.state["release"]["sha"],"version":"test"}
        if args[:3] == ("nginx","-s","reload"):
            self.routed = "blue" if "server 127.0.0.1:18080; }" in Path(self.config["upstream_file"]).read_text().split("\n")[1 if Path(self.config["upstream_file"]).read_text().startswith("#") else 0] else "green"
        if args[:2] == ("docker","rm"):
            del self.containers[args[-1]]
        return subprocess.CompletedProcess(args,0,stdout=output,stderr="")

    def inspect(self, container):
        return self.containers.get(container)

    def control(self, slot, action="state"):
        self.calls.append(("control",slot,action))
        if self.fail == ("control",slot,action):
            raise bg.Refused("注入控制失败")
        state = self.states[slot]
        if action == "activate":
            state["state"] = "active"
        elif action == "drain":
            state["state"] = "draining" if slot in self.busy else "drained"
        elif action == "state" and state["state"] == "draining" and slot not in self.busy:
            state["state"] = "drained"
        elif action == "retire":
            bg.atomic_write(self.root/"run"/(slot+".sock.retired"),json.dumps({"sealed":True,"instance_id":state["instance_id"]}))
        if action == "synthetic":
            return "event: complete\ndata: ok\n\n"
        return dict(state) if action == "state" else ""

    def machine_id(self):
        return "test-machine"

    def resources(self):
        return {}

    def preflight(self, release, needs_capacity=True):
        self.calls.append(("preflight",release["sha"]))
        return {}

    def render(self, slot, release, compose):
        return self.root/(slot+".json")

    def probe(self, expected):
        return self.states[self.routed]["instance_id"] == expected

    def candidate_probe(self, slot, expected):
        return self.fail != ("candidate_probe",) and self.states[slot]["state"] == "active" and self.states[slot]["instance_id"] == expected

    def nginx_workers(self):
        return {}


class BlueGreenTests(unittest.TestCase):
    def setUp(self):
        cache = ROOT/".cache/tmp"
        cache.mkdir(parents=True,exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(prefix="bluegreen-test-",dir=cache)
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        root = self.directory/"blue-green"
        (root/"run").mkdir(parents=True)
        proxy = root/"upstream.conf"
        proxy.write_text("upstream sub2api_active { server 127.0.0.1:18080; }\n")
        (root/"config.json").write_text(json.dumps({"upstream_file":str(proxy)}))
        (root/"state.json").write_text(json.dumps({"schema":1,"generation":0,"active":"blue","phase":"stable","slots":{"blue":{"sha":"0"*40,"image":"app@sha256:"+"0"*64,"instance_id":"old","container_id":"old-container"}}}))
        self.deployment = FakeDeployment(self.directory)

    def release(self, number):
        sha = f"{number:040x}"
        return {"sha":sha,"image":"app@sha256:"+f"{number:064x}","ci_sha":sha,"ci":"success","security":"success","compatible_from":self.deployment.state["slots"][self.deployment.state["active"]]["sha"],"migration_class":"none"}

    def test_missing_migration_state_refuses_adoption(self):
        self.deployment.state_path.unlink()
        with self.assertRaisesRegex(bg.Refused,"首次初始化"):
            bg.Deployment(self.directory)

    def test_twenty_state_machine_cycles(self):
        for index in range(1,21):
            release = self.release(index)
            self.deployment.deploy(release,window=0)
            self.assertEqual("stable",self.deployment.state["phase"])
            calls = list(self.deployment.calls)
            self.deployment.deploy(release,window=0)
            self.assertEqual(calls,self.deployment.calls,"同一发布不重复启动/切流")
        stops = [call for call in self.deployment.calls if call[:2] == ("docker","stop")]
        self.assertEqual(20,len(stops))
        self.assertTrue(all(call[2:4] == ("--timeout","-1") for call in stops))

    def test_busy_old_slot_is_retained_and_next_release_refused(self):
        self.deployment.busy.add("blue")
        first = self.release(1)
        self.deployment.deploy(first,window=0)
        self.assertEqual("green",self.deployment.state["active"])
        self.assertEqual("drain_pending",self.deployment.state["phase"])
        self.assertIn("sub2api-blue",self.deployment.containers)
        with self.assertRaisesRegex(bg.Refused,"旧槽仍有工作"):
            self.deployment.deploy(self.release(2),window=0)
        self.assertEqual("green",self.deployment.state["active"])
        self.deployment.busy.clear()
        self.deployment.deploy(first,window=0)
        self.assertEqual("stable",self.deployment.state["phase"])

    def test_nginx_failure_never_drains_old_instance(self):
        self.deployment.fail = ("nginx","-t")
        with self.assertRaises(bg.Refused):
            self.deployment.deploy(self.release(1),window=0)
        self.assertEqual("blue",self.deployment.state["active"])
        self.assertNotIn(("control","blue","drain"),self.deployment.calls)
        self.assertIn("sub2api-blue",self.deployment.containers)
        self.assertIn("sub2api-green",self.deployment.containers)

    def test_interrupt_after_retire_resumes_without_restarting_candidate(self):
        release = self.release(1)
        self.deployment.fail = ("docker","stop")
        with self.assertRaises(bg.Refused):
            self.deployment.deploy(release,window=0)
        self.assertEqual("retiring",self.deployment.state["phase"])
        self.deployment.fail = None
        starts = sum(call[:2] == ("docker","compose") for call in self.deployment.calls)
        self.deployment.deploy(release,window=0)
        self.assertEqual("stable",self.deployment.state["phase"])
        self.assertEqual(starts,sum(call[:2] == ("docker","compose") for call in self.deployment.calls))

    def test_retirement_requires_exact_receipt(self):
        self.deployment.save(phase="retiring",pending="blue")
        with self.assertRaisesRegex(bg.Refused,"凭据缺失"):
            self.deployment.drain(0)
        bg.atomic_write(self.deployment.root/"run/blue.sock.retired",json.dumps({"sealed":True,"instance_id":"wrong"}))
        with self.assertRaisesRegex(bg.Refused,"不匹配"):
            self.deployment.drain(0)
        self.assertFalse(any(call[:2] == ("docker","stop") for call in self.deployment.calls))

    def test_rollback_after_drain_keeps_new_requests_until_finished(self):
        self.deployment.busy.add("blue")
        release = self.release(1)
        release["rollback_compatible"] = True
        self.deployment.deploy(release, window=0)
        self.deployment.busy.add("green")
        self.deployment.rollback(window=0)
        self.assertEqual("blue", self.deployment.state["active"])
        self.assertEqual("drain_pending", self.deployment.state["phase"])
        self.assertEqual("active", self.deployment.states["blue"]["state"])
        self.assertIn("sub2api-green", self.deployment.containers)
        with self.assertRaisesRegex(bg.Refused, "回滚未完成"):
            self.deployment.deploy(self.release(2), window=0)
        self.deployment.busy.clear()
        self.deployment.rollback(window=0)
        self.assertEqual("stable", self.deployment.state["phase"])
        self.assertNotIn("sub2api-green", self.deployment.containers)
        self.deployment.deploy(self.release(2), window=0)
        self.assertEqual("green", self.deployment.state["active"])

    def test_rollback_requires_compatibility_and_survives_interrupt(self):
        self.deployment.busy.add("blue")
        release = self.release(1)
        self.deployment.deploy(release, window=0)
        with self.assertRaisesRegex(bg.Refused, "兼容审查"):
            self.deployment.rollback(window=0)
        self.assertEqual("green", self.deployment.state["active"])
        self.deployment.state["release"]["rollback_compatible"] = True
        self.deployment.fail = ("nginx", "-s", "reload")
        with self.assertRaises(bg.Refused):
            self.deployment.rollback(window=0)
        self.assertEqual("rolling_back", self.deployment.state["phase"])
        self.assertEqual("green", self.deployment.routed)
        self.deployment.fail = None
        self.deployment.rollback(window=0)
        self.assertEqual("blue", self.deployment.routed)
        self.assertEqual("stable", self.deployment.state["phase"])

    def test_healthy_socket_but_unreachable_port_never_switches(self):
        self.deployment.fail = ("candidate_probe",)
        with self.assertRaisesRegex(bg.Refused, "接入端口未就绪"):
            self.deployment.deploy(self.release(1), window=0)
        self.assertEqual("blue", self.deployment.routed)
        self.assertFalse(any(call[:2] == ("nginx", "-s") for call in self.deployment.calls))
        self.assertNotIn(("control", "blue", "drain"), self.deployment.calls)

    def test_candidate_budget_cannot_hide_larger_running_pool(self):
        expected = {"DATABASE_MAX_OPEN_CONNS":"192", "GOMEMLIMIT":"1GiB", "JWT_SECRET":"shared", "TOTP_ENCRYPTION_KEY":"shared-key"}
        entries = [key+"="+value for key,value in expected.items()]
        bg.require_matching_runtime("blue", expected, entries)
        for key, value in (("DATABASE_MAX_OPEN_CONNS", "512"), ("GOMEMLIMIT", "10GiB"), ("JWT_SECRET", "different"), ("TOTP_ENCRYPTION_KEY", "different")):
            actual = dict(expected, **{key:value})
            with self.assertRaisesRegex(bg.Refused, key):
                bg.require_matching_runtime("blue", expected, [key+"="+value for key,value in actual.items()])

    def test_bootstrap_creates_files_and_is_recoverable(self):
        d = self.deployment
        d.state["slots"]["blue"]["legacy"] = True
        d.state["slots"]["blue"]["sha"] = bg.LEGACY_SHA
        d.config = None
        (self.directory/"docker-compose.yml").write_text("test-compose")
        (self.directory/"docker-compose.sub2api.yml").write_text("test-compose")
        nginx = self.directory/"nginx"
        (nginx/"conf.d").mkdir(parents=True)
        (nginx/"sites-enabled").mkdir()
        for host,port in bg.VHOSTS.values():
            (nginx/"sites-enabled"/("sub2api-"+host)).write_text(f"server {{ listen {port} ssl; server_name {host}; location / {{ proxy_pass http://127.0.0.1:18080; }} }}")
        release = self.release(1)
        with patch.object(bg,"NGINX_ROOT",nginx), patch.object(bg.os,"chown"):
            d.initialize(release)
            self.assertTrue(d.state_path.is_file())
            self.assertTrue((d.root/"config.json").is_file())
            self.assertTrue((d.root/"bootstrap.json").is_file())
            self.assertIn("legacy.sock",Path(d.config["upstream_file"]).read_text())
            self.assertEqual("blue",d.routed,"初始化 reload 不能提前切流")
            # 模拟配置已转换但状态落盘前中断；从 journal 恢复，不猜测或重建旧实例。
            saved = json.loads((d.root/"bootstrap.json").read_text())
            d.state = saved["state"]
            d.state_path.unlink()
            d.config = None
            d.initialize(release)
            self.assertTrue(d.state_path.is_file())
        self.assertFalse(any(c[:2] in (("docker","stop"),("docker","compose")) for c in d.calls))

    def test_actual_resources_allow_overcommit_but_reject_real_pressure(self):
        sample = {"maximum":600, "reserved":3, "used":30, "memory_available":100*1024**3}
        bg.resource_check(sample,{})
        with self.assertRaisesRegex(bg.Refused,"实际可用连接"):
            bg.resource_check(dict(sample,used=580),{})
        with self.assertRaisesRegex(bg.Refused,"实际可用内存"):
            bg.resource_check(dict(sample,memory_available=512*1024**2),{})
        with self.assertRaisesRegex(bg.Refused,"阈值"):
            bg.resource_check(sample,{"database_free_min":0})

    def test_first_legacy_deployment_keeps_old_and_rejects_next(self):
        self.deployment.state["slots"]["blue"]["legacy"] = True
        release = self.release(1)
        self.deployment.deploy(release,window=0)
        self.assertEqual("green",self.deployment.state["active"])
        self.assertEqual("legacy_unverifiable",self.deployment.state["pending_reason"])
        self.assertFalse(any(c[:2] == ("docker","stop") for c in self.deployment.calls))
        starts = sum(c[:2] == ("docker","compose") for c in self.deployment.calls)
        self.deployment.deploy(release,window=0)
        with self.assertRaisesRegex(bg.Refused,"旧槽仍有工作"):
            self.deployment.deploy(self.release(2),window=0)
        self.assertEqual(starts,sum(c[:2] == ("docker","compose") for c in self.deployment.calls))
        self.assertEqual("green",self.deployment.routed)

    def test_legacy_proxy_is_fixed_private_and_disables_retries(self):
        self.deployment.state["slots"]["blue"]["legacy"] = True
        rendered = self.deployment.proxy_config("green")
        self.assertIn("server 127.0.0.1:18082",rendered)
        self.assertIn("listen unix:",rendered)
        self.assertIn("proxy_pass http://127.0.0.1:18080",rendered)
        self.assertIn("proxy_next_upstream off",rendered)
        self.assertNotIn("18081",rendered)

    def test_budget_parsing_requires_explicit_limit(self):
        self.assertEqual(3*1024**3,bg.bytes_value("3GiB"))
        for value in ("off","80%","-1","",None):
            with self.assertRaises(bg.Refused): bg.bytes_value(value)


if __name__ == "__main__":
    unittest.main()
