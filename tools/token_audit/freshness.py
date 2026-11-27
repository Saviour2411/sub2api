"""签名结构、签发时间窗口与可选Bedrock来源标记检查。"""
from __future__ import annotations

import base64
import hashlib
import http.client
import json
import re
import secrets
import time
import uuid
from datetime import datetime, timezone, timedelta
from urllib.parse import urlencode, urlsplit

from .protocol import CLIENT_HEADERS, check_public_peer

LABELS = {"pass": "通过", "fail": "不通过", "unknown": "无法确认", "skipped": "目录缺项"}


def varint(data, pos):
    value = 0
    for offset in range(10):
        if pos >= len(data):
            raise ValueError("截断的varint")
        byte = data[pos]
        pos += 1
        if offset == 9 and byte > 1:
            raise ValueError("varint超过64位")
        value |= (byte & 127) << (offset * 7)
        if byte < 128:
            return value, pos
    raise ValueError("varint过长")


def wire_fields(data):
    """只读取wire结构，不猜测未知字段语义；重复目标字段由调用方拒绝。"""
    fields, pos = {}, 0
    while pos < len(data):
        tag, pos = varint(data, pos)
        number, kind = tag >> 3, tag & 7
        if not 1 <= number < 2**29:
            raise ValueError("非法字段编号")
        if kind == 0:
            value, pos = varint(data, pos)
        elif kind in (1, 2, 5):
            size = 8 if kind == 1 else 4
            if kind == 2:
                size, pos = varint(data, pos)
            if size > len(data) - pos:
                raise ValueError("字段长度超过剩余载荷")
            value, pos = data[pos:pos + size], pos + size
        else:
            raise ValueError("不支持的wire类型")
        fields.setdefault(number, []).append((kind, value))
    return fields


def field(fields, number, kind):
    values = fields.get(number, [])
    if not values:
        return None
    if len(values) != 1 or values[0][0] != kind:
        raise ValueError("目标字段重复或wire类型不匹配")
    return values[0][1]


def decode_signature(signature):
    result = {"decode_status": "failed", "f2_f1_f2": None, "f2_f1_f3": None,
              "f2_f1_f21": None, "f3": None, "base64_layers": 0,
              "signature_sha256": hashlib.sha256(signature.encode()).hexdigest()}
    try:
        if not signature or len(signature) > 65536:
            raise ValueError("签名为空或超过64KiB")
        wire = signature.encode("ascii")
        for layer in range(2):
            wire = base64.b64decode(wire + b"=" * (-len(wire) % 4), altchars=b"-_", validate=True)
            result["base64_layers"] = layer + 1
            if layer or not re.fullmatch(rb"[A-Za-z0-9+/_-]+={0,2}", wire):
                break
        top = wire_fields(wire)
        result["f3"] = field(top, 3, 0)
        outer = field(top, 2, 2)
        inner = field(wire_fields(outer), 1, 2) if outer is not None else None
        nested = wire_fields(inner) if inner is not None else {}
        for number in (2, 3, 21):
            result["f2_f1_f" + str(number)] = field(nested, number, 0)
        result["decode_status"] = "decoded"
    except (ValueError, UnicodeError) as exc:
        result["decode_error"] = str(exc)
    return result


def model_matches(requested, response):
    return response == requested or (requested == "claude-opus-5-5" and response == "claude-opus-5.5")


def inspect_result(result, model):
    samples = []
    complete = (result.get("protocol_success") and result.get("content_complete")
                and result.get("stop_reason") == "end_turn" and result.get("text", "").strip()
                and model_matches(model, result.get("response_model")) and not result.get("secret_echo"))
    for index, block in enumerate(result.get("assistant_content", [])):
        if block.get("type") != "thinking" or not block.get("signature"):
            continue
        item = decode_signature(block["signature"])
        sent, received = result.get("request_sent_at"), result.get("signature_received_at", {}).get(str(index))
        stamp = item["f2_f1_f21"]
        marker = item["f2_f1_f2"] if complete and item["decode_status"] == "decoded" else None
        item.update(index=index, sent_at=sent, received_at=received, freshness="unknown",
                    awsb="pass" if marker == 1 else "unknown" if marker is None else "fail")
        if (complete and result.get("response_format") == "sse" and item["decode_status"] == "decoded"
                and stamp is not None and sent is not None and received is not None
                and result.get("clock_drift_seconds", 999) <= 1 and received >= sent):
            item["freshness"] = "pass" if int(sent) <= stamp <= int(received) else "fail"
            item["seconds_from_request"] = stamp - sent
        if stamp is not None:
            try:
                item["signed_at_beijing"] = datetime.fromtimestamp(stamp, timezone(timedelta(hours=8))).isoformat()
            except (ValueError, OverflowError, OSError):
                item["signed_at_beijing"] = None
        samples.append(item)
    return {"complete": bool(complete), "bedrock_message_prefix": str(result.get("message_id", "")).startswith("msg_bdrk_"),
            "request_model": model, "response_model": result.get("response_model"), "signatures": samples}


