#!/usr/bin/env python3
"""仅需URL和Key的Anthropic渠道Token、缓存、签名与并发测试入口。"""
from __future__ import annotations

import argparse
import json
import math
import os
from pathlib import Path
import sys
import uuid
from datetime import datetime

sys.dont_write_bytecode = True

from token_audit.fixtures import BUILTIN_PATH, DEFAULT_MODEL, fingerprint_from_conversation, load_fingerprint
from token_audit.protocol import MessagesClient, endpoint_url, redact
from token_audit.runner import RunContext, TESTS, observation, run_live_reference, summarize, write_reports
from token_audit.signature_cases import CASES, CORE_CASES
from token_audit.signatures import load_signature_baseline
from token_audit.concurrency import load_baseline, make_profile


def finite_number(value):
    number = float(value)
    if not math.isfinite(number) or number < 0:
        raise argparse.ArgumentTypeError("必须为非负有限数字")
    return number


def parser():
    p = argparse.ArgumentParser(description="缓存对话、Token指纹、思考签名与可选并发测试（Python 3.10+，无需第三方依赖）", add_help=False)
    p.add_argument("-h", "--help", action="help", help="显示参数说明并退出")
    p.add_argument("--url", help="目标地址，支持根地址、/v1或完整/v1/messages")
    p.add_argument("--key", help="目标API Key；也可设TOKEN_AUDIT_API_KEY环境变量")
    p.add_argument("--model", help="测试模型；默认" + DEFAULT_MODEL + "，指定指纹文件时默认使用其模型")
    p.add_argument("--tests", default="conversation,fingerprint", help="逗号分隔测试项，默认conversation,fingerprint")
    p.add_argument("--list-tests", action="store_true", help="列出可用测试项，不发起请求")
    p.add_argument("--list-models", action="store_true", help="列出自带指纹的模型，不代表上游完整模型列表")
    p.add_argument("--fingerprint-file", type=Path, help="载入自定义可信指纹JSON")
    p.add_argument("--save-fingerprint", type=Path, help="从本次成功的三轮对话导出新指纹，请用于你信任的端点")
    p.add_argument("--reference-url", help="可选实时参照地址；所有目标请求结束后再对照真实对话")
    p.add_argument("--reference-key", help="参照API Key；也可设TOKEN_AUDIT_REFERENCE_KEY")
    p.add_argument("--auth", choices=("x-api-key", "bearer"), default="x-api-key", help="目标认证方式")
    p.add_argument("--reference-auth", choices=("x-api-key", "bearer"), default="x-api-key", help="参照认证方式")
    p.add_argument("--timeout", type=finite_number, default=180, help="每次请求超时秒数，默认180")
    p.add_argument("--interval", type=finite_number, default=0, help="真实对话轮次间隔秒数，默认0")
    p.add_argument("--max-tokens", type=int, default=2048, help="真实对话输出上限；固定指纹保留原请求参数")
    p.add_argument("--signature-samples", type=int, default=2, help="每种签名场景的独立样本数，2至5，默认2")
    p.add_argument("--signature-scenarios", default="chat,tool", help="签名场景：chat普通对话、tool工具续接，默认chat,tool")
    p.add_argument("--signature-max-tokens", type=int, default=4096, help="签名测试每请求输出上限，1024至16384，默认4096")
    p.add_argument("--freshness-models", help="现签模型，逗号分隔；默认使用--model，可另加claude-sonnet-4-6")
    p.add_argument("--freshness-samples", type=int, default=2, help="各模型现签样本数，2至5")
    p.add_argument("--freshness-interval", type=finite_number, default=3, help="现签样本间隔，1至60秒")
    p.add_argument("--freshness-max-tokens", type=int, default=2048, help="现签输出上限，1024至8192")
    p.add_argument("--require-awsb", action="store_true", help="现签通过还须f2.f1.f2=1；缺失不通过")
    p.add_argument("--peer-url", help="可选签名续接对端，启用A/B自续接和双向跨渠道回放")
    p.add_argument("--peer-key", help="对端Key，也可设TOKEN_AUDIT_PEER_KEY")
    p.add_argument("--peer-auth", choices=("x-api-key", "bearer"), default="x-api-key", help="续接对端认证方式")
    p.add_argument("--concurrency", type=int, default=200, help="concurrency测试的同步请求数，1至200，默认200")
    p.add_argument("--load-max-tokens", type=int, default=2048, help="并发测试每请求输出上限，默认2048；独立于三轮对话参数")
    p.add_argument("--load-origin", default="未标注", help="并发测试发起环境标签，仅供比较；不会自动登录服务器")
    p.add_argument("--load-baseline-file", type=Path, help="自定义历史并发参考；默认内置2026-11-19可信渠道200并发结果")
    p.add_argument("--tolerance-tokens", type=finite_number, default=32, help="输入偏差绝对容差，默认32Token")
    p.add_argument("--tolerance-ratio", type=finite_number, default=0.005, help="相对容差，默认0.005（0.5%%），与绝对容差取较大值")
    p.add_argument("--output", type=Path, help="全新输出目录，必须位于当前工作目录内")
    p.add_argument("--dry-run", action="store_true", help="只显示请求计划，无需Key，不发起请求或写文件")
    return p


