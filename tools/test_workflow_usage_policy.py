"""检查 CI 用量策略，同时保留验证覆盖与发布边界。"""

from pathlib import Path
import unittest

import yaml


WORKFLOWS = Path(__file__).resolve().parents[1] / ".github" / "workflows"
CHECK_WORKFLOWS = ("backend-ci.yml", "security-scan.yml")


def load_workflow(name):
    # BaseLoader 保留 on 键，避免 YAML 1.1 将其解释为布尔值。
    return yaml.load((WORKFLOWS / name).read_text(encoding="utf-8"), Loader=yaml.BaseLoader)


class WorkflowUsagePolicyTests(unittest.TestCase):
    def test_push_only_checks_main(self):
        for name in CHECK_WORKFLOWS:
            with self.subTest(workflow=name):
                self.assertEqual(load_workflow(name)["on"]["push"], {"branches": ["main"]})

    def test_pull_requests_and_manual_checks_remain_unfiltered(self):
        for name in CHECK_WORKFLOWS:
            with self.subTest(workflow=name):
                events = load_workflow(name)["on"]
                self.assertEqual(events["pull_request"], "")
                self.assertEqual(events["workflow_dispatch"], "")

    def test_new_checks_cancel_only_the_same_workflow_and_pr_or_ref(self):
        for name in CHECK_WORKFLOWS:
            with self.subTest(workflow=name):
                self.assertEqual(load_workflow(name).get("concurrency"), {
                    "group": "${{ github.workflow }}-${{ github.event.pull_request.number || github.ref }}",
                    "cancel-in-progress": "true",
                })

    def test_weekly_security_scan_remains_enabled(self):
        self.assertEqual(load_workflow("security-scan.yml")["on"]["schedule"], [
            {"cron": "0 3 * * 1"},
        ])

    def test_existing_check_jobs_and_macos_coverage_remain(self):
        ci = load_workflow("backend-ci.yml")
        self.assertEqual(set(ci["jobs"]), {
            "shell", "test", "stream-race", "frontend", "golangci-lint",
            "lifecycle-race", "bluegreen-protocol", "release-helpers",
        })
        self.assertEqual(ci["jobs"]["shell"]["runs-on"], "macos-15")
        self.assertEqual(set(load_workflow("security-scan.yml")["jobs"]), {
            "backend-security", "frontend-security",
        })

    def test_release_and_production_workflows_keep_their_boundaries(self):
        release = load_workflow("release.yml")
        self.assertEqual(release["on"]["push"], {"tags": ["v*"]})
        self.assertEqual(release["concurrency"]["cancel-in-progress"], "false")
        for name in ("blue-green-preflight.yml", "legacy-retire.yml"):
            with self.subTest(workflow=name):
                workflow = load_workflow(name)
                self.assertEqual(set(workflow["on"]), {"workflow_dispatch"})
                self.assertNotEqual(workflow.get("concurrency", {}).get("cancel-in-progress"), "true")


if __name__ == "__main__":
    unittest.main()