def catalog(client):
    """分页目录失败或不完整时不把已读部分误当完整目录。"""
    parsed, models, cursor, seen = urlsplit(client.url), set(), None, set()
    statuses = []
    try:
        for _ in range(10):
            cls = http.client.HTTPSConnection if parsed.scheme == "https" else http.client.HTTPConnection
            kwargs = {"timeout": min(client.timeout, 20)}
            if parsed.scheme == "https" and client.ssl_context is not None:
                kwargs["context"] = client.ssl_context
            conn = cls(parsed.hostname, parsed.port, **kwargs)
            try:
                conn.connect()
                check_public_peer(conn)
                headers = {**CLIENT_HEADERS, "Accept": "application/json"}
                headers["Authorization" if client.auth == "bearer" else "x-api-key"] = ("Bearer " if client.auth == "bearer" else "") + client.key
                path = parsed.path.removesuffix("/messages") + "/models"
                query = {"limit": 1000}
                if cursor:
                    query["after_id"] = cursor
                conn.request("GET", path + "?" + urlencode(query), headers=headers)
                response = conn.getresponse()
                statuses.append(response.status)
                if response.status != 200:
                    raise ValueError("目录HTTP " + str(response.status))
                data = response.read(2 * 1024 * 1024 + 1)
                if len(data) > 2 * 1024 * 1024:
                    raise ValueError("模型目录超过上限")
                data = json.loads(data)
                if not isinstance(data, dict) or not isinstance(data.get("data"), list):
                    raise ValueError("目录不是模型列表")
                ids = [v.get("id") for v in data["data"] if isinstance(v, dict)]
                if len(ids) != len(data["data"]) or any(not isinstance(v, str) or not v for v in ids):
                    raise ValueError("目录条目缺少模型ID")
                models.update(ids)
                if data.get("has_more") is False or ("has_more" not in data and data.get("object") == "list"):
                    return {"status": "complete", "models": sorted(models), "http_statuses": statuses}
                if data.get("has_more") is not True:
                    raise ValueError("目录缺少完整性标识")
                cursor = data.get("last_id") or (ids[-1] if ids else None)
                if not cursor or cursor in seen:
                    raise ValueError("目录分页游标不推进")
                seen.add(cursor)
            finally:
                conn.close()
        raise ValueError("目录超过10页，未确认完整")
    except (OSError, ValueError, http.client.HTTPException) as exc:
        return {"status": "unknown", "models": sorted(models), "http_statuses": statuses, "reason": str(exc)}


def request_limit(options):
    return len(options.freshness_models) * options.freshness_samples


def aggregate(rows, models, count, require_awsb, missing=()):
    output = []
    for model in models:
        found = [r["freshness"] for r in rows if r["freshness"]["request_model"] == model]
        signatures = [s for row in found for s in row["signatures"]]
        stamps = [row["signatures"][0]["f2_f1_f21"] for row in found if len(row["signatures"]) == 1]
        valid = len(found) == count and all(r["complete"] and len(r["signatures"]) == 1 for r in found)
        freshness = "unknown"
        if any(s["freshness"] == "fail" for s in signatures):
            freshness = "fail"
        elif valid and all(s["freshness"] == "pass" for s in signatures):
            freshness = "pass" if all(a < b for a, b in zip(stamps, stamps[1:])) else "fail"
        awsb = "fail" if any(s["awsb"] == "fail" for s in signatures) else "pass" if valid and signatures and all(s["awsb"] == "pass" for s in signatures) else "unknown"
        status = "fail" if freshness == "fail" or (require_awsb and awsb == "fail") else "pass" if freshness == "pass" and (not require_awsb or awsb == "pass") else "unknown"
        if model in missing:
            status = "skipped"
        output.append({"model": model, "samples": len(found), "expected_samples": count,
                       "freshness": freshness, "awsb": awsb, "status": status})
    return {"require_awsb": require_awsb, "models": output,
            "status": "fail" if any(m["status"] == "fail" for m in output) else "pass" if all(m["status"] == "pass" for m in output) else "unknown"}


