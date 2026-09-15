"""只读诊断输出不得携带环境凭据或任意容器配置。"""
import importlib.util
import json
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('preflight', Path(__file__).resolve().parents[1]/'blue-green/preflight.py')
preflight = importlib.util.module_from_spec(spec)
spec.loader.exec_module(preflight)


class PreflightTests(unittest.TestCase):
    def test_container_report_uses_allowlist(self):
        data = {'Name': '/sub2api', 'State': {'Status': 'running', 'Error': 'secret'},
                'Image': 'sha256:abc', 'Config': {'Env': ['JWT_SECRET=do-not-publish',
                'DATABASE_PASSWORD=do-not-publish', 'GOMEMLIMIT=1GiB', 'DATABASE_MAX_OPEN_CONNS=192'],
                'Labels': {'secret': 'do-not-publish', 'org.opencontainers.image.revision': 'abc'},
                'Cmd': ['do-not-publish']}, 'HostConfig': {'Memory': 0},
                'NetworkSettings': {'Ports': {}}, 'Mounts': []}
        result = preflight.container_summary(data)
        self.assertEqual(result['environment_budget'], {'GOMEMLIMIT': '1GiB', 'DATABASE_MAX_OPEN_CONNS': '192'})
        self.assertNotIn('do-not-publish', json.dumps(result))
        self.assertNotIn('secret', json.dumps(result))

    def test_missing_budget_is_unknown_not_assumed(self):
        data = {'Name': '/sub2api', 'State': {'Status': 'running'}, 'Image': 'x',
                'Config': {}, 'HostConfig': {}, 'NetworkSettings': {}}
        result = preflight.container_summary(data)
        self.assertEqual(result['environment_budget'], {'GOMEMLIMIT': None, 'DATABASE_MAX_OPEN_CONNS': None})


if __name__ == '__main__':
    unittest.main()
