"""独立生成签名负例，并校验变更只发生在预期字段。"""
from __future__ import annotations

import base64
import copy
import hashlib
import secrets
import string

ALPHABET = string.ascii_letters + string.digits + "+/"
CASES = (
    ("random_a", "等长随机签名A", "signature"),
    ("random_b", "等长随机签名B", "signature"),
    ("char_head", "头部修改一个字符", "signature"),
    ("char_middle", "中部修改一个字符", "signature"),
    ("char_tail", "尾部修改一个字符", "signature"),
    ("char_random", "随机位置修改一个字符", "signature"),
    ("decoded_bitflip", "解码后翻转一个比特", "signature"),
    ("splice", "拼接另一真实签名的片段", "signature"),
    ("truncate_tail", "截去末尾四个字符", "format"),
    ("truncate_half", "截断至约一半长度", "format"),
    ("append", "签名字符串末尾追加字符", "encoding_boundary"),
    ("append_decoded_payload", "解码后追加8字节再编码", "signature"),
    ("empty", "空字符串签名", "format"),
    ("missing", "删除signature字段", "format"),
    ("invalid_alphabet", "签名包含非法编码字符", "format"),
    ("null", "signature设为null", "format"),
    ("foreign_signature", "使用另一响应的完整签名", "binding"),
    ("thinking_append", "保留签名并追加思考文本", "binding"),
    ("thinking_replace", "保留签名并替换思考文本", "binding"),
    ("drop_block", "删除整个思考块", "compatibility_control"),
    ("foreign_block", "替换另一响应的完整思考块", "compatibility_control"),
)
CORE_CATEGORIES = {"signature", "format", "binding"}
CORE_CASES = tuple(name for name, _, category in CASES if category in CORE_CATEGORIES)


def digest(text):
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def changes(before, after, path=()):
    if type(before) is not type(after):
        return [path]
    if isinstance(before, dict):
        result = []
        for key in sorted(set(before) | set(after)):
            if key not in before or key not in after:
                result.append(path + (key,))
            else:
                result.extend(changes(before[key], after[key], path + (key,)))
        return result
    if isinstance(before, list):
        if len(before) != len(after):
            return [path]
        return [p for index, (left, right) in enumerate(zip(before, after))
                for p in changes(left, right, path + (index,))]
    return [] if before == after else [path]


def decode(signature):
    try:
        raw = base64.b64decode(signature + "=" * (-len(signature) % 4), validate=True)
    except (ValueError, UnicodeError) as exc:
        raise ValueError("签名不是可解码的标准Base64，此变更无法执行") from exc
    if not raw:
        raise ValueError("签名解码后为空，此变更无法执行")
    return raw


def encode(raw, original):
    encoded = base64.b64encode(raw).decode("ascii")
    return encoded if original.endswith("=") else encoded.rstrip("=")


def one_char(signature, index):
    raw = decode(signature)
    candidates = list(ALPHABET.replace(signature[index], ""))
    secrets.SystemRandom().shuffle(candidates)
    for char in candidates:
        changed = signature[:index] + char + signature[index + 1:]
        # 尾字符可能只有填充位改变，必须验证实际解码载荷发生变化。
        if decode(changed) != raw:
            return changed
    raise ValueError("该位置无法产生改变解码载荷的单字符变更")


