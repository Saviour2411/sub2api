"""测试项注册、连续对话、指纹对照和报告。"""
from __future__ import annotations

import copy
import csv
import json
import time
import uuid
from dataclasses import dataclass
from datetime import datetime

from .fixtures import conversation_body
from .protocol import input_metrics, is_token_count, redact, utc_now
from .signatures import run_signature, signature_request_limit, summarize_signatures, signature_report, signature_csv_rows
from .concurrency import run_concurrency, report_lines as concurrency_report
from .freshness import run_freshness, request_limit as freshness_limit, report_lines as freshness_report
from .relay import run_relay, request_limit as relay_limit, report_lines as relay_report


@dataclass(frozen=True)
class TestSpec:
    name: str
    description: str
    run: object
    max_requests: object = 3

    def request_limit(self, options):
        return self.max_requests(options) if callable(self.max_requests) else self.max_requests


TESTS = {}


def register(name, description, max_requests=3):
    def decorator(function):
        if name in TESTS:
            raise ValueError("重复测试项: " + name)
        TESTS[name] = TestSpec(name, description, function, max_requests)
        return function
    return decorator


def observation(code, message, level="warning"):
    return {"code": code, "message": message, "level": level}


def usage_observations(result, requested_model):
    notes = []
    if not result["success"]:
        reason = result.get("error") or "结束原因=" + str(result.get("stop_reason")) + "，或没有可见回复"
        notes.append(observation("response_incomplete", reason, "incomplete"))
    if result["missing_usage_fields"]:
        notes.append(observation("usage_missing", "缺少有效Token字段: " + ", ".join(result["missing_usage_fields"]), "incomplete"))
    if result["success"] and result.get("response_model") != requested_model:
        notes.append(observation("model_mismatch", "响应模型与请求模型不一致，Token对照不可作为同模型结论", "incomplete"))
    usage = result["usage"]
    split = usage.get("cache_creation")
    if isinstance(split, dict):
        short, long = split.get("ephemeral_5m_input_tokens"), split.get("ephemeral_1h_input_tokens")
        if is_token_count(long) and long > 0:
            notes.append(observation("ttl_changed", "请求缓存TTL为5m，响应出现1小时缓存创建量"))
        if all(is_token_count(value) for value in (short, long, usage.get("cache_creation_input_tokens"))):
            if short + long != usage["cache_creation_input_tokens"]:
                notes.append(observation("cache_split_mismatch", "缓存创建分项之和不等于创建总量"))
    return notes


def compare_tokens(result, expected_total, model, absolute_tolerance, relative_tolerance, method):
    total = result["total_input_tokens"]
    available = result["success"] and result.get("response_model") == model and total is not None and is_token_count(expected_total) and expected_total > 0
    comparison = {"method": method, "expected_total": expected_total,
                  "status": "unavailable", "ratio": None, "delta": None, "allowed_delta": None}
    if not available:
        return comparison
    tolerance = max(absolute_tolerance, expected_total * relative_tolerance)
    delta = total - expected_total
    comparison.update(ratio=total / expected_total, delta=delta, allowed_delta=tolerance,
                      status="higher" if delta > tolerance else "lower" if delta < -tolerance else "within_tolerance")
    return comparison


def apply_comparison(row, comparison):
    row["comparison"] = comparison
    if comparison["status"] in ("higher", "lower"):
        direction = "偏高" if comparison["status"] == "higher" else "偏低"
        row["observations"].append(observation("input_" + comparison["status"],
            "输入总量" + direction + "，为参照的" + format(comparison["ratio"], ".4f") + "倍；不能据此认定恶意注水"))
    elif comparison["status"] == "unavailable":
        row["observations"].append(observation("comparison_unavailable", "没有有效的同模型输入参照结果", "incomplete"))


def check_carry(previous, current):
    result = {"status": "unavailable", "previous_cached_tokens": None, "delta": None, "start_gap_seconds": None}
    if not previous["success"] or not current["success"] or previous["total_input_tokens"] is None or current["total_input_tokens"] is None:
        return result
    prior = previous["usage"]["cache_read_input_tokens"] + previous["usage"]["cache_creation_input_tokens"]
    delta = current["usage"]["cache_read_input_tokens"] - prior
    gap = (datetime.fromisoformat(current["started_at"]) - datetime.fromisoformat(previous["started_at"])).total_seconds()
    result.update(previous_cached_tokens=prior, delta=delta, start_gap_seconds=gap,
                  status="ttl_uncertain" if gap >= 300 else "equal" if delta == 0 else "different")
    return result


