# 腾讯云 1Panel 部署笔记（diary.ismyh.cn）

目标机：`101.43.122.127`（腾讯云）。**本稿仅说明步骤，尚未 SSH / 改服务器。**

## 0. 你需要先准备的东西

1. **DNS**：`diary.ismyh.cn` A 记录 → `101.43.122.127`（或 CNAME 到已指向该机的域名）。
2. **HTTPS**：1Panel 网站里申请 Let’s Encrypt（域名解析生效后）。
3. **Secrets / 配置**（写入服务器上的 `config.yaml`，勿提交 git）：
   - `auth_token`：长随机串（浏览器登录 + Shortcuts 共用）
   - `github.token`：GitHub fine-grained PAT  
     - 仅仓库 `AKAama/AKAama.github.io`  
     - **Contents: Read and write**  
     - 尽量只允许 **`source` 分支**（不要给 `main` 写权限）
   - `blog_workdir`：服务器上带 **ZenMind 主题 + `node_modules`** 的博客副本路径  
     （git clone 默认拿不到主题 gitlink，请从本机已能 `hexo generate` 的目录 rsync/拷贝）
   - `preview_base_url`: `https://diary.ismyh.cn/preview`
   - `listen`: `0.0.0.0:8080`（或仅本机 + 反代到 127.0.0.1:8080）
4. **博客 CI**：先合并 `AKAama.github.io` 的 source→main Actions（含 vendored 主题），否则发布到 `source` 后线上不会更新。

## 1. 推荐：Docker（1Panel 容器）

镜像构建（在已拉代码的机器上）：

```bash
docker build -t diary-relay:latest .
```

`config.yaml` 示例要点：

```yaml
listen: "0.0.0.0:8080"
auth_token: "…你的长随机串…"
github:
  token: "github_pat_…"
  owner: "AKAama"
  repo: "AKAama.github.io"
  branch: "source"
  posts_path: "source/_posts"
blog_workdir: "/blog"
preview_dir: "/preview"
preview_base_url: "https://diary.ismyh.cn/preview"
db_path: "/data/drafts.db"
timezone: "Asia/Shanghai"
```

运行：

```bash
docker run -d --name diary-relay --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  -v /opt/diary-relay/config.yaml:/app/config.yaml:ro \
  -v /opt/diary-relay/blog:/blog \
  -v /opt/diary-relay/data:/data \
  -v /opt/diary-relay/preview:/preview \
  diary-relay:latest
```

`/opt/diary-relay/blog` 必须含：`_config.yml`、`package.json`、`node_modules/`、`themes/hexo-theme-ZenMind/`、`source/`。

首次可在容器或宿主机对 blog 目录执行一次 `npm ci`（Node 20）。

## 2. 备选：二进制 + 宿主机 Node

```bash
go build -o /opt/diary-relay/diary-relay ./cmd/server
# 需 Node 20+ 与 npx/hexo（blog 目录内 node_modules/.bin/hexo）
# systemd 或 1Panel 进程守护：/opt/diary-relay/diary-relay -config /opt/diary-relay/config.yaml
```

## 3. 1Panel / nginx 反代

新建网站：`diary.ismyh.cn` → 反代 `http://127.0.0.1:8080`。

关键示例：

```nginx
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Authorization $http_authorization;
    proxy_pass_header Authorization;
}
```

Go 已托管 `/`（编辑器）、`/drafts*`、`/preview/`、`/healthz`，一般**不必**再单独 `alias` 预览目录。

申请 SSL 证书并强制 HTTPS。

## 4. 验收

```bash
curl -sS https://diary.ismyh.cn/healthz
# → {"ok":true}

# 浏览器打开 https://diary.ismyh.cn/ 用 auth_token 登录
```

## 5. 安全提醒

- `/preview/<id>/` **无鉴权**（持链接可读草稿预览）；勿把预览 URL 发到公开群。
- PAT 与 `auth_token` 仅放服务器文件 / 1Panel 环境，不要写进仓库。
- 防火墙仅暴露 80/443；8080 绑 `127.0.0.1` 更佳。
