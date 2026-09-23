"""校验构建内上游来源与实际已合入的 Git 历史一致。"""

import json
import re
import subprocess
from datetime import datetime
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
METADATA = ROOT / "backend/internal/pkg/buildmeta/upstream-sync.json"


def validate_metadata(data, git):
    if not isinstance(data, dict) or data.get("repository") != "Wei-Shaw/sub2api":
        raise ValueError("上游仓库无效")
    version = data.get("version", "")
    if not isinstance(version, str) or not re.fullmatch(r"(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)", version):
        raise ValueError("上游版本必须是完整的正式版本号")
    commit = data.get("commit", "")
    if not isinstance(commit, str) or not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise ValueError("上游提交必须是完整 SHA")
    timestamp = data.get("synced_at", "")
    if not isinstance(timestamp, str) or not re.fullmatch(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})", timestamp):
        raise ValueError("同步时间必须符合带时区的 RFC3339 格式")
    synced_at = datetime.fromisoformat(timestamp)
    if synced_at.tzinfo is None:
        raise ValueError("同步时间必须包含时区")
    git("merge-base", "--is-ancestor", commit, "HEAD")
    recorded = git("show", f"{commit}:backend/cmd/server/VERSION").strip().removeprefix("v")
    if recorded != version:
        raise ValueError(f"元数据版本 {version} 与上游提交版本 {recorded} 不一致")


def run_git(*args):
    result = subprocess.run(["git", "-C", str(ROOT), *args], check=True, capture_output=True, text=True, encoding="utf-8")
    return result.stdout


if __name__ == "__main__":
    try:
        validate_metadata(json.loads(METADATA.read_text(encoding="utf-8")), run_git)
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        raise SystemExit(f"上游同步元数据校验失败：{error}") from error
    print("上游同步元数据校验通过")
