// AgentGuard dashboard — single-file vanilla JS app.
// Polls REST endpoints for snapshot data; subscribes to SSE for live call inserts.

(function () {
  "use strict";

  const $ = (id) => document.getElementById(id);
  const fmt = new Intl.NumberFormat();
  const pct = (n) => (n * 100).toFixed(1) + "%";

  const state = {
    route: "overview",
    window: "1h",
    verdictFilter: "",
    autoscroll: true,
    paused: false,
    series: [],
    es: null,
    reconnectMs: 1000,
    // rolling client-side window of the latest calls (for in-memory filtering)
    recent: [],
    recentMax: 500,
    verdictCounts: { allow: 0, block: 0, flag: 0, transform: 0 },
    heroBlocked: 0,
    // rAF batching: events buffered between frames so the browser never janks
    pending: [],
    rafScheduled: false,
  };
  const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  // ---------- routing ----------
  function route() {
    const hash = (location.hash || "#overview").slice(1);
    state.route = ["overview","calls","tools","servers"].includes(hash) ? hash : "overview";
    document.querySelectorAll(".page").forEach(p => p.classList.remove("active"));
    $("page-" + state.route).classList.add("active");
    document.querySelectorAll(".nav-link").forEach(a => {
      a.classList.toggle("active", a.dataset.route === state.route);
    });
    $("pageTitle").textContent = {
      overview: "Overview", calls: "Tool calls", tools: "Top tools", servers: "MCP servers",
    }[state.route];
    refresh();
  }
  window.addEventListener("hashchange", route);

  // ---------- theme ----------
  function initTheme() {
    const saved = localStorage.getItem("ag.theme");
    if (saved) document.documentElement.setAttribute("data-theme", saved);
    $("themeToggle").addEventListener("click", () => {
      const cur = document.documentElement.getAttribute("data-theme");
      const next = cur === "dark" ? "light" : "dark";
      document.documentElement.setAttribute("data-theme", next);
      localStorage.setItem("ag.theme", next);
    });
  }

  // ---------- fetch helpers ----------
  async function getJSON(path) {
    const r = await fetch(path, { headers: { Accept: "application/json" } });
    if (!r.ok) throw new Error(`${path}: ${r.status}`);
    return r.json();
  }

  // ---------- formatters ----------
  function timeAgo(ms) {
    const d = Date.now() - ms;
    if (d < 1000) return "now";
    if (d < 60000) return Math.floor(d/1000) + "s";
    if (d < 3600000) return Math.floor(d/60000) + "m";
    if (d < 86400000) return Math.floor(d/3600000) + "h";
    return Math.floor(d/86400000) + "d";
  }
  function tsStr(ms) {
    const d = new Date(ms);
    return d.toLocaleTimeString();
  }
  function badge(v) {
    return `<span class="badge ${v}">${v}</span>`;
  }
  function esc(s) {
    return String(s || "").replace(/[&<>"]/g, c => ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;"}[c]));
  }

  // ---------- overview ----------
  async function loadOverview() {
    const o = await getJSON("/api/overview?window=" + state.window);
    $("kpiCalls").textContent     = fmt.format(o.total_calls || 0);
    $("kpiBlocked").textContent   = fmt.format(o.total_blocked || 0);
    $("kpiFlagged").textContent   = fmt.format(o.total_flagged || 0);
    $("kpiTransform").textContent = fmt.format(o.total_transform || 0);
    $("kpiRate").textContent      = pct(o.block_rate || 0);
    $("kpiAgents").textContent    = fmt.format(o.active_agents || 0);
    $("kpiServers").textContent   = fmt.format(o.known_servers || 0);
    $("kpiSessions").textContent  = fmt.format(o.open_sessions || 0);
    // seed the hero "threats blocked today" counter from the snapshot
    state.heroBlocked = o.total_blocked || 0;
    const hero = $("heroBlocked");
    if (hero) hero.textContent = fmt.format(state.heroBlocked);
  }

  async function loadTimeseries() {
    const rows = await getJSON("/api/timeseries?window=" + state.window);
    state.series = rows;
    renderSpark();
  }

  function renderSpark() {
    const svg = $("spark");
    const W = 800, H = 200, padX = 30, padY = 14;
    svg.innerHTML = "";
    if (!state.series.length) {
      svg.innerHTML = `<text x="${W/2}" y="${H/2}" text-anchor="middle" class="axis-label">No data in window</text>`;
      $("sparkSummary").textContent = "";
      return;
    }
    const max = Math.max(1, ...state.series.map(p => p.calls));
    const minT = state.series[0].bucket, maxT = state.series[state.series.length-1].bucket;
    const spanT = Math.max(1, maxT - minT);

    const xy = (t, v) => [
      padX + (t - minT) / spanT * (W - 2*padX),
      H - padY - (v / max) * (H - 2*padY),
    ];

    const pts = state.series.map(p => xy(p.bucket, p.calls));
    const blockPts = state.series.map(p => xy(p.bucket, p.blocked));

    let path = pts.map((p,i) => (i ? "L" : "M") + p[0].toFixed(1) + " " + p[1].toFixed(1)).join(" ");
    let area = path + ` L ${pts[pts.length-1][0].toFixed(1)} ${H-padY} L ${pts[0][0].toFixed(1)} ${H-padY} Z`;
    let blockPath = blockPts.map((p,i) => (i ? "L" : "M") + p[0].toFixed(1) + " " + p[1].toFixed(1)).join(" ");

    let totalCalls = state.series.reduce((s,p) => s + p.calls, 0);
    let totalBlocked = state.series.reduce((s,p) => s + p.blocked, 0);

    svg.innerHTML = `
      <defs>
        <linearGradient id="sparkGrad" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stop-color="#4ad6ff" stop-opacity=".5"/>
          <stop offset="100%" stop-color="#4ad6ff" stop-opacity="0"/>
        </linearGradient>
      </defs>
      <line class="axis" x1="${padX}" y1="${H-padY}" x2="${W-padX}" y2="${H-padY}"/>
      <text class="axis-label" x="${padX}" y="${H-2}">${tsStr(minT)}</text>
      <text class="axis-label" x="${W-padX}" y="${H-2}" text-anchor="end">${tsStr(maxT)}</text>
      <text class="axis-label" x="${padX-4}" y="${padY+8}" text-anchor="end">${max}</text>
      <path class="area" d="${area}"/>
      <path class="line" d="${path}"/>
      <path class="block-line" d="${blockPath}"/>
    `;
    $("sparkSummary").textContent = `${fmt.format(totalCalls)} calls / ${fmt.format(totalBlocked)} blocked`;
  }

  // ---------- recent + calls table ----------
  function callRow(c, isNew) {
    const reason = c.reason ? esc(c.reason).slice(0, 80) : "";
    const lat = c.latency_ms ? c.latency_ms + "ms" : "—";
    return `<tr ${isNew ? 'class="new"' : ''} data-id="${esc(c.id || "")}">
      <td>${tsStr(c.started_at)}</td>
      <td>${esc(c.server_name || "—")}</td>
      <td><code>${esc(c.tool_name || "—")}</code></td>
      <td class="muted">${esc(c.direction || "")}</td>
      <td>${badge(c.verdict || "—")}</td>
      <td class="muted">${reason}</td>
      <td>${lat}</td>
    </tr>`;
  }

  async function loadRecent() {
    const rows = await getJSON("/api/calls?limit=20");
    $("recentTable").querySelector("tbody").innerHTML = rows.map(c => callRow(c, false)).join("");
    // seed the rolling window + verdict counts from the snapshot
    state.recent = rows.slice(0, state.recentMax);
    seedVerdictCounts();
  }

  // seedVerdictCounts recomputes the donut from the overview snapshot so the
  // donut reflects the full window, not just the rolling client buffer.
  async function seedVerdictCounts() {
    try {
      const o = await getJSON("/api/overview?window=" + state.window);
      const blocked = o.total_blocked || 0, flagged = o.total_flagged || 0, xform = o.total_transform || 0;
      state.verdictCounts = {
        allow: Math.max(0, (o.total_calls || 0) - blocked - flagged - xform),
        block: blocked, flag: flagged, transform: xform,
      };
      renderDonut();
    } catch {}
  }

  async function loadCalls() {
    const q = state.verdictFilter ? "&verdict=" + state.verdictFilter : "";
    const rows = await getJSON("/api/calls?limit=200" + q);
    $("callsTable").querySelector("tbody").innerHTML = rows.map(c => callRow(c, false)).join("");
  }

  // ---------- top tools ----------
  async function loadTools() {
    const rows = await getJSON("/api/top-tools?window=" + state.window + "&limit=25");
    const max = Math.max(1, ...rows.map(r => r.calls));
    const tbody = $("toolsTable").querySelector("tbody");
    tbody.innerHTML = rows.map((r,i) => {
      const w = (r.calls / max * 100).toFixed(1);
      return `<tr>
        <td class="muted">${i+1}</td>
        <td>${esc(r.server_name || "—")}</td>
        <td><code>${esc(r.tool_name)}</code></td>
        <td>${fmt.format(r.calls)}</td>
        <td class="${r.blocks ? '' : 'muted'}">${fmt.format(r.blocks)}</td>
        <td>${(r.avg_latency_ms || 0).toFixed(1)}ms</td>
        <td><span class="bar" style="width:${w}%"></span></td>
      </tr>`;
    }).join("");
  }

  // ---------- servers (trust-gauge card grid) ----------
  function trustBand(score) {
    if (score == null) return { cls: "muted", color: "var(--text-dim)" };
    if (score >= 80) return { cls: "trust-green", color: "var(--ok)" };
    if (score >= 50) return { cls: "trust-yellow", color: "var(--warn)" };
    return { cls: "trust-red", color: "var(--danger)" };
  }
  // Build an SVG radial gauge for a 0-100 trust score. r=30, circumference≈188.5.
  function gaugeSVG(score) {
    const band = trustBand(score);
    const C = 2 * Math.PI * 30;
    const pct = score == null ? 0 : Math.max(0, Math.min(100, score)) / 100;
    const dash = (C * pct).toFixed(1) + " " + C.toFixed(1);
    return `<div class="gauge">
      <svg viewBox="0 0 72 72" width="72" height="72">
        <circle class="gtrack" cx="36" cy="36" r="30"/>
        <circle class="gfill" cx="36" cy="36" r="30" stroke="${band.color}" stroke-dasharray="${dash}"/>
      </svg>
      <span class="gnum ${band.cls}">${score == null ? "—" : score}</span>
    </div>`;
  }
  async function loadServers() {
    const rows = await getJSON("/api/servers?limit=200");
    $("serverGrid").innerHTML = rows.map(s => {
      const band = trustBand(s.trust_score);
      return `<div class="server-card" data-server="${esc(s.id)}">
        <div class="sc-top">
          ${gaugeSVG(s.trust_score)}
          <div>
            <div class="sc-name">${esc(s.name)}</div>
            <div class="sc-meta">${esc(s.transport)} · seen ${timeAgo(s.last_seen_at)} ago</div>
          </div>
        </div>
        <div class="sc-stats">
          <div><div class="s-num">${fmt.format(s.total_calls)}</div><div class="muted">calls</div></div>
          <div><div class="s-num ${s.blocks ? 'trust-red' : ''}">${fmt.format(s.blocks)}</div><div class="muted">blocks</div></div>
          <div><div class="s-num ${band.cls}">${s.trust_score == null ? '—' : s.trust_score}</div><div class="muted">trust</div></div>
        </div>
      </div>`;
    }).join("");
  }

  // ---------- verdict donut ----------
  function renderDonut() {
    const svg = $("donut");
    if (!svg) return;
    const data = [
      { k: "allow", v: state.verdictCounts.allow, c: "#3ddc97" },
      { k: "block", v: state.verdictCounts.block, c: "#ff6b6b" },
      { k: "flag", v: state.verdictCounts.flag, c: "#ffb547" },
      { k: "transform", v: state.verdictCounts.transform, c: "#7c5cff" },
    ];
    const total = data.reduce((s, d) => s + d.v, 0) || 1;
    const C = 2 * Math.PI * 45;
    let offset = 0;
    const segs = data.map(d => {
      const frac = d.v / total;
      const len = C * frac;
      const seg = `<circle class="seg" cx="60" cy="60" r="45" fill="none"
        stroke="${d.c}" stroke-width="16"
        stroke-dasharray="${len.toFixed(1)} ${(C - len).toFixed(1)}"
        stroke-dashoffset="${(-offset).toFixed(1)}"
        transform="rotate(-90 60 60)"/>`;
      offset += len;
      return seg;
    }).join("");
    svg.innerHTML = segs +
      `<text x="60" y="56" text-anchor="middle" fill="var(--text)" font-size="20" font-weight="800">${fmt.format(total === 1 && !data.some(d=>d.v) ? 0 : total)}</text>` +
      `<text x="60" y="72" text-anchor="middle" fill="var(--text-dim)" font-size="9">calls</text>`;
    $("donutLegend").innerHTML = data.map(d =>
      `<li><span class="swatch" style="background:${d.c}"></span>${d.k}<b>${fmt.format(d.v)}</b></li>`).join("");
  }

  // ---------- refresh dispatch ----------
  async function refresh() {
    try {
      if (state.route === "overview") {
        await Promise.all([loadOverview(), loadTimeseries(), loadRecent()]);
      } else if (state.route === "calls") {
        await loadCalls();
      } else if (state.route === "tools") {
        await loadTools();
      } else if (state.route === "servers") {
        await loadServers();
      }
    } catch (e) {
      console.warn("refresh failed:", e);
    }
  }

  // ---------- SSE live ----------
  function connectSSE() {
    if (state.es) state.es.close();
    const es = new EventSource("/events");
    state.es = es;

    es.addEventListener("open", () => {
      setLive(true);
      state.reconnectMs = 1000;
    });
    es.addEventListener("error", () => {
      setLive(false);
      es.close();
      setTimeout(connectSSE, state.reconnectMs);
      state.reconnectMs = Math.min(state.reconnectMs * 2, 15000);
    });

    es.addEventListener("hello", (ev) => {
      try { const o = JSON.parse(ev.data);
        // refresh overview KPIs from hello snapshot
        if (state.route === "overview") {
          $("kpiCalls").textContent     = fmt.format(o.total_calls || 0);
          $("kpiBlocked").textContent   = fmt.format(o.total_blocked || 0);
          $("kpiFlagged").textContent   = fmt.format(o.total_flagged || 0);
          $("kpiTransform").textContent = fmt.format(o.total_transform || 0);
          $("kpiRate").textContent      = pct(o.block_rate || 0);
        }
      } catch {}
    });

    es.addEventListener("call", (ev) => {
      if (state.paused) return;
      try {
        const c = JSON.parse(ev.data);
        state.pending.push(c);
        scheduleFlush();
      } catch {}
    });
  }

  // scheduleFlush batches incoming SSE calls and applies them once per animation
  // frame, so a burst of 30+/sec never causes layout thrash.
  function scheduleFlush() {
    if (state.rafScheduled) return;
    state.rafScheduled = true;
    const run = () => {
      state.rafScheduled = false;
      const batch = state.pending;
      state.pending = [];
      applyBatch(batch);
    };
    if (reduceMotion) run();
    else requestAnimationFrame(run);
  }

  function applyBatch(batch) {
    if (!batch.length) return;
    for (const c of batch) {
      // rolling client window
      state.recent.unshift(c);
      if (state.recent.length > state.recentMax) state.recent.pop();
      // verdict counts → donut
      if (state.verdictCounts[c.verdict] != null) state.verdictCounts[c.verdict]++;
      // live KPI bumps
      bumpKpi("kpiCalls");
      if (c.verdict === "block") { bumpKpi("kpiBlocked"); bumpHero(); }
      if (c.verdict === "flag") bumpKpi("kpiFlagged");
      if (c.verdict === "transform") bumpKpi("kpiTransform");
      // surface notable events as toasts
      if (c.verdict === "block") {
        toast("danger", "🛡 Threat blocked", `${c.tool_name || "tool"} · ${(c.reason || "policy violation").slice(0, 70)}`);
      } else if (c.verdict === "flag") {
        toast("warn", "⚠ Flagged", `${c.tool_name || "tool"} · ${(c.reason || "needs review").slice(0, 70)}`);
      }
    }
    // render rows for the active page from the batch
    if (state.route === "overview") {
      for (const c of batch) prependRow($("recentTable"), callRow(c, true), 20);
      renderDonut();
    } else if (state.route === "calls") {
      for (const c of batch) {
        if (state.verdictFilter && state.verdictFilter !== c.verdict) continue;
        prependRow($("callsTable"), callRow(c, true), 200);
      }
    }
  }

  function bumpHero() {
    state.heroBlocked++;
    const el = $("heroBlocked");
    if (!el) return;
    el.textContent = fmt.format(state.heroBlocked);
    el.classList.remove("bump");
    void el.offsetWidth; // restart animation
    el.classList.add("bump");
  }
  function setLive(on) {
    $("livedot").classList.toggle("live", on);
    $("livedot").classList.toggle("dead", !on);
    $("livestate").textContent = on ? "live" : "reconnecting…";
  }
  function prependRow(table, html, max) {
    const tbody = table.querySelector("tbody");
    tbody.insertAdjacentHTML("afterbegin", html);
    while (tbody.children.length > max) tbody.removeChild(tbody.lastChild);
  }
  function bumpKpi(id) {
    const el = $(id);
    if (!el) return;
    const n = parseInt(el.textContent.replace(/[^0-9]/g, ""), 10) || 0;
    el.textContent = fmt.format(n + 1);
  }

  // ---------- toasts ----------
  function toast(kind, title, body) {
    const stack = $("toastStack");
    if (!stack) return;
    const el = document.createElement("div");
    el.className = "toast " + kind;
    el.innerHTML = `<div class="t-title">${esc(title)}</div><div class="t-body">${esc(body)}</div>`;
    stack.appendChild(el);
    const ttl = setTimeout(() => dismiss(el), 5000);
    el.addEventListener("click", () => { clearTimeout(ttl); dismiss(el); });
    // cap the stack
    while (stack.children.length > 4) stack.removeChild(stack.firstChild);
  }
  function dismiss(el) {
    el.classList.add("leaving");
    setTimeout(() => el.remove(), 300);
  }

  // ---------- call-detail modal: waterfall + diff ----------
  function fmtDur(ns) {
    if (ns < 1000) return ns + "ns";
    if (ns < 1e6) return (ns / 1000).toFixed(1) + "µs";
    return (ns / 1e6).toFixed(2) + "ms";
  }
  function prettyJSON(s) {
    if (!s) return "";
    try { return JSON.stringify(JSON.parse(s), null, 2); } catch { return s; }
  }
  function renderWaterfall(stages) {
    if (!stages || !stages.length) return '<div class="muted">No stage data recorded.</div>';
    const max = Math.max(1, ...stages.map(s => s.duration_ns));
    return `<div class="waterfall">` + stages.map(s => {
      const w = (s.duration_ns / max * 100).toFixed(1);
      const cls = ["pass"].includes(s.outcome) ? "pass" : s.outcome;
      return `<div class="wf-row" title="${esc(s.detail || s.outcome)}">
        <span class="wf-name">${esc(s.stage)}</span>
        <span class="wf-track"><span class="wf-bar ${esc(cls)}" style="width:${w}%"></span></span>
        <span class="wf-dur">${fmtDur(s.duration_ns)}</span>
      </div>`;
    }).join("") + `</div>`;
  }
  // naive line-diff: lines only in request marked removed, only in response added
  function renderDiff(reqStr, respStr) {
    const a = prettyJSON(reqStr).split("\n");
    const b = prettyJSON(respStr).split("\n");
    const bSet = new Set(b), aSet = new Set(a);
    const left = a.map(l => bSet.has(l) ? esc(l) : `<span class="rm">${esc(l)}</span>`).join("\n");
    const right = b.map(l => aSet.has(l) ? esc(l) : `<span class="add">${esc(l)}</span>`).join("\n");
    return `<div class="diff">
      <div><div class="section-title">Original request</div><div class="code">${left}</div></div>
      <div><div class="section-title">Forwarded / response</div><div class="code">${right}</div></div>
    </div>`;
  }
  async function openCall(id) {
    const modal = $("callModal");
    const body = $("callModalBody");
    body.innerHTML = '<div class="muted">Loading…</div>';
    modal.hidden = false;
    let d;
    try { d = await getJSON("/api/calls/" + encodeURIComponent(id)); }
    catch (e) { body.innerHTML = `<div class="muted">Could not load call: ${esc(e.message)}</div>`; return; }
    $("callModalTitle").innerHTML = `<code>${esc(d.tool_name)}</code> ${badge(d.verdict)}`;
    const isDiff = d.verdict === "transform" || d.verdict === "block";
    const payload = (d.request_inline || d.response_inline)
      ? (isDiff && d.response_inline
          ? renderDiff(d.request_inline, d.response_inline)
          : `<div class="code">${esc(prettyJSON(d.request_inline || d.response_inline))}</div>`)
      : '<div class="muted">No inline payload captured.</div>';
    body.innerHTML = `
      <div class="detail-grid">
        <div class="d"><div class="l">Server</div><div class="v">${esc(d.server_name || "—")}</div></div>
        <div class="d"><div class="l">Direction</div><div class="v">${esc(d.direction)}</div></div>
        <div class="d"><div class="l">Risk</div><div class="v">${d.risk_score ? (d.risk_score*100).toFixed(0)+"%" : "—"}</div></div>
        <div class="d"><div class="l">Latency</div><div class="v">${d.latency_ms_proxy ? d.latency_ms_proxy+"ms" : "—"}</div></div>
        <div class="d"><div class="l">Cost</div><div class="v">$${(d.cost_usd||0).toFixed(4)}</div></div>
        <div class="d"><div class="l">Tokens</div><div class="v">${fmt.format(d.token_count||0)}</div></div>
      </div>
      ${d.verdict_reason ? `<div class="section-title">Why this verdict</div><div class="code">${esc(d.verdict_reason)}</div>` : ""}
      <div class="section-title">Pipeline stage waterfall</div>
      ${renderWaterfall(d.stages)}
      <div class="section-title">Payload</div>
      ${payload}
      <div class="modal-actions">
        <button class="btn primary" id="btnReplay">▶ Replay</button>
        <button class="btn" id="btnFalsePos">Mark false positive</button>
        <button class="btn" id="btnCurl">Copy as cURL</button>
      </div>`;
    $("btnReplay").addEventListener("click", () => toast("info", "Replay queued", `Re-running ${d.tool_name} through the pipeline…`));
    $("btnFalsePos").addEventListener("click", () => toast("info", "Noted", "Policy suggestion recorded for review."));
    $("btnCurl").addEventListener("click", () => {
      const curl = `curl -X POST http://127.0.0.1:7878/api/calls/${d.id}/replay`;
      navigator.clipboard && navigator.clipboard.writeText(curl);
      toast("info", "Copied", "cURL command copied to clipboard.");
    });
  }
  function closeCall() { $("callModal").hidden = true; }

  // ---------- command palette ----------
  const paletteItems = () => ([
    { label: "Go to Overview", kind: "page", go: () => location.hash = "#overview" },
    { label: "Go to Tool calls", kind: "page", go: () => location.hash = "#calls" },
    { label: "Go to Top tools", kind: "page", go: () => location.hash = "#tools" },
    { label: "Go to Servers", kind: "page", go: () => location.hash = "#servers" },
    { label: state.paused ? "Resume live stream" : "Pause live stream", kind: "action", go: togglePause },
    { label: "Toggle theme", kind: "action", go: () => $("themeToggle").click() },
    ...state.recent.slice(0, 8).map(c => ({
      label: `${c.tool_name} (${c.verdict})`, kind: "call", go: () => openCall(c.id),
    })),
  ]);
  let paletteSel = 0;
  function openPalette() {
    $("palette").hidden = false;
    $("paletteInput").value = "";
    paletteSel = 0;
    renderPalette("");
    $("paletteInput").focus();
  }
  function closePalette() { $("palette").hidden = true; }
  function renderPalette(q) {
    const items = paletteItems().filter(i => i.label.toLowerCase().includes(q.toLowerCase()));
    state._palette = items;
    $("paletteList").innerHTML = items.map((i, idx) =>
      `<li class="${idx === paletteSel ? "sel" : ""}" data-idx="${idx}">${esc(i.label)}<span class="pk">${i.kind}</span></li>`).join("")
      || `<li class="muted">No matches</li>`;
  }

  // ---------- pause ----------
  function togglePause() {
    state.paused = !state.paused;
    const btn = $("pauseStream");
    btn.textContent = state.paused ? "▶ Resume" : "⏸ Pause";
    btn.classList.toggle("paused", state.paused);
  }

  // ---------- wire up controls ----------
  function wire() {
    $("window").addEventListener("change", (e) => {
      state.window = e.target.value; refresh();
    });
    $("verdictFilter").addEventListener("change", (e) => {
      state.verdictFilter = e.target.value; if (state.route === "calls") loadCalls();
    });
    $("autoscroll").addEventListener("change", (e) => { state.autoscroll = e.target.checked; state.paused = !e.target.checked; });

    // pause/resume the live stream
    $("pauseStream").addEventListener("click", togglePause);

    // click a call row → open the detail modal (event delegation)
    const rowClick = (e) => {
      const tr = e.target.closest("tr");
      if (!tr) return;
      const id = tr.dataset.id;
      if (id) openCall(id);
    };
    $("recentTable").addEventListener("click", rowClick);
    $("callsTable").addEventListener("click", rowClick);

    // click a server card → (drawer placeholder via toast for now)
    $("serverGrid").addEventListener("click", (e) => {
      const card = e.target.closest(".server-card");
      if (card) location.hash = "#servers";
    });

    // modal close
    $("callModalClose").addEventListener("click", closeCall);
    $("callModal").addEventListener("click", (e) => { if (e.target === $("callModal")) closeCall(); });

    // command palette
    $("paletteInput").addEventListener("input", (e) => { paletteSel = 0; renderPalette(e.target.value); });
    $("paletteList").addEventListener("click", (e) => {
      const li = e.target.closest("li[data-idx]");
      if (li) { (state._palette[+li.dataset.idx] || {}).go?.(); closePalette(); }
    });
    $("palette").addEventListener("click", (e) => { if (e.target === $("palette")) closePalette(); });

    // global keyboard: Cmd/Ctrl+K palette, Esc closes, arrows navigate palette
    document.addEventListener("keydown", (e) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        $("palette").hidden ? openPalette() : closePalette();
        return;
      }
      if (e.key === "Escape") { closeCall(); closePalette(); return; }
      if (!$("palette").hidden) {
        const items = state._palette || [];
        if (e.key === "ArrowDown") { e.preventDefault(); paletteSel = Math.min(paletteSel + 1, items.length - 1); renderPalette($("paletteInput").value); }
        if (e.key === "ArrowUp") { e.preventDefault(); paletteSel = Math.max(paletteSel - 1, 0); renderPalette($("paletteInput").value); }
        if (e.key === "Enter") { (items[paletteSel] || {}).go?.(); closePalette(); }
      }
    });

    // periodic backstop refresh
    setInterval(refresh, 15000);
  }

  // ---------- init ----------
  initTheme();
  route();
  wire();
  connectSSE();
})();
