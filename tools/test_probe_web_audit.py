"""网页鉴权、任务生命周期和凭据隔离回归。"""
import hashlib
import http.client
import json
import os
from pathlib import Path
import secrets
import threading
import time
import unittest
from unittest.mock import patch

from test_probe_token_audit import WorkspaceTest, KEY
from test_probe_freshness_audit import Endpoint, endpoint_server
from token_audit.protocol import check_public_peer
from token_audit_web.server import App, Jobs, normalize, public_url, save_json


class WebTests(WorkspaceTest):
    def setUp(self):
        super().setUp()
        self.jobs = Jobs(self.folder / "runs", public_only=False)
        self.auth = {"salt": "a1" * 16, "iterations": 1000,
                     "hash": hashlib.pbkdf2_hmac("sha256", b"local-password", bytes.fromhex("a1" * 16), 1000).hex()}
        self.server = App(("127.0.0.1", 0), self.jobs, self.auth, "http://127.0.0.1")
        self.origin = "http://127.0.0.1:" + str(self.server.server_port)
        self.server.origin = self.origin
        self.thread = threading.Thread(target=self.server.serve_forever, kwargs={"poll_interval": .01}, daemon=True)
        self.thread.start()
        self.cookie = ""
        self.addCleanup(self.close)

    def close(self):
        with self.jobs.lock:
            for job_id, job in self.jobs.jobs.items():
                if job["state"] in ("queued", "running"):
                    self.jobs.cancel(job_id)
        self.jobs.queue.join()
        self.server.shutdown()
        self.server.server_close()
        self.thread.join()

    def request(self, path, data=None, origin=None, host=None):
        connection = http.client.HTTPConnection("127.0.0.1", self.server.server_port, timeout=10)
        headers = {"Host": host or self.origin.removeprefix("http://"), "Cookie": self.cookie}
        if data is not None:
            headers.update({"Content-Type": "application/json", "Origin": origin or self.origin})
        connection.request("GET" if data is None else "POST", path, json.dumps(data) if data is not None else None, headers)
        response = connection.getresponse()
        raw = response.read()
        cookie = response.getheader("Set-Cookie")
        if cookie:
            self.cookie = cookie.split(";", 1)[0]
        status = response.status
        connection.close()
        try:
            return status, json.loads(raw)
        except ValueError:
            return status, raw

    def login(self):
        self.assertEqual(self.request("/api/login", {"password": "local-password"})[0], 200)

    def test_auth_origin_host_and_traversal(self):
        self.assertEqual(self.request("/api/jobs")[0], 401)
        self.assertEqual(self.request("/api/login", {"password": "wrong"})[0], 401)
        self.assertEqual(self.request("/api/login", {"password": "local-password"}, origin="https://other.example")[0], 403)
        self.assertEqual(self.request("/health", host="other.example")[0], 400)
        self.login()
        self.assertEqual(self.request("/api/me")[0], 200)
        self.assertEqual(self.request("/static/../../server.py")[0], 404)
        self.assertEqual(self.request("/api/logout", {})[0], 200)
        self.assertEqual(self.request("/api/jobs")[0], 401)

    def test_login_rate_limit(self):
        for _ in range(8):
            self.assertEqual(self.request("/api/login", {"password": "wrong"})[0], 401)
        self.assertEqual(self.request("/api/login", {"password": "local-password"})[0], 429)

    def test_job_generation_download_and_secret_redaction(self):
        self.login()
        endpoint = Endpoint()
        with endpoint_server(endpoint) as url:
            config = {"url": url, "key": KEY, "tests": ["signature_freshness"], "freshness_models": "claude-opus-5-5",
                      "freshness_interval": 1, "require_awsb": True, "label": "本地测试"}
            status, plan = self.request("/api/plan", config)
            self.assertEqual(status, 200)
            self.assertEqual(plan["target_requests_max"], 2)
            self.assertEqual(endpoint.requests, [])
            status, job = self.request("/api/jobs", config)
            self.assertEqual(status, 201)
            self.assertNotIn(KEY, json.dumps(job))
            self.jobs.queue.join()
        status, detail = self.request("/api/jobs/" + job["id"])
        self.assertEqual(status, 200)
        self.assertEqual(detail["state"], "finished")
        self.assertEqual(detail["summary"]["signature_freshness"]["status"], "pass")
        self.assertEqual(detail["completed_requests"], 2)
        self.assertNotIn("request", detail["summary"]["rows"][0])
        self.assertEqual(self.request("/api/jobs/" + job["id"] + "/files/report.md")[0], 200)
        for path in self.folder.rglob("*"):
            if path.is_file():
                self.assertNotIn(KEY.encode(), path.read_bytes())
        self.assertEqual(self.request("/api/jobs/" + job["id"] + "/delete", {})[0], 200)
        self.assertFalse((self.jobs.root / job["id"]).exists())

    def test_queued_and_running_cancellation(self):
        self.login()
        with endpoint_server(Endpoint()) as url:
            config = normalize({"url": url, "key": KEY, "tests": ["signature_freshness"], "freshness_models": "claude-opus-5-5", "freshness_interval": 20}, False)
            first = self.jobs.add(config)
            deadline = time.monotonic() + 5
            while self.jobs.jobs[first["id"]]["state"] == "queued" and time.monotonic() < deadline:
                time.sleep(.02)
            second = self.jobs.add(config)
            self.jobs.cancel(second["id"])
            self.jobs.cancel(first["id"])
            self.jobs.queue.join()
        self.assertEqual(self.jobs.jobs[first["id"]]["state"], "cancelled")
        self.assertEqual(self.jobs.jobs[second["id"]]["state"], "cancelled")
        self.assertEqual(self.jobs.pending, {})

    def test_restart_marks_unfinished_without_replay(self):
        job_id = secrets.token_hex(12)
        root = self.folder / "restore"
        (root / job_id).mkdir(parents=True)
        save_json(root / job_id / "job.json", {"id": job_id, "state": "running"})
        restarted = Jobs(root, False)
        self.assertEqual(restarted.jobs[job_id]["state"], "interrupted")
        self.assertEqual(restarted.pending, {})

    def test_private_addresses_and_actual_peer_are_rejected(self):
        for url in ("http://127.0.0.1", "http://169.254.169.254", "http://[::1]"):
            with self.assertRaises(ValueError):
                public_url(url)
        class Socket:
            def getpeername(self):
                return ("127.0.0.1", 80)
        class Connection:
            sock = Socket()
        with patch.dict(os.environ, {"TOKEN_AUDIT_PUBLIC_ONLY": "1"}), self.assertRaises(ValueError):
            check_public_peer(Connection())


if __name__ == "__main__":
    unittest.main()
