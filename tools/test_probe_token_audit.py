"""使用历史指纹和本地HTTP模拟端验证诊断边界，不调用付费上游。"""
from __future__ import annotations

import copy
from contextlib import contextmanager, redirect_stdout, redirect_stderr
import io
import json
from pathlib import Path
import tempfile
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from unittest.mock import patch

from probe_token_audit import main
from token_audit.fixtures import DEFAULT_MODEL, fingerprint_from_conversation, load_fingerprint
from token_audit.protocol import CLIENT_HEADERS, MessageParser, MessagesClient, ProtocolError, SSEDecoder, canonical_hash, endpoint_url, input_metrics
from token_audit.runner import check_carry, compare_tokens

ROOT = Path(__file__).resolve().parents[1]
FIXTURE = load_fingerprint()
KEY = "test-key-must-never-be-written"


def usage(total, read=0):
    return {"input_tokens": 4, "cache_read_input_tokens": read,
            "cache_creation_input_tokens": total - read - 4, "output_tokens": 12,
            "cache_creation": {"ephemeral_5m_input_tokens": total - read - 4, "ephemeral_1h_input_tokens": 0}}


def event(payload, name=None, ending="\n\n"):
    return (("event: " + (name or payload["type"]) + "\n") + "data: " + json.dumps(payload, ensure_ascii=False) + ending).encode("utf-8")


def response_events(body, measured, text="这是实际回复", stop="end_turn"):
    return b"".join([
        event({"type": "message_start", "message": {"id": "msg-test", "model": body["model"], "content": [], "usage": measured}}),
        event({"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}}),
        event({"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": text}}),
        event({"type": "message_delta", "delta": {"stop_reason": stop}, "usage": {"output_tokens": 12}}),
        event({"type": "message_stop"}),
    ])


@contextmanager
def mock_server(callback):
    class Handler(BaseHTTPRequestHandler):
        protocol_version = "HTTP/1.1"

        def do_POST(self):
            body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            try:
                callback(self, body)
            except (BrokenPipeError, ConnectionResetError, ConnectionAbortedError):
                pass

        def log_message(self, *_):
            pass
    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    server.daemon_threads = True
    thread = threading.Thread(target=server.serve_forever, kwargs={"poll_interval": 0.01}, daemon=True)
    thread.start()
    try:
        yield "http://127.0.0.1:" + str(server.server_port)
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=2)


def send(handler, raw, status=200, content_type="text/event-stream", headers=None):
    handler.send_response(status)
    handler.send_header("Content-Type", content_type)
    handler.send_header("Content-Length", str(len(raw)))
    handler.send_header("x-oneapi-request-id", "test-upstream-id")
    for key, value in (headers or {}).items():
        handler.send_header(key, value)
    handler.end_headers()
    handler.wfile.write(raw)
    handler.wfile.flush()


