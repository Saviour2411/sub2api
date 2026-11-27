"""思考签名的来源采集、受控回放、结果分类与报告。"""
from __future__ import annotations

from collections import Counter
import copy
from datetime import datetime
import json
from pathlib import Path
import re
import uuid

from .protocol import canonical_hash
from .signature_cases import CASES, CORE_CASES, digest, make_case

CONTROLS = ("original_before", "original_mid", "original_after")
LABELS = {name: label for name, label, _ in CASES}
LABELS.update(seed="获取独立签名", original_before="前置原样对照",
              original_mid="中途原样对照", original_after="后置原样对照")
OUTCOME_LABELS = {"source_valid": "有效来源", "source_invalid": "来源不可用",
                  "accepted": "完整接受", "rejected_http_400": "HTTP400明确拒绝",
                  "rejected_stream_error": "流内明确拒绝", "inconclusive": "无法判断"}
DEFINITIVE = {"accepted", "rejected_http_400", "rejected_stream_error"}
BUILTIN_SIGNATURE_BASELINE = Path(__file__).parent / "baselines" / "signature-claude-opus-5-5.20261122.json"


def load_signature_baseline(path=None):
    path = Path(path or BUILTIN_SIGNATURE_BASELINE)
    if path.stat().st_size > 256 * 1024:
        raise ValueError("签名参照文件超过256KiB")
    data = json.loads(path.read_text(encoding="utf-8"))
    if (not isinstance(data, dict) or data.get("schema_version") != 1 or data.get("test") != "signature"
            or not all(isinstance(data.get(key), str) and data[key].strip() for key in ("id", "model", "sampled_at"))):
        raise ValueError("签名参照文件格式无效")
    if datetime.fromisoformat(data["sampled_at"]).tzinfo is None:
        raise ValueError("签名参照采样时间必须包含时区")
    cases = data.get("cases")
    if not isinstance(cases, dict) or set(cases) != {"append"}:
        raise ValueError("当前签名参照必须包含末尾追加字符案例")
    case = cases["append"]
    if (not isinstance(case, dict) or case.get("expected_outcome") != "rejected_http_400"
            or type(case.get("expected_http_status")) is not int or case["expected_http_status"] != 400):
        raise ValueError("末尾追加字符参照必须为HTTP400明确拒绝")
    samples = case.get("samples")
    if not isinstance(samples, list) or len(samples) != 4:
        raise ValueError("签名参照必须包含4份有效实测样本")
    identities = set()
    for sample in samples:
        if (not isinstance(sample, dict) or sample.get("scenario") not in ("chat", "tool")
                or type(sample.get("sample")) is not int or sample["sample"] not in (1, 2)
                or sample.get("controls_valid") is not True or sample.get("outcome") != case["expected_outcome"]
                or sample.get("http_status") != case["expected_http_status"]):
            raise ValueError("签名参照样本或原样对照无效")
        for key in ("source_request_sha256", "request_sha256"):
            if not isinstance(sample.get(key), str) or not re.fullmatch(r"[0-9a-f]{64}", sample[key]):
                raise ValueError("签名参照请求SHA256无效")
        identities.add((sample["scenario"], sample["sample"]))
    if identities != {(scenario, sample) for scenario in ("chat", "tool") for sample in (1, 2)}:
        raise ValueError("签名参照缺少独立场景或样本")
    return data


