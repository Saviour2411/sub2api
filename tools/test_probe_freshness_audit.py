"""现签、来源字段和续接的本地协议回归，不调用付费渠道。"""
import base64
import copy
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import threading
import time
import unittest
from unittest.mock import patch

from test_probe_signature_audit import sse_message
from test_probe_token_audit import WorkspaceTest, send, KEY
from token_audit.fixtures import DEFAULT_MODEL
from token_audit.freshness import aggregate, catalog, decode_signature, inspect_result, varint, wire_fields
from token_audit.protocol import MessagesClient


def vi(value):
    result = bytearray()
    while value >= 128:
        result.append((value & 127) | 128)
        value >>= 7
    result.append(value)
    return bytes(result)


def integer(number, value):
    return vi(number << 3) + vi(value)


def binary(number, value):
    return vi((number << 3) | 2) + vi(len(value)) + value


def signature(stamp=None, marker=1, extra=b"", double=False):
    nested = (integer(2, marker) if marker is not None else b"") + integer(3, 2)
    if stamp is not None:
        nested += integer(21, stamp)
    wire = binary(2, binary(1, nested + extra)) + integer(3, 1)
    text = base64.urlsafe_b64encode(wire).rstrip(b"=")
    return (base64.b64encode(text) if double else text).decode()


def result(stamp=100, sent=100.2, received=100.8, **changes):
    value = {"http_status": 200, "protocol_success": True, "content_complete": True, "stop_reason": "end_turn", "text": "12",
             "response_format": "sse", "response_model": DEFAULT_MODEL, "clock_drift_seconds": 0.01,
             "message_id": "msg_bdrk_local", "request_sent_at": sent, "signature_received_at": {"0": received},
             "assistant_content": [{"type": "thinking", "thinking": "合成思考", "signature": signature(stamp)}]}
    value.update(changes)
    return value


class Endpoint:
    def __init__(self, offset=0, missing=False, marker=1, changed_accept=False, refusal=False, models=None, post_refusal=False):
        self.offset, self.missing, self.marker = offset, missing, marker
        self.changed_accept, self.refusal, self.post_refusal = changed_accept, refusal, post_refusal
        self.models = models or [DEFAULT_MODEL, "claude-sonnet-4-6"]
        self.requests, self.controls = [], {}

    def post(self, handler, body):
        self.requests.append(copy.deepcopy(body))
        if "tools" in body:
            if len(body["messages"]) == 1:
                blocks = [{"type": "thinking", "thinking": "检查余数", "signature": signature(int(time.time()))},
                          {"type": "tool_use", "id": "local-tool", "name": "arithmetic_check", "input": {"remainder": 3}}]
                send(handler, sse_message(body, blocks, "tool_use"))
                return
            changed = "23^601" in body["messages"][0]["content"]
            if changed and not self.changed_accept:
                send(handler, b'{"type":"error","error":{"type":"invalid_request_error","message":"Invalid signature"}}', status=400, content_type="application/json")
                return
            prompt = body["messages"][0]["content"]
            self.controls[prompt] = self.controls.get(prompt, 0) + 1
            stop = "refusal" if self.refusal or (self.post_refusal and self.controls[prompt] >= 2) else "end_turn"
            code = body["messages"][-1]["content"][0]["content"]
            send(handler, sse_message(body, [{"type": "text", "text": code if stop == "end_turn" else ""}], stop))
            return
        stamp = None if self.missing else int(time.time()) + self.offset
        blocks = [{"type": "thinking", "thinking": "合成思考", "signature": signature(stamp, self.marker)}, {"type": "text", "text": "13"}]
        send(handler, sse_message(body, blocks, "refusal" if self.refusal else "end_turn"))