class RunContext:
    def __init__(self, client, reference, template, fingerprint, options, directory, secrets):
        self.client, self.reference = client, reference
        self.template, self.fingerprint = template, fingerprint
        self.options, self.directory, self.secrets = options, directory, secrets
        self.rows, self.skips, self.notes = [], [], []
        self.auth_failed = False
        self.stop_reason = None
        self.transient_streak = 0
        self.signature_samples = []
        self.started_at = utc_now()
        self.reference_attempts = 0
        self.concurrency_summary = None
        self.freshness_summary = None
        self.relay_summary = None

    def emit(self, kind, **data):
        event = redact({"kind": kind, **data}, self.secrets)
        with (self.directory / "events.jsonl").open("a", encoding="utf-8") as output:
            output.write(json.dumps(event, ensure_ascii=False) + "\n")

    def say(self, text):
        print(redact(text, self.secrets), flush=True)

    def note(self, code, message, level="incomplete", row=None):
        (row["observations"] if row is not None else self.notes).append(observation(code, message, level))

    def call(self, test_name, number, body, session, *, capture_content=False, check_usage=True, details=None, client=None, endpoint_role="A"):
        if self.stop_reason:
            raise ValueError(self.stop_reason)
        self.say("[" + test_name + "] 第" + str(number) + "轮请求中")
        self.emit("request_start", test=test_name, round=number, started_at=utc_now(), request=body, **(details or {}))
        result = (client or self.client).generate(body, session, capture_content=capture_content)
        if redact(result, self.secrets) != result:
            result["success"] = False
            result["protocol_success"] = False
            result["secret_echo"] = True
            result["error"] = "上游响应回显密钥，停止后续目标请求且不转发受污染的回复"
            self.stop_reason = result["error"]
        row = {"test": test_name, "round": number, "session_id": session,
               "endpoint_role": endpoint_role,
               "request": copy.deepcopy(body), "result": result,
               "cache_check": None, "comparison": None,
               "observations": usage_observations(result, self.options.model) if check_usage else [],
               **(details or {})}
        self.rows.append(row)
        self.emit("target_result", test=test_name, round=number, result=result, **(details or {}))
        self.auth_failed = self.auth_failed or result["http_status"] in (401, 403)
        if result["http_status"] in (401, 403, 429):
            self.stop_reason = "目标端认证失败或限流（HTTP " + str(result["http_status"]) + "），停止后续目标请求"
        if test_name in ("signature", "signature_freshness", "signature_relay"):
            transient = (result["http_status"] is None or (result["http_status"] or 0) >= 500 or
                         (result["http_status"] == 200 and not result["protocol_success"] and not result.get("upstream_error")))
            self.transient_streak = self.transient_streak + 1 if transient else 0
            if self.transient_streak >= 3:
                self.stop_reason = "连续三次传输、断流或服务器错误，停止后续目标请求"
        if self.stop_reason:
            self.note("target_stopped", self.stop_reason, row=row)
        total = result["total_input_tokens"]
        self.say("  HTTP=" + str(result["http_status"]) + "，总输入=" + str(total if total is not None else "未取得") +
                 "，结束=" + str(result.get("stop_reason") or result.get("error")) +
                 "，耗时=" + str(result["elapsed_seconds"]) + "秒")
        return row

    def compare(self, row, expected, method):
        comparison = compare_tokens(row["result"], expected, self.options.model,
                                    self.options.tolerance_tokens, self.options.tolerance_ratio, method)
        apply_comparison(row, comparison)