def compare_signature_reference(context, rows):
    baseline = getattr(context.options, "signature_baseline_data", None)
    if baseline is None:
        return None
    matched_model = context.options.model == baseline["model"]
    result = {"id": baseline["id"], "sampled_at": baseline["sampled_at"], "model": baseline["model"],
              "status": "historical_reference" if matched_model else "unavailable_model", "cases": {},
              "note": "历史行为对照；来源请求参数和原样对照有效时才比较，编码边界差异不证明模型身份或密码学强弱。"}
    sources = {}
    for row in rows:
        detail = row["signature"]
        if detail["case"] == "seed":
            sources[(detail["scenario"], detail["sample"])] = canonical_hash(
                {key: value for key, value in row["request"].items() if key != "metadata"})
    for name, reference in baseline["cases"].items():
        expected = reference["expected_outcome"]
        reference_sources = {sample["source_request_sha256"] for sample in reference["samples"]}
        compared = []
        for row in rows:
            detail = row["signature"]
            if detail["case"] != name:
                continue
            if not matched_model:
                status = "unavailable_model"
            elif sources.get((detail["scenario"], detail["sample"])) not in reference_sources:
                status = "unavailable_source_parameters"
            elif not detail.get("controls_valid"):
                status = "unavailable_controls"
            elif detail.get("outcome") not in DEFINITIVE:
                status = "inconclusive"
            else:
                status = "match" if (detail["outcome"] == expected and row["result"]["http_status"] == reference["expected_http_status"]) else "different"
            detail["reference_comparison"] = {"reference_id": baseline["id"], "expected_outcome": expected,
                                               "expected_http_status": reference["expected_http_status"], "status": status}
            compared.append({"round": row["round"], "scenario": detail["scenario"], "sample": detail["sample"],
                             "outcome": detail["outcome"], "status": status})
        counts = Counter(row["status"] for row in compared)
        expected_samples = len(context.options.signature_scenarios) * context.options.signature_samples
        result["cases"][name] = {"expected_outcome": expected, "expected_http_status": reference["expected_http_status"],
            "reference_samples": len(reference["samples"]), "expected_samples": expected_samples,
            "evaluated_samples": counts["match"] + counts["different"], "matched_samples": counts["match"],
            "different_samples": counts["different"], "unassessed_samples": expected_samples - counts["match"] - counts["different"],
            "rows": compared}
        if counts["different"]:
            context.note("signature_reference_mismatch", LABELS[name] + "有" + str(counts["different"]) +
                         "次与可信参照不同；参照为HTTP400明确拒绝，具体结果单列，不计入核心篡改统计。", "warning")
    return result


def signature_request_limit(options):
    return len(options.signature_scenarios) * options.signature_samples * (1 + len(CASES) + len(CONTROLS))


def error_text(result):
    value = result.get("upstream_error")
    if value:
        return json.dumps(value, ensure_ascii=False) if not isinstance(value, str) else value
    return result.get("error_excerpt") or result.get("error") or ""


def explicit_signature_error(result):
    error = result.get("upstream_error")
    if isinstance(error, dict) and error.get("type") in (
        "authentication_error", "permission_error", "rate_limit_error", "overloaded_error", "api_error"
    ):
        return False
    text = error_text(result).lower().replace("\x60", "")
    patterns = (
        r"(?:invalid|missing|required|empty|malformed|incorrect)\s+(?:thinking\s+)?signature",
        r"signature.{0,100}(?:invalid|missing|required|must be|must have|cannot be empty|not valid|valid string|verification failed|mismatch)",
        r"签名.{0,60}(?:无效|错误|缺失|必填|不能为空|验证失败|校验失败)",
    )
    return any(re.search(pattern, text) for pattern in patterns)


def classify(result, model):
    if result.get("secret_echo"):
        return "inconclusive"
    if result["protocol_success"] and result.get("success") and result.get("response_model") == model:
        return "accepted"
    if result["http_status"] == 400 and explicit_signature_error(result):
        return "rejected_http_400"
    if result["http_status"] == 200 and result.get("response_format") == "sse" and result.get("upstream_error") and explicit_signature_error(result):
        return "rejected_stream_error"
    return "inconclusive"


def make_seed(model, scenario, sample, session, max_tokens):
    problems = (
        "求最小正整数n，使其除以7余3、除以11余5、除以13余7，再计算37乘以n加19。",
        "求最小正整数n，使其除以5余2、除以7余4、除以11余6，再计算29乘以n加17。",
        "求最小正整数n，使其除以7余2、除以11余4、除以13余6，再计算31乘以n加23。",
        "求最小正整数n，使其除以5余1、除以11余7、除以13余9，再计算41乘以n加11。",
        "求最小正整数n，使其除以7余1、除以11余3、除以13余5，再计算43乘以n加13。",
    )
    body = {"model": model, "max_tokens": max_tokens, "stream": True,
            "thinking": {"type": "adaptive", "display": "summarized"},
            "output_config": {"effort": "max" if scenario == "tool" else "high"},
            "system": "你是严谨的数学助手。本题需要多步推理，请认真核验计算后给出简洁答案。",
            "metadata": {"user_id": json.dumps({"device_id": digest(session), "account_uuid": "", "session_id": session}, separators=(",", ":"))}}
    prompt = problems[sample - 1] + "最终只给出n和表达式结果。"
    if scenario == "tool":
        body["system"] = "你是严谨的数学与服务诊断助手。先解决约束问题并核验计算，再用结果决定工具参数。只调用一次查询工具，收到工具结果后用一句话报告失败率。"
        body["tool_choice"] = {"type": "auto"}
        body["tools"] = [{"name": "query_probe_metrics", "description": "查询虚构测试账号最近三天的成功和失败请求数，只读合成数据。",
                          "input_schema": {"type": "object", "properties": {"account_id": {"type": "string"}},
                                           "required": ["account_id"], "additionalProperties": False}}]
        prompt = problems[sample - 1] + "先算出n，将query_probe_metrics的account_id设为字符串sim-后接这个n，调用工具查询该虚构账号。工具返回后计算三天合计失败率并判断是否超过1%。请认真核验余数约束后再调用工具，不要猜测账号编号。"
    body["messages"] = [{"role": "user", "content": [{"type": "text", "text": prompt}]}]
    return body


