"""并发入口、真实同步到达、时延口径和可信参照的本地回归。"""
from contextlib import contextmanager
import copy
import json
from pathlib import Path
import threading
import time
import unittest
from unittest.mock import patch
from http.server import ThreadingHTTPServer, BaseHTTPRequestHandler

from test_probe_token_audit import WorkspaceTest, DEFAULT_MODEL, KEY, event, response_events, send, usage, mock_server
from token_audit.concurrency import compare_baseline, distribution, load_baseline, make_profile
from token_audit.protocol import MessageParser, MessagesClient, canonical_hash
from token_audit.signatures import explicit_signature_error


@contextmanager
def burst_server(callback):
    class Handler(BaseHTTPRequestHandler):
        protocol_version = "HTTP/1.1"

        def log_message(self, *args):
            pass

        def do_POST(self):
            body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            try:
                callback(self, body)
            except (BrokenPipeError, ConnectionResetError, ConnectionAbortedError):
                pass

    class Server(ThreadingHTTPServer):
        request_queue_size = 256
        daemon_threads = True

    server = Server(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, kwargs={"poll_interval": 0.01}, daemon=True)
    thread.start()
    try:
        yield "http://127.0.0.1:" + str(server.server_port)
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=2)


class ConcurrencyTests(WorkspaceTest):
    def run_load(self, url, count, *extra):
        directory = self.folder / "run"
        code, text = self.cli("--url", url, "--key", KEY, "--tests", "concurrency", "--concurrency", str(count),
                              "--timeout", "15", "--output", str(directory), *extra)
        return code, text, json.loads((directory / "summary.json").read_text(encoding="utf-8")), directory

    def test_defaults_and_combined_request_budgets(self):
        code, text = self.cli("--url", "https://invalid.example", "--dry-run")
        self.assertEqual(code, 0)
        self.assertEqual(json.loads(text)["target_requests_max"], 6)
        code, text = self.cli("--url", "https://invalid.example", "--tests", "concurrency", "--dry-run")
        self.assertEqual(code, 0)
        plan = json.loads(text)
        self.assertEqual(plan["target_requests_max"], 200)
        self.assertEqual(plan["reference_requests_max"], 0)
        self.assertEqual(plan["concurrency"]["profile"]["request_template"]["max_tokens"], 2048)
        code, text = self.cli("--url", "https://invalid.example", "--tests",
                              "conversation,fingerprint,signature,concurrency", "--dry-run")
        self.assertEqual(json.loads(text)["target_requests_max"], 306)
        code, text = self.cli("--list-tests")
        self.assertIn("concurrency", text)

    def test_invalid_parameters_rejected_before_network(self):
        for extra in [("--concurrency", "0"), ("--concurrency", "201"), ("--load-max-tokens", "0"),
                      ("--load-origin", ""), ("--load-baseline-file", str(self.folder / "missing.json"))]:
            with self.subTest(extra=extra):
                code, _ = self.cli("--url", "https://invalid.example", "--tests", "concurrency", "--dry-run", *extra)
                self.assertEqual(code, 2)

    def test_all_200_requests_reach_server_before_any_response(self):
        barrier = threading.Barrier(200)
        seen, lock = [], threading.Lock()

        def callback(handler, body):
            with lock:
                seen.append(handler.headers["x-claude-code-session-id"])
            barrier.wait(timeout=10)
            send(handler, response_events(body, usage(100)))

        with burst_server(callback) as url:
            code, text, summary, directory = self.run_load(url, 200, "--timeout", "180")
        metrics = summary["concurrency"]["metrics"]
        self.assertEqual(code, 0, text)
        self.assertEqual(len(set(seen)), 200)
        self.assertEqual(summary["target_requests"], 200)
        self.assertEqual(metrics["successful_requests"], 200)
        self.assertEqual(metrics["peak_sent_in_flight"], 200)
        self.assertEqual(metrics["first_text_seconds_success"]["n"], 200)
        self.assertFalse(summary["input_quantity_evaluated"])
        self.assertEqual(summary["concurrency"]["comparison"]["status"], "historical_reference")
        self.assertIn("p95", summary["concurrency"]["comparison"]["latency_ratios"]["first_text_seconds_success"])
        report = (directory / "report.md").read_text(encoding="utf-8")
        self.assertIn("首字 P95", report)
        self.assertNotIn("总输入 =", report)
        self.assertEqual(len((directory / "concurrency.csv").read_text(encoding="utf-8-sig").splitlines()), 201)
        self.assertEqual(len((directory / "rounds.csv").read_text(encoding="utf-8-sig").splitlines()), 1)

    def test_mixed_failures_preserve_denominator_and_budget_distinction(self):
        seen, lock = [], threading.Lock()

        def callback(handler, body):
            with lock:
                index = len(seen)
                seen.append(body)
            if index in (1, 2):
                send(handler, b'{"error":{"message":"limited or unavailable"}}', status=429 if index == 1 else 503,
                     content_type="application/json")
            elif index == 3:
                send(handler, event({"type": "error", "error": {"type": "api_error", "message": "busy"}}))
            elif index == 4:
                send(handler, response_events(body, usage(100), text="", stop="max_tokens"))
            elif index == 5:
                send(handler, response_events({**body, "model": "other-model"}, usage(100)))
            elif index == 6:
                send(handler, response_events(body, usage(100)).replace(event({"type": "message_stop"}), b""))
            else:
                send(handler, response_events(body, usage(100)))

        with burst_server(callback) as url:
            code, text, summary, _ = self.run_load(url, 7)
        self.assertEqual(code, 1, text)
        metrics = summary["concurrency"]["metrics"]
        self.assertEqual(len(seen), 7)
        self.assertEqual(metrics["successful_requests"], 1)
        self.assertEqual(metrics["protocol_successful_requests"], 3)
        self.assertAlmostEqual(metrics["success_rate"], 1 / 7)
        self.assertEqual(metrics["first_text_seconds_success"]["n"], 1)
        self.assertEqual(metrics["failure_counts"], {"http_429": 1, "http_503": 1, "sse_upstream_error": 1,
                                                   "budget_stopped": 1, "model_mismatch": 1, "incomplete_stream": 1})

    def test_rate_limit_stops_later_test_but_not_burst_accounting(self):
        calls = []

        def callback(handler, body):
            calls.append(body)
            send(handler, b'{"error":{"type":"rate_limit_error"}}', status=429, content_type="application/json")

        with burst_server(callback) as url:
            code, _, summary, _ = self.run_load(url, 3, "--tests", "concurrency,conversation")
        self.assertEqual(code, 3)
        self.assertEqual(len(calls), 3)
        self.assertEqual(summary["target_requests"], 3)
        self.assertEqual(summary["skipped_tests"][0]["test"], "conversation")

    def test_key_echo_redacted_and_csv_formula_escaped(self):
        def callback(handler, body):
            send(handler, response_events(body, usage(100), text=KEY), headers={"x-oneapi-request-id": "=1+1"})

        with burst_server(callback) as url:
            code, text, summary, directory = self.run_load(url, 2)
        self.assertEqual(code, 1, text)
        self.assertEqual(summary["concurrency"]["metrics"]["failure_counts"], {"secret_echo": 2})
        for file in directory.iterdir():
            self.assertNotIn(KEY, file.read_text(encoding="utf-8-sig"))
        self.assertIn("'=1+1", (directory / "concurrency.csv").read_text(encoding="utf-8-sig"))
        self.assertNotIn(KEY, text)

    def test_first_text_excludes_headers_ping_and_thinking(self):
        client_text_seen = threading.Event()
        original_add_text = MessageParser.add_text

        def observe_text(parser, index, text, replace=False):
            original_add_text(parser, index, text, replace)
            if text.strip():
                client_text_seen.set()

        def callback(handler, body):
            handler.send_response(200)
            handler.send_header("Content-Type", "text/event-stream")
            handler.send_header("Connection", "close")
            handler.end_headers()
            handler.close_connection = True
            prefix = [event({"type": "ping"}), event({"type": "message_start", "message": {
                "id": "timed", "model": body["model"], "content": [], "usage": usage(100)}}),
                event({"type": "content_block_delta", "index": 0, "delta": {"type": "thinking_delta", "thinking": "思考"}})]
            handler.wfile.write(b"".join(prefix))
            handler.wfile.flush()
            time.sleep(0.06)
            handler.wfile.write(event({"type": "content_block_delta", "index": 1,
                                      "delta": {"type": "text_delta", "text": "可见"}}))
            handler.wfile.flush()
            if not client_text_seen.wait(timeout=2):
                raise AssertionError("客户端未观察到可见文本")
            time.sleep(0.04)
            handler.wfile.write(event({"type": "message_delta", "delta": {"stop_reason": "end_turn"}})
                                + event({"type": "message_stop"}))
            handler.wfile.flush()

        with mock_server(callback) as url, patch.object(MessageParser, "add_text", observe_text):
            result = MessagesClient(url, KEY, timeout=2).generate({"model": DEFAULT_MODEL}, "timing")
        self.assertTrue(result["success"])
        self.assertLess(result["headers_seconds"], result["first_text_seconds"])
        self.assertGreaterEqual(result["first_text_seconds"], 0.055)
        self.assertGreaterEqual(result["terminal_seconds"] - result["first_text_seconds"], 0.035)

    def test_baseline_is_bound_to_configuration_but_not_a_threshold(self):
        baseline = load_baseline()
        profile = make_profile(DEFAULT_MODEL, 200, 2048, 180)
        self.assertEqual(canonical_hash(profile), baseline["profile_sha256"])
        comparison = compare_baseline(baseline["metrics"], profile, baseline, "另一测试端")
        self.assertEqual(comparison["status"], "historical_reference")
        self.assertFalse(comparison["same_origin_declared"])
        self.assertEqual(comparison["latency_ratios"]["first_text_seconds_success"]["p50"], 1)
        changed = make_profile("other-model", 200, 2048, 180)
        self.assertEqual(compare_baseline(baseline["metrics"], changed, baseline, "服务器1")["status"], "configuration_mismatch")
        for count, budget, timeout, auth in [(100, 2048, 180, "x-api-key"), (200, 4096, 180, "x-api-key"),
                                            (200, 2048, 30, "x-api-key"), (200, 2048, 180, "bearer")]:
            changed = make_profile(DEFAULT_MODEL, count, budget, timeout, auth)
            self.assertEqual(compare_baseline(baseline["metrics"], changed, baseline, "服务器1")["latency_ratios"], {})

    def test_baseline_tampering_and_nonfinite_values_rejected(self):
        original = load_baseline()
        for mutation in ("profile", "counts", "quantile", "nan"):
            data = copy.deepcopy(original)
            if mutation == "profile":
                data["profile"]["requests"] = 100
            elif mutation == "counts":
                data["metrics"]["successful_requests"] = 201
            elif mutation == "quantile":
                data["metrics"]["first_text_seconds_success"]["p95"] = -1
            else:
                data["metrics"]["first_text_seconds_success"]["p50"] = float("nan")
            file = self.folder / (mutation + ".json")
            file.write_text(json.dumps(data), encoding="utf-8")
            with self.assertRaises(ValueError):
                load_baseline(file)

    def test_baseline_metadata_rejected_before_any_request(self):
        for field in ("sampled_at", "origin"):
            data = load_baseline()
            del data[field]
            file = self.folder / (field + ".json")
            file.write_text(json.dumps(data), encoding="utf-8")
            code, _ = self.cli("--url", "https://invalid.example", "--tests", "concurrency",
                               "--load-baseline-file", str(file), "--dry-run")
            self.assertEqual(code, 2)

    def test_stream_timeout_preserves_failure_stage(self):
        def callback(handler, body):
            handler.send_response(200)
            handler.send_header("Content-Type", "text/event-stream")
            handler.send_header("Connection", "close")
            handler.end_headers()
            handler.close_connection = True
            handler.wfile.write(event({"type": "ping"}))
            handler.wfile.flush()
            time.sleep(0.2)

        with mock_server(callback) as url:
            result = MessagesClient(url, KEY, timeout=0.07).generate({"model": DEFAULT_MODEL}, "timeout")
        self.assertTrue(result["timed_out"])
        self.assertEqual(result["stage"], "body")
        self.assertFalse(result["success"])
        self.assertIsNone(result["first_text_seconds"])

    def test_percentiles_and_empty_samples(self):
        self.assertEqual(distribution([1, 2, 3, 4])["p95"], 3.85)
        self.assertEqual(distribution([None])["n"], 0)
        self.assertIsNone(distribution([])["p50"])
        with self.assertRaises(ValueError):
            distribution([float("inf")])

    def test_backtick_signature_error_is_classified(self):
        self.assertTrue(explicit_signature_error({"upstream_error": {
            "type": "invalid_request_error", "message": "Invalid \x60signature\x60 in \x60thinking\x60 block"}}))
        self.assertFalse(explicit_signature_error({"upstream_error": {
            "type": "authentication_error", "message": "invalid signature"}}))


if __name__ == "__main__":
    unittest.main()