def workspace_path(path):
    resolved = Path(path).resolve()
    if not resolved.is_relative_to(Path.cwd().resolve()):
        raise ValueError("输出必须位于当前工作目录内")
    return resolved


def validate(args):
    selected = [part.strip() for part in args.tests.split(",")]
    if not selected or len(selected) != len(set(selected)) or any(name not in TESTS for name in selected):
        raise ValueError("测试项无效或重复，请使用--list-tests查看")
    if not args.url:
        raise ValueError("请提供--url")
    args.url = endpoint_url(args.url)
    if not 0 < args.timeout <= 3600:
        raise ValueError("--timeout必须在0至3600秒之间且大于0")
    if args.interval > 300 or not 1 <= args.max_tokens <= 32768 or args.tolerance_ratio > 1:
        raise ValueError("interval上限300秒，max-tokens范围1至32768，tolerance-ratio上限1")
    args.signature_scenarios = [part.strip() for part in args.signature_scenarios.split(",")]
    if (not 2 <= args.signature_samples <= 5 or not 1024 <= args.signature_max_tokens <= 16384 or
            not args.signature_scenarios or len(set(args.signature_scenarios)) != len(args.signature_scenarios) or
            any(name not in ("chat", "tool") for name in args.signature_scenarios)):
        raise ValueError("签名样本数必须为2至5；输出上限1024至16384；场景只能为不重复的chat、tool")
    args.key = args.key or os.environ.get("TOKEN_AUDIT_API_KEY")
    args.reference_key = args.reference_key or os.environ.get("TOKEN_AUDIT_REFERENCE_KEY")
    args.peer_key = args.peer_key or os.environ.get("TOKEN_AUDIT_PEER_KEY")
    if not args.key and not args.dry_run:
        raise ValueError("请通过--key或TOKEN_AUDIT_API_KEY提供目标密钥")
    if args.reference_url:
        args.reference_url = endpoint_url(args.reference_url)
        if "conversation" not in selected:
            raise ValueError("实时参照用于conversation，请选择该测试项")
        if not args.reference_key and not args.dry_run:
            raise ValueError("实时参照需要--reference-key或TOKEN_AUDIT_REFERENCE_KEY")
    elif args.reference_key:
        raise ValueError("提供了参照密钥但缺少--reference-url")
    if args.peer_url:
        args.peer_url = endpoint_url(args.peer_url)
        if "signature_relay" not in selected:
            raise ValueError("续接对端需要选择signature_relay")
        if not args.peer_key and not args.dry_run:
            raise ValueError("续接对端需要peer-key或TOKEN_AUDIT_PEER_KEY")
    elif args.peer_key:
        raise ValueError("提供了续接对端Key但缺少peer-url")
    for key in (args.key, args.reference_key, args.peer_key):
        if key and (not key.strip() or any(ord(char) < 33 or ord(char) > 126 for char in key)):
            raise ValueError("API Key必须是无空白的ASCII文本")
    fingerprint = load_fingerprint(args.fingerprint_file or BUILTIN_PATH)
    args.model = args.model or fingerprint["model"]
    if not args.model.strip():
        raise ValueError("模型名不能为空")
    args.freshness_models = [v.strip() for v in (args.freshness_models or args.model).split(",")]
    if (not 1 <= len(args.freshness_models) <= 4 or len(set(args.freshness_models)) != len(args.freshness_models)
            or any(not v or len(v) > 150 for v in args.freshness_models)
            or not 2 <= args.freshness_samples <= 5 or not 1 <= args.freshness_interval <= 60
            or not 1024 <= args.freshness_max_tokens <= 8192):
        raise ValueError("现签模型1至4个，样本2至5个，间隔1至60秒，输出上限1024至8192")
    if args.require_awsb and "signature_freshness" not in selected:
        raise ValueError("AWSB检查需要选择signature_freshness")
    if not 1 <= args.concurrency <= 200 or not 1 <= args.load_max_tokens <= 32768:
        raise ValueError("并发数必须为1至200，并发输出预算必须为1至32768")
    if not args.load_origin.strip() or len(args.load_origin) > 120:
        raise ValueError("并发测试来源标签必须为1至120个字符")
    args.load_baseline_data = load_baseline(args.load_baseline_file) if "concurrency" in selected else None
    args.signature_baseline_data = load_signature_baseline() if "signature" in selected else None
    if args.save_fingerprint:
        if "conversation" not in selected:
            raise ValueError("保存指纹需要conversation测试")
        args.save_fingerprint = workspace_path(args.save_fingerprint)
        if args.save_fingerprint.exists():
            raise ValueError("指纹目标已存在，拒绝覆盖")
    args.output = workspace_path(args.output or Path("output") / ("token-audit-" + datetime.now().strftime("%Y%m%d-%H%M%S") + "-" + uuid.uuid4().hex[:8]))
    if args.output.exists():
        raise ValueError("输出目录已存在，请指定全新目录，避免覆盖历史结果")
    if args.save_fingerprint == args.output or (args.save_fingerprint and args.output.is_relative_to(args.save_fingerprint)):
        raise ValueError("指纹文件不能与输出目录或其父目录重叠")
    reserved = {"summary.json", "report.md", "rounds.csv", "signature-cases.csv", "concurrency.csv", "events.jsonl", "plan.json"}
    if args.save_fingerprint and args.save_fingerprint.parent == args.output and args.save_fingerprint.name in reserved:
        raise ValueError("指纹文件名与报告输出冲突")
    return selected, fingerprint