@register("conversation", "三轮真实连续对话；独立前缀，检查缓存命中、衔接和TTL")
def conversation(context):
    session = str(uuid.uuid4())
    body = conversation_body(context.template, context.options.model, session, context.options.max_tokens)
    previous = None
    for number in range(1, 4):
        if previous:
            if context.options.interval:
                time.sleep(context.options.interval)
            body["messages"].append({"role": "assistant", "content": [{"type": "text", "text": previous["result"]["text"]}]})
            body["messages"].append(copy.deepcopy(context.template["cases"][number - 1]["request"]["messages"][-1]))
        row = context.call("conversation", number, body, session)
        if previous:
            row["cache_check"] = check_carry(previous["result"], row["result"])
            status = row["cache_check"]["status"]
            if status == "different":
                row["observations"].append(observation("cache_carry_difference", "缓存衔接差=" + str(row["cache_check"]["delta"]) + "；需区分缓存失效、改写及计量差异"))
            elif status == "ttl_uncertain":
                row["observations"].append(observation("cache_ttl_uncertain", "相邻请求跨越5分钟窗口，不能按冷暖衔接判定", "incomplete"))
        if not row["result"]["success"]:
            break
        previous = row


@register("fingerprint", "重放三条历史固定请求；按绑定模型和请求SHA256比较输入总量")
def fingerprint(context):
    if context.fingerprint["model"] != context.options.model:
        context.skips.append({"test": "fingerprint", "reason": "此模型没有匹配指纹，未使用其他模型的标准"})
        context.say("[fingerprint] 跳过：此模型没有匹配指纹")
        return
    for case in context.fingerprint["cases"]:
        body = copy.deepcopy(case["request"])
        session = str(uuid.uuid4())
        try:
            session = json.loads(body.get("metadata", {}).get("user_id", "{}"))["session_id"]
        except (ValueError, TypeError, KeyError):
            pass
        row = context.call("fingerprint", case["round"], body, session)
        if row["result"]["request_sha256"] != case["request_sha256"]:
            raise ValueError("重放请求与指纹SHA256不同")
        row["fingerprint_id"] = context.fingerprint["id"]
        context.compare(row, input_metrics(case["expected_usage"])["total_input_tokens"], "pinned_fingerprint")
        # 重放的是三个固定请求快照，各自可独立测量，不使用响应拼接历史。
        if context.stop_reason:
            break


@register("signature", "思考签名篡改：两种场景、独立来源、21种变更及前中后原样对照（默认100次）", max_requests=signature_request_limit)
def signature(context):
    run_signature(context)


@register("concurrency", "单轮突发并发；接口/正文成功率、首字与完整响应P50/P90/P95及可信历史对照（默认200次）",
          max_requests=lambda options: options.concurrency)
def concurrency(context):
    run_concurrency(context)


@register("signature_freshness", "现签时间窗口、签名结构和可选AWSB标记；缺少时间戳不算通过", max_requests=freshness_limit)
def signature_freshness(context):
    run_freshness(context)


@register("signature_relay", "工具续接、修改原题与可选双向跨渠道续接", max_requests=relay_limit)
def signature_relay(context):
    run_relay(context)


def run_live_reference(context):
    if not context.reference:
        return
    for row in [r for r in context.rows if r["test"] == "conversation"]:
        if redact(row["request"], context.secrets) != row["request"]:
            context.compare(row, None, "live_reference")
            continue
        if not row["result"]["success"] or row["result"]["total_input_tokens"] is None:
            context.compare(row, None, "live_reference")
            continue
        context.say("[reference] 同请求参照，第" + str(row["round"]) + "轮")
        context.emit("reference_start", round=row["round"], request=row["request"])
        result = context.reference.generate(row["request"], row["session_id"])
        context.reference_attempts += 1
        row["reference_result"] = result
        context.emit("reference_result", round=row["round"], result=result)
        valid = result["success"] and result["response_model"] == context.options.model
        expected = result["total_input_tokens"] if valid else None
        if result["request_sha256"] != row["result"]["request_sha256"]:
            raise ValueError("实时参照与目标请求SHA256不同")
        context.compare(row, expected, "live_reference")
        if result["http_status"] in (401, 403):
            context.notes.append(observation("reference_auth_failed", "参照端认证失败，停止后续参照请求", "incomplete"))
            break


