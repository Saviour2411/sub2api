"""版本化参照指纹；完整请求哈希决定是否可用于精确对照。"""
from __future__ import annotations

import copy
import json
from pathlib import Path

from .protocol import canonical_hash, input_metrics, is_token_count, utc_now

DEFAULT_MODEL = "claude-opus-5-5"
BUILTIN_PATH = Path(__file__).with_name("fingerprints") / "claude-opus-5-5.20261104.json"


def load_fingerprint(path=BUILTIN_PATH):
    path = Path(path)
    if path.stat().st_size > 2 * 1024 * 1024:
        raise ValueError("指纹文件超过2MiB")
    data = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(data, dict) or data.get("schema_version") != 1:
        raise ValueError("指纹schema_version必须为1")
    if not isinstance(data.get("id"), str) or not isinstance(data.get("model"), str):
        raise ValueError("指纹必须包含id和model")
    cases = data.get("cases")
    if not isinstance(cases, list) or len(cases) != 3:
        raise ValueError("当前指纹必须包含三轮请求")
    previous = None
    for index, case in enumerate(cases):
        body = case.get("request") if isinstance(case, dict) else None
        if not isinstance(body, dict) or body.get("model") != data["model"] or body.get("stream") is not True:
            raise ValueError("指纹请求的model或stream不匹配")
        if set(body) - {"model", "stream", "max_tokens", "system", "messages", "metadata"}:
            raise ValueError("当前指纹仅支持纯文本Messages字段")
        if not is_token_count(body.get("max_tokens")) or not 1 <= body["max_tokens"] <= 32768:
            raise ValueError("指纹max_tokens无效")
        messages = body.get("messages")
        if not isinstance(messages, list) or len(messages) != index * 2 + 1:
            raise ValueError("指纹对话轮次不连续")
        for position, message in enumerate(messages):
            if message.get("role") != ("user" if position % 2 == 0 else "assistant"):
                raise ValueError("指纹角色顺序无效")
            blocks = message.get("content")
            if not isinstance(blocks, list) or not blocks:
                raise ValueError("指纹消息必须含文本块")
            for block in blocks:
                if not isinstance(block, dict) or block.get("type") != "text" or not isinstance(block.get("text"), str):
                    raise ValueError("指纹只支持纯文本内容块")
            if position % 2 == 0 and blocks[-1].get("cache_control") != {"type": "ephemeral", "ttl": "5m"}:
                raise ValueError("指纹用户消息必须保留5分钟缓存断点")
        if previous is not None and messages[:-2] != previous["messages"]:
            raise ValueError("指纹各轮没有保留相同历史")
        if previous is not None and {k: v for k, v in body.items() if k != "messages"} != {k: v for k, v in previous.items() if k != "messages"}:
            raise ValueError("指纹各轮公共参数不一致")
        expected = case.get("expected_usage")
        if not isinstance(expected, dict) or input_metrics(expected)["total_input_tokens"] is None:
            raise ValueError("指纹缺少完整输入usage，不能以0补齐")
        if case.get("request_sha256") != canonical_hash(body):
            raise ValueError("指纹请求SHA256不匹配，文件可能已被修改")
        if case.get("round") != index + 1:
            raise ValueError("指纹round编号无效")
        previous = body
    return data


def conversation_body(template, model, session, max_tokens):
    body = copy.deepcopy(template["cases"][0]["request"])
    body["model"] = model
    body["max_tokens"] = max_tokens
    body["metadata"] = {"user_id": json.dumps({"device_id": canonical_hash(session), "account_uuid": "", "session_id": session}, separators=(",", ":"))}
    # 随机前缀置于首条用户文本开头，防止其他会话预热同一长前缀。
    body["messages"][0]["content"][0]["text"] = "Independent audit session: " + session + ".\n" + body["messages"][0]["content"][0]["text"]
    return body


def fingerprint_from_conversation(rows, model):
    if len(rows) != 3 or any(not r["result"]["success"] or r["result"]["total_input_tokens"] is None for r in rows):
        raise ValueError("三轮未全部成功或usage不完整，不能保存可信指纹")
    if any(r["result"]["response_model"] != model for r in rows):
        raise ValueError("响应模型与请求模型不一致，不能保存可信指纹")
    return {"schema_version": 1, "id": model + "." + utc_now().replace(":", "-"),
            "model": model, "captured_at": utc_now(),
            "source": {"label": "用户指定的可信端点", "method": "三轮真实连续对话生成usage；可信性由使用者确认"},
            "cases": [{"round": r["round"], "request": copy.deepcopy(r["request"]),
                       "request_sha256": r["result"]["request_sha256"],
                       "expected_usage": copy.deepcopy(r["result"]["usage"]),
                       "observed_at": r["result"]["started_at"], "response_ids": r["result"]["ids"]}
                      for r in rows]}
