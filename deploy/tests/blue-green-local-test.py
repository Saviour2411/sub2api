#!/usr/bin/env python3
"""验证本地验收不会绕过镜像身份、迁移及原CI门禁。"""
import copy
import importlib.util
from pathlib import Path
import unittest
import sys
sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[2]
def module(name, path):
    spec = importlib.util.spec_from_file_location(name, ROOT/path)
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value
e = module("local_evidence", "deploy/blue-green/evidence.py")
bg = module("local_deploy", "deploy/blue-green/deploy.py")
class LocalEvidenceTests(unittest.TestCase):
    def release(self):
        sha = "a"*40
        report = {"version":1,"sha":sha,"checks":{name:{"exit_code":0,"command":["test",name],"log_sha256":"b"*64} for name in e.LOCAL_CHECKS}}
        return {"sha":sha,"image":"sha256:"+"c"*64,"validation_mode":"local","local_validation":report,"ci":"not_run","security":"not_run"}
    def test_local_checks_and_original_ci_gate(self):
        release = self.release()
        bg.validate_release_checks(release)
        e.checked_local_report(release["local_validation"],release["sha"])
        for field, value in (("image","mutable:latest"),("ci","success"),("validation_mode","unknown")):
            bad = dict(release, **{field:value})
            with self.assertRaises(RuntimeError): bg.validate_release_checks(bad)
        for change in ("missing","failed","sha"):
            bad = copy.deepcopy(release)
            if change == "missing": del bad["local_validation"]["checks"]["database"]
            if change == "failed": bad["local_validation"]["checks"]["race"]["exit_code"] = 1
            if change == "sha": bad["local_validation"]["sha"] = "d"*40
            with self.assertRaises(RuntimeError): bg.validate_release_checks(bad)
            with self.assertRaises(RuntimeError): e.checked_local_report(bad["local_validation"],bad["sha"])
        ci = {"sha":"a"*40,"image":"repo/image@sha256:"+"b"*64,"ci_sha":"a"*40,"ci":"success","security":"success"}
        bg.validate_release_checks(ci)
        ci["security"]="not_run"
        with self.assertRaises(RuntimeError): bg.validate_release_checks(ci)
    def test_local_image_never_pulls_and_verifies_id(self):
        deployment = object.__new__(bg.Deployment)
        release = self.release()
        calls=[]
        import json
        import subprocess
        image={"Id":release["image"],"Config":{"Labels":{"org.opencontainers.image.revision":release["sha"]}}}
        def run(*args):
            calls.append(args)
            return subprocess.CompletedProcess(args,0,json.dumps([image]),"")
        deployment.run=run
        deployment.pull_release_image(release)
        self.assertFalse(any(call[:2] == ("docker","pull") for call in calls))
        image["Id"]="sha256:"+"d"*64
        with self.assertRaises(RuntimeError): deployment.verify_release_image(release)
    def test_quality_v2_fixed_baseline(self):
        import json
        approval=next(a for a in json.loads((ROOT/e.APPROVALS_PATH).read_text(encoding="utf-8"))["approvals"] if a.get("profile")==e.QUALITY_V2_PROFILE)
        path=approval["path"];sha="b"*40
        for contract in approval["legacy_contracts"]: contract["checksum"]=e.checksum("旧契约")
        changed="A\t"+path
        def git(*args):
            if args[0]=="diff":return changed
            if args[0]=="show" and args[1]==sha+":"+e.APPROVALS_PATH:return json.dumps({"version":1,"approvals":[approval]})
            if args[0]=="show" and args[1]==sha+":"+path:return (ROOT/path).read_text(encoding="utf-8")
            return "旧契约"
        self.assertEqual("expand",e.migration_evidence(e.QUALITY_V2_BASE,sha,git)["migration_class"])
        with self.assertRaises(RuntimeError):e.migration_evidence("c"*40,sha,git)
        changed += "\nA\tbackend/migrations/999_extra.sql"
        with self.assertRaises(RuntimeError):e.migration_evidence(e.QUALITY_V2_BASE,sha,git)
if __name__ == "__main__": unittest.main()