def summarize(context, selected):
    signature_summary = summarize_signatures(context) if "signature" in selected else None
    notes = list(context.notes)
    for row in context.rows:
        notes.extend(row["observations"])
    for name in selected:
        attempted = [r for r in context.rows if r["test"] == name]
        if len(attempted) != TESTS[name].request_limit(context.options):
            notes.append(observation("test_incomplete", name + "未取得全部预期轮次", "incomplete"))
    if context.reference and any(r["test"] == "conversation" and r.get("comparison") is None for r in context.rows):
        notes.append(observation("reference_incomplete", "部分对话没有完成实时参照", "incomplete"))
    incomplete = bool(context.skips or any(n["level"] == "incomplete" for n in notes))
    warnings = any(n["level"] == "warning" for n in notes)
    status = "incomplete" if incomplete else "observations" if warnings else "completed"
    comparisons = [r["comparison"] for r in context.rows if r["comparison"] and r["comparison"]["status"] != "unavailable"]
    return {"schema_version": 1, "started_at": context.started_at, "finished_at": utc_now(),
            "model": context.options.model, "url": context.client.url,
            "reference_url": context.reference.url if context.reference else None,
            "tests": selected, "status": status, "exit_code": 3 if incomplete else 1 if warnings else 0,
            "target_requests": sum(r.get("endpoint_role", "A") == "A" for r in context.rows),
            "peer_requests": sum(r.get("endpoint_role") == "B" for r in context.rows), "reference_requests": context.reference_attempts,
            "successful_requests": sum(r["result"]["success"] for r in context.rows),
            "compared_requests": len(comparisons), "input_quantity_evaluated": bool(comparisons),
            "tolerance_tokens": context.options.tolerance_tokens, "tolerance_ratio": context.options.tolerance_ratio,
            "skipped_tests": context.skips, "observations": notes, "rows": context.rows,
            "signature": signature_summary, "concurrency": context.concurrency_summary,
            "signature_freshness": context.freshness_summary, "signature_relay": context.relay_summary}


def request_id(result):
    ids = result.get("ids", {})
    return ids.get("request-id") or ids.get("x-request-id") or ids.get("x-oneapi-request-id") or result.get("message_id") or "未返回"


def report_text(summary):
    labels = {"completed": "已完成所选检查", "observations": "已完成，存在需要核验的偏差", "incomplete": "存在未完成或无法判断的检查"}
    fmt = lambda value: "未取得" if value is None else str(value)
    lines = ["# 渠道Token、缓存、签名与并发诊断", "", "结果：" + labels[summary["status"]],
             "模型：" + summary["model"], "时间（UTC）：" + summary["started_at"] + " 至 " + summary["finished_at"],
             "目标请求：" + str(summary["target_requests"]) + "，实时参照请求：" + str(summary["reference_requests"]), ""]
    if any(name in ("conversation", "fingerprint") for name in summary["tests"]):
        lines.extend(["总输入 = 普通输入 + 缓存读取 + 缓存创建。命中率 = 缓存读取 / 总输入。衔接差 = 本轮读取 - 上轮（读取 + 创建）。",
                      "固定指纹仅用于相同模型、完全相同请求的输入总量比较，不能证明模型身份或恶意注水。固定快照可能已经预热，缓存衔接只检查独立前缀的真实对话。",
                      "容差 = max(" + str(summary["tolerance_tokens"]) + " Token，参照输入的" + format(summary["tolerance_ratio"], ".2%") + ")。计量偏差不等同于实际账本扣费差额。", "",
                      "| 测试项 | 轮次 | 普通输入 | 缓存读取 | 缓存创建 | 总输入 | 参照总输入 | 倍数 | 命中率 | 衔接差 | 结果 |",
                      "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|"])
    for row in summary["rows"]:
        if row["test"] not in ("conversation", "fingerprint"):
            continue
        r, comp, carry = row["result"], row.get("comparison") or {}, row.get("cache_check") or {}
        usage = r["usage"]
        values = [row["test"], row["round"], usage.get("input_tokens"), usage.get("cache_read_input_tokens"),
                  usage.get("cache_creation_input_tokens"), r["total_input_tokens"], comp.get("expected_total"),
                  format(comp["ratio"], ".4f") if comp.get("ratio") is not None else None,
                  format(r["cache_hit_rate"], ".2%") if r["cache_hit_rate"] is not None else None,
                  carry.get("delta"), "完成" if r["success"] else "失败或拒绝"]
        lines.append("| " + " | ".join(fmt(x).replace("|", "\\|") for x in values) + " |")
    if summary["signature"]:
        lines.extend(signature_report(summary))
    if summary.get("concurrency"):
        lines.extend(concurrency_report(summary["concurrency"]))
    lines.extend(freshness_report(summary))
    lines.extend(relay_report(summary))
    lines.extend(["", "## 观察", ""])
    if not summary["input_quantity_evaluated"] and any(name in ("conversation", "fingerprint") for name in summary["tests"]):
        lines.append("- 本次未取得有效的输入总量参照，只能查看缓存和响应状态。")
    for message in dict.fromkeys(n["message"] for n in summary["observations"]):
        lines.append("- " + message.replace("\n", " "))
    for skipped in summary["skipped_tests"]:
        lines.append("- " + skipped["test"] + "：" + skipped["reason"])
    lines.extend(["", "## 请求定位", "", "| 测试项 | 轮次 | 开始时间（UTC） | 请求或消息ID |", "|---|---:|---|---|"])
    for row in summary["rows"]:
        lines.append("| " + row["test"] + " | " + str(row["round"]) + " | " + row["result"]["started_at"] + " | " + str(request_id(row["result"])) + " |")
    return "\n".join(lines) + "\n"