@contextmanager
def endpoint_server(endpoint):
    class Handler(BaseHTTPRequestHandler):
        protocol_version = "HTTP/1.1"
        def handle(self):
            try:
                super().handle()
            except (BrokenPipeError, ConnectionResetError, ConnectionAbortedError):
                pass
        def do_GET(self):
            if "after_id=" in self.path:
                payload = {"data": [{"id": m} for m in endpoint.models[1:]], "has_more": False}
            else:
                payload = {"data": [{"id": endpoint.models[0]}], "has_more": len(endpoint.models) > 1, "last_id": endpoint.models[0]}
            send(self, json.dumps(payload).encode(), content_type="application/json")
        def do_POST(self):
            body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            endpoint.post(self, body)
        def log_message(self, *_):
            pass
    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    server.daemon_threads = True
    worker = threading.Thread(target=server.serve_forever, kwargs={"poll_interval": .01}, daemon=True)
    worker.start()
    try:
        yield "http://127.0.0.1:" + str(server.server_port)
    finally:
        server.shutdown()
        server.server_close()
        worker.join()


class WireTests(unittest.TestCase):
    def test_base64_and_double_encoding(self):
        for double in (False, True):
            decoded = decode_signature(signature(1795000000, double=double))
            self.assertEqual(decoded["decode_status"], "decoded")
            self.assertEqual([decoded[k] for k in ("f2_f1_f2", "f2_f1_f3", "f3", "f2_f1_f21")], [1, 2, 1, 1795000000])
            self.assertEqual(decoded["base64_layers"], 2 if double else 1)

    def test_malformed_or_duplicate_fields_never_pass(self):
        for value in ("bad!", "", base64.b64encode(b"\x12\xff").decode(), signature(100, extra=integer(21, 100)), signature(100, extra=integer(2, 1))):
            self.assertEqual(decode_signature(value)["decode_status"], "failed")
        with self.assertRaises(ValueError):
            varint(b"\xff" * 11, 0)
        with self.assertRaises(ValueError):
            wire_fields(b"\x00")

    def test_missing_timestamp_not_replaced_by_hash(self):
        detail = inspect_result(result(stamp=None), DEFAULT_MODEL)
        self.assertEqual(detail["signatures"][0]["freshness"], "unknown")
        self.assertEqual(detail["signatures"][0]["awsb"], "pass")

    def test_time_window_and_protocol_gates(self):
        for stamp, expected in ((99, "fail"), (100, "pass"), (101, "fail")):
            self.assertEqual(inspect_result(result(stamp), DEFAULT_MODEL)["signatures"][0]["freshness"], expected)
        for mutation in ({"stop_reason": "refusal"}, {"protocol_success": False}, {"response_format": "json"},
                         {"response_model": "other"}, {"clock_drift_seconds": 2}, {"secret_echo": True}):
            self.assertEqual(inspect_result(result(**mutation), DEFAULT_MODEL)["signatures"][0]["freshness"], "unknown")

    def test_two_independent_windows_and_optional_source(self):
        rows = [{"freshness": inspect_result(result(100), DEFAULT_MODEL)}, {"freshness": inspect_result(result(104, 104.1, 104.8), DEFAULT_MODEL)}]
        self.assertEqual(aggregate(rows, [DEFAULT_MODEL], 2, True)["status"], "pass")
        rows[1]["freshness"]["signatures"][0]["awsb"] = "fail"
        self.assertEqual(aggregate(rows, [DEFAULT_MODEL], 2, True)["status"], "fail")
        self.assertEqual(aggregate(rows, [DEFAULT_MODEL], 2, False)["status"], "pass")
        rows[1]["freshness"]["signatures"][0]["f2_f1_f21"] = 100
        self.assertEqual(aggregate(rows, [DEFAULT_MODEL], 2, False)["status"], "fail")

    def test_signature_fragment_receive_times(self):
        with endpoint_server(Endpoint()) as url:
            value = MessagesClient(url, KEY).generate({"model": DEFAULT_MODEL, "stream": True, "messages": []}, "session", capture_content=True)
        self.assertTrue(value["content_complete"])
        self.assertGreaterEqual(value["signature_received_at"]["0"], value["request_sent_at"])
        self.assertEqual(inspect_result(value, DEFAULT_MODEL)["signatures"][0]["freshness"], "pass")