def source_problem(result, scenario, model):
    if not result["protocol_success"] or not result.get("content_complete") or result.get("secret_echo"):
        return "来源响应未完整结束或内容无法无损回传"
    if result.get("response_model") != model:
        return "来源响应模型与请求模型不同"
    blocks = result.get("assistant_content", [])
    thinking = [block for block in blocks if block.get("type") == "thinking"]
    if len(thinking) != 1 or not isinstance(thinking[0].get("thinking"), str) or not thinking[0].get("signature"):
        return "未取得唯一、可回传且带有效字段的思考签名块"
    expected = "tool_use" if scenario == "tool" else "end_turn"
    if result.get("stop_reason") != expected:
        return "来源结束原因不是" + expected
    calls = [b for b in blocks if b.get("type") == "tool_use"]
    if scenario == "tool":
        if (len(calls) != 1 or calls[0].get("name") != "query_probe_metrics" or not calls[0].get("id") or
                not isinstance(calls[0].get("input", {}).get("account_id"), str)):
            return "来源未返回唯一且格式完整的预期工具调用"
    elif calls or not result.get("text", "").strip():
        return "普通对话来源缺少可见答案或意外请求工具"
    return None


def continuation(seed, result, scenario, sample):
    body = copy.deepcopy(seed)
    body["messages"].append({"role": "assistant", "content": copy.deepcopy(result["assistant_content"])})
    if scenario == "tool":
        call = next(b for b in result["assistant_content"] if b["type"] == "tool_use")
        fixture = {"account_id": call["input"]["account_id"], "days": [
            {"success": 990, "failed": 10}, {"success": 980, "failed": 20}, {"success": 995, "failed": 5 + sample}]}
        blocks = [{"type": "tool_result", "tool_use_id": call["id"], "content": json.dumps(fixture, ensure_ascii=False, separators=(",", ":"))}]
    else:
        blocks = [{"type": "text", "text": "请用两个等式核验你上轮的n对前两个除数的余数，保持简短。"}]
    body["messages"].append({"role": "user", "content": blocks})
    return body


