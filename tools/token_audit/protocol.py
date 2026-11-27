"""Anthropic Messages 请求、严格流式解析及脱敏。"""
from __future__ import annotations

import codecs
import copy
import hashlib
import http.client
import json
import ipaddress
import os
import socket
import threading
import time
from datetime import datetime, timezone
from urllib.parse import urlsplit, urlunsplit

TOKEN_FIELDS = ("input_tokens", "cache_read_input_tokens", "cache_creation_input_tokens")
MAX_RESPONSE_BYTES = 8 * 1024 * 1024
MAX_EVENT_BYTES = 1024 * 1024
MAX_TEXT_CHARS = 512 * 1024
CLIENT_HEADERS = {
    "Content-Type": "application/json",
    "Accept": "text/event-stream, application/json",
    "anthropic-version": "2023-06-01",
    "User-Agent": "claude-cli/2.1.294 (external, sdk-py)",
    "x-app": "cli",
}


def utc_now():
    return datetime.now(timezone.utc).isoformat()


def canonical_hash(value):
    raw = json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(raw.encode("utf-8")).hexdigest()


def redact(value, secrets):
    if isinstance(value, str):
        for secret in sorted(set(secrets), key=len, reverse=True):
            if secret:
                value = value.replace(secret, "[已隐藏]")
        return value
    if isinstance(value, dict):
        return {redact(k, secrets): redact(v, secrets) for k, v in value.items()}
    if isinstance(value, list):
        return [redact(item, secrets) for item in value]
    return value


def endpoint_url(url):
    parsed = urlsplit(url.strip())
    if parsed.scheme not in ("http", "https") or not parsed.hostname:
        raise ValueError("URL必须是完整的HTTP或HTTPS地址")
    if parsed.username or parsed.password or parsed.query or parsed.fragment:
        raise ValueError("URL不能包含用户名、密码、查询参数或片段")
    if parsed.port is not None and not 1 <= parsed.port <= 65535:
        raise ValueError("URL端口无效")
    path = parsed.path.rstrip("/")
    if not path.endswith("/v1/messages"):
        path += "/messages" if path.endswith("/v1") else "/v1/messages"
    return urlunsplit((parsed.scheme, parsed.netloc, path, "", ""))


def merge_usage(target, source):
    if not isinstance(source, dict):
        return
    for key, value in source.items():
        if value is None:
            continue
        if isinstance(value, dict):
            if not isinstance(target.get(key), dict):
                target[key] = {}
            merge_usage(target[key], value)
        else:
            target[key] = value


def check_public_peer(conn):
    """网页服务在发送密钥前检查实际对端，防止DNS重绑定。"""
    if os.environ.get("TOKEN_AUDIT_PUBLIC_ONLY") == "1":
        address = ipaddress.ip_address(conn.sock.getpeername()[0].split("%", 1)[0])
        if not address.is_global or address.is_multicast:
            raise ValueError("网页服务仅允许公网渠道地址")


def is_token_count(value):
    return type(value) is int and value >= 0


def input_metrics(usage):
    missing = [name for name in TOKEN_FIELDS if not is_token_count(usage.get(name))]
    total = None if missing else sum(usage[name] for name in TOKEN_FIELDS)
    return {"missing_usage_fields": missing, "total_input_tokens": total,
            "cache_hit_rate": usage["cache_read_input_tokens"] / total if total else None}


class ProtocolError(ValueError):
    """响应格式不足以证明一次完整的Messages调用。"""