def write_reports(context, summary):
    clean = redact(summary, context.secrets)
    (context.directory / "summary.json").write_text(json.dumps(clean, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    (context.directory / "report.md").write_text(report_text(clean), encoding="utf-8")
    with (context.directory / "rounds.csv").open("w", encoding="utf-8-sig", newline="") as handle:
        writer = csv.writer(handle)
        writer.writerow(["测试项", "轮次", "开始时间UTC", "普通输入", "缓存读取", "缓存创建", "总输入", "参照总输入", "输入倍数", "缓存命中率", "衔接差", "HTTP", "结束原因", "请求ID", "观察"])
        for row in clean["rows"]:
            if row["test"] not in ("conversation", "fingerprint"):
                continue
            r, comp, carry = row["result"], row.get("comparison") or {}, row.get("cache_check") or {}
            values = [row["test"], row["round"], r["started_at"], *(r["usage"].get(k) for k in ("input_tokens", "cache_read_input_tokens", "cache_creation_input_tokens")), r["total_input_tokens"], comp.get("expected_total"), comp.get("ratio"), r["cache_hit_rate"], carry.get("delta"), r["http_status"], r["stop_reason"], request_id(r), "；".join(n["message"] for n in row["observations"])]
            # 上游错误及ID可能含电子表格公式前缀，只转义文本，保留数值类型。
            writer.writerow(["'" + v if isinstance(v, str) and v.startswith(("=", "+", "-", "@", "\t", "\r")) else v for v in values])
    if clean["signature"]:
        with (context.directory / "signature-cases.csv").open("w", encoding="utf-8-sig", newline="") as handle:
            writer = csv.writer(handle)
            for values in signature_csv_rows(clean):
                writer.writerow(["'" + v if isinstance(v, str) and v.startswith(("=", "+", "-", "@", "\t", "\r")) else v for v in values])
    if clean.get("concurrency"):
        with (context.directory / "concurrency.csv").open("w", encoding="utf-8-sig", newline="") as handle:
            writer = csv.writer(handle)
            writer.writerow(["序号", "开始时间UTC", "HTTP", "接口正常结束", "完整正文成功", "分类", "失败阶段",
                             "首字秒", "完整响应秒", "观察耗时秒", "响应头秒", "建连秒", "结束原因", "请求ID", "错误"])
            for row in sorted((r for r in clean["rows"] if r["test"] == "concurrency"), key=lambda r: r["round"]):
                result = row["result"]
                values = [row["round"], result["started_at"], result["http_status"], result["protocol_success"],
                          result["success"], result["failure_kind"], result["stage"], result["first_text_seconds"],
                          result["terminal_seconds"], result["elapsed_seconds"], result["headers_seconds"],
                          result["connect_seconds"], result["stop_reason"], request_id(result), result["error"]]
                writer.writerow(["'" + v if isinstance(v, str) and v.startswith(("=", "+", "-", "@", "\t", "\r")) else v for v in values])
