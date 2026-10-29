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
    current: null, // draft object or null for new
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
    ["btn-save", "btn-preview", "btn-publish", "btn-delete", "btn-new", "btn-refresh", "btn-login"].forEach((id) => {
      const el = $(id);
      if (!el) return;
      if (id === "btn-login") {
        el.disabled = busy;
        return;
      }
    });
    $("btn-save").disabled = busy;
    $("btn-refresh").disabled = busy;
    $("btn-new").disabled = busy;
    const has = !!(state.current && state.current.id);
    $("btn-preview").disabled = busy || !has || !state.current.preview_url;
    $("btn-publish").disabled = busy || !has;
    $("btn-delete").disabled = busy || !has;
  }

  function fillForm(draft) {
    state.current = draft;
    $("field-title").value = (draft && draft.title) || "";
    $("field-tags").value = draft && draft.tags ? draft.tags.join(", ") : "";
    $("field-content").value = (draft && draft.content) || "";
    if (draft && draft.id) {
      $("draft-status").textContent = "编辑中 · " + draft.filename;
      $("meta").textContent =
        "ID: " +
        draft.id +
        " · 创建 " +
        formatTime(draft.created_at) +
        " · 更新 " +
        formatTime(draft.updated_at);
    } else {
      $("draft-status").textContent = "新草稿";
      $("meta").textContent = "";
    }
    renderList();
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
      if (state.current && state.current.id === d.id) btn.classList.add("active");
      btn.innerHTML =
        '<span class="title"></span><span class="sub"></span>';
      btn.querySelector(".title").textContent = d.title || d.filename || d.id;
      btn.querySelector(".sub").textContent = formatTime(d.updated_at) + " · " + (d.filename || "");
      btn.addEventListener("click", () => openDraft(d.id));
      li.appendChild(btn);
      ul.appendChild(li);
    });
  }

  async function refreshList() {
    const data = await api("/drafts");
    state.drafts = Array.isArray(data) ? data : data.drafts || [];
    renderList();
  }

  async function openDraft(id) {
    setBusy(true);
    setMsg("");
    try {
      const d = await api("/drafts/" + encodeURIComponent(id));
      fillForm(d);
    } catch (e) {
      setMsg(e.message || "加载失败", "err");
      setBusy(false);
    }
  }

  function newDraft() {
    fillForm(null);
    setMsg("");
    $("field-content").focus();
  }

  async function saveDraft() {
    const content = $("field-content").value;
    if (!content.trim()) {
      setMsg("正文不能为空", "err");
      return;
    }
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
      fillForm(d);
    } catch (e) {
      setMsg(e.message || "保存失败", "err");
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
      const res = await api("/drafts/" + encodeURIComponent(state.current.id) + "/publish", {
        method: "POST",
      });
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
      show("main");
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
    state.drafts = [];
    $("login-token").value = "";
    show("login");
  });
  $("btn-new").addEventListener("click", newDraft);
  $("btn-refresh").addEventListener("click", async () => {
    setBusy(true);
    try {
      await refreshList();
      setMsg("列表已刷新", "ok");
    } catch (e) {
      setMsg(e.message || "刷新失败", "err");
    } finally {
      setBusy(false);
    }
  });
  $("btn-save").addEventListener("click", saveDraft);
  $("btn-preview").addEventListener("click", openPreview);
  $("btn-publish").addEventListener("click", publishDraft);
  $("btn-delete").addEventListener("click", deleteDraft);

  tryEnter();
})();
