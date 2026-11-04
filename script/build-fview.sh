#!/usr/bin/env bash
# fview 本地验收构建:版本号 git describe 推导注入 ldflags(裸 wails build 的 version
# 是空串,关于页只会显示 dev);产物拷 ~/Documents 交付(先删旧包防嵌套,md5 双端核对)。
# 用法:script/build-fview.sh [--keep]   # --keep 只构建不交付
set -euo pipefail
cd "$(dirname "$0")/.."

VER=$(git describe --tags --match 'v*' 2>/dev/null || echo dev)
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date +%Y-%m-%d)
echo "==> building fview (version=$VER commit=$COMMIT date=$DATE)"
(cd fview && wails build -ldflags "-X main.version=$VER -X main.commit=$COMMIT -X main.date=$DATE")

if [ "${1:-}" = "--keep" ]; then
    echo "==> done (未交付): fview/build/bin/fview.app"
    exit 0
fi

rm -rf ~/Documents/fview.app
cp -R fview/build/bin/fview.app ~/Documents/
md5 -q fview/build/bin/fview.app/Contents/MacOS/fview ~/Documents/fview.app/Contents/MacOS/fview
echo "==> delivered: ~/Documents/fview.app ($VER)"
