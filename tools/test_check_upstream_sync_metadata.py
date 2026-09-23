"""上游来源校验的回归测试，不依赖网络或真实仓库写入。"""

import subprocess
import unittest
from unittest.mock import Mock

from check_upstream_sync_metadata import validate_metadata


class UpstreamMetadataTests(unittest.TestCase):
    def setUp(self):
        self.data = {
            "repository": "Wei-Shaw/sub2api", "version": "0.2.7",
            "commit": "a" * 40, "synced_at": "2026-09-20T23:09:09+08:00",
        }
        self.git = Mock(side_effect=["", "0.2.7\n"])

    def test_valid_source(self):
        validate_metadata(self.data, self.git)
        self.assertEqual(self.git.call_count, 2)

    def test_invalid_fields(self):
        for field, value in [("repository", "other/repo"), ("version", "v0.2.7"), ("version", "0.2.7-rc.1"), ("version", "0.2"), ("commit", "abcdef"), ("synced_at", "2026-09-20"), ("synced_at", "2026-09-20 23:09:09+08:00"), ("synced_at", None), ("synced_at", 123)]:
            with self.subTest(field=field, value=value):
                with self.assertRaises(ValueError):
                    validate_metadata({**self.data, field: value}, Mock())

    def test_invalid_document(self):
        for value in [None, [], "invalid"]:
            with self.assertRaises(ValueError):
                validate_metadata(value, Mock())

    def test_version_mismatch(self):
        self.git.side_effect = ["", "0.1.244"]
        with self.assertRaises(ValueError):
            validate_metadata(self.data, self.git)

    def test_commit_not_integrated(self):
        self.git.side_effect = subprocess.CalledProcessError(1, "git")
        with self.assertRaises(subprocess.CalledProcessError):
            validate_metadata(self.data, self.git)


if __name__ == "__main__":
    unittest.main()
