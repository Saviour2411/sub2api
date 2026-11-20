"""单轮并发能力测试、延迟统计及绑定请求参数的历史可信参照。"""
from __future__ import annotations

from collections import Counter
from concurrent.futures import FIRST_COMPLETED, ThreadPoolExecutor, wait
import copy
from datetime import datetime
import hashlib
import json
import math
from pathlib import Path
import platform
import ssl
import threading
import time
import uuid

from .protocol import CLIENT_HEADERS, MessagesClient, canonical_hash, redact, utc_now

PROMPT = "请用中文写一段80至100字的说明，介绍排查HTTP API请求超时的基本步骤，涵盖客户端、网络、网关和上游服务。只输出正文，不要标题、列表、代码或额外解释。"
BUILTIN_BASELINE = Path(__file__).parent / "baselines" / "concurrency-claude-opus-5-5.20261119.json"
METRICS = ("first_text_seconds_success", "response_seconds_success")


def distribution(values):
    data = sorted(float(value) for value in values if value is not None)
    result = dict.fromkeys(("mean", "min", "max", "p50", "p90", "p95", "p99"))
    result["n"] = len(data)
    if not data:
        return result
    if any(not math.isfinite(value) or value < 0 for value in data):
        raise ValueError("延迟样本必须为非负有限数字")
    result.update(mean=sum(data) / len(data), min=data[0], max=data[-1])
    for percentile in (50, 90, 95, 99):
        position = (len(data) - 1) * percentile / 100
        lower, upper = math.floor(position), math.ceil(position)
        result[f"p{percentile}"] = data[lower] + (data[upper] - data[lower]) * (position - lower)
    return {key: round(value, 6) if isinstance(value, float) else value for key, value in result.items()}


def make_profile(model, concurrency, max_tokens, timeout, auth="x-api-key"):
    return {"version": 1, "request_template": {"model": model, "max_tokens": max_tokens,
            "stream": True, "messages": [{"role": "user", "content": PROMPT}]},
            "requests": concurrency, "concurrency": concurrency, "timeout_seconds": float(timeout),
            "connect_timeout_seconds": min(30.0, float(timeout)), "auth": auth,
            "http_version": "1.1", "reuse_connections": False,
            "headers": dict(CLIENT_HEADERS), "session_strategy": "unique_metadata_v1"}


def make_request(profile, session):
    body = copy.deepcopy(profile["request_template"])
    body["metadata"] = {"user_id": json.dumps({
        "device_id": hashlib.sha256(session.encode()).hexdigest(),
        "account_uuid": "", "session_id": session}, separators=(",", ":"))}
    return body


def load_baseline(path=None):
    def reject_constant(value):
        raise ValueError("参考文件不能包含非有限数字：" + value)

    data = json.loads(Path(path or BUILTIN_BASELINE).read_text(encoding="utf-8"), parse_constant=reject_constant)
    if (not isinstance(data, dict) or data.get("schema_version") != 1 or data.get("test") != "concurrency"
            or not isinstance(data.get("id"), str) or not data["id"]):
        raise ValueError("并发参考文件格式无效")
    origin = data.get("origin")
    if (not isinstance(data.get("sampled_at"), str) or not isinstance(origin, dict)
            or not isinstance(origin.get("label"), str) or not origin["label"].strip()):
        raise ValueError("并发参考缺少采样时间或测试来源")
    try:
        sampled_at = datetime.fromisoformat(data["sampled_at"])
        if sampled_at.tzinfo is None:
            raise ValueError("缺少时区")
    except ValueError as exc:
        raise ValueError("并发参考采样时间必须包含有效时区") from exc
    profile, metrics = data.get("profile"), data.get("metrics")
    if not isinstance(profile, dict) or not isinstance(metrics, dict):
        raise ValueError("并发参考缺少请求参数或统计")
    if data.get("profile_sha256") != canonical_hash(profile):
        raise ValueError("并发参考的请求参数SHA256不匹配")
    count = profile.get("requests")
    if type(count) is not int or not 1 <= count <= 200 or profile.get("concurrency") != count:
        raise ValueError("并发参考必须是一轮1至200请求的同步突发")
    if type(metrics.get("requests")) is not int or metrics["requests"] != count:
        raise ValueError("并发参考请求样本数不匹配")
    for field in ("successful_requests", "protocol_successful_requests"):
        if type(metrics.get(field)) is not int or not 0 <= metrics[field] <= count:
            raise ValueError("并发参考成功样本数无效")
    if metrics["successful_requests"] > metrics["protocol_successful_requests"]:
        raise ValueError("完整正文成功数不能大于接口正常结束数")
    for metric in METRICS:
        values = metrics.get(metric)
        if not isinstance(values, dict) or type(values.get("n")) is not int or values["n"] != metrics["successful_requests"]:
            raise ValueError("并发参考延迟样本数无效")
        ordered = [values.get(key) for key in ("min", "p50", "p90", "p95", "p99", "max")]
        if not values["n"]:
            if any(value is not None for value in ordered) or values.get("mean") is not None:
                raise ValueError("无有效样本时延迟必须为null")
        elif (any(type(value) not in (int, float) or not math.isfinite(value) or value < 0
                  for value in ordered + [values.get("mean")]) or ordered != sorted(ordered)
              or not ordered[0] <= values["mean"] <= ordered[-1]):
            raise ValueError("并发参考延迟统计无效")
    return data


