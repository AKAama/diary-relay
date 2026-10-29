#!/usr/bin/env bash
# Local smoke: mock GitHub + minimal hexo site + diary-relay API.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

SMOKE_DIR="$ROOT/smoke-run"
# Keep blog/node_modules across runs to save time; wipe data/preview/state
mkdir -p "$SMOKE_DIR"
rm -rf "$SMOKE_DIR"/{data,preview}
mkdir -p "$SMOKE_DIR"/{data,preview,blog}
KEEP_BLOG_NM=0
if [[ -d "$SMOKE_DIR/blog/node_modules/hexo" ]]; then KEEP_BLOG_NM=1; fi

cleanup() {
  [[ -n "${SRV_PID:-}" ]] && kill "$SRV_PID" 2>/dev/null || true
  [[ -n "${MOCK_PID:-}" ]] && kill "$MOCK_PID" 2>/dev/null || true
}
trap cleanup EXIT

echo "==> minimal hexo site (landscape theme)"
cd "$SMOKE_DIR/blog"
if [[ "$KEEP_BLOG_NM" != "1" ]]; then
  npm init -y >/dev/null 2>&1
  npm install --no-fund --no-audit \
    hexo@8 hexo-generator-index hexo-generator-archive \
    hexo-generator-category hexo-generator-tag \
    hexo-renderer-ejs hexo-renderer-marked hexo-renderer-stylus \
    hexo-theme-landscape >/dev/null
else
  echo "    (reusing existing node_modules)"
fi

mkdir -p source/_posts themes
ln -sfn ../node_modules/hexo-theme-landscape themes/landscape
ls -la themes/

# hexo-cli requires package.json "hexo": {"version": "..."} to expose generate
python3 -c 'import json;from pathlib import Path;p=Path("package.json");d=json.loads(p.read_text());d["hexo"]={"version":"8.1.2"};d["private"]=True;p.write_text(json.dumps(d,indent=2)+"\n")'
./node_modules/.bin/hexo version >/dev/null

cat > _config.yml <<'YML'
title: smoke-diary
author: smoke
language: zh-CN
timezone: Asia/Shanghai
url: http://127.0.0.1:18081
permalink: :year/:month/:day/:title/
source_dir: source
public_dir: public
theme: landscape
YML

# seed one post so index isn't empty
cat > source/_posts/hello.md <<'MD'
---
title: hello
date: 2026-01-01 12:00:00
tags:
---
seed
MD

cd "$ROOT"

echo "==> build binaries"
go build -o "$SMOKE_DIR/diary-relay" ./cmd/server
go build -o "$SMOKE_DIR/mockgithub" ./scripts/mockgithub

echo "==> start mock github on :18080"
MOCK_ADDR=":18080" "$SMOKE_DIR/mockgithub" >"$SMOKE_DIR/mockgithub.log" 2>&1 &
MOCK_PID=$!
sleep 0.3

cat > "$SMOKE_DIR/config.yaml" <<YML
listen: "127.0.0.1:18081"
auth_token: "smoke-token"
github:
  token: "fake"
  owner: "AKAama"
  repo: "AKAama.github.io"
  branch: "source"
  posts_path: "source/_posts"
  api_base_url: "http://127.0.0.1:18080"
blog_workdir: "$SMOKE_DIR/blog"
preview_dir: "$SMOKE_DIR/preview"
preview_base_url: "http://127.0.0.1:18081/preview"
db_path: "$SMOKE_DIR/data/drafts.db"
timezone: "Asia/Shanghai"
YML

echo "==> start diary-relay on :18081"
"$SMOKE_DIR/diary-relay" -config "$SMOKE_DIR/config.yaml" >"$SMOKE_DIR/server.log" 2>&1 &
SRV_PID=$!

for i in $(seq 1 30); do
  if curl -sf "http://127.0.0.1:18081/healthz" >/dev/null; then break; fi
  sleep 0.2
  if ! kill -0 "$SRV_PID" 2>/dev/null; then
    echo "server died:"; cat "$SMOKE_DIR/server.log"; exit 1
  fi
done
curl -sf "http://127.0.0.1:18081/healthz" | grep -q ok

AUTH="Authorization: Bearer smoke-token"
BASE="http://127.0.0.1:18081"

echo "==> unauthorized rejected"
code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE/drafts" -H "Content-Type: application/json" -d '{"content":"x"}')
[[ "$code" == "401" ]]

echo "==> create draft + preview"
CREATE=$(curl -sf -X POST "$BASE/drafts" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"content":"冒烟测试正文 hello smoke","tags":["smoke"]}')
echo "$CREATE" | tee "$SMOKE_DIR/create.json"
ID=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])' <<<"$CREATE")
PREV=$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("preview_url",""))' <<<"$CREATE")
WARN=$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("warning",""))' <<<"$CREATE")
if [[ -n "$WARN" ]]; then
  echo "PREVIEW WARNING: $WARN"
  echo "---- server.log ----"; tail -50 "$SMOKE_DIR/server.log"
  exit 1
fi
[[ -n "$ID" && -n "$PREV" ]]
echo "id=$ID preview=$PREV"

echo "==> fetch preview HTML"
# Hexo permalink may put post under date path; check index and that preview dir is non-empty
curl -sf "$PREV" -o "$SMOKE_DIR/preview-index.html" || curl -sf "${PREV}index.html" -o "$SMOKE_DIR/preview-index.html"
ls -la "$SMOKE_DIR/preview/$ID" | head
grep -q . "$SMOKE_DIR/preview-index.html"
# Theme CSS must be rooted under /preview/<id>/ (not site "/")
grep -E 'href="/preview/'"$ID"'/css/' "$SMOKE_DIR/preview-index.html" | head -1 | grep -q css || {
  echo "CSS hrefs missing /preview/$ID/ prefix:" >&2
  grep -oE 'href="[^"]*css[^"]*"' "$SMOKE_DIR/preview-index.html" | head -5 >&2 || true
  exit 1
}
# body should appear somewhere in generated site
grep -R -q "冒烟测试正文" "$SMOKE_DIR/preview/$ID" || {
  echo "preview missing body text"; find "$SMOKE_DIR/preview/$ID" -type f | head -30; exit 1
}
# permalink page should exist under date path
find "$SMOKE_DIR/preview/$ID" -type d -name "DailyJournal*" | grep -q . || \
  find "$SMOKE_DIR/preview/$ID" -path "*DailyJournal*" | grep -q . || {
    echo "no DailyJournal permalink path"; find "$SMOKE_DIR/preview/$ID" -name index.html; exit 1
  }

echo "==> get / update"
curl -sf -H "$AUTH" "$BASE/drafts/$ID" | grep -q "$ID"
curl -sf -X PUT "$BASE/drafts/$ID" -H "$AUTH" -H "Content-Type: application/json" \
  -d '{"content":"更新后的正文 updated","tags":["smoke"]}' | grep -q preview_url

echo "==> publish via mock GitHub"
PUB=$(curl -sf -X POST "$BASE/drafts/$ID/publish" -H "$AUTH")
echo "$PUB"
echo "$PUB" | grep -q commit_url
grep -q "PUT source/_posts" "$SMOKE_DIR/mockgithub.log"

echo "==> delete"
code=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE/drafts/$ID" -H "$AUTH")
[[ "$code" == "204" ]]

echo ""
echo "SMOKE OK"