def run_signature(context):
    options = context.options
    seen = set()

    def call(scenario, sample, case, body, session, mutation=None):
        number = 1 + sum(r["test"] == "signature" for r in context.rows)
        details = {"scenario": scenario, "sample": sample, "case": case, "label": LABELS[case],
                   "category": "source" if case == "seed" else "original_control" if case in CONTROLS else mutation["category"],
                   "core": case in CORE_CASES, "mutation": mutation}
        context.say("[signature] " + scenario + "样本" + str(sample) + "：" + LABELS[case])
        row = context.call("signature", number, body, session, capture_content=True,
                           check_usage=False, details={"signature": details})
        if case == "seed":
            reason = source_problem(row["result"], scenario, options.model)
            details.update(outcome="source_invalid" if reason else "source_valid", reason=reason)
        else:
            details["outcome"] = classify(row["result"], options.model)
        context.emit("signature_classification", round=number, signature=details)
        return row

    def control(source, name):
        row = call(source["scenario"], source["sample"], name, source["baseline"], source["session"])
        valid = row["signature"]["outcome"] == "accepted"
        if not valid:
            context.note("signature_control_failed", source["scenario"] + "样本" + str(source["sample"]) + LABELS[name] + "失败，本样本不计入有效统计", row=row)
        return valid

    for scenario in options.signature_scenarios:
        sources = []
        for sample in range(1, options.signature_samples + 1):
            if context.stop_reason:
                return
            session = str(uuid.uuid4())
            seed = make_seed(options.model, scenario, sample, session, options.signature_max_tokens)
            batch = {"scenario": scenario, "sample": sample, "source_valid": False,
                     "independent": False, "skipped_cases": []}
            context.signature_samples.append(batch)
            row = call(scenario, sample, "seed", seed, session)
            batch["source_round"] = row["round"]
            if row["signature"]["outcome"] != "source_valid":
                context.note("signature_source_invalid", scenario + "样本" + str(sample) + "：" + row["signature"]["reason"], row=row)
                continue
            index, block = next((i, b) for i, b in enumerate(row["result"]["assistant_content"]) if b["type"] == "thinking")
            sha = digest(block["signature"])
            batch.update(signature_sha256=sha, signature_length=len(block["signature"]), thinking_characters=len(block["thinking"]))
            if sha in seen:
                row["signature"].update(outcome="source_invalid", reason="来源重复返回同一签名，不能作为独立样本")
                context.note("signature_duplicate_source", scenario + "样本" + str(sample) + "重复返回同一签名，不能作为独立样本", row=row)
                continue
            seen.add(sha)
            batch.update(source_valid=True, independent=True)
            sources.append({"scenario": scenario, "sample": sample, "session": session, "block": block,
                            "index": index, "batch": batch, "baseline": continuation(seed, row["result"], scenario, sample)})
        for source in sources:
            if context.stop_reason:
                return
            donor = next((other for other in sources if other is not source), None)
            source["batch"]["donor_sample"] = donor["sample"] if donor else None
            if not control(source, "original_before"):
                continue
            for position, (name, _, _) in enumerate(CASES):
                if context.stop_reason:
                    return
                if position == 10 and not control(source, "original_mid"):
                    break
                try:
                    body, mutation = make_case(source["baseline"], source["index"], donor["block"] if donor else None, name)
                except ValueError as exc:
                    source["batch"]["skipped_cases"].append({"case": name, "reason": str(exc)})
                    context.note("signature_case_skipped", scenario + "样本" + str(source["sample"]) + LABELS[name] + "无法执行：" + str(exc))
                    continue
                call(scenario, source["sample"], name, body, source["session"], mutation)
            else:
                if not context.stop_reason:
                    control(source, "original_after")


def summarize_signatures(context):
    rows = [r for r in context.rows if r["test"] == "signature"]
    samples = copy.deepcopy(context.signature_samples)
    for sample in samples:
        group = [r for r in rows if r["signature"]["scenario"] == sample["scenario"] and r["signature"]["sample"] == sample["sample"]]
        controls = {r["signature"]["case"]: r["signature"].get("outcome") for r in group if r["signature"]["case"] in CONTROLS}
        valid = sample["source_valid"] and sample["independent"] and all(controls.get(name) == "accepted" for name in CONTROLS)
        sample.update(controls=controls, controls_valid=valid)
        for row in group:
            detail = row["signature"]
            detail["controls_valid"] = valid
            detail["eligible_for_core_statistics"] = bool(valid and detail["core"] and detail.get("outcome") in DEFINITIVE)
    core = [r for r in rows if r["signature"]["core"]]
    eligible = [r for r in core if r["signature"].get("eligible_for_core_statistics")]
    counts = Counter(r["signature"]["outcome"] for r in eligible)
    expected_samples = len(context.options.signature_scenarios) * context.options.signature_samples
    expected_core = expected_samples * len(CORE_CASES)
    if counts["accepted"]:
        context.note("signature_mutation_accepted", "原样对照有效的核心变更中，有" + str(counts["accepted"]) + "次被完整接受；需要渠道核验过滤、重签、协议转换或验签策略，不能据此证明模型身份", "warning")
    if len(eligible) != expected_core:
        context.note("signature_incomplete", "签名核心变更可判定" + str(len(eligible)) + "/" + str(expected_core) + "次；原样对照失败、跳过或请求异常不计作成功验签")
    unknown = [r for r in rows if r["signature"].get("outcome") == "inconclusive"]
    if unknown:
        context.note("signature_request_inconclusive", "有" + str(len(unknown)) + "次签名请求无法判定；普通400、模型不一致、限流、服务器错误、空回复和断流均不视为明确拒绝")
    category_counts = {}
    for category in ("signature", "format", "binding"):
        category_counts[category] = dict(Counter(r["signature"]["outcome"] for r in eligible if r["signature"]["category"] == category))
    reference = compare_signature_reference(context, rows)
    return {"expected_samples": expected_samples, "valid_control_samples": sum(s["controls_valid"] for s in samples),
            "expected_core_cases": expected_core, "attempted_core_cases": len(core), "evaluated_core_cases": len(eligible),
            "accepted_core_cases": counts["accepted"], "http_400_rejected_core_cases": counts["rejected_http_400"],
            "stream_rejected_core_cases": counts["rejected_stream_error"], "unassessed_core_cases": expected_core - len(eligible),
            "categories": category_counts, "samples": samples,
            "outcomes": dict(Counter(r["signature"].get("outcome", "inconclusive") for r in rows)), "reference": reference}