class MessageParser:
    def __init__(self):
        self.usage = {}
        self.usage_start = {}
        self.usage_delta = {}
        self.texts = {}
        self.block_types = {}
        self.stop_reason = None
        self.response_model = None
        self.message_id = None
        self.started = False
        self.terminal_complete = False
        self.event_counts = {}
        self.text_chars = 0
        self.thinking_chars = 0
        self.upstream_error = None
        self.timing_started = time.perf_counter()
        self.first_text_seconds = None
        self.last_text_seconds = None
        self.terminal_seconds = None

    def add_text(self, index, text, replace=False):
        if not isinstance(text, str) or type(index) is not int or index < 0:
            raise ProtocolError("文本块格式无效")
        self.text_chars += len(text)
        if self.text_chars > MAX_TEXT_CHARS:
            raise ProtocolError("回答文本超过上限")
        self.texts[index] = text if replace else self.texts.get(index, "") + text
        if text.strip():
            elapsed = time.perf_counter() - self.timing_started
            if self.first_text_seconds is None:
                self.first_text_seconds = elapsed
            self.last_text_seconds = elapsed

    def start(self, message):
        if self.started or not isinstance(message, dict):
            raise ProtocolError("message_start重复或无效")
        self.started = True
        self.message_id = message.get("id")
        self.response_model = message.get("model")
        merge_usage(self.usage_start, message.get("usage"))
        merge_usage(self.usage, message.get("usage"))
        for index, block in enumerate(message.get("content") or []):
            if not isinstance(block, dict):
                raise ProtocolError("content不是有效内容块")
            if block.get("type") == "text":
                self.add_text(index, block.get("text", ""), replace=True)

    def consume(self, event_name, payload):
        if not isinstance(payload, dict):
            raise ProtocolError("SSE数据不是JSON对象")
        kind = payload.get("type") or event_name
        if event_name and payload.get("type") and event_name != payload["type"]:
            raise ProtocolError("SSE事件名与type不一致")
        self.event_counts[kind] = self.event_counts.get(kind, 0) + 1
        if kind == "error" or payload.get("error"):
            self.upstream_error = copy.deepcopy(payload.get("error", payload))
            raise ProtocolError("上游错误事件: " + json.dumps(payload.get("error", payload), ensure_ascii=False)[:2000])
        if kind == "ping":
            return
        if kind == "message_start":
            self.start(payload.get("message"))
            return
        if not self.started:
            raise ProtocolError("message_start之前出现消息事件")
        if kind == "content_block_start":
            block = payload.get("content_block") or {}
            index = payload.get("index", 0)
            self.block_types[index] = block.get("type")
            if block.get("type") == "text":
                self.add_text(index, block.get("text", ""), replace=True)
        elif kind == "content_block_delta":
            delta = payload.get("delta") or {}
            if delta.get("type") == "text_delta":
                self.add_text(payload.get("index", 0), delta.get("text", ""))
            elif delta.get("type") == "thinking_delta":
                self.thinking_chars += len(delta.get("thinking", ""))
        elif kind == "message_delta":
            merge_usage(self.usage_delta, payload.get("usage"))
            merge_usage(self.usage, payload.get("usage"))
            if (payload.get("delta") or {}).get("stop_reason"):
                self.stop_reason = payload["delta"]["stop_reason"]
        elif kind == "message_stop":
            if not self.stop_reason:
                raise ProtocolError("message_stop之前缺少stop_reason")
            self.terminal_complete = True
            self.terminal_seconds = time.perf_counter() - self.timing_started

    def json_response(self, payload):
        if not isinstance(payload, dict) or payload.get("type") != "message" or payload.get("error"):
            if isinstance(payload, dict):
                self.upstream_error = copy.deepcopy(payload.get("error"))
            raise ProtocolError("响应不是成功的Messages对象: " + json.dumps(payload, ensure_ascii=False)[:2000])
        self.start(payload)
        self.stop_reason = payload.get("stop_reason")
        if not self.stop_reason:
            raise ProtocolError("JSON响应缺少stop_reason")
        self.terminal_complete = True
        self.terminal_seconds = time.perf_counter() - self.timing_started

    def result(self):
        return {"usage": copy.deepcopy(self.usage), "usage_start": self.usage_start,
                "usage_delta": self.usage_delta, "text": "".join(self.texts[i] for i in sorted(self.texts)),
                "stop_reason": self.stop_reason, "response_model": self.response_model,
                "message_id": self.message_id, "terminal_complete": self.terminal_complete,
                "event_counts": self.event_counts, "thinking_chars": self.thinking_chars,
                "upstream_error": self.upstream_error,
                "first_text_seconds": self.first_text_seconds,
                "last_text_seconds": self.last_text_seconds,
                "terminal_seconds": self.terminal_seconds}


