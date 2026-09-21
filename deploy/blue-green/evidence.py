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
ADD_COLUMN = re.compile(
    r"ALTER\s+TABLE\s+([a-z_][a-z0-9_]*)\s+ADD\s+COLUMN\s+IF\s+NOT\s+EXISTS\s+"
    r"([a-z_][a-z0-9_]*)\s+(jsonb|text|boolean|smallint|integer|bigint|uuid)\s*;", re.I)


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
        if status in ("A", "M") and re.fullmatch(r"backend/migrations/[a-zA-Z0-9_]+_test\.go", path):
            continue
        require(status == "A" and re.fullmatch(r"backend/migrations/[0-9][a-zA-Z0-9_]+\.sql", path),
                "迁移门禁拒绝修改历史 SQL、删除、重命名、Ent schema 或其他未知文件")
        content = git("show", sha+":"+path)
        column = additive_column(content)
        if approvals is None:
            approvals = json.loads(git("show", sha+":"+APPROVALS_PATH))
            require(approvals.get("version") == 1 and isinstance(approvals.get("approvals"), list), "兼容审批格式无效")
        matches = [entry for entry in approvals["approvals"] if entry.get("compatible_from") == base and entry.get("path") == path]
        require(len(matches) == 1, "新增字段缺少针对当前旧版本的唯一兼容审批")
        approval = matches[0]
        require(approval.get("checksum") == checksum(content), "迁移内容与兼容审批摘要不符")
        require(approval.get("column") == column, "新增字段与兼容审批不符")
        contracts = approval.get("legacy_contracts", [])
        require(isinstance(contracts, list) and contracts, "缺少旧版读写契约审查")
        for contract in contracts:
            source = contract.get("path", "")
            require(re.fullmatch(r"backend/[a-zA-Z0-9_/]+\.go", source) is not None and not source.endswith("_test.go"), "旧版契约路径无效")
            require(checksum(git("show", base+":"+source)) == contract.get("checksum"), "旧版读写实现与审批摘要不符")
        policy["migrations"].append({"filename": path.rsplit("/", 1)[1], "checksum": checksum(content), **column})
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
