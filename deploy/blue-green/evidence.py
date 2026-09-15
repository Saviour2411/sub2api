#!/usr/bin/env python3
"""在 Actions 中绑定固定提交、已完成门禁、镜像摘要和当前迁移基线。"""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import urllib.parse
import urllib.request

REQUIRED = {
    "backend-ci.yml": {"shell", "test", "stream-race", "frontend", "golangci-lint", "lifecycle-race", "bluegreen-protocol"},
    "security-scan.yml": {"backend-security", "frontend-security"},
}
MIGRATION_PATHS = ["backend/migrations", "backend/ent/schema"]


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


def migration_evidence(base, sha, git):
    require(re.fullmatch(r"[a-f0-9]{40}", base), "活动提交无效")
    git("cat-file", "-e", base+"^{commit}")
    changed = git("diff", "--name-only", base, sha, "--", *MIGRATION_PATHS).strip()
    # 自动路径只批准完全没有迁移变化的发布；任何迁移变化需另行审定，不能猜测兼容。
    require(not changed, "迁移/模型 schema 发生变化，普通自动蓝绿发布拒绝执行，需另行兼容审查")
    return {"compatible_from": base, "migration_class": "none", "rollback_compatible": True}


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
    require(state.get("schema") == 1 and state.get("active") in ("blue", "green"), "未完成首次迁移，拒绝生成普通蓝绿发布记录")
    base = state["slots"][state["active"]]["sha"]
    # SSH 重试必须复用同一份记录；已经切流时沿用该发布的原始兼容基线。
    prior = state.get("release", {})
    if prior.get("sha") == args.sha and prior.get("image") == args.image:
        base = prior["compatible_from"]
    result = {"sha": args.sha, "image": args.image, "ci_sha": args.sha, "ci": "success", "security": "success", "checks": checks, **migration_evidence(base, args.sha, git)}
    path = Path(args.output)
    path.write_text(json.dumps(result, ensure_ascii=False, indent=2)+"\n")
    path.chmod(0o600)
    print("已绑定固定提交、镜像摘要、完整门禁和无迁移变化基线")


if __name__ == "__main__":
    main()