class FreshnessIntegrationTests(WorkspaceTest):
    def run_test(self, endpoint, name, *extra):
        output = self.folder / name
        with endpoint_server(endpoint) as url:
            code, text = self.cli("--url", url, "--key", KEY, "--tests", "signature_freshness", "--freshness-interval", "1", "--output", str(output), *extra)
        self.assertNotIn(KEY, text)
        data = json.loads((output / "summary.json").read_text(encoding="utf-8"))
        return code, data

    def test_current_timestamps_and_catalog_pagination(self):
        endpoint = Endpoint()
        code, data = self.run_test(endpoint, "current", "--freshness-models", DEFAULT_MODEL + ",claude-sonnet-4-6", "--require-awsb")
        self.assertEqual(code, 0)
        self.assertEqual(data["signature_freshness"]["status"], "pass")
        self.assertEqual(data["target_requests"], 4)
        self.assertEqual(len(data["signature_freshness"]["catalog"]["http_statuses"]), 2)
        self.assertEqual(len({r["messages"][0]["content"] for r in endpoint.requests}), 4)

    def test_old_and_missing_and_wrong_marker(self):
        for name, endpoint, expected, exit_code in (("old", Endpoint(offset=-86400), "fail", 1),
                ("missing", Endpoint(missing=True), "unknown", 3), ("wrong-marker", Endpoint(marker=2), "fail", 1)):
            code, data = self.run_test(endpoint, name, "--require-awsb")
            self.assertEqual((code, data["signature_freshness"]["status"]), (exit_code, expected))

    def test_catalog_missing_model_skips_without_generation(self):
        endpoint = Endpoint(models=["other-model"])
        code, data = self.run_test(endpoint, "absent")
        self.assertEqual(code, 3)
        self.assertEqual(data["target_requests"], 0)
        self.assertEqual(endpoint.requests, [])

    def test_key_never_persisted(self):
        code, data = self.run_test(Endpoint(), "secrets")
        for path in (self.folder / "secrets").iterdir():
            self.assertNotIn(KEY.encode(), path.read_bytes())


class RelayTests(WorkspaceTest):
    def test_cross_direction_preserves_history_and_exact_echo(self):
        first, second = Endpoint(), Endpoint()
        output = self.folder / "cross"
        with endpoint_server(first) as a, endpoint_server(second) as b:
            code, _ = self.cli("--url", a, "--key", KEY, "--tests", "signature_relay", "--peer-url", b,
                               "--peer-key", "second-key-private", "--output", str(output))
        data = json.loads((output / "summary.json").read_text(encoding="utf-8"))
        self.assertEqual(code, 0)
        self.assertEqual((data["target_requests"], data["peer_requests"]), (5, 5))
        self.assertEqual(data["signature_relay"]["controls"], {"A": True, "B": True})
        cross = [r for r in data["rows"] if r["relay"]["case"] == "cross"]
        self.assertEqual([r["relay"]["outcome"] for r in cross], ["accepted", "accepted"])
        for row in cross:
            source = next(r for r in data["rows"] if r["relay"]["case"] == "seed" and r["relay"]["source"] == row["relay"]["source"])
            self.assertEqual(row["request"]["messages"][0], source["request"]["messages"][0])
            self.assertEqual(row["request"]["messages"][1]["content"], source["result"]["assistant_content"])
        for path in output.iterdir():
            self.assertNotIn(b"second-key-private", path.read_bytes())

    def test_changed_question_acceptance_warns_and_refusal_invalidates(self):
        for name, endpoint, expected in (("accepted", Endpoint(changed_accept=True), 1), ("refusal", Endpoint(post_refusal=True), 3)):
            output = self.folder / name
            with endpoint_server(endpoint) as url:
                code, _ = self.cli("--url", url, "--key", KEY, "--tests", "signature_relay", "--output", str(output))
            self.assertEqual(code, expected)
            data = json.loads((output / "summary.json").read_text(encoding="utf-8"))
            if name == "refusal":
                self.assertFalse(data["signature_relay"]["controls"]["A"])


if __name__ == "__main__":
    unittest.main()
