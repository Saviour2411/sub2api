#!/usr/bin/env python3
"""不访问网络，验证发布证据不能使用错误提交、跳过门禁或未知迁移。"""
import importlib.util
from pathlib import Path
import sys
import unittest

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location("evidence", ROOT/"deploy/blue-green/evidence.py")
e = importlib.util.module_from_spec(spec)
spec.loader.exec_module(e)


class EvidenceTests(unittest.TestCase):
    def run_record(self, **changes):
        return dict({"id": 1, "head_sha": "a"*40, "event": "push", "status": "completed", "conclusion": "success", "html_url": "test", "run_attempt": 2}, **changes)

    def test_exact_sha_complete_jobs_and_attempt(self):
        run = self.run_record()
        jobs = lambda _: [{"name": "test", "conclusion": "success"}]
        result = e.checked_run([run, self.run_record(id=2, status="in_progress", conclusion=None)], "a"*40, {"test"}, jobs)
        self.assertEqual(1, result["run_id"])
        self.assertEqual(2, result["attempt"])
        for change in ({"head_sha":"b"*40}, {"conclusion":"failure"}, {"event":"pull_request"}):
            with self.assertRaises(RuntimeError):
                e.checked_run([self.run_record(**change)], "a"*40, {"test"}, jobs)
        for conclusion in ("skipped", "cancelled", "failure"):
            with self.assertRaises(RuntimeError):
                e.checked_run([run], "a"*40, {"test"}, lambda _: [{"name":"test", "conclusion":conclusion}])
        with self.assertRaises(RuntimeError):
            e.checked_run([run], "a"*40, {"test", "race"}, jobs)

    def test_migration_changes_fail_closed(self):
        base, sha = "a"*40, "b"*40
        self.assertEqual("none", e.migration_evidence(base, sha, lambda *args: "")["migration_class"])
        with self.assertRaisesRegex(RuntimeError, "迁移"):
            e.migration_evidence(base, sha, lambda *args: "schema.sql" if args[0] == "diff" else "")


if __name__ == "__main__":
    unittest.main()