def peak_in_flight(rows, sent_only=False):
    events = []
    for row in rows:
        if sent_only and row["request_sent_seconds"] is None:
            continue
        start = row["start_offset_seconds"] + (row["request_sent_seconds"] if sent_only else 0)
        events.extend(((start, 1), (row["end_offset_seconds"], -1)))
    current, peak = 0, 0
    for _, delta in sorted(events, key=lambda event: (event[0], -event[1])):
        current += delta
        peak = max(peak, current)
    return peak


def outcome(result):
    if result.get("secret_echo"):
        return "secret_echo"
    if result["success"]:
        return "success"
    if result.get("timed_out"):
        return "connection_timeout" if result["stage"] == "connect" else "response_timeout"
    status = result["http_status"]
    if status is not None and status != 200:
        return "http_" + str(status)
    if result.get("upstream_error"):
        return "sse_upstream_error"
    if status == 200 and not result["terminal_complete"]:
        return "incomplete_stream" if result.get("response_format") == "sse" else "invalid_response"
    if result.get("error"):
        return "transport_error"
    if result["response_model"] != result["requested_model"]:
        return "model_mismatch"
    if result["stop_reason"] == "max_tokens":
        return "budget_stopped"
    if not result["text"].strip():
        return "empty_text"
    return "stop_reason_" + str(result["stop_reason"])


def metrics_from_rows(rows, elapsed):
    good = [row for row in rows if row["success"]]
    count = len(rows)
    starts = [row["start_offset_seconds"] for row in rows]
    sends = [row["start_offset_seconds"] + row["request_sent_seconds"]
             for row in rows if row["request_sent_seconds"] is not None]
    protocol_count = sum(row["protocol_success"] for row in rows)
    return {"requests": count, "successful_requests": len(good),
            "protocol_successful_requests": protocol_count,
            "success_rate": len(good) / count if count else None,
            "protocol_success_rate": protocol_count / count if count else None,
            "failure_counts": dict(Counter(row["failure_kind"] for row in rows if not row["success"])),
            "http_status_counts": dict(Counter(str(row["http_status"]) for row in rows)),
            "response_model_counts": dict(Counter(str(row["response_model"]) for row in rows)),
            "response_format_counts": dict(Counter(row.get("response_format", "none") for row in rows)),
            "peak_in_flight": peak_in_flight(rows), "peak_sent_in_flight": peak_in_flight(rows, True),
            "sent_requests": len(sends), "start_spread_seconds": max(starts) - min(starts) if starts else None,
            "send_spread_seconds": max(sends) - min(sends) if sends else None,
            "wave_elapsed_seconds": elapsed, "successful_requests_per_second": len(good) / elapsed if elapsed else None,
            "first_text_seconds_success": distribution(row["first_text_seconds"] for row in good),
            "response_seconds_success": distribution(row["terminal_seconds"] for row in good),
            "first_text_seconds_all_observed": distribution(row["first_text_seconds"] for row in rows),
            "observed_seconds_all": distribution(row["elapsed_seconds"] for row in rows),
            "failed_request_seconds": distribution(row["elapsed_seconds"] for row in rows if not row["success"]),
            "connect_seconds_all": distribution(row["connect_seconds"] for row in rows),
            "headers_seconds_all": distribution(row["headers_seconds"] for row in rows),
            "output_tokens_success": distribution(row["usage"].get("output_tokens") for row in good)}