class WorkspaceTest(unittest.TestCase):
    def setUp(self):
        (ROOT / "tmp").mkdir(exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(dir=ROOT / "tmp", prefix="token-audit-test-")
        self.addCleanup(self.temp.cleanup)
        self.folder = Path(self.temp.name)

    def cli(self, *args):
        out = io.StringIO()
        with redirect_stdout(out), redirect_stderr(out):
            code = main(list(args))
        return code, out.getvalue()


class FingerprintTests(WorkspaceTest):
    def test_historical_fingerprint_is_complete_and_exact(self):
        self.assertEqual([input_metrics(c["expected_usage"])["total_input_tokens"] for c in FIXTURE["cases"]], [18049, 19926, 21816])
        self.assertEqual([c["request"]["model"] for c in FIXTURE["cases"]], [DEFAULT_MODEL] * 3)
        for case in FIXTURE["cases"]:
            self.assertEqual(canonical_hash(case["request"]), case["request_sha256"])
        for index in (1, 2):
            previous = FIXTURE["cases"][index - 1]["expected_usage"]
            current = FIXTURE["cases"][index]["expected_usage"]
            self.assertEqual(previous["cache_read_input_tokens"] + previous["cache_creation_input_tokens"], current["cache_read_input_tokens"])

    def test_modified_prompt_or_model_rejects_fingerprint(self):
        for change in ("text", "model", "usage"):
            data = copy.deepcopy(FIXTURE)
            if change == "text":
                data["cases"][0]["request"]["messages"][0]["content"][0]["text"] += "changed"
            elif change == "model":
                data["model"] = "other-model"
            else:
                del data["cases"][0]["expected_usage"]["input_tokens"]
            target = self.folder / (change + ".json")
            target.write_text(json.dumps(data), encoding="utf-8")
            with self.subTest(change=change), self.assertRaises(ValueError):
                load_fingerprint(target)

    def test_unknown_model_dry_run_does_not_apply_default_standard(self):
        code, text = self.cli("--url", "https://example.com/v1", "--model", "other-model", "--dry-run")
        self.assertEqual(code, 0)
        plan = json.loads(text)
        self.assertIsNone(plan["fingerprint_id"])
        self.assertIsNone(plan["fingerprint_expected_totals"])
        self.assertEqual(plan["target_requests_max"], 3)

    def test_bad_options_do_not_make_network_calls(self):
        for args in (("--tests", "missing"), ("--timeout", "0"), ("--url", "https://example.com/?key=secret"), ("--tests", "conversation,conversation")):
            with patch("token_audit.protocol.MessagesClient.generate") as generate:
                code, _ = self.cli("--url", "https://example.com", "--key", KEY, *args)
                self.assertEqual(code, 2)
                generate.assert_not_called()

    def test_output_and_fingerprint_never_overwrite(self):
        target = self.folder / "existing.json"
        target.write_text("original", encoding="utf-8")
        code, _ = self.cli("--url", "https://example.com", "--key", KEY, "--save-fingerprint", str(target))
        self.assertEqual(code, 2)
        self.assertEqual(target.read_text(), "original")
        code, _ = self.cli("--url", "https://example.com", "--key", KEY, "--output", str(self.folder))
        self.assertEqual(code, 2)


class ProtocolTests(unittest.TestCase):
    def parse(self, raw, bytewise=False):
        parser = MessageParser()
        decoder = SSEDecoder(parser)
        for chunk in ([raw[i:i + 1] for i in range(len(raw))] if bytewise else [raw]):
            decoder.feed(chunk)
        decoder.finish()
        return parser.result()

    def test_utf8_crlf_and_final_cumulative_usage(self):
        body = {"model": DEFAULT_MODEL}
        raw = response_events(body, usage(1000), text="真实回答")
        raw = raw.replace(b'"usage": {"output_tokens": 12}', b'"usage": {"input_tokens": 7, "output_tokens": 12}')
        result = self.parse(raw.replace(b"\n", b"\r\n"), bytewise=True)
        self.assertEqual(result["text"], "真实回答")
        self.assertEqual(result["usage"]["input_tokens"], 7)
        self.assertEqual(result["usage"]["cache_creation_input_tokens"], 996)
        self.assertEqual(result["usage_start"]["input_tokens"], 4)

    def test_multiline_data_comments_and_type_fallback(self):
        parser = MessageParser()
        decoder = SSEDecoder(parser)
        start = {"type": "message_start", "message": {"model": DEFAULT_MODEL, "content": [], "usage": usage(1000)}}
        decoder.feed(b": comment\n\n" + event(start))
        decoder.feed(b'event: message_delta\ndata: {\ndata: "delta":{"stop_reason":"end_turn"}\ndata: }\n\n')
        decoder.feed(b'event: message_stop\ndata: {}\n\n')
        decoder.finish()
        self.assertTrue(parser.terminal_complete)

    def test_incomplete_terminal_frame_is_not_success(self):
        raw = response_events({"model": DEFAULT_MODEL}, usage(1000)).rstrip(b"\n")
        with self.assertRaisesRegex(ProtocolError, "分隔符"):
            self.parse(raw)

    def test_missing_start_stop_and_error_after_text_are_not_success(self):
        full = response_events({"model": DEFAULT_MODEL}, usage(1000))
        cases = [event({"type": "message_stop"}), full.rsplit(b"event: message_stop", 1)[0],
                 full.rsplit(b"event: message_stop", 1)[0] + event({"type": "error", "error": {"message": "failed"}}),
                 full.rsplit(b"event: message_stop", 1)[0] + b"data: [DONE]\n\n"]
        for raw in cases:
            with self.subTest(raw=raw[-100:]), self.assertRaises(ProtocolError):
                self.parse(raw)

    def test_usage_missing_and_negative_are_not_zero(self):
        for value in ({"input_tokens": 4}, {"input_tokens": -1, "cache_read_input_tokens": 0, "cache_creation_input_tokens": 0}, {"input_tokens": True, "cache_read_input_tokens": 0, "cache_creation_input_tokens": 0}):
            self.assertIsNone(input_metrics(value)["total_input_tokens"])
        self.assertEqual(input_metrics({"input_tokens": 0, "cache_read_input_tokens": 0, "cache_creation_input_tokens": 0})["total_input_tokens"], 0)

    def test_endpoint_accepts_root_v1_and_complete_path(self):
        for url in ("https://example.com", "https://example.com/v1/", "https://example.com/v1/messages/"):
            self.assertEqual(endpoint_url(url), "https://example.com/v1/messages")
        self.assertEqual(endpoint_url("https://example.com/proxy/v1"), "https://example.com/proxy/v1/messages")
        for url in ("https://user:secret@example.com", "file:///tmp/a", "https://example.com/#secret"):
            with self.assertRaises(ValueError):
                endpoint_url(url)

    def test_redirect_is_not_followed_and_json_is_supported(self):
        calls = []
        def callback(handler, body):
            calls.append(handler.path)
            if handler.path.startswith("/redirect"):
                send(handler, b"moved", status=302, headers={"Location": "/other"})
            else:
                send(handler, json.dumps({"type": "message", "model": body["model"], "id": "json-test", "content": [{"type": "text", "text": "OK"}], "usage": usage(1000), "stop_reason": "end_turn"}).encode(), content_type="application/json")
        with mock_server(callback) as url:
            result = MessagesClient(url + "/redirect", KEY).generate({"model": DEFAULT_MODEL}, "session")
            self.assertFalse(result["success"])
            self.assertEqual(result["http_status"], 302)
            result = MessagesClient(url, KEY).generate({"model": DEFAULT_MODEL}, "session")
            self.assertTrue(result["success"])
            self.assertEqual(result["total_input_tokens"], 1000)
        self.assertEqual(len(calls), 2)

    def test_terminal_closes_without_http_eof_and_timeout_keeps_partial_usage(self):
        release = threading.Event()
        def callback(handler, body):
            handler.send_response(200)
            handler.send_header("Content-Type", "text/event-stream")
            handler.end_headers()
            if handler.path.startswith("/hang"):
                raw = event({"type": "message_start", "message": {"model": body["model"], "usage": usage(1000)}})
            else:
                raw = response_events(body, usage(1000))
            handler.wfile.write(raw)
            handler.wfile.flush()
            release.wait(2)
            handler.close_connection = True
        try:
            with mock_server(callback) as url:
                complete = MessagesClient(url, KEY, timeout=0.5).generate({"model": DEFAULT_MODEL}, "session")
                self.assertTrue(complete["success"])
                self.assertLess(complete["elapsed_seconds"], 0.5)
                partial = MessagesClient(url + "/hang", KEY, timeout=0.15).generate({"model": DEFAULT_MODEL}, "session")
                self.assertFalse(partial["success"])
                self.assertIn("超时", partial["error"])
                self.assertEqual(partial["total_input_tokens"], 1000)
        finally:
            release.set()


class WorkflowTests(WorkspaceTest):
    def service(self, calls, factor=1):
        totals = {case["request_sha256"]: case["expected_usage"] for case in FIXTURE["cases"]}
        cache = {}
        def callback(handler, body):
            calls.append((handler.path, copy.deepcopy(body), dict(handler.headers)))
            digest = canonical_hash(body)
            if digest in totals:
                measured = copy.deepcopy(totals[digest])
                if factor != 1:
                    for key in ("input_tokens", "cache_read_input_tokens", "cache_creation_input_tokens"):
                        measured[key] = round(measured[key] * factor)
                    measured["cache_creation"]["ephemeral_5m_input_tokens"] = measured["cache_creation_input_tokens"]
            else:
                total = 100 + sum(len(block["text"]) for message in body["messages"] for block in message["content"]) // 4
                key = (handler.path, handler.headers["x-claude-code-session-id"])
                measured = usage(total, min(cache.get(key, 0), total - 4))
                cache[key] = total - 4
            send(handler, response_events(body, measured, text="真实回复第" + str((len(body["messages"]) + 1) // 2) + "轮"))
        return callback

    def test_default_six_requests_real_history_and_builtin_exact_replay(self):
        calls = []
        output = self.folder / "result"
        with mock_server(self.service(calls)) as url:
            code, console = self.cli("--url", url, "--key", KEY, "--output", str(output))
        self.assertEqual(code, 0, console)
        self.assertEqual(len(calls), 6)
        summary = json.loads((output / "summary.json").read_text(encoding="utf-8"))
        self.assertEqual(summary["compared_requests"], 3)
        self.assertEqual(summary["rows"][1]["cache_check"]["delta"], 0)
        self.assertEqual(calls[1][1]["messages"][1]["content"][0]["text"], "真实回复第1轮")
        self.assertEqual(calls[2][1]["messages"][3]["content"][0]["text"], "真实回复第2轮")
        self.assertIn("Independent audit session:", calls[0][1]["messages"][0]["content"][0]["text"])
        for index in range(3):
            self.assertEqual(calls[index + 3][1], FIXTURE["cases"][index]["request"])
            self.assertIsNone(summary["rows"][index + 3]["cache_check"])
        for path in output.iterdir():
            self.assertNotIn(KEY, path.read_text(encoding="utf-8-sig"))
        self.assertNotIn(KEY, console)

    def test_higher_input_does_not_hide_behind_correct_cache(self):
        calls = []
        output = self.folder / "higher"
        with mock_server(self.service(calls, factor=1.5)) as url:
            code, _ = self.cli("--url", url, "--key", KEY, "--output", str(output))
        summary = json.loads((output / "summary.json").read_text(encoding="utf-8"))
        self.assertEqual(code, 1)
        self.assertEqual([r["comparison"]["status"] for r in summary["rows"][3:]], ["higher"] * 3)
        self.assertEqual(summary["rows"][1]["cache_check"]["delta"], 0)

    def test_live_reference_runs_after_all_targets_with_identical_requests(self):
        calls = []
        output = self.folder / "paired"
        with mock_server(self.service(calls)) as url:
            code, console = self.cli("--url", url + "/target", "--key", KEY, "--reference-url", url + "/reference", "--reference-key", "different-reference-key", "--output", str(output))
        self.assertEqual(code, 0, console)
        self.assertEqual(len(calls), 9)
        self.assertTrue(all(c[0].startswith("/target/") for c in calls[:6]))
        for index in range(3):
            self.assertEqual(calls[index][1], calls[6 + index][1])
        self.assertEqual(json.loads((output / "summary.json").read_text(encoding="utf-8"))["compared_requests"], 6)

    def test_failed_auth_stops_new_calls_and_error_echo_is_redacted(self):
        calls = []
        def callback(handler, body):
            calls.append(body)
            send(handler, json.dumps({"error": KEY}).encode(), status=401, content_type="application/json")
        output = self.folder / "denied"
        with mock_server(callback) as url:
            code, text = self.cli("--url", url, "--key", KEY, "--output", str(output))
        self.assertEqual(code, 3)
        self.assertEqual(len(calls), 1)
        self.assertNotIn(KEY, text)
        for path in output.iterdir():
            self.assertNotIn(KEY, path.read_text(encoding="utf-8-sig"))

    def test_refusal_does_not_fabricate_next_turn(self):
        calls = []
        def callback(handler, body):
            calls.append(body)
            send(handler, response_events(body, usage(1000), text="", stop="refusal"))
        output = self.folder / "refusal"
        with mock_server(callback) as url:
            code, _ = self.cli("--url", url, "--key", KEY, "--tests", "conversation", "--output", str(output))
        self.assertEqual(code, 3)
        self.assertEqual(len(calls), 1)

    def test_echoed_key_is_not_forwarded_to_reference_or_next_round(self):
        calls = []
        def callback(handler, body):
            calls.append(handler.path)
            send(handler, response_events(body, usage(1000), text=KEY))
        output = self.folder / "echoed"
        with mock_server(callback) as url:
            code, console = self.cli("--url", url + "/target", "--key", KEY, "--tests", "conversation",
                                     "--reference-url", url + "/reference", "--reference-key", "reference-key", "--output", str(output))
        self.assertEqual(code, 3)
        self.assertEqual(calls, ["/target/v1/messages"])
        self.assertNotIn(KEY, console)
        for path in output.iterdir():
            self.assertNotIn(KEY, path.read_text(encoding="utf-8-sig"))

    def test_model_mismatch_invalidates_reference_comparison(self):
        result = {"success": True, "total_input_tokens": 1000, "response_model": "another-model"}
        comparison = compare_tokens(result, 1000, DEFAULT_MODEL, 32, 0.005, "test")
        self.assertEqual(comparison["status"], "unavailable")
        self.assertIsNone(comparison["ratio"])

    def test_other_model_can_export_then_reuse_its_own_fingerprint(self):
        calls = []
        captured = self.folder / "trusted.json"
        output = self.folder / "capture"
        with mock_server(self.service(calls)) as url:
            code, console = self.cli("--url", url, "--key", KEY, "--model", "custom-claude-model", "--tests", "conversation", "--output", str(output), "--save-fingerprint", str(captured))
            self.assertEqual(code, 0, console)
            self.assertFalse(json.loads((output / "summary.json").read_text(encoding="utf-8"))["input_quantity_evaluated"])
            loaded = load_fingerprint(captured)
            self.assertEqual(loaded["model"], "custom-claude-model")
            second = self.folder / "replayed"
            code, console = self.cli("--url", url, "--key", KEY, "--tests", "fingerprint", "--fingerprint-file", str(captured), "--output", str(second))
            self.assertEqual(code, 0, console)
        self.assertEqual(len(calls), 6)

    def test_unknown_model_default_run_is_incomplete_instead_of_passed(self):
        calls = []
        output = self.folder / "unknown"
        with mock_server(self.service(calls)) as url:
            code, _ = self.cli("--url", url, "--key", KEY, "--model", "no-fingerprint-model", "--output", str(output))
        self.assertEqual(code, 3)
        self.assertEqual(len(calls), 3)
        summary = json.loads((output / "summary.json").read_text(encoding="utf-8"))
        self.assertFalse(summary["input_quantity_evaluated"])
        self.assertEqual(summary["skipped_tests"][0]["test"], "fingerprint")

    def test_ttl_change_and_missing_usage_are_visible(self):
        def callback(handler, body):
            measured = usage(1000)
            measured["cache_creation"] = {"ephemeral_5m_input_tokens": 0, "ephemeral_1h_input_tokens": 996}
            del measured["cache_read_input_tokens"]
            send(handler, response_events(body, measured))
        output = self.folder / "missing"
        with mock_server(callback) as url:
            code, _ = self.cli("--url", url, "--key", KEY, "--tests", "conversation", "--output", str(output))
        self.assertEqual(code, 3)
        summary = json.loads((output / "summary.json").read_text(encoding="utf-8"))
        codes = {n["code"] for n in summary["observations"]}
        self.assertTrue({"ttl_changed", "usage_missing"}.issubset(codes))
        self.assertIsNone(summary["rows"][0]["result"]["total_input_tokens"])

    def test_cache_expiry_is_not_reported_as_inflation(self):
        first = {"success": True, "total_input_tokens": 1000, "usage": usage(1000), "started_at": "2026-11-05T00:00:00+00:00"}
        second = {"success": True, "total_input_tokens": 1100, "usage": usage(1100), "started_at": "2026-11-05T00:06:00+00:00"}
        result = check_carry(first, second)
        self.assertEqual(result["status"], "ttl_uncertain")
        self.assertEqual(result["delta"], -996)
        second["started_at"] = "2026-11-05T00:00:20+00:00"
        self.assertEqual(check_carry(first, second)["status"], "different")


if __name__ == "__main__":
    unittest.main()
