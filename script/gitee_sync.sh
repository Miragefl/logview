#!/bin/bash
set -euo pipefail

# gitee_sync.sh - 校验并补传 gitee release 资产
# 背景: gitee 的 release 镜像同步不可靠,可能只传部分资产即中断(v0.18.2 只传了
# 2/7),linux 用户升级 404。本脚本对照 dist/ 产物补齐缺失资产并复验。
# Usage: ./script/gitee_sync.sh <tag> [dist_dir]
# token: 环境变量 GITEE_TOKEN 或 ~/.gitee_token(600)
# 依赖: curl + python3(仅 JSON 解析)

TAG="${1:?usage: gitee_sync.sh <tag> [dist_dir]}"
DIST="${2:-dist}"
REPO="Mtok/logview"
API="https://gitee.com/api/v5/repos/$REPO"

TOKEN="${GITEE_TOKEN:-$(cat ~/.gitee_token 2>/dev/null || true)}"
if [[ -z "$TOKEN" ]]; then
    echo "ERROR: no gitee token (set GITEE_TOKEN or write ~/.gitee_token)"
    exit 1
fi

# dist 内应有资产: checksums.txt + 各平台 tar.gz(与 goreleaser 产物对齐)
shopt -s nullglob
WANT=("checksums.txt")
for f in "$DIST"/*.tar.gz; do WANT+=("$(basename "$f")"); done
if [[ ${#WANT[@]} -lt 2 ]]; then
    echo "ERROR: no goreleaser artifacts found in $DIST (run release first)"
    exit 1
fi
echo "==> gitee assets for $TAG: want ${WANT[*]}"

# 查 release;不存在则建壳。注意 gitee 坑:release 不存在时此接口返回 200+null
# (非 404),python 侧须显式防空;建壳时 tag 若未同步到 gitee,须给 target_commitish
# 在指定分支 HEAD 自动建 tag。
REL_ID=$(curl -sf --max-time 30 "$API/releases/tags/$TAG?access_token=$TOKEN" \
    | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["id"] if d else "")' 2>/dev/null || true)
if [[ -z "$REL_ID" ]]; then
    echo "    release not found on gitee, creating shell..."
    REL_ID=$(curl -sf --max-time 30 -X POST "$API/releases" \
        -d "access_token=$TOKEN" -d "tag_name=$TAG" -d "name=$TAG" \
        -d "target_commitish=main" \
        -d "body=mirror of https://github.com/Miragefl/logview/releases/tag/$TAG" \
        | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["id"] if d else "")' 2>/dev/null) || {
        echo "ERROR: failed to create gitee release (is tag $TAG synced to gitee yet?)"
        exit 1
    }
fi

assets() { # 输出 release 现有资产名,每行一个
    curl -sf --max-time 30 "$API/releases/$REL_ID?access_token=$TOKEN" \
        | python3 -c 'import json,sys
for a in json.load(sys.stdin).get("assets", []):
    print(a["name"])'
}

HAVE=$(assets)

# 补传缺失资产(与镜像同步机制一致的 attach_files 接口)
UPLOADED=0
for name in "${WANT[@]}"; do
    grep -qxF "$name" <<<"$HAVE" && continue
    f="$DIST/$name"
    [[ -f "$f" ]] || { echo "ERROR: $f missing in dist"; exit 1; }
    echo "    uploading $name ..."
    CODE=$(curl -s -o /dev/null -w '%{http_code}' --max-time 300 \
        -F "access_token=$TOKEN" -F "file=@$f" \
        "$API/releases/$REL_ID/attach_files")
    if [[ "$CODE" != "201" && "$CODE" != "200" ]]; then
        echo "ERROR: upload $name failed (http $CODE)"
        exit 1
    fi
    UPLOADED=$((UPLOADED+1))
done

# 复验:全部 WANT 资产必须到位
FINAL=$(assets)
for name in "${WANT[@]}"; do
    grep -qxF "$name" <<<"$FINAL" \
        || { echo "ERROR: $name still missing after upload"; exit 1; }
done
echo "==> gitee assets complete ($(( ${#WANT[@]} - UPLOADED )) existing, $UPLOADED uploaded)"