def main(argv=None):
    p = parser()
    args = p.parse_args(argv)
    if args.list_tests:
        for spec in TESTS.values():
            print(spec.name + "：" + spec.description)
        return 0
    if args.list_models:
        print(DEFAULT_MODEL + "：内置2026-11-04三轮实测指纹；其他模型可用--model或自定义指纹")
        return 0
    secrets = [args.key or os.environ.get("TOKEN_AUDIT_API_KEY", ""), args.reference_key or os.environ.get("TOKEN_AUDIT_REFERENCE_KEY", ""), args.peer_key or os.environ.get("TOKEN_AUDIT_PEER_KEY", "")]
    try:
        selected, fingerprint = validate(args)
        plan = {"model": args.model, "url": args.url, "tests": selected,
                "target_requests_max": sum(TESTS[name].request_limit(args) for name in selected if name != "fingerprint" or args.model == fingerprint["model"]),
                "reference_requests_max": 3 if args.reference_url else 0,
                "fingerprint_id": fingerprint["id"] if "fingerprint" in selected and fingerprint["model"] == args.model else None,
                "fingerprint_expected_totals": [sum(c["expected_usage"][k] for k in ("input_tokens", "cache_read_input_tokens", "cache_creation_input_tokens")) for c in fingerprint["cases"]] if "fingerprint" in selected and fingerprint["model"] == args.model else None,
                "reference_url": args.reference_url,
                "peer_url": args.peer_url,
                "peer_requests_max": 5 if args.peer_url and "signature_relay" in selected else 0,
                "signature_freshness": {"models": args.freshness_models, "samples": args.freshness_samples,
                                        "interval": args.freshness_interval, "require_awsb": args.require_awsb,
                                        "catalog_requests_max": 10, "missing_timestamp": "unknown"} if "signature_freshness" in selected else None,
                "execution_order": selected + (["live_reference"] if args.reference_url else []),
                "signature": {"scenarios": args.signature_scenarios, "samples_per_scenario": args.signature_samples,
                              "cases_per_sample": len(CASES), "core_cases_per_sample": len(CORE_CASES),
                              "original_controls_per_sample": 3, "source_requests_per_sample": 1,
                              "max_tokens_per_request": args.signature_max_tokens,
                              "reference_id": args.signature_baseline_data["id"],
                              "reference_sampled_at": args.signature_baseline_data["sampled_at"],
                              "append_expected_outcome": args.signature_baseline_data["cases"]["append"]["expected_outcome"],
                              "thinking": "adaptive/summarized；chat effort=high，tool effort=max"} if "signature" in selected else None,
                "concurrency": {"profile": make_profile(args.model, args.concurrency, args.load_max_tokens, args.timeout, args.auth),
                                "origin": args.load_origin, "reference_id": args.load_baseline_data["id"],
                                "reference_mode": "离线历史参考，不发送参照请求；不同参数不计算性能倍数"} if "concurrency" in selected else None,
                "note": "固定指纹重放不检查缓存衔接；真实对话不套用历史回复Token数。伪造签名仅回传本渠道来源；显式选择跨渠道续接时才向对端回放。所有测试不重试，可能产生用量费用。"}
        plan["target_requests_max"] -= plan["peer_requests_max"]
        if args.dry_run:
            print(json.dumps(redact(plan, secrets), ensure_ascii=False, indent=2))
            return 0
        client = MessagesClient(args.url, args.key, args.timeout, args.auth)
        reference = MessagesClient(args.reference_url, args.reference_key, args.timeout, args.reference_auth) if args.reference_url else None
        args.output.mkdir(parents=True, exist_ok=False)
        (args.output / "plan.json").write_text(json.dumps(redact(plan, secrets), ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    except (OSError, ValueError, TypeError, KeyError, AttributeError) as exc:
        print("配置错误：" + redact(str(exc), secrets), file=sys.stderr)
        return 2
    context = RunContext(client, reference, fingerprint, fingerprint, args, args.output, secrets)
    context.say("模型：" + args.model + "；最多" + str(plan["target_requests_max"] + plan["reference_requests_max"] + plan["peer_requests_max"]) + "次生成请求，可能产生上游费用。")
    interrupted = False
    try:
        for name in selected:
            if context.stop_reason:
                context.skips.append({"test": name, "reason": context.stop_reason})
                continue
            TESTS[name].run(context)
        run_live_reference(context)
        if args.save_fingerprint:
            captured = fingerprint_from_conversation([r for r in context.rows if r["test"] == "conversation"], args.model)
            if redact(captured, secrets) != captured:
                raise ValueError("响应回显了密钥，不能把受污染的对话保存成指纹")
            args.save_fingerprint.parent.mkdir(parents=True, exist_ok=True)
            with args.save_fingerprint.open("x", encoding="utf-8") as handle:
                json.dump(captured, handle, ensure_ascii=False, indent=2)
                handle.write("\n")
            context.say("已保存新的实测指纹：" + str(args.save_fingerprint))
    except KeyboardInterrupt:
        interrupted = True
        context.notes.append(observation("interrupted", "用户中断，保留已完成结果", "incomplete"))
    except (OSError, ValueError, TypeError, KeyError, AttributeError) as exc:
        context.notes.append(observation("run_error", str(exc), "incomplete"))
    summary = summarize(context, selected)
    try:
        write_reports(context, summary)
    except OSError as exc:
        context.say("报告保存失败：" + str(exc))
        return 2
    labels = {"completed": "所选检查已完成", "observations": "存在需要核验的偏差", "incomplete": "存在未完成或无法判断的检查"}
    context.say(labels[summary["status"]] + "；目标请求" + str(summary["target_requests"]) + "次，参照请求" + str(summary["reference_requests"]) + "次。")
    if not summary["input_quantity_evaluated"] and any(name in ("conversation", "fingerprint") for name in selected):
        context.say("未取得有效输入总量参照，不能据此判断Token是否偏高。")
    if summary["signature"]:
        sig = summary["signature"]
        context.say("签名核心变更：可判定" + str(sig["evaluated_core_cases"]) + "/" + str(sig["expected_core_cases"]) +
                    "；完整接受" + str(sig["accepted_core_cases"]) + "；HTTP400明确拒绝" + str(sig["http_400_rejected_core_cases"]) +
                    "；流内明确拒绝" + str(sig["stream_rejected_core_cases"]) + "。")
    if summary["concurrency"]:
        metrics = summary["concurrency"]["metrics"]
        context.say("并发测试：完整正文成功" + str(metrics["successful_requests"]) + "/" + str(metrics["requests"])
                    + "；接口正常结束（含预算停止）" + str(metrics["protocol_successful_requests"])
                    + "；实际已发送在途峰值" + str(metrics["peak_sent_in_flight"]) + "。")
    for message in dict.fromkeys(item["message"] for item in summary["observations"]):
        context.say("- " + message)
    context.say("报告：" + str(args.output / "report.md"))
    return 130 if interrupted else summary["exit_code"]


if __name__ == "__main__":
    raise SystemExit(main())