def compare_baseline(metrics, profile, baseline, origin):
    comparison = {"reference_id": baseline["id"], "reference_sampled_at": baseline["sampled_at"],
                  "reference_origin": baseline.get("origin"), "current_origin": origin,
                  "same_origin_declared": origin == baseline.get("origin", {}).get("label"),
                  "status": "historical_reference", "latency_ratios": {},
                  "note": "历史可信渠道样本，不是固定性能门槛；环境、时段、思考量及输出长度会影响延迟，不能据此判断模型真假或Token注水。"}
    if canonical_hash(profile) != baseline["profile_sha256"]:
        comparison.update(status="configuration_mismatch", reason="模型、并发、提示词、输出预算、认证或超时等参数不同，不计算性能倍数")
        return comparison
    reference = baseline["metrics"]
    comparison["success_rate_delta_pp"] = (metrics["success_rate"] - reference["successful_requests"] / reference["requests"]) * 100
    for name in METRICS:
        comparison["latency_ratios"][name] = {}
        for percentile in ("p50", "p90", "p95"):
            current, old = metrics[name][percentile], reference[name][percentile]
            comparison["latency_ratios"][name][percentile] = current / old if current is not None and old else None
    return comparison


def run_concurrency(context):
    options = context.options
    profile = make_profile(options.model, options.concurrency, options.load_max_tokens, options.timeout, options.auth)
    baseline = options.load_baseline_data
    run_id = uuid.uuid4()
    client = MessagesClient(context.client.url, context.client.key, options.timeout, options.auth,
                            ssl_context=ssl.create_default_context(), connect_timeout=min(30.0, options.timeout))
    barrier = threading.Barrier(options.concurrency + 1)
    clock = {}
    rows = []
    context.emit("concurrency_plan", profile=profile, origin=options.load_origin,
                 profile_sha256=canonical_hash(profile), reference_id=baseline["id"])
    context.say("[concurrency] 同步发起" + str(options.concurrency) + "请求；不重试，连接阶段上限30秒")

    def worker(index):
        session = str(uuid.uuid5(run_id, str(index)))
        body = make_request(profile, session)
        barrier.wait(timeout=60)
        started = time.perf_counter()
        result = client.generate(body, session)
        result.update(sequence=index, requested_model=options.model, client_request_id=session,
                      start_offset_seconds=started - clock["start"],
                      end_offset_seconds=started - clock["start"] + result["elapsed_seconds"])
        result["success"] = bool(result["success"] and result["response_model"] == options.model)
        if redact(result, context.secrets) != result:
            result.update(success=False, protocol_success=False, secret_echo=True,
                          error="上游响应回显密钥，结果已脱敏，后续测试停止")
        result["failure_kind"] = outcome(result)
        return {"test": "concurrency", "round": index, "session_id": session, "request": body,
                "result": redact(result, context.secrets), "cache_check": None,
                "comparison": None, "observations": []}

    with ThreadPoolExecutor(max_workers=options.concurrency) as pool:
        pending = {pool.submit(worker, index) for index in range(1, options.concurrency + 1)}
        clock["start"] = time.perf_counter()
        started_at = utc_now()
        barrier.wait(timeout=60)
        last_progress = 0.0
        while pending:
            done, pending = wait(pending, timeout=5, return_when=FIRST_COMPLETED)
            for future in done:
                row = future.result()
                rows.append(row["result"])
                context.rows.append(row)
                context.emit("target_result", test="concurrency", round=row["round"], result=row["result"])
            elapsed = time.perf_counter() - clock["start"]
            if elapsed - last_progress >= 10 or not pending:
                context.say("[concurrency] 已完成" + str(len(rows)) + "/" + str(options.concurrency)
                            + "，完整成功" + str(sum(row["success"] for row in rows)))
                last_progress = elapsed
    metrics = metrics_from_rows(rows, time.perf_counter() - clock["start"])
    context.concurrency_summary = {"started_at": started_at, "finished_at": utc_now(),
        "profile": profile, "profile_sha256": canonical_hash(profile), "origin": options.load_origin,
        "client_platform": platform.platform(), "python_version": platform.python_version(),
        "metrics": metrics, "baseline": baseline,
        "comparison": compare_baseline(metrics, profile, baseline, options.load_origin)}
    context.emit("concurrency_summary", summary=context.concurrency_summary)
    if metrics["successful_requests"] < metrics["requests"]:
        context.note("concurrency_outcomes", "并发请求未完整输出正文的分类：" + json.dumps(metrics["failure_counts"], ensure_ascii=False), "warning")
    if metrics["peak_sent_in_flight"] < options.concurrency:
        context.note("concurrency_arrival_peak", "客户端目标并发" + str(options.concurrency) + "，实际已发送在途峰值"
                     + str(metrics["peak_sent_in_flight"]) + "；包含建连或调度影响，不保证全部请求同时到达渠道", "warning")
    if metrics["response_format_counts"].get("json", 0):
        context.note("concurrency_non_streaming", "部分响应为JSON而非SSE；其首字只能在完整正文收到后计时，详见响应格式分布", "warning")
    if any(row.get("secret_echo") or row["http_status"] in (401, 403, 429) for row in rows):
        context.stop_reason = "并发轮出现认证失败、限流或密钥回显，停止后续测试项；本轮在途请求已完成统计"