def signature_csv_rows(summary):
    yield ["序号", "场景", "样本", "测试情况", "类别", "核心变更", "HTTP", "判定", "原样对照有效", "计入核心统计", "开始时间UTC", "请求ID", "消息ID", "请求SHA256", "变更路径", "解码载荷改变", "错误", "可信参照ID", "参照预期", "参照比较"]
    for row in summary["rows"]:
        if row["test"] != "signature":
            continue
        detail, result = row["signature"], row["result"]
        mutation = detail.get("mutation") or {}
        reference = detail.get("reference_comparison") or {}
        yield [row["round"], detail["scenario"], detail["sample"], detail["label"], detail["category"], detail["core"],
               result["http_status"], OUTCOME_LABELS.get(detail.get("outcome"), "无法判断"), detail.get("controls_valid", False),
               detail.get("eligible_for_core_statistics", False), result["started_at"], json.dumps(result.get("ids", {}), ensure_ascii=False),
               result.get("message_id"), result["request_sha256"], ";".join(mutation.get("changed_paths", [])),
               mutation.get("decoded_payload_changed"), detail.get("reason") or error_text(result),
               reference.get("reference_id"), OUTCOME_LABELS.get(reference.get("expected_outcome")), reference.get("status")]


def signature_report(summary):
    data = summary["signature"]
    lines = ["", "## 思考签名测试", "",
             "前、中、后原样对照全部成功的样本：" + str(data["valid_control_samples"]) + "/" + str(data["expected_samples"]) + "。",
             "核心变更：计划" + str(data["expected_core_cases"]) + "次，可判定" + str(data["evaluated_core_cases"]) +
             "次；完整接受" + str(data["accepted_core_cases"]) + "次，HTTP400明确拒绝" + str(data["http_400_rejected_core_cases"]) +
             "次，流内明确拒绝" + str(data["stream_rejected_core_cases"]) + "次，未判定" + str(data["unassessed_core_cases"]) + "次。",
             "核心统计包含签名内容、格式与绑定变更，分项见summary.json。格式拒绝本身不证明存在密码学验签。",
             "删除整个思考块、换用另一完整真实思考块及签名字符串末尾追加单独列为兼容性或编码边界，不计入核心统计；末尾追加字符可能被Base64解码器忽略。",
             "后置对照失败会使该样本此前的核心结果失效。接口接受篡改不等同于模型造假，也可能是网关删除、重签或转换了内容；响应模型名不证明模型身份。",
             "工具结果为本地合成数据，不执行模型发出的工具调用。逐项请求、错误与变更位置见signature-cases.csv和summary.json。"]
    reference = data.get("reference")
    if reference:
        lines.extend(["", "签名可信参照：" + reference["id"] + "；采样时间：" + reference["sampled_at"] + "。", reference["note"]])
        if reference["status"] == "unavailable_model":
            lines.append("本次模型与签名参照不同，不套用历史预期。")
        for name, case in reference["cases"].items():
            lines.append(LABELS[name] + "：可信实测" + str(case["reference_samples"]) + "/" + str(case["reference_samples"]) +
                         "为HTTP400明确拒绝；本次一致" + str(case["matched_samples"]) + "次、不同" + str(case["different_samples"]) +
                         "次、未评估" + str(case["unassessed_samples"]) + "次。模型或来源参数不匹配、原样对照无效及无法判断的请求不计为一致。")
    lines.extend(["", "| 序号 | 场景 | 样本 | 测试情况 | HTTP | 判定 | 原样对照有效 |", "|---:|---|---:|---|---:|---|---|"])
    for values in signature_csv_rows(summary):
        if values[0] == "序号":
            continue
        cells = [values[i] for i in (0, 1, 2, 3, 6, 7, 8)]
        lines.append("| " + " | ".join(str(v).replace("|", r"\|").replace("\n", " ") if v is not None else "未取得" for v in cells) + " |")
    return lines
