"""使用本地模拟接口验证签名矩阵，不请求真实上游或执行工具。"""
from __future__ import annotations

import base64
import copy
import csv
import hashlib
import json
import unittest
from unittest.mock import patch

from test_probe_token_audit import FIXTURE, KEY, WorkspaceTest, event, mock_server, response_events, send, usage
from token_audit.fixtures import DEFAULT_MODEL
from token_audit.protocol import ContentMessageParser, MessagesClient, ProtocolError, SSEDecoder, canonical_hash
from token_audit.signature_cases import CASES, CORE_CASES, decode, make_case
from token_audit.signatures import classify, load_signature_baseline


def source_blocks(number, tool=False):
    raw = hashlib.sha256(str(number).encode()).digest() * 3 + b"x"
    thinking = {"type": "thinking", "thinking": "本地合成思考" + str(number),
                "signature": base64.b64encode(raw).decode("ascii")}
    content = [thinking, {"type": "text", "text": "本地模拟答案" + str(number)}]
    if tool:
        content.append({"type": "tool_use", "id": "tool-" + str(number), "name": "query_probe_metrics",
                        "input": {"account_id": "sim-" + str(number)}})
    return content


def sse_message(body, content, stop="end_turn"):
    parts = [event({"type": "message_start", "message": {"type": "message", "id": "msg-local", "model": body["model"], "content": [], "usage": usage(100)}})]
    for index, block in enumerate(content):
        start = copy.deepcopy(block)
        deltas = []
        if block["type"] in ("thinking", "text"):
            fields = ("thinking", "signature") if block["type"] == "thinking" else ("text",)
            for field in fields:
                value, start[field] = block[field], ""
                cut = max(1, len(value) // 2)
                for offset in range(0, len(value), cut):
                    deltas.append({"type": field + "_delta", field: value[offset:offset + cut]})
        elif block["type"] == "tool_use":
            start["input"] = {}
            raw = json.dumps(block["input"], ensure_ascii=False)
            deltas = [{"type": "input_json_delta", "partial_json": raw[:5]}, {"type": "input_json_delta", "partial_json": raw[5:]}]
        parts.append(event({"type": "content_block_start", "index": index, "content_block": start}))
        parts.extend(event({"type": "content_block_delta", "index": index, "delta": delta}) for delta in deltas)
        parts.append(event({"type": "content_block_stop", "index": index}))
    parts.extend([event({"type": "message_delta", "delta": {"stop_reason": stop}, "usage": {"output_tokens": 12}}), event({"type": "message_stop"})])
    return b"".join(parts)


def reply(handler, body, content, stop="end_turn", sse=False):
    if sse:
        send(handler, sse_message(body, content, stop))
    else:
        payload = {"type": "message", "id": "msg-local", "model": body["model"], "content": content, "stop_reason": stop, "usage": usage(100)}
        send(handler, json.dumps(payload, ensure_ascii=False).encode("utf-8"), content_type="application/json")


class SignatureEndpoint:
    def __init__(self, mode="strict", seed_fault=None, post_failure=False, fail_status=None, control_failure=None):
        self.mode, self.seed_fault = mode, seed_fault
        self.post_failure, self.fail_status = post_failure, fail_status
        self.control_failure = 3 if post_failure else control_failure
        self.calls, self.sources, self.controls = [], {}, {}

    def __call__(self, handler, body):
        self.calls.append(copy.deepcopy(body))
        if self.fail_status:
            send(handler, b'{"type":"error","error":{"type":"api_error","message":"local failure"}}', status=self.fail_status, content_type="application/json")
            return
        session = handler.headers["x-claude-code-session-id"]
        sse = len(self.calls) % 2 == 0
        if len(body["messages"]) == 1:
            blocks = source_blocks(len(self.sources) + 1, "tools" in body)
            if self.seed_fault == "no_signature":
                blocks = [block for block in blocks if block["type"] != "thinking"]
            elif self.seed_fault == "duplicate":
                blocks[0]["signature"] = source_blocks(1)[0]["signature"]
            elif self.seed_fault == "opaque":
                blocks[0]["signature"] = "opaque!" + str(len(self.sources))
            elif self.seed_fault in ("thinking", "signature", "tool_input"):
                if self.seed_fault == "tool_input":
                    blocks[-1]["input"]["account_id"] = KEY
                else:
                    blocks[0][self.seed_fault] = KEY
            elif self.seed_fault == "unknown_block":
                blocks.append({"type": "unknown_vendor_block", "data": "不可回放"})
                sse = False
            self.sources[session] = copy.deepcopy(blocks)
            response_body = {**body, "model": "different-model"} if self.seed_fault == "model" else body
            reply(handler, response_body, blocks, "tool_use" if "tools" in body else "end_turn", sse)
            return
        content = body["messages"][1]["content"]
        if content == self.sources[session]:
            self.controls[session] = self.controls.get(session, 0) + 1
            if self.controls[session] == self.control_failure:
                send(handler, b'{"error":{"message":"post control failed"}}', status=500, content_type="application/json")
                return
        signed = next((block for block in content if block["type"] == "thinking"), None)
        valid = signed is None
        if signed is not None:
            for source in self.sources.values():
                original = next((block for block in source if block["type"] == "thinking"), {})
                if signed.get("thinking") != original.get("thinking"):
                    continue
                if signed.get("signature") == original.get("signature"):
                    valid = True
                elif self.mode == "padding":
                    try:
                        # 显式模拟忽略填充符后字符，避免Python版本改变夹具行为。
                        if not isinstance(signed["signature"], str):
                            raise ValueError("签名不是字符串")
                        prefix = signed["signature"].split("=", 1)[0]
                        valid = decode(prefix) == decode(original["signature"])
                    except (ValueError, KeyError, TypeError):
                        pass
                if valid:
                    break
        if not valid and self.mode != "loose":
            message = "unsupported request parameter" if self.mode == "generic" else "Invalid signature in thinking block"
            error = {"type": "error", "error": {"type": "invalid_request_error", "message": message}}
            if self.mode == "stream":
                send(handler, event(error))
            elif self.mode == "json_error":
                send(handler, json.dumps(error).encode(), content_type="application/json")
            elif self.mode == "disconnect":
                raw = sse_message(body, [{"type": "text", "text": "部分回答"}]).removesuffix(event({"type": "message_stop"}))
                send(handler, raw)
            else:
                send(handler, json.dumps(error).encode(), status=400, content_type="application/json")
            return
        reply(handler, body, [{"type": "text", "text": "续接已完成"}], sse=sse)


class ContentParserTests(unittest.TestCase):
    def test_fragmented_thinking_signature_and_tool_json_are_lossless(self):
        expected = source_blocks(1, tool=True)
        parser = ContentMessageParser()
        decoder = SSEDecoder(parser)
        for byte in sse_message({"model": DEFAULT_MODEL}, expected, "tool_use"):
            decoder.feed(bytes([byte]))
        decoder.finish()
        self.assertEqual(parser.result()["assistant_content"], expected)
        self.assertTrue(parser.result()["content_complete"])
        self.assertEqual(parser.result()["thinking_chars"], len(expected[0]["thinking"]))

    def test_json_preserves_redacted_blocks_and_extensions(self):
        blocks = source_blocks(2, tool=True)
        blocks[0]["vendor_field"] = {"unchanged": [1, 2]}
        blocks.insert(1, {"type": "redacted_thinking", "data": "opaque"})
        parser = ContentMessageParser()
        parser.json_response({"type": "message", "id": "msg", "model": DEFAULT_MODEL, "content": blocks, "stop_reason": "tool_use"})
        self.assertEqual(parser.result()["assistant_content"], blocks)

    def test_unknown_delta_and_unclosed_block_fail(self):
        for raw in (sse_message({"model": DEFAULT_MODEL}, source_blocks(1)).replace(b'"signature_delta"', b'"unknown_delta"'),
                    sse_message({"model": DEFAULT_MODEL}, source_blocks(1)).replace(event({"type": "content_block_stop", "index": 1}), b"")):
            with self.subTest(raw=raw[:30]), self.assertRaises(ProtocolError):
                decoder = SSEDecoder(ContentMessageParser())
                decoder.feed(raw)
                decoder.finish()


class MutationTests(unittest.TestCase):
    def setUp(self):
        self.original = {"model": DEFAULT_MODEL, "system": "固定系统内容", "messages": [
            {"role": "user", "content": "原始问题"}, {"role": "assistant", "content": source_blocks(1, True)},
            {"role": "user", "content": [{"type": "tool_result", "tool_use_id": "tool-1", "content": "合成结果"}]}]}
        self.donor = source_blocks(2)[0]

    def test_all_cases_change_only_allowed_fields_and_keep_baseline(self):
        saved = copy.deepcopy(self.original)
        self.assertEqual((len(CASES), len(CORE_CASES)), (21, 18))
        for name, _, category in CASES:
            with self.subTest(case=name):
                changed, mutation = make_case(self.original, 0, self.donor, name)
                self.assertNotEqual(changed, self.original)
                self.assertEqual(changed["messages"][0], self.original["messages"][0])
                self.assertEqual(changed["messages"][2], self.original["messages"][2])
                self.assertEqual(changed["system"], self.original["system"])
                self.assertTrue(mutation["changes_within_expected_fields"])
                self.assertEqual(mutation["category"], category)
        self.assertEqual(self.original, saved)

    def test_single_char_really_changes_payload_including_padding_bits(self):
        for length in (1, 2, 3, 31, 32, 97):
            self.original["messages"][1]["content"][0]["signature"] = base64.b64encode(bytes(range(length))).decode()
            for name in ("char_head", "char_middle", "char_tail", "char_random"):
                with self.subTest(length=length, case=name):
                    _, mutation = make_case(self.original, 0, self.donor, name)
                    self.assertEqual(mutation["differing_characters"], 1)
                    self.assertTrue(mutation["decoded_payload_changed"])

    def test_string_append_and_real_payload_append_are_separate(self):
        _, trailing = make_case(self.original, 0, self.donor, "append")
        changed, payload = make_case(self.original, 0, self.donor, "append_decoded_payload")
        self.assertFalse(trailing["core"])
        self.assertFalse(trailing["decoded_payload_changed"])
        self.assertTrue(payload["core"])
        self.assertTrue(payload["decoded_payload_changed"])
        self.assertEqual(len(decode(changed["messages"][1]["content"][0]["signature"])) - len(decode(self.original["messages"][1]["content"][0]["signature"])), 8)

    def test_byte_cases_and_donor_fail_explicitly_when_unavailable(self):
        self.original["messages"][1]["content"][0]["signature"] = "opaque!"
        for name in ("random_a", "char_tail", "decoded_bitflip", "append_decoded_payload", "splice"):
            with self.subTest(case=name), self.assertRaises(ValueError):
                make_case(self.original, 0, self.donor, name)
        for name in ("splice", "foreign_signature", "foreign_block"):
            with self.subTest(case=name), self.assertRaises(ValueError):
                make_case(self.original, 0, None, name)


class SignatureWorkflowTests(WorkspaceTest):
    def run_probe(self, endpoint, *args):
        folder = self.folder / ("run-" + str(len(list(self.folder.iterdir()))))
        with mock_server(endpoint) as url:
            code, output = self.cli("--url", url, "--key", KEY, "--tests", "signature", "--output", str(folder), *args)
        summary = json.loads((folder / "summary.json").read_text(encoding="utf-8"))
        return code, summary, output, folder

    def test_cli_default_and_dynamic_budgets(self):
        for arguments, count in (((), 6), (("--tests", "signature"), 100),
                                 (("--tests", "conversation,fingerprint,signature"), 106),
                                 (("--tests", "signature", "--signature-samples", "3", "--signature-scenarios", "tool"), 75)):
            with self.subTest(arguments=arguments):
                code, text = self.cli("--url", "https://example.com", "--dry-run", *arguments)
                self.assertEqual(code, 0)
                plan = json.loads(text)
                self.assertEqual(plan["target_requests_max"], count)
                if arguments and arguments[1] == "signature":
                    self.assertIsNone(plan["fingerprint_id"])
                    self.assertEqual(plan["signature"]["reference_id"], "trusted-saviour-claude-opus-5-5-signature-20261122")
                    self.assertEqual(plan["signature"]["append_expected_outcome"], "rejected_http_400")
        self.assertIn("signature：", self.cli("--list-tests")[1])

    def test_invalid_options_never_connect(self):
        options = (("--signature-samples", "1"), ("--signature-samples", "6"), ("--signature-scenarios", "chat,chat"),
                   ("--signature-scenarios", "invalid"), ("--signature-scenarios", ""), ("--signature-max-tokens", "1"),
                   ("--reference-url", "https://example.com", "--reference-key", KEY))
        for option in options:
            with self.subTest(option=option), patch("token_audit.protocol.MessagesClient.generate") as generate:
                code, _ = self.cli("--url", "https://example.com", "--key", KEY, "--tests", "signature", *option)
                self.assertEqual(code, 2)
                generate.assert_not_called()

    def test_strict_endpoint_completes_100_requests_without_treating_400_as_failure(self):
        endpoint = SignatureEndpoint()
        code, summary, _, folder = self.run_probe(endpoint)
        self.assertEqual(code, 0)
        self.assertEqual(summary["target_requests"], 100)
        self.assertEqual(summary["signature"]["http_400_rejected_core_cases"], 72)
        self.assertEqual(summary["signature"]["accepted_core_cases"], 0)
        self.assertEqual(summary["signature"]["valid_control_samples"], 4)
        self.assertEqual(summary["observations"], [])
        reference = summary["signature"]["reference"]["cases"]["append"]
        self.assertEqual((reference["matched_samples"], reference["different_samples"], reference["unassessed_samples"]), (4, 0, 0))
        self.assertIn("可信实测4/4为HTTP400明确拒绝", (folder / "report.md").read_text(encoding="utf-8"))
        with (folder / "signature-cases.csv").open(encoding="utf-8-sig", newline="") as handle:
            csv_rows = list(csv.DictReader(handle))
            self.assertEqual(len(csv_rows), 100)
            append_rows = [r for r in csv_rows if r["测试情况"] == "签名字符串末尾追加字符"]
            self.assertEqual(len(append_rows), 4)
            self.assertTrue(all(r["参照比较"] == "match" and r["核心变更"] == "False" for r in append_rows))
        with (folder / "rounds.csv").open(encoding="utf-8-sig", newline="") as handle:
            self.assertEqual(len(list(csv.reader(handle))), 1)
        for row in summary["rows"]:
            if row["signature"]["case"].startswith("original_"):
                source = next(r for r in summary["rows"] if r["signature"]["case"] == "seed" and r["session_id"] == row["session_id"])
                self.assertEqual(row["request"]["messages"][1]["content"], source["result"]["assistant_content"])
                if row["signature"]["scenario"] == "tool":
                    tool = next(b for b in source["result"]["assistant_content"] if b["type"] == "tool_use")
                    self.assertEqual(row["request"]["messages"][2]["content"][0]["tool_use_id"], tool["id"])

    def test_loose_endpoint_warns_and_compatibility_cases_are_excluded(self):
        code, summary, _, _ = self.run_probe(SignatureEndpoint("loose"))
        self.assertEqual(code, 1)
        self.assertEqual(summary["signature"]["accepted_core_cases"], 72)
        self.assertEqual(summary["signature"]["evaluated_core_cases"], 72)
        self.assertEqual(summary["signature"]["outcomes"]["accepted"], 96)
        self.assertTrue(any(n["code"] == "signature_mutation_accepted" for n in summary["observations"]))

    def test_padding_acceptance_differs_from_new_reference_without_changing_core_counts(self):
        code, summary, _, _ = self.run_probe(SignatureEndpoint("padding"))
        self.assertEqual(code, 1)
        self.assertEqual(summary["signature"]["http_400_rejected_core_cases"], 72)
        self.assertEqual(summary["signature"]["accepted_core_cases"], 0)
        reference = summary["signature"]["reference"]["cases"]["append"]
        self.assertEqual((reference["matched_samples"], reference["different_samples"]), (0, 4))
        self.assertTrue(any(n["code"] == "signature_reference_mismatch" for n in summary["observations"]))

    def test_changed_source_budget_does_not_apply_reference(self):
        code, summary, _, _ = self.run_probe(SignatureEndpoint("padding"), "--signature-max-tokens", "3072", "--signature-scenarios", "chat")
        self.assertEqual(code, 0)
        reference = summary["signature"]["reference"]["cases"]["append"]
        self.assertEqual((reference["evaluated_samples"], reference["unassessed_samples"]), (0, 2))
        self.assertTrue(all(r["status"] == "unavailable_source_parameters" for r in reference["rows"]))

    def test_extra_samples_without_historical_prompt_are_not_assumed_to_match(self):
        code, summary, _, _ = self.run_probe(SignatureEndpoint(), "--signature-samples", "3", "--signature-scenarios", "chat")
        self.assertEqual(code, 0)
        reference = summary["signature"]["reference"]["cases"]["append"]
        self.assertEqual((reference["matched_samples"], reference["unassessed_samples"]), (2, 1))
        self.assertEqual(reference["rows"][-1]["status"], "unavailable_source_parameters")

    def test_reference_is_validated_before_network_and_not_needed_by_default_tests(self):
        data = load_signature_baseline()
        path = self.folder / "invalid-reference.json"
        for invalid in ([], {**data, "sampled_at": "2026-11-22"},
                        {**data, "cases": {"append": {**data["cases"]["append"], "expected_http_status": 200}}},
                        {**data, "cases": {"append": {**data["cases"]["append"], "samples": data["cases"]["append"]["samples"][:3]}}}):
            path.write_text(json.dumps(invalid), encoding="utf-8")
            with patch("token_audit.signatures.BUILTIN_SIGNATURE_BASELINE", path), patch.object(MessagesClient, "generate") as generate:
                code, _ = self.cli("--url", "https://example.com", "--tests", "signature", "--key", KEY)
                self.assertEqual(code, 2)
                generate.assert_not_called()
                self.assertEqual(self.cli("--url", "https://example.com", "--dry-run")[0], 0)

    def test_generic_400_is_not_successful_verification(self):
        code, summary, _, _ = self.run_probe(SignatureEndpoint("generic"), "--signature-scenarios", "chat")
        self.assertEqual(code, 3)
        self.assertEqual(summary["signature"]["http_400_rejected_core_cases"], 0)
        self.assertEqual(summary["signature"]["unassessed_core_cases"], 36)
        reference = summary["signature"]["reference"]["cases"]["append"]
        self.assertEqual((reference["evaluated_samples"], reference["unassessed_samples"]), (0, 2))

    def test_stream_signature_rejection_is_counted_separately(self):
        code, summary, _, _ = self.run_probe(SignatureEndpoint("stream"), "--signature-scenarios", "tool")
        self.assertEqual(code, 1)
        self.assertEqual(summary["signature"]["stream_rejected_core_cases"], 36)
        self.assertEqual(summary["signature"]["http_400_rejected_core_cases"], 0)
        reference = summary["signature"]["reference"]["cases"]["append"]
        self.assertEqual(reference["different_samples"], 2)
        self.assertTrue(all(r["outcome"] == "rejected_stream_error" for r in reference["rows"]))

    def test_failed_final_controls_invalidate_earlier_accepted_cases(self):
        code, summary, _, _ = self.run_probe(SignatureEndpoint("loose", post_failure=True), "--signature-scenarios", "chat")
        self.assertEqual(code, 3)
        self.assertEqual(summary["signature"]["attempted_core_cases"], 36)
        self.assertEqual(summary["signature"]["evaluated_core_cases"], 0)
        self.assertEqual(summary["signature"]["accepted_core_cases"], 0)
        self.assertFalse(any(n["code"] == "signature_mutation_accepted" for n in summary["observations"]))
        self.assertFalse(any(n["code"] == "signature_reference_mismatch" for n in summary["observations"]))
        self.assertEqual(summary["signature"]["reference"]["cases"]["append"]["evaluated_samples"], 0)

    def test_first_and_middle_control_failure_stop_the_sample(self):
        for failure, attempts in ((1, 4), (2, 26)):
            with self.subTest(failure=failure):
                endpoint = SignatureEndpoint("loose", control_failure=failure)
                code, summary, _, _ = self.run_probe(endpoint, "--signature-scenarios", "chat")
                self.assertEqual(code, 3)
                self.assertEqual(len(endpoint.calls), attempts)
                self.assertEqual(summary["signature"]["evaluated_core_cases"], 0)
                self.assertFalse(any(r["signature"]["case"] == "original_after" for r in summary["rows"]))

    def test_incomplete_stream_stops_after_three_without_counting_rejection(self):
        endpoint = SignatureEndpoint("disconnect")
        code, summary, _, _ = self.run_probe(endpoint, "--signature-scenarios", "chat")
        self.assertEqual(code, 3)
        self.assertEqual(len(endpoint.calls), 6)
        self.assertEqual(summary["signature"]["stream_rejected_core_cases"], 0)
        self.assertEqual(summary["signature"]["evaluated_core_cases"], 0)

    def test_http_200_json_error_is_not_a_stream_rejection(self):
        code, summary, _, _ = self.run_probe(SignatureEndpoint("json_error"), "--signature-scenarios", "chat")
        self.assertEqual(code, 3)
        self.assertEqual(summary["signature"]["stream_rejected_core_cases"], 0)
        self.assertEqual(summary["signature"]["http_400_rejected_core_cases"], 0)

    def test_combined_run_keeps_original_token_tests_and_signature_reports_separate(self):
        endpoint = SignatureEndpoint()
        token_calls = []
        expected = {case["request_sha256"]: case["expected_usage"] for case in FIXTURE["cases"]}

        def combined(handler, body):
            if (body.get("thinking") or {}).get("type") == "adaptive":
                self.assertEqual(len(token_calls), 6)
                endpoint(handler, body)
            else:
                token_calls.append(body)
                number = (len(body["messages"]) + 1) // 2
                measured = expected.get(canonical_hash(body)) or usage(number * 100, (number - 1) * 100 - 4 if number > 1 else 0)
                send(handler, response_events(body, measured))

        code, summary, _, folder = self.run_probe(combined, "--tests", "conversation,fingerprint,signature")
        self.assertEqual(code, 0)
        self.assertEqual(summary["target_requests"], 106)
        self.assertEqual(summary["compared_requests"], 3)
        self.assertEqual(summary["signature"]["evaluated_core_cases"], 72)
        with (folder / "rounds.csv").open(encoding="utf-8-sig", newline="") as handle:
            self.assertEqual(len(list(csv.reader(handle))), 7)

    def test_no_signature_wrong_model_or_unknown_content_never_fabricates_history(self):
        for fault in ("no_signature", "model", "unknown_block"):
            with self.subTest(fault=fault):
                endpoint = SignatureEndpoint(seed_fault=fault)
                code, summary, _, _ = self.run_probe(endpoint, "--signature-scenarios", "chat")
                self.assertEqual(code, 3)
                self.assertEqual(len(endpoint.calls), 2)
                self.assertTrue(all(len(body["messages"]) == 1 for body in endpoint.calls))
                self.assertEqual(summary["signature"]["evaluated_core_cases"], 0)

    def test_duplicate_sources_and_opaque_signature_are_incomplete(self):
        for fault in ("duplicate", "opaque"):
            with self.subTest(fault=fault):
                code, summary, _, _ = self.run_probe(SignatureEndpoint(seed_fault=fault), "--signature-scenarios", "chat")
                self.assertEqual(code, 3)
                self.assertLess(summary["signature"]["evaluated_core_cases"], 36)
                self.assertTrue(any(s["skipped_cases"] for s in summary["signature"]["samples"]))

    def test_secret_in_thinking_signature_or_tool_input_stops_and_is_redacted(self):
        for fault in ("thinking", "signature", "tool_input"):
            with self.subTest(fault=fault):
                endpoint = SignatureEndpoint(seed_fault=fault)
                code, summary, output, folder = self.run_probe(endpoint, "--signature-scenarios", "tool")
                self.assertEqual(code, 3)
                self.assertEqual(len(endpoint.calls), 1)
                self.assertNotIn(KEY, output)
                for file in folder.iterdir():
                    self.assertNotIn(KEY, file.read_text(encoding="utf-8-sig"))
                self.assertTrue(summary["rows"][0]["result"]["secret_echo"])

    def test_auth_rate_limit_and_consecutive_server_errors_stop_without_retry(self):
        for status, attempts in ((401, 1), (403, 1), (429, 1), (503, 3)):
            with self.subTest(status=status):
                endpoint = SignatureEndpoint(fail_status=status)
                code, summary, _, _ = self.run_probe(endpoint, "--signature-scenarios", "chat", "--signature-samples", "5")
                self.assertEqual(code, 3)
                self.assertEqual(len(endpoint.calls), attempts)
                self.assertEqual(summary["signature"]["evaluated_core_cases"], 0)

    def test_selected_model_samples_and_output_budget_are_used(self):
        endpoint = SignatureEndpoint()
        code, summary, _, _ = self.run_probe(endpoint, "--model", "local-model", "--signature-scenarios", "chat", "--signature-samples", "3", "--signature-max-tokens", "3072", "--max-tokens", "100")
        self.assertEqual(code, 0)
        self.assertEqual(summary["target_requests"], 75)
        self.assertEqual(summary["signature"]["http_400_rejected_core_cases"], 54)
        self.assertTrue(all(b["model"] == "local-model" and b["max_tokens"] == 3072 for b in endpoint.calls))
        self.assertEqual(summary["signature"]["reference"]["status"], "unavailable_model")

    def test_interruption_keeps_results_but_never_counts_unbracketed_cases(self):
        original = MessagesClient.generate
        count = 0

        def interrupted(client, *args, **kwargs):
            nonlocal count
            count += 1
            if count == 8:
                raise KeyboardInterrupt
            return original(client, *args, **kwargs)

        with patch.object(MessagesClient, "generate", interrupted):
            code, summary, _, _ = self.run_probe(SignatureEndpoint("loose"), "--signature-scenarios", "chat")
        self.assertEqual(code, 130)
        self.assertEqual(summary["target_requests"], 7)
        self.assertEqual(summary["signature"]["evaluated_core_cases"], 0)

    def test_classifier_rejects_false_positive_errors_and_incomplete_success(self):
        result = {"protocol_success": False, "success": False, "http_status": 400, "upstream_error": None}
        for text, expected in (("Invalid signature in thinking block", "rejected_http_400"),
                               ("messages.1.content.0.thinking.signature: Field required", "rejected_http_400"),
                               ("signature: Input should be a valid string", "rejected_http_400"),
                               ("unsupported thinking parameter", "inconclusive")):
            self.assertEqual(classify({**result, "error_excerpt": text}, DEFAULT_MODEL), expected)
        self.assertEqual(classify({**result, "upstream_error": {"type": "rate_limit_error", "message": "signature verification failed"}}, DEFAULT_MODEL), "inconclusive")
        self.assertEqual(classify({**result, "http_status": 200, "error": "ProtocolError: Invalid signature"}, DEFAULT_MODEL), "inconclusive")
        self.assertEqual(classify({**result, "http_status": 200, "protocol_success": True, "success": True, "response_model": "other"}, DEFAULT_MODEL), "inconclusive")


if __name__ == "__main__":
    unittest.main()