def report_lines(summary):
    metrics, comparison, baseline = summary["metrics"], summary["comparison"], summary["baseline"]
    reference = baseline["metrics"]
    comparable = comparison["status"] == "historical_reference"
    lines = ["", "## 并发能力", "", "模式：一轮同步突发，独立新连接，不重试。测试来源：" + summary["origin"] + "。",
             "接口正常结束包含max_tokens预算停止；完整正文成功要求end_turn、非空正文及返回模型名匹配。延迟单位为秒，分位数仅统计完整成功请求。",
             "首字是第一个非空白可见文本，不包含响应头、ping和思考；完整响应以message_stop为终态。时间包含连接、网络、排队和生成。", "",
             "| 指标 | 本次 | 可信历史参考 |", "|---|---:|---:|"]
    for label, key in (("接口正常结束率", "protocol_successful_requests"), ("完整正文成功率", "successful_requests")):
        current = f"{metrics[key]}/{metrics['requests']}（{metrics[key]/metrics['requests']:.2%}）"
        old = f"{reference[key]}/{reference['requests']}（{reference[key]/reference['requests']:.2%}）" if comparable else "参数不匹配"
        lines.append("| " + label + " | " + current + " | " + old + " |")
    for name, label in zip(METRICS, ("首字", "完整响应")):
        for quantile in ("p50", "p90", "p95"):
            current, old = metrics[name][quantile], reference[name][quantile]
            lines.append("| " + label + " " + quantile.upper() + " | " + (f"{current:.3f}" if current is not None else "无样本")
                         + " | " + (f"{old:.3f}" if comparable and old is not None else "未比较") + " |")
    lines.extend(["", "客户端在途峰值=" + str(metrics["peak_in_flight"]) + "；已发送在途峰值=" + str(metrics["peak_sent_in_flight"])
                  + "；实际发送=" + str(metrics["sent_requests"]) + "；完成发送跨度=" + str(metrics["send_spread_seconds"]) + "秒。",
                  "未完整正文分类：" + json.dumps(metrics["failure_counts"], ensure_ascii=False) + "。",
                  "响应格式分布：" + json.dumps(metrics["response_format_counts"], ensure_ascii=False) + "。",
                  "参考：" + baseline["id"] + "；采样时间：" + baseline["sampled_at"] + "。",
                  comparison.get("reason", comparison["note"]),
                  "测试来源与可信样本相同的声明：" + ("是（仅按来源标签）" if comparison["same_origin_declared"] else "否或未标注；不能把差异全部归因于渠道") + "。"])
    return lines
