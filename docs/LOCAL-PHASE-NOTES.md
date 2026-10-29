# 本机阶段说明（未 push）

## diary-relay 改动摘要
- 同域 Web 编辑器：`webui/`（`index.html` / `app.js` / `style.css`），由 Go embed 提供
- 新增 `GET /drafts` 列表
- 修正 `config.example.yaml` 的 `posts_path: source/_posts`
- 部署笔记：`docs/DEPLOY-1PANEL.md`

## 本地启动编辑器
```bash
cd /workspace/diary-relay
cp config.example.yaml config.yaml   # 填入 auth_token；可暂空 github.token / blog_workdir
# 或使用已生成的 config.local-dev.yaml
make run CONFIG=config.local-dev.yaml
# 浏览器打开 http://127.0.0.1:18080/  ，Token = config 里的 auth_token
```

## 建议 commit message（待你确认后再 push）
- diary-relay: `feat: add Chinese web Markdown editor, GET /drafts, fix posts_path`
- AKAama.github.io: `ci: restore source→main Hexo deploy; vendor ZenMind theme for CI`