class ContentMessageParser(MessageParser):
    """签名测试专用：无损保存可回传的内容块，拒绝无法还原的增量。"""

    def __init__(self):
        super().__init__()
        self.blocks = {}
        self.open_blocks = set()
        self.json_parts = {}
        self.signature_received_at = {}

    def remember(self, index, block, streaming=False):
        if type(index) is not int or index != len(self.blocks) or not isinstance(block, dict):
            raise ProtocolError("内容块索引重复、不连续或格式无效")
        kind = block.get("type")
        if kind not in ("text", "thinking", "redacted_thinking", "tool_use"):
            raise ProtocolError("无法无损回传内容块: " + str(kind))
        for field in {"text": ("text",), "thinking": ("thinking", "signature"),
                      "redacted_thinking": ("data",), "tool_use": ("id", "name")}[kind]:
            if field in block and not isinstance(block[field], str):
                raise ProtocolError("内容块字段不是字符串: " + field)
        if kind == "tool_use" and not isinstance(block.get("input"), dict):
            raise ProtocolError("工具输入不是JSON对象")
        self.blocks[index] = copy.deepcopy(block)
        if kind == "thinking":
            self.thinking_chars += len(block.get("thinking", ""))
            if block.get("signature"):
                self.signature_received_at[index] = time.time()
        if streaming:
            self.open_blocks.add(index)

    def start(self, message):
        super().start(message)
        for index, block in enumerate(message.get("content") or []):
            self.remember(index, block)

    def consume(self, event_name, payload):
        super().consume(event_name, payload)
        kind = payload.get("type") or event_name
        index = payload.get("index")
        if kind == "content_block_start":
            if self.open_blocks:
                raise ProtocolError("上一内容块尚未结束")
            self.remember(index, payload.get("content_block"), streaming=True)
        elif kind == "content_block_delta":
            if type(index) is not int or index not in self.open_blocks:
                raise ProtocolError("增量没有对应的未关闭内容块")
            delta, block = payload.get("delta") or {}, self.blocks[index]
            field = {"text_delta": "text", "thinking_delta": "thinking",
                     "signature_delta": "signature"}.get(delta.get("type"))
            if field:
                expected = "text" if field == "text" else "thinking"
                if block.get("type") != expected or not isinstance(delta.get(field), str):
                    raise ProtocolError("增量与内容块类型不一致或字段无效")
                block[field] = block.get(field, "") + delta[field]
                if field == "signature" and delta[field]:
                    self.signature_received_at[index] = time.time()
            elif delta.get("type") == "input_json_delta":
                if block.get("type") != "tool_use" or not isinstance(delta.get("partial_json"), str):
                    raise ProtocolError("工具JSON增量无效")
                if block.get("input"):
                    raise ProtocolError("无法无损合并非空初始工具输入与JSON增量")
                self.json_parts[index] = self.json_parts.get(index, "") + delta["partial_json"]
            else:
                raise ProtocolError("无法无损回传未知增量: " + str(delta.get("type")))
        elif kind == "content_block_stop":
            if type(index) is not int or index not in self.open_blocks:
                raise ProtocolError("内容块结束事件无对应块")
            if index in self.json_parts:
                value = json.loads(self.json_parts.pop(index))
                if not isinstance(value, dict):
                    raise ProtocolError("工具输入增量不是JSON对象")
                self.blocks[index]["input"] = value
            self.open_blocks.remove(index)
        elif kind == "message_stop" and self.open_blocks:
            self.terminal_complete = False
            raise ProtocolError("消息结束时仍有未关闭的内容块")

    def result(self):
        result = super().result()
        result["assistant_content"] = [copy.deepcopy(self.blocks[i]) for i in sorted(self.blocks)]
        result["content_complete"] = self.terminal_complete and not self.open_blocks
        result["signature_received_at"] = {str(i): value for i, value in self.signature_received_at.items()}
        return result


class SSEDecoder:
    def __init__(self, parser):
        self.parser = parser
        self.decoder = codecs.getincrementaldecoder("utf-8")("strict")
        self.buffer = ""
        self.lines = []
        self.event_bytes = 0

    def feed(self, chunk):
        self.buffer += self.decoder.decode(chunk)
        while "\n" in self.buffer and not self.parser.terminal_complete:
            line, self.buffer = self.buffer.split("\n", 1)
            line = line.removesuffix("\r")
            self.event_bytes += len(line.encode("utf-8")) + 1
            if self.event_bytes > MAX_EVENT_BYTES:
                raise ProtocolError("单个SSE事件超过上限")
            if line:
                self.lines.append(line)
                continue
            names, data = [], []
            for entry in self.lines:
                if entry.startswith(":"):
                    continue
                field, _, value = entry.partition(":")
                value = value.removeprefix(" ")
                if field == "event":
                    names.append(value)
                elif field == "data":
                    data.append(value)
            self.lines = []
            self.event_bytes = 0
            if data:
                raw = "\n".join(data)
                if raw.strip() == "[DONE]":
                    raise ProtocolError("Messages需要message_stop，不能用[DONE]代替")
                self.parser.consume(names[-1] if names else "", json.loads(raw))
        if not self.parser.terminal_complete and self.event_bytes + len(self.buffer.encode("utf-8")) > MAX_EVENT_BYTES:
            raise ProtocolError("单个SSE事件超过上限")

    def finish(self):
        if self.parser.terminal_complete:
            return
        self.decoder.decode(b"", final=True)
        if self.buffer or self.lines:
            raise ProtocolError("流在完整事件分隔符之前断开")
        raise ProtocolError("流结束但缺少message_stop")