def run_freshness(context):
    options = context.options
    context.say("[signature_freshness] 检查完整模型目录")
    directory = catalog(context.client)
    context.emit("model_catalog", catalog=directory)
    if any(status in (401, 403, 429) for status in directory["http_statuses"]):
        context.stop_reason = "模型目录认证失败或限流，停止后续请求"
    missing = []
    for model in options.freshness_models:
        if directory["status"] == "complete" and model not in directory["models"]:
            missing.append(model)
            context.note("freshness_model_absent", model + "不在完整模型目录中，跳过该模型")
            continue
        for sample in range(options.freshness_samples):
            if context.stop_reason:
                break
            if sample:
                time.sleep(options.freshness_interval)
            session = str(uuid.uuid4())
            body = {"model": model, "max_tokens": options.freshness_max_tokens, "stream": True,
                    "thinking": {"type": "adaptive", "display": "summarized"}, "output_config": {"effort": "medium"},
                    "messages": [{"role": "user", "content": "计算17^" + str(827 + sample * 2) + "除以83的余数，只输出整数。题号 " + secrets.token_hex(8)}]}
            number = 1 + sum(r["test"] == "signature_freshness" for r in context.rows)
            row = context.call("signature_freshness", number, body, session, capture_content=True, check_usage=False)
            row["freshness"] = inspect_result(row["result"], model)
            context.emit("freshness_result", round=number, freshness=row["freshness"])
    rows = [r for r in context.rows if r["test"] == "signature_freshness"]
    summary = aggregate(rows, options.freshness_models, options.freshness_samples, options.require_awsb, missing)
    summary["catalog"] = directory
    context.freshness_summary = summary
    for model in summary["models"]:
        if model["status"] != "pass":
            context.note("signature_freshness_" + model["status"], model["model"] + "：现签" + LABELS[model["freshness"]] +
                         ("；AWSB标记" + LABELS[model["awsb"]] if options.require_awsb else ""),
                         "warning" if model["status"] == "fail" else "incomplete")


def report_lines(summary):
    data = summary.get("signature_freshness")
    if not data:
        return []
    lines = ["", "## 现签与AWSB标记", "", "通过要求：至少两次完整流式响应的f21均在各自发出请求至收到完整签名的秒级窗口内，且逐轮递增。缺失、解码失败、拒答和断流均不算通过。",
             "可选AWSB检查仅按f2.f1.f2=1比对结构特征，不证明实际AWS调用归属。", "",
             "| 模型 | 样本 | 现签 | AWSB标记 | 综合 |", "|---|---:|---|---|---|"]
    for model in data["models"]:
        lines.append("| " + " | ".join([model["model"], str(model["samples"]) + "/" + str(model["expected_samples"]), LABELS[model["freshness"]], LABELS[model["awsb"]], LABELS[model["status"]]]) + " |")
    lines.extend(["", "| 模型/轮次 | 消息前缀 | f2.f1.f2 | f2.f1.f3 | 顶层f3 | f21 | 签发时间（北京时间） | 窗口判定 |", "|---|---|---|---|---|---|---|---|"])
    for row in summary["rows"]:
        if row["test"] != "signature_freshness":
            continue
        detail = row["freshness"]
        for item in detail["signatures"]:
            values = [detail["request_model"] + "/" + str(row["round"]), str(detail["bedrock_message_prefix"]), item["f2_f1_f2"], item["f2_f1_f3"], item["f3"], item["f2_f1_f21"], item.get("signed_at_beijing"), LABELS[item["freshness"]]]
            lines.append("| " + " | ".join(str(v) if v is not None else "缺失" for v in values) + " |")
    return lines
