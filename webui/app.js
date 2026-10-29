(() => {
  const TOKEN_KEY = "diary_relay_token";
  const $ = (id) => document.getElementById(id);

  const views = {
    login: $("view-login"),
    main: $("view-main"),
  };

  const state = {
    token: localStorage.getItem(TOKEN_KEY) || "",
    drafts: [],
    posts: [],
    sidebar: "drafts", // drafts | posts
    // mode: new | draft | post
    mode: "new",
    current: null, // draft object
    currentPost: null, // { filename, path, sha, title, tags, date, content }
    busy: false,
  };

  function show(view) {
    Object.values(views).forEach((el) => el.classList.add("hidden"));
    views[view].classList.remove("hidden");
  }

  function setMsg(text, kind) {
    const el = $("msg");
    if (!text) {
      el.hidden = true;
      el.textContent = "";
      return;
    }
    el.hidden = false;
    el.className = "msg " + (kind || "");
    el.textContent = text;
  }

  function parseTags(raw) {
    return String(raw || "")
      .split(/[,，]/)
      .map((s) => s.trim())
      .filter(Boolean);
  }

  function formatTime(iso) {
    if (!iso) return "";
    try {
      return new Date(iso).toLocaleString("zh-CN", { hour12: false });
    } catch {
      return iso;
    }
  }

  async function api(path, options = {}) {
    const headers = Object.assign({ Accept: "application/json" }, options.headers || {});
    if (state.token) headers.Authorization = "Bearer " + state.token;
    if (options.body && !headers["Content-Type"]) {
      headers["Content-Type"] = "application/json";
    }
    const res = await fetch(path, Object.assign({}, options, { headers }));
    const text = await res.text();
    let data = null;
    if (text) {
      try {
        data = JSON.parse(text);
      } catch {
        data = { error: text };
      }
    }
    if (!res.ok) {
      const err = new Error((data && data.error) || res.statusText || "请求失败");
      err.status = res.status;
      err.data = data;
      throw err;
    }
    return data;
  }

  function setBusy(busy) {
    state.busy = busy;
    $("btn-save").disabled = busy;
    $("btn-refresh").disabled = busy;
    $("btn-refresh-posts").disabled = busy;
    $("btn-new").disabled = busy;
    const isDraft = state.mode === "draft" && state.current && state.current.id;
    const isPost = state.mode === "post" && state.currentPost;
    $("btn-preview").disabled = busy || !isDraft || !state.current.preview_url;
    $("btn-publish").disabled = busy || !isDraft;
    $("btn-delete").disabled = busy || !isDraft;
    if (isPost) {
      $("btn-save").textContent = "保存到博客";
      $("btn-preview").classList.add("hidden");
      $("btn-publish").classList.add("hidden");
      $("btn-delete").classList.add("hidden");
    } else {
      $("btn-save").textContent = state.mode === "draft" ? "保存草稿" : "保存草稿";
      $("btn-preview").classList.remove("hidden");
      $("btn-publish").classList.remove("hidden");
      $("btn-delete").classList.remove("hidden");
    }
  }

  function updateModeBanner() {
    const banner = $("mode-banner");
    const wrapDate = $("wrap-date");
    if (state.mode === "post" && state.currentPost) {
      banner.classList.remove("hidden");
      banner.className = "mode-banner post";
      banner.textContent =
        "正在编辑已发布文章：" +
        state.currentPost.filename +
        "（保存将直接更新 GitHub source/_posts）";
      wrapDate.hidden = false;
      $("draft-status").textContent = "已发布 · " + state.currentPost.filename;
      $("meta").textContent =
        "路径: " +
        (state.currentPost.path || "") +
        (state.currentPost.date ? " · 日期 " + state.currentPost.date : "") +
        (state.currentPost.sha ? " · sha " + state.currentPost.sha.slice(0, 7) : "");
    } else if (state.mode === "draft" && state.current && state.current.id) {
      banner.classList.remove("hidden");
      banner.className = "mode-banner draft";
      banner.textContent = "正在编辑本地草稿（需点「发布到博客」才会写入仓库）";
      wrapDate.hidden = true;
      $("draft-status").textContent = "草稿 · " + state.current.filename;
      $("meta").textContent =
        "ID: " +
        state.current.id +
        " · 创建 " +
        formatTime(state.current.created_at) +
        " · 更新 " +
        formatTime(state.current.updated_at);
    } else {
      banner.classList.add("hidden");
      banner.textContent = "";
      wrapDate.hidden = true;
      $("draft-status").textContent = "新草稿";
      $("meta").textContent = "";
    }
  }

  function fillDraftForm(draft) {
    state.mode = draft && draft.id ? "draft" : "new";
    state.current = draft;
    state.currentPost = null;
    $("field-title").value = (draft && draft.title) || "";
    $("field-tags").value = draft && draft.tags ? draft.tags.join(", ") : "";
    $("field-content").value = (draft && draft.content) || "";
    $("field-date").value = "";
    updateModeBanner();
    renderList();
    renderPostList();
    setBusy(false);
  }

  function fillPostForm(post) {
    state.mode = "post";
    state.current = null;
    state.currentPost = post;
    $("field-title").value = (post && post.title) || "";
    $("field-tags").value = post && post.tags ? post.tags.join(", ") : "";
    $("field-content").value = (post && post.content) || "";
    $("field-date").value = (post && post.date) || "";
    updateModeBanner();
    renderList();
    renderPostList();
    setBusy(false);
  }

  function renderList() {
    const ul = $("draft-list");
    ul.innerHTML = "";
    const empty = $("draft-empty");
    empty.hidden = state.drafts.length > 0;
    state.drafts.forEach((d) => {
      const li = document.createElement("li");
      const btn = document.createElement("button");
      btn.type = "button";
      if (state.mode === "draft" && state.current && state.current.id === d.id) {
        btn.classList.add("active");
      }
      btn.innerHTML = '<span class="title"></span><span class="sub"></span>';
      btn.querySelector(".title").textContent = d.title || d.filename || d.id;
      btn.querySelector(".sub").textContent =
        formatTime(d.updated_at) + " · " + (d.filename || "");
      btn.addEventListener("click", () => openDraft(d.id));
      li.appendChild(btn);
      ul.appendChild(li);
    });
  }

  function renderPostList() {
    const ul = $("post-list");
    ul.innerHTML = "";
    const empty = $("post-empty");
    empty.hidden = state.posts.length > 0;
    state.posts.forEach((p) => {
      const li = document.createElement("li");
      const btn = document.createElement("button");
      btn.type = "button";
      if (
        state.mode === "post" &&
        state.currentPost &&
        state.currentPost.filename === p.filename
      ) {
        btn.classList.add("active");
      }
      btn.innerHTML = '<span class="title"></span><span class="sub"></span>';
      btn.querySelector(".title").textContent = p.title || p.filename;
      btn.querySelector(".sub").textContent = p.filename;
      btn.addEventListener("click", () => openPost(p.filename));
      li.appendChild(btn);
      ul.appendChild(li);
    });
  }

  function setSidebar(which) {
    state.sidebar = which;
    const draftsTab = $("tab-drafts");
    const postsTab = $("tab-posts");
    draftsTab.classList.toggle("active", which === "drafts");
    postsTab.classList.toggle("active", which === "posts");
    draftsTab.setAttribute("aria-selected", which === "drafts" ? "true" : "false");
    postsTab.setAttribute("aria-selected", which === "posts" ? "true" : "false");
    $("panel-drafts").classList.toggle("hidden", which !== "drafts");
    $("panel-posts").classList.toggle("hidden", which !== "posts");
  }

  async function refreshList() {
    const data = await api("/drafts");
    state.drafts = Array.isArray(data) ? data : data.drafts || [];
    renderList();
  }

  async function refreshPosts() {
    const data = await api("/posts");
    state.posts = Array.isArray(data) ? data : [];
    // newest-looking filenames last in GitHub dir listing is undefined; keep API order, reverse for recent-first feel
    state.posts = state.posts.slice().reverse();
    renderPostList();
  }

  async function openDraft(id) {
    setBusy(true);
    setMsg("");
    setSidebar("drafts");
    try {
      const d = await api("/drafts/" + encodeURIComponent(id));
      fillDraftForm(d);
    } catch (e) {
      setMsg(e.message || "加载失败", "err");
      setBusy(false);
    }
  }

  async function openPost(filename) {
    setBusy(true);
    setMsg("");
    setSidebar("posts");
    try {
      const p = await api("/posts/" + encodeURIComponent(filename));
      fillPostForm(p);
    } catch (e) {
      setMsg(e.message || "加载文章失败", "err");
      setBusy(false);
    }
  }

  function newDraft() {
    setSidebar("drafts");
    fillDraftForm(null);
    setMsg("");
    $("field-content").focus();
  }

  async function saveCurrent() {
    const content = $("field-content").value;
    if (!content.trim()) {
      setMsg("正文不能为空", "err");
      return;
    }
    if (state.mode === "post" && state.currentPost) {
      await savePost();
      return;
    }
    await saveDraft();
  }

  async function saveDraft() {
    const content = $("field-content").value;
    const payload = {
      title: $("field-title").value.trim(),
      content,
      tags: parseTags($("field-tags").value),
    };
    setBusy(true);
    setMsg("保存中…");
    try {
      let d;
      if (state.current && state.current.id) {
        d = await api("/drafts/" + encodeURIComponent(state.current.id), {
          method: "PUT",
          body: JSON.stringify(payload),
        });
      } else {
        d = await api("/drafts", {
          method: "POST",
          body: JSON.stringify(payload),
        });
      }
      if (d.warning) {
        setMsg("已保存，但预览失败：" + d.warning, "err");
      } else {
        setMsg("草稿已保存", "ok");
      }
      await refreshList();
      fillDraftForm(d);
    } catch (e) {
      setMsg(e.message || "保存失败", "err");
      setBusy(false);
    }
  }

  async function savePost() {
    if (!state.currentPost || !state.currentPost.sha) {
      setMsg("缺少文件 sha，请重新打开文章", "err");
      return;
    }
    if (
      !confirm(
        "确认将修改保存到 GitHub source/_posts/" +
          state.currentPost.filename +
          "？\n（若 PAT 仍为只读，会返回 403）"
      )
    ) {
      return;
    }
    const payload = {
      title: $("field-title").value.trim(),
      content: $("field-content").value,
      tags: parseTags($("field-tags").value),
      date: $("field-date").value.trim(),
      sha: state.currentPost.sha,
    };
    setBusy(true);
    setMsg("正在更新已发布文章…");
    try {
      const p = await api("/posts/" + encodeURIComponent(state.currentPost.filename), {
        method: "PUT",
        body: JSON.stringify(payload),
      });
      let msg = "已更新：" + (p.filename || state.currentPost.filename);
      setMsg(msg, "ok");
      if (p.commit_url) {
        const a = document.createElement("a");
        a.href = p.commit_url;
        a.target = "_blank";
        a.rel = "noopener";
        a.textContent = "查看提交";
        $("msg").appendChild(document.createTextNode(" "));
        $("msg").appendChild(a);
      }
      await refreshPosts();
      fillPostForm(p);
    } catch (e) {
      setMsg(e.message || "更新失败", "err");
      setBusy(false);
    }
  }

  function openPreview() {
    if (!state.current || !state.current.preview_url) {
      setMsg("请先保存草稿以生成预览", "err");
      return;
    }
    window.open(state.current.preview_url, "_blank", "noopener");
  }

  async function publishDraft() {
    if (!state.current || !state.current.id) return;
    if (!confirm("确认发布到 GitHub source/_posts？发布后会触发博客构建（若已配置 CI）。")) {
      return;
    }
    setBusy(true);
    setMsg("发布中…");
    try {
      const res = await api(
        "/drafts/" + encodeURIComponent(state.current.id) + "/publish",
        { method: "POST" }
      );
      setMsg("发布成功：" + (res.commit_url || res.filename || "完成"), "ok");
      if (res.commit_url) {
        const a = document.createElement("a");
        a.href = res.commit_url;
        a.target = "_blank";
        a.rel = "noopener";
        a.textContent = "查看提交";
        $("msg").appendChild(document.createTextNode(" "));
        $("msg").appendChild(a);
      }
      try {
        await refreshPosts();
      } catch (_) {}
    } catch (e) {
      setMsg(e.message || "发布失败", "err");
    } finally {
      setBusy(false);
    }
  }

  async function deleteDraft() {
    if (!state.current || !state.current.id) return;
    if (!confirm("确定删除该草稿与预览？")) return;
    setBusy(true);
    try {
      await api("/drafts/" + encodeURIComponent(state.current.id), { method: "DELETE" });
      setMsg("已删除", "ok");
      await refreshList();
      newDraft();
    } catch (e) {
      setMsg(e.message || "删除失败", "err");
      setBusy(false);
    }
  }

  async function tryEnter() {
    if (!state.token) {
      show("login");
      return;
    }
    try {
      await refreshList();
      try {
        await refreshPosts();
      } catch (e) {
        // list posts may fail if github not configured; still allow drafts
        console.warn("refresh posts", e);
      }
      show("main");
      setSidebar("drafts");
      if (state.drafts.length) {
        await openDraft(state.drafts[0].id);
      } else {
        newDraft();
      }
    } catch (e) {
      if (e.status === 401) {
        localStorage.removeItem(TOKEN_KEY);
        state.token = "";
        $("login-error").hidden = false;
        $("login-error").textContent = "Token 无效，请重新登录";
        show("login");
        return;
      }
      $("login-error").hidden = false;
      $("login-error").textContent = e.message || "无法连接服务";
      show("login");
    }
  }

  $("btn-login").addEventListener("click", async () => {
    const token = $("login-token").value.trim();
    if (!token) {
      $("login-error").hidden = false;
      $("login-error").textContent = "请输入 Token";
      return;
    }
    state.token = token;
    localStorage.setItem(TOKEN_KEY, token);
    $("login-error").hidden = true;
    await tryEnter();
  });
  $("login-token").addEventListener("keydown", (e) => {
    if (e.key === "Enter") $("btn-login").click();
  });
  $("btn-logout").addEventListener("click", () => {
    localStorage.removeItem(TOKEN_KEY);
    state.token = "";
    state.current = null;
    state.currentPost = null;
    state.drafts = [];
    state.posts = [];
    $("login-token").value = "";
    show("login");
  });
  $("btn-new").addEventListener("click", newDraft);
  $("btn-refresh").addEventListener("click", async () => {
    setBusy(true);
    try {
      await refreshList();
      setMsg("草稿列表已刷新", "ok");
    } catch (e) {
      setMsg(e.message || "刷新失败", "err");
    } finally {
      setBusy(false);
    }
  });
  $("btn-refresh-posts").addEventListener("click", async () => {
    setBusy(true);
    try {
      await refreshPosts();
      setMsg("已发布列表已刷新", "ok");
    } catch (e) {
      setMsg(e.message || "刷新失败", "err");
    } finally {
      setBusy(false);
    }
  });
  $("tab-drafts").addEventListener("click", () => setSidebar("drafts"));
  $("tab-posts").addEventListener("click", async () => {
    setSidebar("posts");
    if (!state.posts.length) {
      setBusy(true);
      try {
        await refreshPosts();
      } catch (e) {
        setMsg(e.message || "加载已发布文章失败", "err");
      } finally {
        setBusy(false);
      }
    }
  });
  $("btn-save").addEventListener("click", saveCurrent);
  $("btn-preview").addEventListener("click", openPreview);
  $("btn-publish").addEventListener("click", publishDraft);
  $("btn-delete").addEventListener("click", deleteDraft);

  tryEnter();

  // ---- Image upload (button / paste / drag-drop) ----
  const MAX_IMAGE = 10 * 1024 * 1024;
  let uploading = 0;

  function insertAtCursor(text) {
    const ta = $("field-content");
    const start = ta.selectionStart ?? ta.value.length;
    const end = ta.selectionEnd ?? start;
    const before = ta.value.slice(0, start);
    const pre = before && !before.endsWith("\n") ? "\n" : "";
    const ins = pre + text + "\n";
    ta.value = before + ins + ta.value.slice(end);
    const pos = start + ins.length;
    ta.setSelectionRange(pos, pos);
    ta.dispatchEvent(new Event("input", { bubbles: true }));
  }

  function setUploadStatus(text, isErr) {
    const el = $("upload-status");
    el.textContent = text || "";
    el.classList.toggle("error", !!isErr);
  }

  function uploadImage(file) {
    return new Promise((resolve, reject) => {
      if (file.size > MAX_IMAGE) return reject(new Error("图片超过 10MB"));
      const fd = new FormData();
      fd.append("file", file, file.name || "paste.png");
      const xhr = new XMLHttpRequest();
      xhr.open("POST", "/images");
      xhr.setRequestHeader("Authorization", "Bearer " + state.token);
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) {
          const pct = Math.round((e.loaded / e.total) * 100);
          setUploadStatus(`上传中 ${file.name || "图片"} ${pct}%${pct === 100 ? "（处理中…）" : ""}`);
        }
      };
      xhr.onerror = () => reject(new Error("网络错误"));
      xhr.onload = () => {
        let data = null;
        try { data = JSON.parse(xhr.responseText); } catch { data = { error: xhr.responseText }; }
        if (xhr.status >= 200 && xhr.status < 300 && data && data.markdown) resolve(data);
        else if (xhr.status === 413) reject(new Error("图片太大（上限 10MB）"));
        else reject(new Error((data && data.error) || "上传失败 " + xhr.status));
      };
      xhr.send(fd);
    });
  }

  async function handleImageFiles(files) {
    const list = Array.from(files || []).filter((f) => f && (f.type.startsWith("image/") || /\.(jpe?g|png|gif|webp|heic|heif)$/i.test(f.name || "")));
    if (!list.length) return;
    uploading++;
    $("btn-image").disabled = true;
    let ok = 0;
    try {
      for (const f of list) {
        try {
          const res = await uploadImage(f);
          insertAtCursor(res.markdown);
          ok++;
        } catch (e) {
          setUploadStatus(`上传失败：${f.name || "图片"}：${e.message}`, true);
          return;
        }
      }
      setUploadStatus(`已上传 ${ok} 张图片（保存草稿后预览可见）`);
    } finally {
      uploading--;
      if (!uploading) $("btn-image").disabled = false;
    }
  }

  $("btn-image").addEventListener("click", () => $("file-image").click());
  $("file-image").addEventListener("change", (e) => {
    handleImageFiles(e.target.files);
    e.target.value = "";
  });
  $("field-content").addEventListener("paste", (e) => {
    const files = e.clipboardData && e.clipboardData.files;
    if (files && files.length) {
      e.preventDefault();
      handleImageFiles(files);
    }
  });
  const ta = $("field-content");
  ["dragenter", "dragover"].forEach((ev) =>
    ta.addEventListener(ev, (e) => {
      if (e.dataTransfer && Array.from(e.dataTransfer.types || []).includes("Files")) {
        e.preventDefault();
        ta.classList.add("dragover");
      }
    })
  );
  ta.addEventListener("dragleave", () => ta.classList.remove("dragover"));
  ta.addEventListener("drop", (e) => {
    ta.classList.remove("dragover");
    if (e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files.length) {
      e.preventDefault();
      ta.focus();
      handleImageFiles(e.dataTransfer.files);
    }
  });
})();
