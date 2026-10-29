# diary-relay

手机写日记 → 服务端用你的 Hexo 主题生成预览 → 确认后通过 GitHub Contents API 把 `.md` 提交到 `AKAama/AKAama.github.io` 的 **`source`** 分支（现有部署流水线负责 `main`，本服务**绝不**写 `main`）。

> **本仓库交付物仅为本地代码。** 不要在未明确授权时 push / 改 blog 仓库 / SSH 部署。

## 功能

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/` | 浏览器 Markdown 编辑器（同域，中文 UI） |
| GET | `/drafts` | 列出草稿（新→旧） |
| POST | `/drafts` | 创建草稿 `{title,content,tags[]}` → `{id,preview_url}` |
| GET | `/drafts/:id` | 读取草稿 |
| PUT | `/drafts/:id` | 更新并重新生成预览 |
| POST | `/drafts/:id/publish` | 提交 md 到 `source` 分支，返回 commit URL |
| DELETE | `/drafts/:id` | 删除草稿与预览 |
| GET | `/posts` | 列出 `source/_posts` 下已发布 `.md` |
| GET | `/posts/:name` | 读取已发布文章（解析 front matter，返回 sha） |
| PUT | `/posts/:name` | 更新已发布文章（需 sha 乐观锁） |
| GET | `/healthz` | 健康检查（无需鉴权） |
| GET | `/preview/<id>/…` | 静态预览（Go 直接托管，nginx 可选） |

除 `/healthz`、`/preview/` 与编辑器静态页（`/`、`/app.js`、`/style.css`）外，均需：

```
Authorization: Bearer <auth_token>
```

## 浏览器编辑器

启动服务后打开 `http://127.0.0.1:8080/`（线上为 `https://diary.ismyh.cn/`）：

1. 输入与 `auth_token` 相同的 Bearer Token（保存在浏览器 `localStorage`）
2. 侧栏可切换 **草稿** / **已发布文章**：点已发布条目即可查看并编辑，**保存到博客** 走 `PUT /posts/:name`
3. 新建流程不变：撰写 → **保存草稿**（主题预览）→ **打开预览** → **发布到博客**

手机与桌面浏览器均可；仍兼容 iOS 快捷指令调用同一 API。详见 `docs/DEPLOY-1PANEL.md`。

## 文件名与 front matter

- 文件名：`DailyJournal-MMDD.md`；若当天已有同名（仓库或本地草稿），依次 `-2`、`-3`…
- 默认标题：`DailyJournal_MMDD`
- Front matter 与现有日记一致：

```yaml
---
title: DailyJournal_1218
date: 2025-12-18 23:05:12
tags:
---
正文…
```

有 tags 时写成 YAML 列表。时区默认 `Asia/Shanghai`。

## 主题 / blog_workdir（必读）

博客主题 `themes/hexo-theme-ZenMind` 在 GitHub 上是 **gitlink（submodule 指针）且没有 `.gitmodules`**，`git clone --recurse-submodules` **拿不到主题**。

请把你本机已经装好主题的博客目录拷到服务器，然后在 `config.yaml` 里把 `blog_workdir` 指过去，例如：

```yaml
blog_workdir: "/home/alex/blog"   # 内含 themes/hexo-theme-ZenMind/、node_modules、_config.yml
```

另外 `package.json` 里需要有 `"hexo": {"version": "…"}` 字段（`hexo-cli` 靠它识别项目并注册 `generate` 命令）；正式博客一般已有。


预览时服务会：

1. 用 mutex 串行化 `hexo generate`（避免并发草稿互相踩）
2. 把草稿 md 写到 `source/_posts/.diary-relay-<id>-…md`（临时）
3. 用 `_config.yml` + 临时 override 把 `public_dir` 指到 `preview/<id>/`
4. 生成完删除临时 post / override；静态结果留在 `preview/<id>/`，由本服务挂载到 `/preview/<id>/`

## 配置

```bash
cp config.example.yaml config.yaml
# 编辑 auth_token、github.token、blog_workdir、preview_base_url …
```

GitHub fine-grained PAT：仅授权该仓库 **Contents: Read and write**，目标分支建议只开 `source`。`config` 里 `github.branch` 被强制为 `source`，写成别的会启动失败。

## 运行

```bash
make tidy
make check          # go vet + go test
make run            # 读 ./config.yaml
# 或
go run ./cmd/server -config config.yaml
```

### Docker

镜像含 Go 编译产物 + Node 20（供 `npx hexo`）。仍需挂载带主题和 `node_modules` 的 blog 目录：

```bash
docker build -t diary-relay .
docker run --rm -p 8080:8080 \
  -v "$PWD/config.yaml:/app/config.yaml:ro" \
  -v /path/to/blog:/blog \
  -v "$PWD/data:/data" -v "$PWD/preview:/preview" \
  diary-relay
```

### nginx（可选）

Go 已能直接提供 `/preview/`。若要用 nginx 反代：

```nginx
location /preview/ {
    alias /path/to/diary-relay/preview/;
}
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Authorization $http_authorization;
}
```

## curl 示例

```bash
TOKEN=your-auth-token
BASE=http://127.0.0.1:8080

# 健康检查
curl -s "$BASE/healthz"

# 创建草稿
curl -s -X POST "$BASE/drafts" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"content":"今天骑车去了滨江","tags":["骑行"]}'
# → {"id":"…","preview_url":"http://127.0.0.1:8080/preview/<id>/",…}

# 打开 preview_url 后发布
curl -s -X POST "$BASE/drafts/<id>/publish" \
  -H "Authorization: Bearer $TOKEN"
# → {"commit_url":"https://github.com/…/commit/…"}

# 删除
curl -s -X DELETE "$BASE/drafts/<id>" \
  -H "Authorization: Bearer $TOKEN"
```

## iOS 快捷指令（Shortcuts）配方

1. **询问输入**「今天的日记」→ 存到变量 `正文`
2. （可选）**询问**「标题」；空则服务端用 `DailyJournal_MMDD`
3. **获取 URL 内容**：
   - URL：`https://你的域名/drafts`
   - 方法：POST
   - Headers：`Authorization: Bearer …`，`Content-Type: application/json`
   - 请求体：JSON `{"content":正文}`
4. **获取字典值** `preview_url` → **打开 URL**（Safari 预览主题渲染）
5. 另做一条「发布」快捷指令：询问草稿 id（或从剪贴板），`POST /drafts/{id}/publish`，把返回的 `commit_url` 用通知/打开 URL 展示
6. （可选）第三条：`DELETE /drafts/{id}` 清理未发布草稿

建议把 `auth_token` 放进「密码」或「数据保险箱」，不要写死在分享出去的快捷指令里。

## 开发

```bash
make check    # vet + test
make smoke    # 本地冒烟（mock GitHub + 最小 hexo 站）
make tarball  # 生成 /workspace/diary-relay.tgz
```

## 限制

- 预览依赖本机/容器内可用的 `npx hexo` 与完整 `blog_workdir`（含主题与依赖）
- `hexo generate` 串行；高峰时预览会排队
- 发布只走 Contents API 单文件；不改 `main`、不触发本地 git push
