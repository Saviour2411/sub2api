#!/usr/bin/env python3
"""发布状态机单元测试；不把模拟切换当作真实 Nginx/应用验收。"""
import importlib.util
import json
import os
from pathlib import Path
import socket
import subprocess
import sys
import tempfile
import time
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
        self.legacy_gate_ready = False
        self.routed = self.state["active"]
        self.states = {slot: {"state":"active", "instance_id":entry["instance_id"], "version":"test"} for slot,entry in self.state["slots"].items()}
        self.containers = {"sub2api-"+slot: {"Id":entry["container_id"], "Config":{"Image":entry["image"]}} for slot,entry in self.state["slots"].items()}

    def run(self, *args, check=True):
        self.calls.append(args)
        if self.fail and self.fail == args[:len(self.fail)]:
            raise bg.Refused("注入失败")
        if args[:3] == ("docker","image","inspect"):
            output = json.dumps([{"Config":{"Labels":{"org.opencontainers.image.revision": self.state["release"]["sha"]}}}])
        elif args[:5] == ("docker","exec","sub2api-redis","redis-cli","--raw"):
            output = "0\n5\n" if args[5] == "EVAL" else "1\n"
        elif args == ("nginx","-T"):
            output = "include /etc/nginx/conf.d/*.conf;"
        else:
            output = ""
        if args[:2] == ("docker","compose"):
            slot = args[-1]
            self.containers["sub2api-"+slot] = {"Id":"container-"+self.state["release"]["sha"],"Config":{"Image":self.state["release"]["image"]},"State":{"Running":True}}
            self.states[slot] = {"state":"standby","instance_id":"instance-"+self.state["release"]["sha"],"version":"test"}
        if args[:3] == ("nginx","-s","reload"):
            self.routed = "blue" if "server 127.0.0.1:18080; }" in Path(self.config["upstream_file"]).read_text().split("\n")[1 if Path(self.config["upstream_file"]).read_text().startswith("#") else 0] else "green"
        if args[:2] in (("docker","stop"),("docker","rm")):
            name = next(name for name,entry in self.containers.items() if name == args[-1] or entry["Id"] == args[-1])
            if args[1] == "rm":
                del self.containers[name]
            else:
                self.containers[name]["State"] = {"Running":False,"ExitCode":0 if args[3] == "-1" else 137}
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

    def mount(self, container, source, destination):
        self.calls.append(("mount",container,str(source),destination))
        bg.require(container in self.containers, "挂载容器不存在")

    def resources(self):
        return {}

    def preflight(self, release, needs_capacity=True):
        self.calls.append(("preflight",release["sha"]))
        return {}

    def pull_release_image(self, release):
        self.calls.append(("pull",release["sha"]))

    def wait_legacy_retire_gate(self, window, quiet_seconds):
        self.calls.append(("legacy_gate",window,quiet_seconds))
        if not self.legacy_gate_ready:
            raise bg.Refused("旧版退役门禁未通过：quiet_window")
        return {"eligible":True}

    def retire_legacy_for_release(self, release):
        self.calls.append(("legacy_retire",release["sha"]))
        old = self.state["pending"]
        slots = dict(self.state["slots"])
        del slots[old]
        self.containers.pop("sub2api-"+old,None)
        self.state.update(slots=slots,pending=None,phase="stable",pending_reason=None,legacy_retained=False,
                          legacy_retired_at=time.time(),legacy_retire_operation=None,
                          prepared_release=release,
                          legacy_retire_result={"release_sha":release["sha"],"usage_drop_count":0,"forced_shutdown_count":0,"clean":True})
        Path(self.config["upstream_file"]).write_text(self.proxy_config(self.state["active"],include_legacy=False))

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
        self.assertTrue(all(call[2:4] == ("-t","-1") for call in stops))

    def test_failed_expansion_keeps_old_instance_and_does_not_restart_candidate(self):
        release = dict(self.release(1),migration_class="expand",migration_policy={"version":1,"migrations":[{}]})
        original = self.deployment.run
        def execute(*args, **kwargs):
            result = original(*args, **kwargs)
            if args[:2] == ("docker","compose"):
                self.deployment.containers["sub2api-green"]["State"]["Running"] = False
            return result
        with patch.object(self.deployment,"run",side_effect=execute):
            with self.assertRaisesRegex(bg.Refused,"禁止自动重试"):
                self.deployment.deploy(release,window=0)
        self.assertEqual("blue",self.deployment.state["active"])
        self.assertNotIn(("control","blue","drain"),self.deployment.calls)
        self.assertFalse(any(call[:2] in (("docker","stop"),("docker","update"),("docker","restart")) for call in self.deployment.calls))

    def test_successful_expansion_restores_restart_policy_before_switch(self):
        release = dict(self.release(1),migration_class="expand",migration_policy={"version":1,"migrations":[{}]})
        self.deployment.deploy(release,window=0)
        calls = self.deployment.calls
        self.assertLess(calls.index(("control","green","synthetic")),calls.index(("docker","update","--restart=on-failure","sub2api-green")))
        self.assertLess(calls.index(("docker","update","--restart=on-failure","sub2api-green")),calls.index(("control","green","activate")))
        self.assertFalse(bg.same_release(release,dict(release,migration_policy={"version":1,"migrations":[]})))

    def test_expansion_render_disables_restart_and_binds_policy(self):
        release = dict(self.release(1),migration_class="expand",migration_policy={"version":1,"migrations":[{"filename":"999_test.sql"}]})
        compose = {"services":{"sub2api":{"environment":{},"volumes":[]}},"networks":{}}
        with patch.object(bg.os,"chown"):
            path = bg.Deployment.render(self.deployment,"green",release,compose)
        spec = json.loads(path.read_text())["services"]["green"]
        self.assertEqual("no",spec["restart"])
        self.assertEqual(release["migration_policy"],json.loads(spec["environment"]["SUB2API_BLUE_GREEN_MIGRATIONS"]))

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

    def prepare_forced_retirement(self):
        deployment = self.deployment
        deployment.busy.add("blue")
        deployment.deploy(self.release(1),window=0)
        deployment.force_after_window = True
        deployment.config["machine_id"] = "test-machine"
        deployment.state["drain_started_at"] = time.time()-3601
        for container in deployment.containers.values():
            container["State"] = {"Running":True,"ExitCode":0}
        deployment.states["blue"].update(work={"http":3,"sse":2,"usage_pending":1},session_leases=4)
        return deployment

    def test_expired_drain_forces_only_pending_identity_and_clears_socket(self):
        deployment = self.prepare_forced_retirement()
        listener = socket.socket(socket.AF_UNIX)
        listener.bind(str(deployment.root/"run/blue.sock"))
        listener.close()
        deployment.drain(3600)
        self.assertEqual("stable",deployment.state["phase"])
        self.assertEqual("green",deployment.state["active"])
        self.assertIsNone(deployment.state["pending"])
        self.assertFalse((deployment.root/"run/blue.sock").exists())
        self.assertNotIn("sub2api-blue",deployment.containers)
        self.assertIn(("docker","stop","-t","60","old-container"),deployment.calls)
        result = deployment.state["forced_retire_result"]
        self.assertEqual(137,result["exit_code"])
        self.assertTrue(result["usage_loss_unknown"])
        self.assertFalse(result["clean_exit"])
        self.assertEqual(5,result["affinity_removed"])
        self.assertNotIn("drained_at",deployment.state)
        self.assertNotIn(("control","blue","retire"),deployment.calls)

    def test_hour_window_still_allows_natural_retirement(self):
        deployment = self.prepare_forced_retirement()
        deployment.state["drain_started_at"] = time.time()-60
        with patch.object(bg.time,"sleep",side_effect=lambda _: deployment.busy.clear()):
            deployment.drain(3600)
        self.assertEqual("stable",deployment.state["phase"])
        self.assertNotIn("forced_retire_result",deployment.state)
        self.assertIn(("docker","stop","-t","-1","sub2api-blue"),deployment.calls)

    def test_forced_retirement_refuses_unhealthy_active(self):
        deployment = self.prepare_forced_retirement()
        deployment.fail = ("candidate_probe",)
        with self.assertRaisesRegex(bg.Refused,"双入口未就绪"):
            deployment.drain(3600)
        self.assertFalse(any(call[:2] == ("docker","stop") for call in deployment.calls))
        self.assertEqual("green",deployment.state["active"])

    def test_unreachable_old_control_can_be_forced_after_deadline(self):
        deployment = self.prepare_forced_retirement()
        deployment.fail = ("control","blue","state")
        deployment.drain(3600)
        self.assertEqual("stable",deployment.state["phase"])
        self.assertTrue(deployment.state["forced_retire_result"]["before"]["work_unknown"])

    def test_old_control_failure_before_deadline_does_not_force_early(self):
        deployment = self.prepare_forced_retirement()
        deployment.state["drain_started_at"] = time.time()-60
        deployment.fail = ("control","blue","state")
        def recovered(_):
            deployment.fail = None
            deployment.busy.clear()
        with patch.object(bg.time,"sleep",side_effect=recovered):
            deployment.drain(3600)
        self.assertNotIn("forced_retire_result",deployment.state)

    def test_forced_retirement_refuses_replaced_old_container(self):
        deployment = self.prepare_forced_retirement()
        deployment.containers["sub2api-blue"]["Id"] = "unrelated"
        with self.assertRaisesRegex(bg.Refused,"旧容器身份"):
            deployment.drain(3600)
        self.assertFalse(any(call[:2] == ("docker","stop") for call in deployment.calls))

    def test_forced_retirement_refuses_active_slot(self):
        deployment = self.prepare_forced_retirement()
        deployment.state["pending"] = "green"
        with self.assertRaisesRegex(bg.Refused,"非活动旧槽"):
            deployment.force_retire_pending()

    def test_forced_retirement_resumes_stop_without_resetting_grace(self):
        deployment = self.prepare_forced_retirement()
        deployment.fail = ("docker","stop")
        with self.assertRaises(bg.Refused):
            deployment.drain(3600)
        self.assertEqual("force_retiring",deployment.state["phase"])
        deployment.state["forced_retire_operation"]["stop_started_at"] -= 100
        deployment.fail = None
        deployment.drain(3600)
        self.assertIn(("docker","stop","-t","0","old-container"),deployment.calls)
        self.assertEqual("stable",deployment.state["phase"])

    def test_failed_affinity_cleanup_preserves_evidence_and_retries(self):
        deployment = self.prepare_forced_retirement()
        with patch.object(deployment,"clear_retired_affinity",side_effect=bg.Refused("redis unavailable")):
            with self.assertRaisesRegex(bg.Refused,"redis unavailable"):
                deployment.drain(3600)
        self.assertEqual("force_retiring",deployment.state["phase"])
        self.assertFalse(deployment.containers["sub2api-blue"]["State"]["Running"])
        self.assertIn("forced_retire_result",deployment.state)
        deployment.drain(3600)
        self.assertEqual(1,sum(call[:2] == ("docker","stop") for call in deployment.calls))
        self.assertEqual("stable",deployment.state["phase"])

    def test_old_nginx_workers_block_reuse_but_are_not_killed(self):
        deployment = self.prepare_forced_retirement()
        with patch.object(deployment,"old_workers_present",return_value=True):
            with self.assertRaisesRegex(bg.Refused,"旧代理 worker"):
                deployment.drain(3600)
        self.assertEqual("force_retiring",deployment.state["phase"])
        self.assertIn("blue",deployment.state["slots"])
        self.assertFalse(any(call[0] in ("kill","pkill") for call in deployment.calls))
        deployment.drain(3600)
        self.assertEqual("stable",deployment.state["phase"])

    def test_next_release_reuses_forced_retired_slot(self):
        deployment = self.prepare_forced_retirement()
        deployment.deploy(self.release(2),window=3600)
        self.assertEqual("blue",deployment.state["active"])
        self.assertEqual("stable",deployment.state["phase"])
        self.assertIn("sub2api-blue",deployment.containers)

    def test_forced_retirement_requires_original_drain_timestamp(self):
        deployment = self.prepare_forced_retirement()
        deployment.state.pop("drain_started_at")
        deployment.state.pop("switched_at")
        with self.assertRaisesRegex(bg.Refused,"起始时间"):
            deployment.drain(3600)

    def test_force_resume_refuses_changed_active_identity(self):
        deployment = self.prepare_forced_retirement()
        deployment.fail = ("docker","stop")
        with self.assertRaises(bg.Refused):
            deployment.drain(3600)
        deployment.fail = None
        deployment.state["forced_retire_operation"]["active_instance_id"] = "different"
        with self.assertRaisesRegex(bg.Refused,"恢复身份"):
            deployment.drain(3600)
        self.assertTrue(deployment.containers["sub2api-blue"]["State"]["Running"])

    def test_affinity_cleanup_timeout_checkpoints_cursor_and_count(self):
        deployment = self.prepare_forced_retirement()
        deployment.state["forced_retire_result"] = {"instance_id":"old","affinity_removed":2}
        reply = subprocess.CompletedProcess([],0,stdout="17\n3\n",stderr="")
        with patch.object(deployment,"run",return_value=reply), patch.object(bg.time,"monotonic",side_effect=[0,61]):
            with self.assertRaisesRegex(bg.Refused,"已保存游标"):
                deployment.clear_retired_affinity("old")
        self.assertEqual("17",deployment.state["forced_retire_result"]["affinity_cursor"])
        self.assertEqual(5,deployment.state["forced_retire_result"]["affinity_removed"])
        deployment.clear_retired_affinity("old")
        scan = next(call for call in deployment.calls if call[:5] == ("docker","exec","sub2api-redis","redis-cli","--raw") and call[5] == "EVAL")
        self.assertEqual(("17","old"),scan[-2:])

    def test_forced_retirement_preserves_cleanup_count_across_resume(self):
        deployment = self.prepare_forced_retirement()
        with patch.object(deployment,"clear_retired_affinity",return_value=7), patch.object(deployment,"old_workers_present",return_value=True):
            with self.assertRaises(bg.Refused):
                deployment.drain(3600)
        first_stop = deployment.state["forced_retire_result"]["stopped_at"]
        with patch.object(deployment,"clear_retired_affinity",return_value=2):
            deployment.drain(3600)
        self.assertEqual(9,deployment.state["forced_retire_result"]["affinity_removed"])
        self.assertEqual(first_stop,deployment.state["forced_retire_result"]["stopped_at"])

    def test_failed_log_capture_never_claims_clean_force_exit(self):
        deployment = self.prepare_forced_retirement()
        original_run = deployment.run
        def execute(*args, **kwargs):
            if args[:2] == ("docker","logs"):
                return subprocess.CompletedProcess(args,1,stdout="",stderr="logs unavailable")
            return original_run(*args,**kwargs)
        with patch.object(deployment,"run",side_effect=execute):
            deployment.drain(3600)
        self.assertFalse(deployment.state["forced_retire_result"]["logs_verified"])
        self.assertFalse(deployment.state["forced_retire_result"]["clean_exit"])
        self.assertTrue(deployment.state["forced_retire_result"]["usage_loss_unknown"])

    def test_forced_retirement_resumes_after_container_removal(self):
        deployment = self.prepare_forced_retirement()
        original_run = deployment.run
        def execute(*args, **kwargs):
            result = original_run(*args,**kwargs)
            if args[:2] == ("docker","rm"):
                raise bg.Refused("removed before interruption")
            return result
        with patch.object(deployment,"run",side_effect=execute):
            with self.assertRaisesRegex(bg.Refused,"interruption"):
                deployment.drain(3600)
        self.assertNotIn("sub2api-blue",deployment.containers)
        self.assertEqual("force_retiring",deployment.state["phase"])
        deployment.drain(3600)
        self.assertEqual("stable",deployment.state["phase"])

    def test_release_enables_hour_deadline_with_sufficient_job_timeout(self):
        workflow = (ROOT/".github/workflows/release.yml").read_text()
        self.assertIn("--window 3600 --force-after-window",workflow)
        self.assertIn("timeout-minutes: 120",workflow.split("  deploy-production:",1)[1])

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

    def test_resume_ignores_new_ci_run_but_not_image_or_baseline(self):
        release = self.release(1)
        self.deployment.busy.add("blue")
        self.deployment.deploy(release,window=0)
        retry = dict(release, checks={"backend-ci.yml":{"run_id":999}})
        self.deployment.deploy(retry,window=0)
        self.assertEqual("green",self.deployment.routed)
        for key in ("sha","image","compatible_from"):
            self.assertFalse(bg.same_release(retry,dict(release,**{key:"different"})))

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
        with self.assertRaisesRegex(bg.Refused,"旧版退役门禁未通过"):
            self.deployment.deploy(self.release(2),window=0)
        self.assertEqual(starts,sum(c[:2] == ("docker","compose") for c in self.deployment.calls))
        self.assertEqual("green",self.deployment.routed)

    def test_next_release_retires_legacy_then_reuses_blue(self):
        self.deployment.state["slots"]["blue"]["legacy"] = True
        first = self.release(1)
        self.deployment.deploy(first,window=0)
        self.deployment.legacy_gate_ready = True
        second = self.release(2)
        self.deployment.deploy(second,window=0,legacy_quiet=0)
        self.assertEqual("blue",self.deployment.state["active"])
        self.assertEqual("stable",self.deployment.state["phase"])
        self.assertNotIn("legacy",json.dumps(self.deployment.state["slots"]))
        pull = self.deployment.calls.index(("pull",second["sha"]))
        retire = self.deployment.calls.index(("legacy_retire",second["sha"]))
        start = next(i for i,c in enumerate(self.deployment.calls) if i > retire and c[:2] == ("docker","compose"))
        self.assertLess(pull,retire)
        self.assertLess(retire,start)
        self.assertEqual(1,self.deployment.calls.count(("pull",second["sha"])))
        self.assertIn(("legacy_gate",1200,0),self.deployment.calls)
        self.assertNotIn("18081",Path(self.deployment.config["upstream_file"]).read_text())

    def test_legacy_gate_requires_unchanged_log_and_quiet_window(self):
        sample = {"access_log_marker":[1,2,3,4],"blockers":[]}
        report = bg.update_legacy_gate({},sample,10,100)
        self.assertFalse(report["eligible"])
        self.assertEqual(100,report["first_clear_at"])
        report = bg.update_legacy_gate(report,sample,10,109)
        self.assertFalse(report["eligible"])
        report = bg.update_legacy_gate(report,sample,10,110)
        self.assertTrue(report["eligible"])
        changed = dict(sample,access_log_marker=[1,2,4,5])
        report = bg.update_legacy_gate(report,changed,10,111)
        self.assertFalse(report["eligible"])
        self.assertIsNone(report["first_clear_at"])
        blocked = dict(changed,blockers=["legacy_tcp_connections"])
        report = bg.update_legacy_gate(report,blocked,10,112)
        self.assertFalse(report["eligible"])

    def test_legacy_gate_resets_when_active_instance_changes(self):
        path = self.deployment.root/"legacy-retire.json"
        path.write_text(json.dumps({"eligible":True,"first_clear_at":1,"access_log_marker":[1,2,3,4],
                                    "legacy_container_id":"legacy","switched_at":10,"active_slot":"green",
                                    "active_instance_id":"replaced"}))
        sample = {"schema":1,"legacy_container_id":"legacy","switched_at":10,"active_slot":"green",
                  "active_instance_id":"current","access_log_marker":[1,2,3,4],"blockers":[]}
        with patch.object(self.deployment,"ensure_legacy_observer"), patch.object(self.deployment,"legacy_gate_sample",return_value=sample):
            with self.assertRaisesRegex(bg.Refused,"quiet_window"):
                bg.Deployment.wait_legacy_retire_gate(self.deployment,0,10)
        report = json.loads(path.read_text())
        self.assertEqual("current",report["active_instance_id"])
        self.assertFalse(report["eligible"])
        self.assertIsNotNone(report["first_clear_at"])

    def test_legacy_retire_recovery_uses_persisted_gate_and_local_image(self):
        self.deployment.state["slots"]["blue"]["legacy"] = True
        first = self.release(1)
        self.deployment.deploy(first,window=0)
        second = self.release(2)
        old = self.deployment.state["pending"]
        active = self.deployment.state["active"]
        operation = {"release":second,"legacy_container_id":self.deployment.state["slots"][old]["container_id"],
                     "active_slot":active,"active_instance_id":self.deployment.state["slots"][active]["instance_id"],
                     "image_prepared":True,"gate":{"eligible":True}}
        self.deployment.state.update(legacy_retire_operation=operation,
                                     legacy_retire_result={"release_sha":second["sha"],"exit_code":0,"usage_drop_count":0,
                                                           "forced_shutdown_count":0,"clean":True})
        with patch.object(self.deployment,"verify_release_image") as verify_image, \
             patch.object(self.deployment,"legacy_connection_counts",return_value={"tcp_18080":0,"legacy_unix":0}), \
             patch.object(self.deployment,"probe",return_value=True):
            result = bg.Deployment.retire_legacy_for_release(self.deployment,second)
        verify_image.assert_called_once_with(second)
        self.assertTrue(result["clean"])
        self.assertIsNone(self.deployment.state["pending"])
        self.assertNotIn(old,self.deployment.state["slots"])
        self.assertEqual(second,self.deployment.state["prepared_release"])
        self.assertIn(("nginx","-s","reload"),self.deployment.calls)
        self.assertIn(("ss","-Hxl"),self.deployment.calls)

    def test_legacy_observer_preserves_log_marker_on_rerun(self):
        deployment = self.deployment
        deployment.state["slots"]["blue"]["legacy"] = True
        deployment.deploy(self.release(1),window=0)
        logfile = deployment.root/"legacy.access.log"
        logfile.write_text("")
        with patch.object(bg,"LEGACY_ACCESS_LOG",logfile):
            before = deployment.legacy_access_marker()
            bg.Deployment.ensure_legacy_observer(deployment)
            bg.Deployment.ensure_legacy_observer(deployment)
            self.assertEqual(before,deployment.legacy_access_marker())

    def test_legacy_observer_restores_proxy_after_invalid_configuration(self):
        deployment = self.deployment
        deployment.state["slots"]["blue"]["legacy"] = True
        deployment.deploy(self.release(1),window=0)
        proxy = Path(deployment.config["upstream_file"])
        before = proxy.read_text()
        original_run = deployment.run
        def invalid_once(*args, **kwargs):
            if args == ("nginx","-t") and proxy.read_text() != before:
                raise bg.Refused("invalid configuration")
            return original_run(*args,**kwargs)
        with patch.object(bg,"LEGACY_ACCESS_LOG",deployment.root/"legacy.access.log"), \
             patch.object(deployment,"run",side_effect=invalid_once):
            with self.assertRaisesRegex(bg.Refused,"invalid configuration"):
                bg.Deployment.ensure_legacy_observer(deployment)
        self.assertEqual(before,proxy.read_text())

    def test_legacy_retirement_cannot_claim_clean_when_logs_unavailable(self):
        deployment = self.deployment
        deployment.state["slots"]["blue"]["legacy"] = True
        deployment.deploy(self.release(1),window=0)
        release = self.release(2)
        old = deployment.state["pending"]
        active = deployment.state["active"]
        container = deployment.containers.pop("sub2api-"+old)
        container["State"] = {"Running":False,"ExitCode":0}
        deployment.containers["sub2api"] = container
        deployment.state.update(legacy_stop_started_at="2026-09-16T04:00:00Z",legacy_retire_operation={
            "release":release,"legacy_container_id":container["Id"],"active_slot":active,
            "active_instance_id":deployment.state["slots"][active]["instance_id"],
            "image_prepared":True,"gate":{"eligible":True}})
        original_run = deployment.run
        def unavailable_logs(*args, **kwargs):
            if args[:2] == ("docker","logs"):
                return subprocess.CompletedProcess(args,1,stdout="",stderr="logs unavailable")
            return original_run(*args,**kwargs)
        with patch.object(deployment,"verify_release_image"), \
             patch.object(deployment,"legacy_connection_counts",return_value={"tcp_18080":0,"legacy_unix":0}), \
             patch.object(deployment,"run",side_effect=unavailable_logs):
            result = bg.Deployment.retire_legacy_for_release(deployment,release)
        self.assertFalse(result["clean"])
        self.assertFalse(result["logs_verified"])
        self.assertEqual(release["sha"],result["release_sha"])
        self.assertIsNone(deployment.state["pending"])

    def test_legacy_retire_rejects_stale_gate_before_sealing(self):
        self.deployment.state["slots"]["blue"]["legacy"] = True
        first = self.release(1)
        self.deployment.deploy(first,window=0)
        second = self.release(2)
        old = self.deployment.state["pending"]
        active = self.deployment.state["active"]
        marker = [1,2,3,4]
        report = {"eligible":True,"legacy_container_id":self.deployment.state["slots"][old]["container_id"],
                  "switched_at":self.deployment.state["switched_at"],"active_slot":active,
                  "active_instance_id":"stale","access_log_marker":marker}
        (self.deployment.root/"legacy-retire.json").write_text(json.dumps(report))
        sample = dict(report,active_instance_id=self.deployment.state["slots"][active]["instance_id"],blockers=[])
        before = Path(self.deployment.config["upstream_file"]).read_text()
        with patch.object(self.deployment,"legacy_gate_sample",return_value=sample):
            with self.assertRaisesRegex(bg.Refused,"实例身份"):
                bg.Deployment.retire_legacy_for_release(self.deployment,second)
        self.assertEqual(before,Path(self.deployment.config["upstream_file"]).read_text())

    def test_recovered_release_does_not_pull_after_legacy_retirement(self):
        self.deployment.state["slots"]["blue"]["legacy"] = True
        first = self.release(1)
        self.deployment.deploy(first,window=0)
        second = self.release(2)
        self.deployment.state["legacy_retire_operation"] = {"release":second}
        self.deployment.calls.clear()
        self.deployment.deploy(second,window=0)
        self.assertNotIn(("pull",second["sha"]),self.deployment.calls)
        self.assertEqual("blue",self.deployment.state["active"])

    def test_prepared_release_survives_post_retirement_restart(self):
        self.deployment.state["slots"]["blue"]["legacy"] = True
        first = self.release(1)
        self.deployment.deploy(first,window=0)
        second = self.release(2)
        old = self.deployment.state.pop("pending")
        del self.deployment.state["slots"][old]
        self.deployment.containers.pop("sub2api-"+old,None)
        self.deployment.state.update(phase="stable",prepared_release=second)
        self.deployment.calls.clear()
        with patch.object(self.deployment,"verify_release_image") as verify_image:
            self.deployment.deploy(second,window=0)
        verify_image.assert_called_once_with(second)
        self.assertNotIn(("pull",second["sha"]),self.deployment.calls)
        self.assertEqual("blue",self.deployment.state["active"])

    def test_next_tag_can_resume_legacy_retirement_before_candidate_exists(self):
        deployment = self.deployment
        deployment.state["slots"]["blue"]["legacy"] = True
        deployment.deploy(self.release(1),window=0)
        interrupted = self.release(2)
        replacement = self.release(3)
        deployment.state.update(phase="legacy_retiring",legacy_retire_operation={"release":interrupted})
        deployment.containers.pop("sub2api-blue",None)
        deployment.calls.clear()
        deployment.deploy(replacement,window=0)
        preflight = deployment.calls.index(("preflight",replacement["sha"]))
        pulled = deployment.calls.index(("pull",replacement["sha"]))
        retired = deployment.calls.index(("legacy_retire",replacement["sha"]))
        self.assertLess(preflight,pulled)
        self.assertLess(pulled,retired)
        self.assertEqual("blue",deployment.state["active"])
        self.assertEqual("stable",deployment.state["phase"])

    def test_failed_replacement_preflight_preserves_original_retirement(self):
        deployment = self.deployment
        deployment.state["slots"]["blue"]["legacy"] = True
        deployment.deploy(self.release(1),window=0)
        interrupted = self.release(2)
        deployment.state.update(phase="legacy_retiring",legacy_retire_operation={"release":interrupted})
        deployment.containers.pop("sub2api-blue",None)
        with patch.object(deployment,"preflight",side_effect=bg.Refused("preflight rejected")):
            with self.assertRaisesRegex(bg.Refused,"preflight rejected"):
                deployment.deploy(self.release(3),window=0)
        self.assertEqual(interrupted,deployment.state["legacy_retire_operation"]["release"])
        self.assertEqual("green",deployment.routed)

    def test_stop_uses_compatible_infinite_timeout_option(self):
        self.deployment.deploy(self.release(1),window=0)
        stops = [call for call in self.deployment.calls if call[:2] == ("docker","stop")]
        self.assertEqual([("docker","stop","-t","-1","sub2api-blue")],stops)

    def test_legacy_proxy_is_fixed_private_and_disables_retries(self):
        self.deployment.state["slots"]["blue"]["legacy"] = True
        rendered = self.deployment.proxy_config("green")
        self.assertIn("server 127.0.0.1:18082",rendered)
        self.assertIn("listen unix:",rendered)
        self.assertIn("sub2api-legacy.access.log",rendered)
        self.assertIn("sub2api-legacy.access.log combined",rendered)
        self.assertIn("proxy_pass http://127.0.0.1:18080",rendered)
        self.assertIn("proxy_next_upstream off",rendered)
        self.assertNotIn("18081",rendered)
        self.assertNotIn("legacy.sock",self.deployment.proxy_config("green",include_legacy=False))

    def test_loopback_probes_trust_existing_origin_certificate(self):
        d = self.deployment
        d.config["probes"] = [{"url":f"https://{host}:{port}/readyz", "resolve":f"{host}:{port}:127.0.0.1"} for host,port in bg.VHOSTS.values()]
        bg.Deployment.probe(d,"old")
        self.assertIn("--cacert",d.calls[-1])
        self.assertNotIn("--insecure",d.calls[-1])
        self.assertIn("--noproxy",d.calls[-1])
        observer = bg.HealthObserver()
        calls = []
        def run(args, **kwargs):
            calls.append(args)
            observer.stop.set()
            return subprocess.CompletedProcess(args,0,stdout="200",stderr="")
        with patch.object(bg.subprocess,"run",side_effect=run):
            observer.observe()
        self.assertEqual(2,len(calls))
        self.assertIn("--cacert",calls[0])
        self.assertNotIn("--cacert",calls[1])
        self.assertTrue(all("--insecure" not in c and "-k" not in c for c in calls))

    def test_budget_parsing_requires_explicit_limit(self):
        self.assertEqual(3*1024**3,bg.bytes_value("3GiB"))
        for value in ("off","80%","-1","",None):
            with self.assertRaises(bg.Refused): bg.bytes_value(value)


if __name__ == "__main__":
    unittest.main()
