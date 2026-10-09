#!/usr/bin/env python3
"""在 Actions 中绑定固定提交、已完成门禁、镜像摘要和当前迁移基线。"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import urllib.parse
import urllib.request

REQUIRED = {
    "backend-ci.yml": {"shell", "test", "stream-race", "frontend", "golangci-lint", "lifecycle-race", "bluegreen-protocol", "release-helpers"},
    "security-scan.yml": {"backend-security", "frontend-security"},
}
MIGRATION_PATHS = ["backend/migrations", "backend/ent/schema"]
APPROVALS_PATH = "deploy/blue-green/expand-approvals.json"
SPECIAL_MIGRATIONS = {
    "242_drop_platform_check_constraints.sql": ("platform-guarded-242-v1", "a7725f31f637208d200e0f37d5270e9f263743d99fa9e4f1bb9a7387e8be5a61"),
    "239_channel_reasoning_effort_multipliers.sql": ("reasoning-empty-239-v1", "66feb546785dfa385d8efd268a40276cd8ffdb0cead7026cd79e339a3b69edf1"),
    "240_affiliate_ledger_operation_id.sql": ("affiliate-operation-240-v1", "3823bfea5f64feb58f5fcebc341c6877eaefc97834b4cd652c8e83ad08ed78da"),
    "241_add_payment_order_bonus_amount.sql": ("bonus-existing-241-v1", "18b6a524a9873d3e4a45e6f9bd03b6e69f4384b701825c659817e017f2e8244f"),
    "241_add_typesafe_platform.sql": ("platform-guarded-241-v1", "b6559525bf8d0b5d7c617e7415944f0d1fae8c408e9131ace84139795f66dcd2"),
}
PLATFORM_SCHEMA = {
    "path": "backend/ent/schema/user_platform_quota.go",
    "from_checksum": "88511bf2651dcde080ecb915e7ee699bfb93a5ff6df66e6ea4ba0cc229ea00d3",
    "to_checksum": "1b62bc2d0d95a67b4908d7615c032e0376d0ae890f15ca3f7019f0f2a75521a4",
}
ADD_COLUMN = re.compile(
    r"ALTER\s+TABLE\s+([a-z_][a-z0-9_]*)\s+ADD\s+COLUMN\s+IF\s+NOT\s+EXISTS\s+"
    r"([a-z_][a-z0-9_]*)\s+(jsonb|text|boolean|smallint|integer|bigint|uuid)\s*;", re.I)

PLATFORM242_SCHEMAS = [
    {
        "path": "backend/ent/schema/user_platform_quota.go",
        "from_checksum": "1b62bc2d0d95a67b4908d7615c032e0376d0ae890f15ca3f7019f0f2a75521a4",
        "to_checksum": "6f4fbb9b052bc3dcc7bcb89c898712063e51851ff0afcea012bcd345bbe7ab86"
    },
    {
        "path": "backend/ent/schema/composite_model_route.go",
        "from_checksum": "1334a714ec4e7efbb9686b94086c26a3d72af4660e7521af528dfe3968255633",
        "to_checksum": "cc2e48c8fe7305805f106de7b8cec77a94aee3650cb0009ae5d60cba2434f6d6"
    }
]


def reviewed_platform242_schemas(base, sha, changed, git):
    lines = changed.splitlines()
    require("A\tbackend/migrations/242_drop_platform_check_constraints.sql" in lines
            and all("M\t" + item["path"] in lines for item in PLATFORM242_SCHEMAS), "242原SQL与两处固定Ent差异必须同时存在")
    for item in PLATFORM242_SCHEMAS:
        for ref, key in ((base, "from_checksum"), (sha, "to_checksum")):
            require(checksum(git("show", ref+":"+item["path"])) == item[key], "242专项Ent差异摘要不符")


def reviewed_platform_schema(base, sha, changed, git):
    require("A\tbackend/migrations/241_add_typesafe_platform.sql" in changed.splitlines()
            and "M\t" + PLATFORM_SCHEMA["path"] in changed.splitlines(), "241平台SQL与固定Ent差异必须同时存在")
    for ref, key in ((base, "from_checksum"), (sha, "to_checksum")):
        require(checksum(git("show", ref+":"+PLATFORM_SCHEMA["path"])) == PLATFORM_SCHEMA[key], "241专项Ent差异摘要不符")


def require(value, message):
    if not value:
        raise RuntimeError(message)


def checked_run(runs, sha, required, jobs):
    # tag 本身可能正在跑第二次 CI；使用同一不可变提交已经完成的分支门禁。
    for run in sorted(runs, key=lambda item: item["id"], reverse=True):
        if run.get("head_sha") != sha or run.get("event") != "push" or run.get("status") != "completed" or run.get("conclusion") != "success":
            continue
        results = {job["name"]: job for job in jobs(run)}
        if required <= results.keys() and all(results[name].get("conclusion") == "success" for name in required):
            return {"run_id": run["id"], "attempt": run.get("run_attempt", 1), "sha": sha, "jobs": sorted(required), "url": run["html_url"]}
    raise RuntimeError("固定提交缺少全部成功且未跳过的 CI/安全门禁")


def checksum(content):
    return hashlib.sha256(content.strip().encode("utf-8")).hexdigest()


def additive_column(content):
    # 只识别完整的受限语法，不把通用 SQL 的前缀匹配当作兼容证明。
    sql = "\n".join(line for line in content.splitlines() if not line.lstrip().startswith("--")).strip()
    match = ADD_COLUMN.fullmatch(sql)
    require(match is not None, "仅允许单条新增可空字段，禁止默认值、约束、回填及其他 SQL")
    table, column, data_type = (value.lower() for value in match.groups())
    require(len(table) <= 63 and len(column) <= 63 and not table.startswith("pg_"), "字段或表名超出审批范围")
    return {"table": table, "column": column, "data_type": data_type}


def migration_evidence(base, sha, git):
    require(re.fullmatch(r"[a-f0-9]{40}", base), "活动提交无效")
    git("cat-file", "-e", base+"^{commit}")
    changed = git("diff", "--no-renames", "--name-status", base, sha, "--", *MIGRATION_PATHS).strip()
    policy = {"version": 1, "migrations": []}
    result = {"compatible_from": base, "migration_class": "none", "rollback_compatible": True, "migration_policy": policy}
    if not changed:
        return result
    approvals = None
    for line in changed.splitlines():
        fields = line.split("\t")
        require(len(fields) == 2, "迁移差异格式不受支持")
        status, path = fields
        if status == "M" and path in {item["path"] for item in PLATFORM242_SCHEMAS} and "A\tbackend/migrations/242_drop_platform_check_constraints.sql" in changed.splitlines():
            reviewed_platform242_schemas(base, sha, changed, git)
            continue
        if status == "M" and path == PLATFORM_SCHEMA["path"]:
            reviewed_platform_schema(base, sha, changed, git)
            continue
        if status in ("A", "M") and re.fullmatch(r"backend/migrations/[a-zA-Z0-9_]+_test\.go", path):
            continue
        require(status == "A" and re.fullmatch(r"backend/migrations/[0-9][a-zA-Z0-9_]+\.sql", path),
                "迁移门禁拒绝修改历史 SQL、删除、重命名、Ent schema 或其他未知文件")
        content = git("show", sha+":"+path)
        filename = path.rsplit("/", 1)[1]
        special = SPECIAL_MIGRATIONS.get(filename)
        if special:
            require(checksum(content) == special[1], "专项迁移固定 SQL 摘要不符")
            contract = {"profile": special[0]}
            if special[0] == "platform-guarded-242-v1":
                reviewed_platform242_schemas(base, sha, changed, git)
        else:
            contract = additive_column(content)
        if approvals is None:
            approvals = json.loads(git("show", sha+":"+APPROVALS_PATH))
            require(approvals.get("version") == 1 and isinstance(approvals.get("approvals"), list), "兼容审批格式无效")
        matches = [entry for entry in approvals["approvals"] if entry.get("compatible_from") == base and entry.get("path") == path]
        require(len(matches) == 1, "新增字段缺少针对当前旧版本的唯一兼容审批")
        approval = matches[0]
        require(approval.get("checksum") == checksum(content), "迁移内容与兼容审批摘要不符")
        if special:
            require(approval.get("profile") == special[0] and "column" not in approval, "专项迁移与兼容审批不符")
            if special[0] == "platform-guarded-241-v1":
                require(approval.get("schema_change") == PLATFORM_SCHEMA, "241平台审批缺少固定Ent差异")
                reviewed_platform_schema(base, sha, changed, git)
            if special[0] == "platform-guarded-242-v1":
                require(approval.get("schema_changes") == PLATFORM242_SCHEMAS, "242平台审批缺少两处固定Ent差异")
        else:
            require(approval.get("column") == contract and "profile" not in approval, "新增字段与兼容审批不符")
        contracts = approval.get("legacy_contracts", [])
        require(isinstance(contracts, list) and contracts, "缺少旧版读写契约审查")
        for legacy_contract in contracts:
            source = legacy_contract.get("path", "")
            require(re.fullmatch(r"backend/[a-zA-Z0-9_/]+\.go", source) is not None and not source.endswith("_test.go"), "旧版契约路径无效")
            require(checksum(git("show", base+":"+source)) == legacy_contract.get("checksum"), "旧版读写实现与审批摘要不符")
        policy["migrations"].append({"filename": filename, "checksum": checksum(content), **contract})
    if policy["migrations"]:
        result["migration_class"] = "expand"
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repository", required=True)
    parser.add_argument("--sha", required=True)
    parser.add_argument("--image", required=True)
    parser.add_argument("--state", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    require(re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", args.repository), "仓库名无效")
    require(re.fullmatch(r"[a-f0-9]{40}", args.sha), "提交无效")
    require(re.fullmatch(r"[A-Za-z0-9._/:-]+@sha256:[a-f0-9]{64}", args.image), "需要固定镜像摘要")
    token = os.environ.get("GITHUB_TOKEN", "")
    require(token, "缺少只读 Actions API 令牌")
    api = "https://api.github.com/repos/"+args.repository

    def get(path):
        req = urllib.request.Request(api+path, headers={"Authorization": "Bearer "+token, "Accept": "application/vnd.github+json", "X-GitHub-Api-Version": "2022-11-28"})
        with urllib.request.urlopen(req, timeout=30) as response:
            return json.load(response)

    def git(*arguments):
        return subprocess.check_output(["git", *arguments], text=True).strip()

    require(git("rev-parse", "HEAD") == args.sha, "执行器代码不是候选提交")
    checks = {}
    for workflow, required in REQUIRED.items():
        runs = get("/actions/workflows/"+workflow+"/runs?"+urllib.parse.urlencode({"head_sha": args.sha, "event": "push", "per_page": 100}))["workflow_runs"]
        checks[workflow] = checked_run(runs, args.sha, required, lambda run: get(f'/actions/runs/{run["id"]}/attempts/{run.get("run_attempt", 1)}/jobs?per_page=100')["jobs"])
    state = json.loads(Path(args.state).read_text())
    require(state.get("schema") == 1 and state.get("active") in ("blue", "green"), "活动实例基线无效")
    base = state["slots"][state["active"]]["sha"]
    # SSH 重试必须复用同一份记录；已经切流时沿用该发布的原始兼容基线。
    prior = state.get("release", {})
    if prior.get("sha") == args.sha and prior.get("image") == args.image:
        base = prior["compatible_from"]
    result = {"sha": args.sha, "image": args.image, "ci_sha": args.sha, "ci": "success", "security": "success", "checks": checks, **migration_evidence(base, args.sha, git)}
    path = Path(args.output)
    path.write_text(json.dumps(result, ensure_ascii=False, indent=2)+"\n")
    path.chmod(0o600)
    print("已绑定固定提交、镜像摘要、完整门禁及迁移兼容证据："+result["migration_class"])


if __name__ == "__main__":
    main()
