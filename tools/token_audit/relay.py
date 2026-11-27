"""工具结果续接、原题绑定对照与可选双向跨渠道回放。"""
import copy
import secrets
import uuid

from .freshness import catalog, model_matches
from .protocol import MessagesClient
from .signatures import explicit_signature_error


def request_limit(options):
    return 10 if options.peer_url else 4


def outcome(result, model, code):
    if result.get("secret_echo"):
        return "unknown"
    if result.get("http_status") == 400 and explicit_signature_error(result):
        return "rejected_signature"
    if (result.get("protocol_success") and result.get("stop_reason") == "end_turn"
            and model_matches(model, result.get("response_model")) and result.get("text", "").strip() == code):
        return "accepted"
    return "unknown"


def run_relay(context):
    options = context.options
    clients = {"A": context.client}
    if options.peer_url:
        clients["B"] = MessagesClient(options.peer_url, options.peer_key, options.timeout, options.peer_auth)
    sources, controls, cases, directories = {}, {}, [], {}

    def call(source, target, case, body, session, code=None):
        detail = {"source": source, "target": target, "direction": source + " → " + target,
                  "case": case, "code": code, "outcome": "unknown"}
        number = 1 + sum(r["test"] == "signature_relay" for r in context.rows)
        row = context.call("signature_relay", number, body, session, capture_content=True,
                           check_usage=False, details={"relay": detail}, client=clients[target], endpoint_role=target)
        if code:
            detail["outcome"] = outcome(row["result"], body["model"], code)
        cases.append(detail)
        return row

    try:
        for label in clients:
            if context.stop_reason:
                break
            directory = catalog(clients[label])
            directories[label] = directory
            context.emit("model_catalog", endpoint_role=label, catalog=directory)
            if any(status in (401, 403, 429) for status in directory["http_statuses"]):
                context.stop_reason = label + "模型目录认证失败或限流，停止后续请求"
                break
            if directory["status"] == "complete" and options.model not in directory["models"]:
                context.note("relay_model_absent", label + "完整模型目录中缺少" + options.model)
                continue
            session = str(uuid.uuid4())
            body = {"model": options.model, "max_tokens": options.signature_max_tokens, "stream": True,
                    "thinking": {"type": "adaptive", "display": "summarized"}, "output_config": {"effort": "medium"},
                    "tools": [{"name": "arithmetic_check", "description": "接收余数并返回本次测试的六位数字编号",
                               "input_schema": {"type": "object", "properties": {"remainder": {"type": "integer"}},
                                                "required": ["remainder"], "additionalProperties": False}}],
                    "messages": [{"role": "user", "content": "求19^607除以223的余数，调用arithmetic_check，收到结果后只输出其中的六位编号，不要其他文字。题号 " + secrets.token_hex(8)}]}
            row = call(label, label, "seed", body, session)
            result = row["result"]
            blocks = result.get("assistant_content", [])
            tools = [b for b in blocks if b.get("type") == "tool_use"]
            valid = (result.get("protocol_success") and result.get("content_complete") and not result.get("secret_echo")
                     and result.get("stop_reason") == "tool_use" and model_matches(options.model, result.get("response_model"))
                     and any(b.get("type") == "thinking" and b.get("signature") for b in blocks)
                     and len(tools) == 1 and tools[0].get("name") == "arithmetic_check" and tools[0].get("id")
                     and type(tools[0].get("input", {}).get("remainder")) is int)
            row["relay"]["outcome"] = "source_valid" if valid else "unknown"
            if not valid:
                context.note("relay_source_invalid", label + "未取得完整的带签名工具调用")
                continue
            code = str(secrets.randbelow(900000) + 100000)
            baseline = copy.deepcopy(body)
            baseline["messages"].extend([{"role": "assistant", "content": copy.deepcopy(blocks)},
                {"role": "user", "content": [{"type": "tool_result", "tool_use_id": tools[0]["id"], "content": code}]}])
            sources[label] = (baseline, session, code)
        for label, (body, session, code) in sources.items():
            if context.stop_reason:
                break
            controls[label] = [call(label, label, "original_before", body, session, code)["relay"]["outcome"]]
        for label, (body, session, code) in sources.items():
            if context.stop_reason:
                break
            if controls.get(label) != ["accepted"]:
                continue
            changed = copy.deepcopy(body)
            changed["messages"][0]["content"] = changed["messages"][0]["content"].replace("19^607", "23^601")
            call(label, label, "changed_question", changed, session, code)
        if len(sources) == 2 and all(controls.get(label) == ["accepted"] for label in clients):
            for source, target in (("A", "B"), ("B", "A")):
                if context.stop_reason:
                    break
                body, session, _ = sources[source]
                body = copy.deepcopy(body)
                code = str(secrets.randbelow(900000) + 100000)
                body["messages"][-1]["content"][0]["content"] = code
                call(source, target, "cross", body, session, code)
        for label, (body, session, code) in sources.items():
            if context.stop_reason:
                break
            controls.setdefault(label, []).append(call(label, label, "original_after", body, session, code)["relay"]["outcome"])
    finally:
        valid = {label: controls.get(label) == ["accepted", "accepted"] for label in clients}
        for item in cases:
            item["controls_valid"] = valid[item["source"]] and valid[item["target"]]
            if item["case"] == "changed_question" and item["controls_valid"] and item["outcome"] == "accepted":
                context.note("relay_binding_accepted", item["direction"] + "修改原题后仍正确回显编号，未表现出该前缀绑定拒绝", "warning")
            if item["outcome"] == "unknown" or not item["controls_valid"]:
                context.note("relay_unconfirmed", item["direction"] + "的" + item["case"] + "无法判定或原样对照无效")
        context.relay_summary = {"controls": valid, "cases": cases, "catalogs": directories, "peer_url": options.peer_url,
                                 "note": "跨渠道通过或拒签只描述兼容性，不证明实际AWS来源；refusal不计作验签拒绝。"}
        context.emit("relay_summary", relay=context.relay_summary)


def report_lines(summary):
    data = summary.get("signature_relay")
    if not data:
        return []
    names = {"seed": "取得来源", "original_before": "前置原样对照", "original_after": "后置原样对照",
             "changed_question": "只改原题", "cross": "跨渠道续接"}
    labels = {"source_valid": "有效来源", "accepted": "正确回显编号", "rejected_signature": "HTTP400拒签", "unknown": "无法确认"}
    lines = ["", "## 工具续接与会话绑定", "", data["note"], "", "| 方向 | 案例 | 结果 | 原样对照有效 |", "|---|---|---|---|"]
    for item in data["cases"]:
        lines.append("| " + " | ".join([item["direction"], names[item["case"]], labels[item["outcome"]], str(item["controls_valid"])]) + " |")
    return lines
