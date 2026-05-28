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
    series: [],
    es: null,
    reconnectMs: 1000,
  };

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
    return `<tr ${isNew ? 'class="new"' : ''}>
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
    const rows = await getJSON("/api/calls?limit=10");
    $("recentTable").querySelector("tbody").innerHTML = rows.map(c => callRow(c, false)).join("");
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

  // ---------- servers ----------
  async function loadServers() {
    const rows = await getJSON("/api/servers?limit=200");
    const tbody = $("serversTable").querySelector("tbody");
    tbody.innerHTML = rows.map(s => `
      <tr>
        <td><strong>${esc(s.name)}</strong></td>
        <td class="muted">${esc(s.transport)}</td>
        <td>${s.trust_score != null ? s.trust_score : '<span class="muted">—</span>'}</td>
        <td>${fmt.format(s.total_calls)}</td>
        <td class="${s.blocks ? '' : 'muted'}">${fmt.format(s.blocks)}</td>
        <td class="muted">${timeAgo(s.first_seen_at)} ago</td>
        <td class="muted">${timeAgo(s.last_seen_at)} ago</td>
      </tr>`).join("");
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
      if (!state.autoscroll) return;
      try {
        const c = JSON.parse(ev.data);
        const row = callRow(c, true);
        if (state.route === "overview") {
          prependRow($("recentTable"), row, 10);
        } else if (state.route === "calls") {
          if (state.verdictFilter && state.verdictFilter !== c.verdict) return;
          prependRow($("callsTable"), row, 200);
        }
        // bump live KPIs
        bumpKpi("kpiCalls");
        if (c.verdict === "block") bumpKpi("kpiBlocked");
        if (c.verdict === "flag") bumpKpi("kpiFlagged");
        if (c.verdict === "transform") bumpKpi("kpiTransform");
      } catch {}
    });
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

  // ---------- wire up controls ----------
  function wire() {
    $("window").addEventListener("change", (e) => {
      state.window = e.target.value; refresh();
    });
    $("verdictFilter").addEventListener("change", (e) => {
      state.verdictFilter = e.target.value; if (state.route === "calls") loadCalls();
    });
    $("autoscroll").addEventListener("change", (e) => { state.autoscroll = e.target.checked; });
    // periodic backstop refresh
    setInterval(refresh, 15000);
  }

  // ---------- init ----------
  initTheme();
  route();
  wire();
  connectSSE();
})();
