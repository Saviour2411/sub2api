#!/usr/bin/env python3
"""不访问网络，验证发布证据不能使用错误提交、跳过门禁或未知迁移。"""
import importlib.util
import copy
import json
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

    def expansion_fixture(self):
        base, sha = "a"*40, "b"*40
        path = "backend/migrations/999_test.sql"
        content = "-- 审核来源\nALTER TABLE logs ADD COLUMN IF NOT EXISTS meta JSONB;"
        source = "backend/internal/repository/logs.go"
        approval = {"compatible_from":base, "path":path, "checksum":e.checksum(content),
                    "column":{"table":"logs", "column":"meta", "data_type":"jsonb"},
                    "legacy_contracts":[{"path":source, "checksum":e.checksum("旧版明确列名读写")}]}
        values = {"diff":"A\t"+path, sha+":"+path:content,
                  sha+":"+e.APPROVALS_PATH:json.dumps({"version":1,"approvals":[approval]}),
                  base+":"+source:"旧版明确列名读写"}
        def git(*args):
            if args[0] == "diff":
                return values["diff"]
            if args[0] == "show":
                return values[args[1]]
            return ""
        return base, sha, path, approval, values, git

    def test_reviewed_nullable_expansion_is_bound_to_runtime_policy(self):
        base, sha, path, _, values, git = self.expansion_fixture()
        values["diff"] += "\nA\tbackend/migrations/999_test_test.go"
        result = e.migration_evidence(base, sha, git)
        self.assertEqual("expand", result["migration_class"])
        self.assertTrue(result["rollback_compatible"])
        self.assertEqual(base, result["compatible_from"])
        entry = result["migration_policy"]["migrations"][0]
        self.assertEqual("999_test.sql", entry["filename"])
        self.assertEqual(e.checksum(values[sha+":"+path]), entry["checksum"])

    def test_approval_cannot_cover_other_baselines_files_or_old_code(self):
        base, sha, path, original, values, git = self.expansion_fixture()
        for field, value in (("compatible_from","c"*40), ("checksum","0"*64),
                             ("column",{"table":"other"}), ("legacy_contracts",[])):
            with self.subTest(field=field):
                approval = dict(original, **{field:value})
                values[sha+":"+e.APPROVALS_PATH] = json.dumps({"version":1,"approvals":[approval]})
                with self.assertRaises(RuntimeError):
                    e.migration_evidence(base, sha, git)
        approval = copy.deepcopy(original)
        approval["legacy_contracts"][0]["checksum"] = "0"*64
        values[sha+":"+e.APPROVALS_PATH] = json.dumps({"version":1,"approvals":[approval]})
        with self.assertRaisesRegex(RuntimeError, "旧版读写"):
            e.migration_evidence(base, sha, git)

    def test_rejects_history_schema_deletion_and_ambiguous_sql(self):
        base, sha, path, _, values, git = self.expansion_fixture()
        for diff in ("M\t"+path, "D\t"+path, "A\tbackend/ent/schema/log.go", "A\tbackend/migrations/migrations.go"):
            values["diff"] = diff
            with self.subTest(diff=diff), self.assertRaises(RuntimeError):
                e.migration_evidence(base, sha, git)
        for sql in ("ALTER TABLE logs DROP COLUMN meta;", "ALTER TABLE logs ADD COLUMN IF NOT EXISTS meta JSONB NOT NULL;",
                    "ALTER TABLE logs ADD COLUMN IF NOT EXISTS meta JSONB DEFAULT NULL;",
                    "ALTER TABLE logs ADD COLUMN IF NOT EXISTS meta TEXT REFERENCES other(id);",
                    "ALTER TABLE logs ADD COLUMN IF NOT EXISTS meta JSONB; DELETE FROM logs;",
                    "ALTER TABLE logs ADD COLUMN IF NOT EXISTS meta custom_domain;",
                    "ALTER TABLE logs ADD COLUMN IF NOT EXISTS meta JSONB /* comment */;",
                    "ALTER TABLE logs ADD COLUMN IF NOT EXISTS meta JSONB, ADD COLUMN other TEXT;",
                    "ALTER TABLE public.logs ADD COLUMN IF NOT EXISTS meta JSONB;"):
            with self.subTest(sql=sql), self.assertRaises(RuntimeError):
                e.additive_column(sql)

    def test_repository_approval_matches_immutable_sql(self):
        approvals = json.loads((ROOT/e.APPROVALS_PATH).read_text())
        for entry in approvals["approvals"]:
            content = (ROOT/entry["path"]).read_text()
            self.assertEqual(entry["checksum"],e.checksum(content))
            if "profile" in entry:
                self.assertEqual((entry["profile"], entry["checksum"]), e.SPECIAL_MIGRATIONS[Path(entry["path"]).name])
            else:
                self.assertEqual(entry["column"],e.additive_column(content))

    def test_special_profiles_require_exact_baseline_code_and_content(self):
        approvals = json.loads((ROOT/e.APPROVALS_PATH).read_text())
        for approval in approvals["approvals"]:
            if "profile" not in approval:
                continue
            for contract in approval["legacy_contracts"]:
                contract["checksum"] = e.checksum("旧版契约")
            base, sha = approval["compatible_from"], "b"*40
            path = approval["path"]
            content = (ROOT/path).read_text()
            def git(*args):
                if args[0] == "diff": return "A\t"+path
                if args[0] == "show" and args[1] == sha+":"+path: return content
                if args[0] == "show" and args[1] == sha+":"+e.APPROVALS_PATH: return json.dumps(approvals)
                return "旧版契约" if args[0] == "show" else ""
            result = e.migration_evidence(base, sha, git)
            self.assertEqual(approval["profile"],result["migration_policy"]["migrations"][0]["profile"])
            content += "\n-- 未经批准的变化"
            with self.assertRaisesRegex(RuntimeError, "固定 SQL"):
                e.migration_evidence(base, sha, git)


if __name__ == "__main__":
    unittest.main()