class MessagesClient:
    def __init__(self, url, key, timeout=180.0, auth="x-api-key", *, ssl_context=None, connect_timeout=None):
        self.url = endpoint_url(url)
        self.key = key
        self.timeout = timeout
        self.auth = auth
        self.ssl_context = ssl_context
        self.connect_timeout = connect_timeout

    def generate(self, body, session, *, capture_content=False):
        started = time.perf_counter()
        result = {"started_at": utc_now(), "http_status": None, "error": None,
                  "request_sha256": canonical_hash(body), "ids": {}, "stage": "connect",
                  "connect_seconds": None, "request_sent_seconds": None, "headers_seconds": None,
                  "first_body_byte_seconds": None, "timed_out": False}
        parser = ContentMessageParser() if capture_content else MessageParser()
        parser.timing_started = started
        parsed = urlsplit(self.url)
        conn_type = http.client.HTTPSConnection if parsed.scheme == "https" else http.client.HTTPConnection
        kwargs = {"timeout": min(self.timeout, self.connect_timeout) if self.connect_timeout else self.timeout}
        if parsed.scheme == "https" and self.ssl_context is not None:
            kwargs["context"] = self.ssl_context
        conn = conn_type(parsed.hostname, parsed.port, **kwargs)
        response, timer = None, None
        expired = threading.Event()
        try:
            conn.connect()
            check_public_peer(conn)
            result["connect_seconds"] = time.perf_counter() - started
            remaining = self.timeout - (time.perf_counter() - started)
            if remaining <= 0:
                raise TimeoutError("连接阶段超过总超时")
            sock = conn.sock
            sock.settimeout(remaining)

            def expire():
                expired.set()
                try:
                    sock.shutdown(socket.SHUT_RDWR)
                except OSError:
                    pass

            timer = threading.Timer(remaining, expire)
            timer.daemon = True
            timer.start()
            headers = {**CLIENT_HEADERS, "x-claude-code-session-id": session,
                       "x-client-request-id": session}
            if self.auth == "bearer":
                headers["Authorization"] = "Bearer " + self.key
            else:
                headers["x-api-key"] = self.key
            raw = json.dumps(body, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
            result["request_bytes"] = len(raw)
            result["stage"] = "send"
            result["request_sent_at"] = time.time()
            send_clock = time.perf_counter()
            conn.request("POST", parsed.path, body=raw, headers=headers)
            result["request_sent_seconds"] = time.perf_counter() - started
            result["stage"] = "headers"
            response = conn.getresponse()
            result["headers_seconds"] = time.perf_counter() - started
            result["http_status"] = response.status
            result["ids"] = {k.lower(): v for k, v in response.getheaders()
                             if k.lower() in ("request-id", "cf-ray", "x-client-request-id") or k.lower().endswith("-request-id")}
            is_sse = "text/event-stream" in response.getheader("content-type", "").lower()
            result["response_format"] = "sse" if is_sse else "json"
            result["stage"] = "body"
            stream = SSEDecoder(parser)
            chunks, received = [], 0
            limit = MAX_RESPONSE_BYTES if response.status == 200 else 16384
            while True:
                chunk = response.read1(min(65536, limit - received + 1))
                if not chunk:
                    break
                if result["first_body_byte_seconds"] is None:
                    result["first_body_byte_seconds"] = time.perf_counter() - started
                received += len(chunk)
                if received > limit:
                    if response.status == 200:
                        raise ProtocolError("响应体超过上限")
                    chunks.append(chunk[:max(0, limit - sum(len(c) for c in chunks))])
                    break
                if response.status == 200 and is_sse:
                    stream.feed(chunk)
                    if parser.terminal_complete:
                        break
                else:
                    chunks.append(chunk)
            result["response_bytes"] = received
            result["clock_drift_seconds"] = abs((time.time() - result["request_sent_at"]) - (time.perf_counter() - send_clock))
            if expired.is_set():
                raise TimeoutError("请求超过总超时")
            if response.status != 200:
                result["error"] = "HTTP " + str(response.status)
                result["error_excerpt"] = b"".join(chunks).decode("utf-8", "replace")[:4000]
                try:
                    error_payload = json.loads(b"".join(chunks))
                    if isinstance(error_payload, dict):
                        parser.upstream_error = error_payload.get("error")
                except (ValueError, UnicodeError):
                    pass
            elif is_sse:
                stream.finish()
            else:
                parser.json_response(json.loads(b"".join(chunks)))
        except (OSError, http.client.HTTPException, ValueError, TypeError, KeyError, AttributeError) as exc:
            result["timed_out"] = expired.is_set() or isinstance(exc, TimeoutError)
            result["exception_type"] = type(exc).__name__
            result["exception_message"] = str(exc)[:2000]
            result["error"] = "请求超过总超时" if expired.is_set() or isinstance(exc, TimeoutError) else type(exc).__name__ + ": " + str(exc)[:2000]
        finally:
            observed_end = time.perf_counter()
            if timer:
                timer.cancel()
            if response:
                response.close()
            conn.close()
        result.update(parser.result())
        result.update(input_metrics(parser.usage))
        result["finished_at"] = utc_now()
        result["elapsed_seconds"] = observed_end - started
        result["protocol_success"] = result["http_status"] == 200 and parser.terminal_complete and not result["error"]
        result["success"] = bool(result["protocol_success"] and result["text"].strip() and parser.stop_reason == "end_turn")
        return result