def make_case(original, block_index, donor, name):
    category = next((kind for case, _, kind in CASES if case == name), None)
    if category is None:
        raise ValueError("未知签名测试项: " + name)
    candidate = copy.deepcopy(original)
    block = candidate["messages"][1]["content"][block_index]
    signature = block["signature"]
    source = ("messages", 1, "content", block_index)
    expected_paths = {source + ("signature",)}
    if name in ("splice", "foreign_signature", "foreign_block"):
        if not donor or donor.get("signature") == signature:
            raise ValueError("缺少签名不同的独立来源")
    changed, bit_position = None, None
    if name in ("random_a", "random_b"):
        raw = decode(signature)
        random_raw = secrets.token_bytes(len(raw))
        if random_raw == raw:
            random_raw = bytes([raw[0] ^ 1]) + raw[1:]
        changed = encode(random_raw, signature)
    elif name.startswith("char_"):
        decode(signature)
        size = len(signature.rstrip("="))
        index = {"char_head": 0, "char_middle": size // 2,
                 "char_tail": size - 1, "char_random": secrets.randbelow(size)}[name]
        changed = one_char(signature, index)
    elif name == "decoded_bitflip":
        raw = bytearray(decode(signature))
        index, mask = len(raw) // 2, 1 << secrets.randbelow(8)
        raw[index] ^= mask
        bit_position = {"byte_index": index, "mask": mask}
        changed = encode(raw, signature)
    elif name == "splice":
        raw, donor_raw = decode(signature), decode(donor["signature"])
        cut = len(raw) // 2
        payload = (donor_raw * ((len(raw) - cut) // len(donor_raw) + 1))[:len(raw) - cut]
        if raw[cut:] == payload:
            raise ValueError("拼接后的签名载荷未变化")
        changed = encode(raw[:cut] + payload, signature)
    elif name == "truncate_tail":
        changed = signature[:-4]
    elif name == "truncate_half":
        changed = signature[:len(signature) // 2]
    elif name == "append":
        changed = signature + "".join(secrets.choice(ALPHABET) for _ in range(8))
    elif name == "append_decoded_payload":
        changed = encode(decode(signature) + secrets.token_bytes(8), signature)
    elif name == "empty":
        changed = ""
    elif name == "missing":
        del block["signature"]
    elif name == "invalid_alphabet":
        index = len(signature) // 2
        changed = signature[:index] + "!" + signature[index + 1:]
    elif name == "null":
        block["signature"] = None
    elif name == "foreign_signature":
        changed = donor["signature"]
    elif name in ("thinking_append", "thinking_replace"):
        block["thinking"] = (block["thinking"] + "\n这是一段额外加入的合成校验内容。"
                             if name == "thinking_append" else "本段是替换后的合成思考文本，与原回复不同。")
        expected_paths = {source + ("thinking",)}
    elif name == "drop_block":
        del candidate["messages"][1]["content"][block_index]
        expected_paths = {("messages", 1, "content")}
    elif name == "foreign_block":
        candidate["messages"][1]["content"][block_index] = copy.deepcopy(donor)
        expected_paths = {source + (field,) for field in set(block) | set(donor)}
    if changed is not None:
        block["signature"] = changed
    actual = set(changes(original, candidate))
    if not actual or not actual <= expected_paths:
        raise ValueError("测试变更为空或包含非预期字段: " + name)
    mutation = {"case": name, "category": category, "core": category in CORE_CATEGORIES,
                "changed_paths": [".".join(map(str, p)) for p in sorted(actual, key=str)],
                "original_length": len(signature), "original_sha256": digest(signature),
                "changes_within_expected_fields": True, "decoded_bitflip": bit_position,
                "decoded_payload_changed": None}
    if isinstance(changed, str):
        mutation.update(changed_length=len(changed), changed_sha256=digest(changed),
                        differing_characters=sum(a != b for a, b in zip(signature, changed)) + abs(len(signature) - len(changed)))
        try:
            # 字符串尾随追加单独用宽松解码记录：填充符后的字符可能被忽略。
            decoded = (base64.b64decode(changed + "=" * (-len(changed) % 4), validate=False)
                       if name == "append" else decode(changed))
            mutation["decoded_payload_changed"] = decoded != decode(signature)
        except (ValueError, UnicodeError):
            pass
        if name.startswith("char_"):
            mutation["changed_index"] = next(i for i, (a, b) in enumerate(zip(signature, changed)) if a != b)
            if mutation["differing_characters"] != 1 or mutation["decoded_payload_changed"] is not True:
                raise ValueError("单字符修改未改变实际签名载荷")
        if name in ("random_a", "random_b") and len(changed) != len(signature):
            raise ValueError("随机签名长度不一致")
    return candidate, mutation
