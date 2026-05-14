package engine

import (
	"net/http"
	"strings"
)

func (e *Engine) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = strings.NewReader(dashboardHTML).WriteTo(w)
}

const dashboardHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>ROAD Control Deck</title>
  <style>
    :root {
      --bg: #0d1412;
      --bg-2: #151f1b;
      --panel: rgba(246, 239, 214, 0.94);
      --ink: #101714;
      --muted: #647169;
      --line: rgba(16, 23, 20, 0.14);
      --accent: #ffb02e;
      --accent-2: #43d6a4;
      --danger: #d94f3d;
      --warn: #bd7b00;
      --ok: #0b8c5d;
      --shadow: 0 24px 80px rgba(0, 0, 0, 0.34);
      --mono: "Cascadia Mono", "Lucida Console", monospace;
      --sans: "Aptos Display", "Bahnschrift", "Candara", "Trebuchet MS", sans-serif;
    }

    * { box-sizing: border-box; }

    body {
      margin: 0;
      min-height: 100vh;
      color: var(--ink);
      font-family: var(--sans);
      background:
        radial-gradient(circle at 12% 10%, rgba(255, 176, 46, 0.22), transparent 34rem),
        radial-gradient(circle at 86% 2%, rgba(67, 214, 164, 0.24), transparent 30rem),
        linear-gradient(135deg, var(--bg) 0%, var(--bg-2) 48%, #2a2419 100%);
    }

    button, input, select { font: inherit; }
    button { border: 0; cursor: pointer; }

    .shell {
      width: min(1320px, calc(100% - 32px));
      margin: 0 auto;
      padding: 26px 0 38px;
    }

    .hero {
      display: grid;
      grid-template-columns: 1.45fr 0.55fr;
      gap: 16px;
      margin-bottom: 16px;
    }

    .title-card, .status-card, .card, .auth-panel {
      position: relative;
      overflow: hidden;
      border: 1px solid rgba(255, 255, 255, 0.18);
      box-shadow: var(--shadow);
    }

    .title-card {
      min-height: 230px;
      padding: 30px;
      color: #f6ecd0;
      background:
        linear-gradient(120deg, rgba(255, 176, 46, 0.24), transparent 48%),
        linear-gradient(145deg, rgba(24, 35, 31, 0.98), rgba(12, 19, 16, 0.94));
      border-radius: 30px;
    }

    .title-card:after {
      content: "";
      position: absolute;
      right: -88px;
      top: -82px;
      width: 280px;
      height: 280px;
      border-radius: 999px;
      border: 44px solid rgba(67, 214, 164, 0.15);
    }

    .eyebrow {
      margin: 0 0 10px;
      color: var(--accent-2);
      font-size: 13px;
      font-weight: 900;
      letter-spacing: 0.18em;
      text-transform: uppercase;
    }

    h1 {
      margin: 0;
      max-width: 820px;
      font-size: clamp(46px, 8vw, 92px);
      line-height: 0.9;
      letter-spacing: -0.065em;
    }

    .hero-copy {
      max-width: 680px;
      margin: 18px 0 0;
      color: rgba(246, 236, 208, 0.72);
      font-size: 17px;
      line-height: 1.5;
    }

    .toolbar {
      display: flex;
      flex-wrap: wrap;
      gap: 9px;
      margin-top: 22px;
    }

    .btn, .ghost-btn, .small-btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
      border-radius: 999px;
      font-weight: 900;
      text-decoration: none;
    }

    .btn {
      min-height: 38px;
      padding: 9px 13px;
      color: #122018;
      background: var(--accent-2);
      font-size: 13px;
    }

    .ghost-btn {
      min-height: 38px;
      padding: 9px 13px;
      color: #f6ecd0;
      background: rgba(255, 255, 255, 0.09);
      border: 1px solid rgba(255, 255, 255, 0.15);
      font-size: 13px;
    }

    .status-card {
      display: grid;
      align-content: space-between;
      min-height: 230px;
      padding: 24px;
      color: #f6ecd0;
      background: linear-gradient(160deg, rgba(67, 214, 164, 0.2), rgba(255, 176, 46, 0.12)), #131d19;
      border-radius: 30px;
    }

    .pulse {
      display: inline-flex;
      align-items: center;
      gap: 10px;
      font-weight: 900;
      letter-spacing: 0.05em;
      text-transform: uppercase;
    }

    .pulse-dot {
      width: 12px;
      height: 12px;
      border-radius: 999px;
      background: var(--accent-2);
      box-shadow: 0 0 0 10px rgba(67, 214, 164, 0.28);
      animation: breathe 1.6s ease-in-out infinite;
    }

    @keyframes breathe {
      0%, 100% { transform: scale(0.82); opacity: 0.75; }
      50% { transform: scale(1); opacity: 1; }
    }

    .big-number {
      margin: 18px 0 5px;
      font-size: clamp(42px, 7vw, 74px);
      font-weight: 950;
      line-height: 0.9;
      letter-spacing: -0.07em;
    }

    .subtle { color: rgba(246, 236, 208, 0.68); }
    .mono { font-family: var(--mono); overflow-wrap: anywhere; }

    .auth-panel {
      display: none;
      gap: 10px;
      align-items: end;
      margin-bottom: 16px;
      padding: 16px;
      color: #f6ecd0;
      background: linear-gradient(135deg, rgba(217, 79, 61, 0.26), rgba(255, 176, 46, 0.12)), #181f1b;
      border-radius: 24px;
    }

    .auth-panel.visible { display: grid; grid-template-columns: 1fr 1fr auto auto; }

    label {
      display: grid;
      gap: 7px;
      color: rgba(246, 236, 208, 0.72);
      font-size: 12px;
      font-weight: 900;
      letter-spacing: 0.08em;
      text-transform: uppercase;
    }

    input, select {
      width: 100%;
      min-height: 38px;
      padding: 9px 11px;
      color: #f6ecd0;
      background: rgba(255, 255, 255, 0.08);
      border: 1px solid rgba(255, 255, 255, 0.16);
      border-radius: 13px;
      outline: none;
    }

    .grid {
      display: grid;
      grid-template-columns: repeat(4, 1fr);
      gap: 14px;
      margin-bottom: 14px;
    }

    .card {
      padding: 20px;
      background: var(--panel);
      border-radius: 24px;
      backdrop-filter: blur(16px);
    }

    .metric-label {
      color: var(--muted);
      font-size: 12px;
      font-weight: 950;
      letter-spacing: 0.12em;
      text-transform: uppercase;
    }

    .metric-value {
      margin-top: 12px;
      font-size: 34px;
      font-weight: 950;
      letter-spacing: -0.05em;
    }

    .metric-foot {
      margin-top: 6px;
      color: var(--muted);
      font-size: 13px;
    }

    .layout {
      display: grid;
      grid-template-columns: 0.86fr 1.14fr;
      gap: 14px;
      margin-bottom: 14px;
    }

    .section-title {
      display: flex;
      align-items: baseline;
      justify-content: space-between;
      gap: 12px;
      margin-bottom: 16px;
    }

    h2 {
      margin: 0;
      font-size: 23px;
      letter-spacing: -0.04em;
    }

    .pill {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      padding: 7px 10px;
      border: 1px solid var(--line);
      border-radius: 999px;
      color: var(--muted);
      font-size: 12px;
      font-weight: 900;
      white-space: nowrap;
    }

    .tabs {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      margin-bottom: 14px;
    }

    .tab {
      min-height: 38px;
      padding: 9px 13px;
      border-radius: 999px;
      color: #f6ecd0;
      background: rgba(255, 255, 255, 0.08);
      border: 1px solid rgba(255, 255, 255, 0.14);
      font-size: 13px;
      font-weight: 900;
    }

    .tab.active {
      color: #122018;
      background: var(--accent);
    }

    .tab-panel { display: none; }
    .tab-panel.active { display: block; }

    .kv {
      display: grid;
      grid-template-columns: 145px minmax(0, 1fr);
      gap: 9px 14px;
      font-size: 14px;
    }

    .kv div:nth-child(odd) {
      color: var(--muted);
      font-weight: 900;
    }

    .callout {
      margin-top: 14px;
      padding: 13px;
      border: 1px solid rgba(16, 23, 20, 0.12);
      border-radius: 18px;
      background: rgba(255, 255, 255, 0.34);
    }

    .inline-actions {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
      margin-top: 10px;
    }

    .small-btn {
      min-height: 32px;
      padding: 7px 10px;
      color: var(--ink);
      background: rgba(67, 214, 164, 0.28);
      border: 1px solid rgba(16, 23, 20, 0.12);
      font-size: 12px;
    }

    .plugin-list {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 10px;
    }

    .plugin {
      border: 1px solid var(--line);
      border-radius: 18px;
      padding: 13px;
      background: rgba(255, 255, 255, 0.36);
    }

    .plugin-head {
      display: flex;
      justify-content: space-between;
      gap: 10px;
      margin-bottom: 10px;
    }

    .plugin strong {
      display: block;
      overflow-wrap: anywhere;
    }

    .tag-row {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
      margin: 8px 0 0;
    }

    .tag {
      padding: 5px 8px;
      border-radius: 999px;
      color: var(--muted);
      background: rgba(16, 23, 20, 0.06);
      font-size: 11px;
      font-weight: 900;
    }

    .tag.ok { color: var(--ok); background: rgba(67, 214, 164, 0.16); }
    .tag.warn { color: var(--warn); background: rgba(255, 176, 46, 0.2); }
    .tag.danger { color: var(--danger); background: rgba(217, 79, 61, 0.12); }

    .mini {
      display: grid;
      grid-template-columns: repeat(3, 1fr);
      gap: 8px;
      color: var(--muted);
      font-size: 12px;
    }

    .mini b {
      display: block;
      color: var(--ink);
      font-size: 15px;
    }

    table {
      width: 100%;
      border-collapse: collapse;
      font-size: 13px;
    }

    th, td {
      padding: 12px 10px;
      border-bottom: 1px solid var(--line);
      text-align: left;
      vertical-align: top;
    }

    th {
      color: var(--muted);
      font-size: 11px;
      letter-spacing: 0.12em;
      text-transform: uppercase;
    }

    .empty {
      padding: 26px;
      border: 1px dashed rgba(16, 23, 20, 0.22);
      border-radius: 18px;
      color: var(--muted);
      text-align: center;
    }

    .danger { color: var(--danger); }
    .ok { color: var(--ok); }
    .warn { color: var(--warn); }

    @media (max-width: 980px) {
      .hero, .layout { grid-template-columns: 1fr; }
      .grid { grid-template-columns: repeat(2, 1fr); }
      .plugin-list { grid-template-columns: 1fr; }
      .auth-panel.visible { grid-template-columns: 1fr; }
    }

    @media (max-width: 560px) {
      .shell { width: min(100% - 20px, 1320px); padding-top: 12px; }
      .title-card, .status-card, .card, .auth-panel { border-radius: 20px; }
      .title-card { min-height: 0; padding: 22px; }
      .grid { grid-template-columns: 1fr; }
      .kv { grid-template-columns: 1fr; }
      table { display: block; overflow-x: auto; white-space: nowrap; }
    }
  </style>
</head>
<body>
  <main class="shell">
    <section class="hero">
      <div class="title-card">
        <p class="eyebrow">ROAD Proxy v3</p>
        <h1>Control Deck</h1>
        <p class="hero-copy">Embedded local web dashboard for server health, active sessions, plugins, UDP diagnostics, security posture, and ready-to-copy WebSocket endpoint hints. No Electron, no external assets, no build step.</p>
        <div class="toolbar">
          <button class="btn" id="refreshNow">Refresh now</button>
          <button class="ghost-btn" id="pauseToggle">Pause</button>
          <button class="ghost-btn" id="clearToken">Clear token</button>
          <select id="refreshEvery" aria-label="Refresh interval">
            <option value="1000">1s refresh</option>
            <option value="2000" selected>2s refresh</option>
            <option value="5000">5s refresh</option>
            <option value="10000">10s refresh</option>
          </select>
        </div>
      </div>
      <div class="status-card">
        <div class="pulse"><span class="pulse-dot"></span><span id="statusText">Connecting</span></div>
        <div>
          <div id="pingValue" class="big-number">-</div>
          <div class="subtle">browser to control API RTT</div>
        </div>
        <div class="subtle mono" id="updatedAt">waiting for first sample</div>
      </div>
    </section>

    <section class="auth-panel" id="authPanel">
      <label>Auth header
        <input id="authHeader" value="X-ROAD-Token" autocomplete="off">
      </label>
      <label>Token
        <input id="authToken" type="password" autocomplete="off" placeholder="ROAD shared token">
      </label>
      <button class="btn" id="saveToken">Use token</button>
      <div class="subtle" id="authMessage">Control API returned 401. Enter the same token used by ROAD clients.</div>
    </section>

    <section class="grid">
      <div class="card">
        <div class="metric-label">Active clients</div>
        <div class="metric-value" id="activeClients">0</div>
        <div class="metric-foot" id="totalConnections">0 total sessions</div>
      </div>
      <div class="card">
        <div class="metric-label">Plugins</div>
        <div class="metric-value" id="pluginCount">0</div>
        <div class="metric-foot" id="defaultPlugin">default: -</div>
      </div>
      <div class="card">
        <div class="metric-label">Traffic</div>
        <div class="metric-value" id="trafficTotal">0 B</div>
        <div class="metric-foot" id="trafficRate">0 B/s live rate</div>
      </div>
      <div class="card">
        <div class="metric-label">UDP health</div>
        <div class="metric-value" id="errorCount">0</div>
        <div class="metric-foot" id="udpHealth">errors, loss, jitter</div>
      </div>
    </section>

    <section class="tabs" aria-label="Dashboard sections">
      <button class="tab active" data-tab="overview">Overview</button>
      <button class="tab" data-tab="sessions">Sessions</button>
      <button class="tab" data-tab="plugins">Plugins</button>
      <button class="tab" data-tab="udp">UDP Diagnostics</button>
      <button class="tab" data-tab="security">Security</button>
      <button class="tab" data-tab="api">API</button>
    </section>

    <section id="tab-overview" class="tab-panel active">
      <div class="layout">
        <div class="card">
          <div class="section-title">
            <h2>Runtime</h2>
            <span class="pill" id="planePill">control</span>
          </div>
          <div class="kv">
            <div>Control listen</div><div class="mono" id="controlListen">-</div>
            <div>Data listen</div><div class="mono" id="dataListen">-</div>
            <div>WebSocket path</div><div class="mono" id="wsPath">-</div>
            <div>Default target</div><div class="mono" id="defaultTarget">-</div>
            <div>Buffer size</div><div class="mono" id="bufferSize">-</div>
            <div>Uptime</div><div class="mono" id="uptime">-</div>
          </div>
          <div class="callout">
            <div class="metric-label">Derived WebSocket endpoint</div>
            <div class="mono" id="derivedEndpoint">-</div>
            <div class="inline-actions">
              <button class="small-btn" data-copy="derivedEndpoint">Copy endpoint</button>
              <button class="small-btn" data-copy="defaultPluginValue">Copy default plugin</button>
            </div>
            <span id="defaultPluginValue" hidden></span>
          </div>
        </div>

        <div class="card">
          <div class="section-title">
            <h2>Live Plugins</h2>
            <span class="pill" id="pluginPill">0 loaded</span>
          </div>
          <div class="plugin-list" id="overviewPluginList"></div>
        </div>
      </div>
    </section>

    <section id="tab-sessions" class="tab-panel">
      <div class="card">
        <div class="section-title">
          <h2>Sessions</h2>
          <span class="pill" id="sessionPill">0 active</span>
        </div>
        <div id="sessionMount"></div>
      </div>
    </section>

    <section id="tab-plugins" class="tab-panel">
      <div class="card">
        <div class="section-title">
          <h2>Plugin Catalog</h2>
          <span class="pill" id="availablePill">0 available</span>
        </div>
        <div id="pluginCatalog"></div>
      </div>
    </section>

    <section id="tab-udp" class="tab-panel">
      <div class="card">
        <div class="section-title">
          <h2>UDP Diagnostics</h2>
          <span class="pill">loss / reorder / MTU</span>
        </div>
        <div id="udpMount"></div>
      </div>
    </section>

    <section id="tab-security" class="tab-panel">
      <div class="layout">
        <div class="card">
          <div class="section-title">
            <h2>Security</h2>
            <span class="pill" id="authPill">auth unknown</span>
          </div>
          <div class="kv">
            <div>Auth</div><div id="authEnabled">-</div>
            <div>Auth header</div><div class="mono" id="authHeaderInfo">-</div>
            <div>Tokens</div><div id="tokenCount">-</div>
            <div>Plugin API</div><div id="pluginAPIState">-</div>
            <div>Trust proxy IP</div><div id="trustProxy">-</div>
          </div>
        </div>
        <div class="card">
          <div class="section-title">
            <h2>Limits</h2>
            <span class="pill">public hardening</span>
          </div>
          <div class="kv">
            <div>Allowed hosts</div><div class="mono" id="allowedHosts">-</div>
            <div>Allowed origins</div><div class="mono" id="allowedOrigins">-</div>
            <div>Max connections</div><div id="maxConnections">-</div>
            <div>Max per IP</div><div id="maxPerIP">-</div>
            <div>Rate/min/IP</div><div id="rateLimit">-</div>
          </div>
        </div>
      </div>
    </section>

    <section id="tab-api" class="tab-panel">
      <div class="card">
        <div class="section-title">
          <h2>Control API</h2>
          <span class="pill">same origin</span>
        </div>
        <div id="apiMount"></div>
      </div>
    </section>
  </main>

  <script>
    const state = {
      lastBytes: 0,
      lastAt: 0,
      paused: false,
      interval: null,
      lastInfo: null,
      authToken: sessionStorage.getItem("road.control.token") || "",
      authHeader: sessionStorage.getItem("road.control.header") || "X-ROAD-Token"
    };

    const el = (id) => document.getElementById(id);

    function text(id, value) {
      const node = el(id);
      if (node) node.textContent = value;
    }

    function showAuth(show, message) {
      const panel = el("authPanel");
      panel.classList.toggle("visible", !!show);
      if (message) text("authMessage", message);
      el("authToken").value = state.authToken;
      el("authHeader").value = state.authHeader || "X-ROAD-Token";
    }

    function requestHeaders() {
      const headers = { "Accept": "application/json" };
      const token = state.authToken.trim();
      const header = (state.authHeader || "X-ROAD-Token").trim() || "X-ROAD-Token";
      if (!token) return headers;
      if (header.toLowerCase() === "authorization") {
        headers[header] = "Bearer " + token;
      } else {
        headers[header] = token;
        headers["Authorization"] = "Bearer " + token;
      }
      if (header.toLowerCase() !== "x-road-token") {
        headers["X-ROAD-Token"] = token;
      }
      return headers;
    }

    async function fetchJSON(path) {
      const res = await fetch(path, { cache: "no-store", headers: requestHeaders() });
      if (!res.ok) {
        const err = new Error(path + " http " + res.status);
        err.status = res.status;
        throw err;
      }
      return await res.json();
    }

    async function fetchText(path) {
      const res = await fetch(path, { cache: "no-store", headers: requestHeaders() });
      if (!res.ok) {
        const err = new Error(path + " http " + res.status);
        err.status = res.status;
        throw err;
      }
      return await res.text();
    }

    async function measurePing() {
      const start = performance.now();
      await fetchJSON("/api/ping");
      return performance.now() - start;
    }

    function fmtBytes(n) {
      n = Number(n || 0);
      if (n < 1024) return n + " B";
      const units = ["KB", "MB", "GB", "TB"];
      let v = n / 1024;
      let i = 0;
      while (v >= 1024 && i < units.length - 1) {
        v = v / 1024;
        i++;
      }
      return v.toFixed(v >= 100 ? 0 : v >= 10 ? 1 : 2) + " " + units[i];
    }

    function fmtPercent(n) {
      n = Number(n || 0);
      return n.toFixed(n >= 10 ? 1 : 2) + "%";
    }

    function fmtMS(n) {
      n = Number(n || 0);
      return n.toFixed(n >= 10 ? 1 : 2) + " ms";
    }

    function fmtBool(value) {
      return value ? "enabled" : "disabled";
    }

    function fmtList(values) {
      if (!Array.isArray(values) || values.length === 0) return "not restricted";
      return values.join(", ");
    }

    function fmtLimit(value) {
      value = Number(value || 0);
      return value > 0 ? String(value) : "unlimited";
    }

    function fmtDuration(seconds) {
      seconds = Number(seconds || 0);
      const h = Math.floor(seconds / 3600);
      const m = Math.floor((seconds % 3600) / 60);
      const s = Math.floor(seconds % 60);
      if (h > 0) return h + "h " + m + "m " + s + "s";
      if (m > 0) return m + "m " + s + "s";
      return s + "s";
    }

    function escapeHTML(value) {
      return String(value == null ? "" : value).replace(/[&<>"']/g, (ch) => ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#39;"
      }[ch]));
    }

    function updateTraffic(stats) {
      const total = Number(stats.total_bytes_rx || 0) + Number(stats.total_bytes_tx || 0);
      text("trafficTotal", fmtBytes(total));
      const now = performance.now();
      if (state.lastAt > 0) {
        const rate = Math.max(0, total - state.lastBytes) / ((now - state.lastAt) / 1000);
        text("trafficRate", fmtBytes(rate) + "/s live rate");
      }
      state.lastBytes = total;
      state.lastAt = now;
    }

    function pluginNames(stats, plugins) {
      const fromStats = Object.keys(stats.plugins || {});
      const enabled = plugins && Array.isArray(plugins.enabled) ? plugins.enabled : [];
      const loaded = plugins && Array.isArray(plugins.loaded) ? plugins.loaded.map((p) => p.name) : [];
      return Array.from(new Set(enabled.concat(loaded).concat(fromStats))).filter(Boolean).sort();
    }

    function pluginInfoByName(plugins) {
      const out = {};
      if (plugins && Array.isArray(plugins.loaded)) {
        plugins.loaded.forEach((p) => { if (p && p.name) out[p.name] = p; });
      }
      if (plugins && plugins.default && plugins.default.name) {
        out[plugins.default.name] = plugins.default;
      }
      return out;
    }

    function renderPluginSummary(stats, plugins) {
      const mount = el("overviewPluginList");
      const names = pluginNames(stats, plugins);
      const info = pluginInfoByName(plugins);
      text("pluginCount", String(names.length));
      text("pluginPill", names.length + " loaded");
      const def = plugins && plugins.default && plugins.default.name ? plugins.default.name : "-";
      text("defaultPlugin", "default: " + def);
      text("defaultPluginValue", def);
      if (plugins && plugins.error) {
        mount.innerHTML = '<div class="empty">Plugin API unavailable: ' + escapeHTML(plugins.error) + '</div>';
        return;
      }
      if (!names.length) {
        mount.innerHTML = '<div class="empty">No plugin stats yet.</div>';
        return;
      }
      mount.innerHTML = names.map((name) => {
        const p = (stats.plugins || {})[name] || {};
        const meta = info[name] || {};
        const udp = p.udp || {};
        const rx = udp.rx || {};
        const peerClass = meta.udp_peer_broadcast ? "tag warn" : "tag ok";
        return '<div class="plugin">' +
          '<div class="plugin-head"><strong class="mono">' + escapeHTML(name) + '</strong><span class="tag">' + escapeHTML(meta.target_network || "-") + '</span></div>' +
          '<div class="mini">' +
          '<span>active<b>' + Number(p.active_connections || 0) + '</b></span>' +
          '<span>rx<b>' + fmtBytes(p.total_bytes_rx || 0) + '</b></span>' +
          '<span>loss<b>' + fmtPercent(rx.loss_percent || 0) + '</b></span>' +
          '</div>' +
          '<div class="tag-row">' +
          '<span class="tag">' + escapeHTML(meta.runtime_mode || "runtime") + '</span>' +
          '<span class="tag">reply ' + escapeHTML(meta.udp_reply_policy || "any") + '</span>' +
          '<span class="' + peerClass + '">peer broadcast ' + (meta.udp_peer_broadcast ? "on" : "off") + '</span>' +
          '</div>' +
          '</div>';
      }).join("");
    }

    function renderPluginCatalog(plugins) {
      const mount = el("pluginCatalog");
      if (plugins && plugins.error) {
        text("availablePill", "private");
        mount.innerHTML = '<div class="empty">Plugin catalog is private or unavailable: ' + escapeHTML(plugins.error) + '</div>';
        return;
      }
      const loaded = plugins && Array.isArray(plugins.loaded) ? plugins.loaded : [];
      const available = plugins && Array.isArray(plugins.available) ? plugins.available : [];
      const enabled = plugins && Array.isArray(plugins.enabled) ? plugins.enabled : [];
      text("availablePill", available.length + " available");
      if (!loaded.length && !available.length) {
        mount.innerHTML = '<div class="empty">No plugins found.</div>';
        return;
      }
      const loadedNames = new Set(loaded.map((p) => p.name));
      const loadedHTML = loaded.map((p) => pluginCatalogCard(p, true)).join("");
      const unloadedHTML = available.filter((name) => !loadedNames.has(name)).map((name) => pluginCatalogCard({ name: name }, enabled.includes(name))).join("");
      mount.innerHTML = '<div class="plugin-list">' + loadedHTML + unloadedHTML + '</div>';
    }

    function pluginCatalogCard(p, enabled) {
      const name = p.name || "unknown";
      const peer = p.udp_peer_broadcast ? '<span class="tag warn">peer broadcast on</span>' : '<span class="tag ok">peer broadcast off</span>';
      const target = [p.target_network, p.target_address].filter(Boolean).join(" ") || "not loaded";
      return '<div class="plugin">' +
        '<div class="plugin-head"><strong class="mono">' + escapeHTML(name) + '</strong><span class="tag ' + (enabled ? "ok" : "") + '">' + (enabled ? "enabled" : "available") + '</span></div>' +
        '<div class="metric-foot">' + escapeHTML(p.description || target) + '</div>' +
        '<div class="tag-row">' +
          '<span class="tag">' + escapeHTML(p.version || "version n/a") + '</span>' +
          '<span class="tag">' + escapeHTML(p.runtime_mode || "not loaded") + '</span>' +
          peer +
        '</div>' +
        '<div class="inline-actions">' +
          '<button class="small-btn" data-api-open="/api/plugin/info/' + encodeURIComponent(name) + '">plugin.json</button>' +
          '<button class="small-btn" data-api-open="/api/plugin/config/' + encodeURIComponent(name) + '">config.json</button>' +
          '<button class="small-btn" data-api-open="/api/plugin/download/' + encodeURIComponent(name) + '">bundle</button>' +
        '</div>' +
      '</div>';
    }

    function renderSessions(sessions) {
      const list = sessions && Array.isArray(sessions.sessions) ? sessions.sessions : [];
      text("sessionPill", list.length + " active");
      text("activeClients", String(list.length));
      const mount = el("sessionMount");
      if (!list.length) {
        mount.innerHTML = '<div class="empty">No active sessions. Start a ROAD client to see live rows here.</div>';
        return;
      }
      const rows = list.map((s) => {
        const rx = ((s.udp || {}).rx || {});
        const tx = ((s.udp || {}).tx || {});
        return '<tr>' +
          '<td class="mono">' + escapeHTML(s.id || "-") + '</td>' +
          '<td>' + escapeHTML(s.plugin || "-") + '<br><span class="metric-foot">' + escapeHTML(s.transport || "-") + " / " + escapeHTML(s.network || "-") + '</span></td>' +
          '<td class="mono">' + escapeHTML(s.remote_addr || "-") + '</td>' +
          '<td class="mono">' + escapeHTML(s.target_addr || "-") + '</td>' +
          '<td>' + fmtBytes(s.bytes_rx || 0) + ' / ' + fmtBytes(s.bytes_tx || 0) + '</td>' +
          '<td>' + fmtDuration(s.age_seconds) + ' / ' + fmtDuration(s.idle_seconds) + '</td>' +
          '<td>rx ' + fmtMS(rx.jitter_ms || 0) + '<br>tx ' + fmtMS(tx.jitter_ms || 0) + '</td>' +
          '<td>' + fmtPercent(rx.loss_percent || 0) + '</td>' +
          '</tr>';
      }).join("");
      mount.innerHTML = '<table><thead><tr><th>ID</th><th>Plugin</th><th>Remote</th><th>Target</th><th>RX / TX</th><th>Age / Idle</th><th>Jitter</th><th>Loss</th></tr></thead><tbody>' + rows + '</tbody></table>';
    }

    function renderUDP(stats) {
      const udp = stats.udp || {};
      const rx = udp.rx || {};
      const tx = udp.tx || {};
      const row = (label, flow) => '<tr>' +
        '<td>' + label + '</td>' +
        '<td>' + Number(flow.packets || 0) + '</td>' +
        '<td>' + fmtBytes(flow.bytes || 0) + '</td>' +
        '<td>' + fmtMS(flow.jitter_ms || 0) + '</td>' +
        '<td>' + fmtMS(flow.max_gap_ms || 0) + '</td>' +
        '<td>' + fmtPercent(flow.loss_percent || 0) + '</td>' +
        '<td>' + Number(flow.reordered_packets || 0) + ' / ' + Number(flow.duplicate_packets || 0) + '</td>' +
        '<td>' + Number(flow.max_payload_bytes || 0) + ' B</td>' +
        '<td>' + Number(flow.packets_over_1200 || 0) + ' / ' + Number(flow.packets_over_1400 || 0) + ' / ' + Number(flow.packets_over_1472 || 0) + '</td>' +
        '</tr>';
      el("udpMount").innerHTML = '<table><thead><tr><th>Flow</th><th>Packets</th><th>Bytes</th><th>Jitter</th><th>Max gap</th><th>Loss</th><th>Reorder / Dup</th><th>Max payload</th><th>&gt;1200 / &gt;1400 / &gt;1472</th></tr></thead><tbody>' + row("RX client to target", rx) + row("TX target to client", tx) + '</tbody></table>';
    }

    function updatePorts(info, stats) {
      const runtime = info.runtime || {};
      text("planePill", info.plane || "control");
      text("controlListen", info.control_listen || info.listen || "-");
      text("dataListen", info.data_listen || "-");
      text("wsPath", info.ws_path || "-");
      text("defaultTarget", info.default_target || "-");
      text("bufferSize", fmtBytes(runtime.buffer_size || info.buffer || 0));
      text("uptime", fmtDuration(stats.uptime_seconds || 0));
      text("derivedEndpoint", deriveEndpoint(info));
    }

    function deriveEndpoint(info) {
      const scheme = location.protocol === "https:" ? "wss://" : "ws://";
      const host = location.hostname || "127.0.0.1";
      const dataListen = info.data_listen || "";
      const port = extractPort(dataListen) || extractPort(location.host) || "8080";
      const path = info.ws_path || "/ws";
      return scheme + host + ":" + port + path;
    }

    function extractPort(value) {
      value = String(value || "");
      const match = value.match(/:(\d+)$/);
      return match ? match[1] : "";
    }

    function updateSecurity(info) {
      const security = info.security || {};
      const auth = info.auth || {};
      const authEnabled = Boolean(security.auth_enabled || auth.enabled);
      const header = security.auth_header || auth.header || "-";
      if (authEnabled && !state.authToken && header !== "-") {
        state.authHeader = header;
        el("authHeader").value = header;
      }
      text("authPill", authEnabled ? "auth enabled" : "auth disabled");
      text("authEnabled", authEnabled ? "enabled" : "disabled");
      text("authHeaderInfo", header || "-");
      text("tokenCount", String(security.tokens_count || auth.tokens_count || 0));
      text("pluginAPIState", security.plugin_api_public === false ? "private" : "public");
      text("trustProxy", fmtBool(security.trust_proxy_headers));
      text("allowedHosts", fmtList(security.allowed_hosts));
      text("allowedOrigins", fmtList(security.allowed_origins));
      text("maxConnections", fmtLimit(security.max_connections));
      text("maxPerIP", fmtLimit(security.max_connections_per_ip));
      text("rateLimit", fmtLimit(security.rate_limit_per_minute));
    }

    function updateHealth(stats, health, pingMS) {
      const udp = stats.udp || {};
      const rx = udp.rx || {};
      text("statusText", "Live");
      el("statusText").classList.remove("danger");
      text("pingValue", fmtMS(pingMS));
      text("totalConnections", Number(stats.total_connections || 0) + " total sessions");
      text("errorCount", String(Number(stats.errors || 0)));
      text("udpHealth", "loss " + fmtPercent(rx.loss_percent || 0) + ", jitter " + fmtMS(rx.jitter_ms || 0));
      text("updatedAt", "updated " + new Date().toLocaleTimeString());
      if (health && health.status && health.status !== "ok") {
        text("statusText", health.status);
      }
    }

    function renderAPIRoutes() {
      const routes = [
        ["Health", "/api/health"],
        ["Ping", "/api/ping"],
        ["Info", "/api/info"],
        ["Stats", "/api/stats"],
        ["Sessions", "/api/sessions"],
        ["Plugins", "/api/plugins"]
      ];
      el("apiMount").innerHTML = '<table><thead><tr><th>Name</th><th>Route</th><th>Action</th></tr></thead><tbody>' + routes.map((r) => {
        return '<tr><td>' + escapeHTML(r[0]) + '</td><td class="mono">' + escapeHTML(r[1]) + '</td><td><button class="small-btn" data-copy-value="' + escapeHTML(location.origin + r[1]) + '">Copy URL</button> <button class="small-btn" data-api-open="' + escapeHTML(r[1]) + '">Open</button></td></tr>';
      }).join("") + '</tbody></table>';
    }

    async function refresh() {
      if (state.paused) return;
      try {
        const pluginsPromise = fetchJSON("/api/plugins").catch((err) => ({ error: err.message, enabled: [] }));
        const results = await Promise.all([
          fetchJSON("/api/stats"),
          fetchJSON("/api/sessions"),
          pluginsPromise,
          fetchJSON("/api/info"),
          fetchJSON("/api/health").catch(() => null),
          measurePing()
        ]);
        const stats = results[0];
        const sessions = results[1];
        const plugins = results[2];
        const info = results[3];
        const health = results[4];
        const pingMS = results[5];
        state.lastInfo = info;
        showAuth(false);
        updateHealth(stats, health, pingMS);
        updateTraffic(stats);
        updatePorts(info, stats);
        updateSecurity(info);
        renderPluginSummary(stats, plugins);
        renderPluginCatalog(plugins);
        renderSessions(sessions);
        renderUDP(stats);
        renderAPIRoutes();
      } catch (err) {
        if (err.status === 401) {
          showAuth(true, "Control API requires a ROAD token. Enter it once; it stays in this browser session only.");
        }
        text("statusText", "Disconnected");
        el("statusText").classList.add("danger");
        text("updatedAt", err.message || String(err));
      }
    }

    function resetTimer() {
      if (state.interval) clearInterval(state.interval);
      const ms = Number(el("refreshEvery").value || 2000);
      state.interval = setInterval(refresh, ms);
    }

    document.querySelectorAll(".tab").forEach((button) => {
      button.addEventListener("click", () => {
        document.querySelectorAll(".tab").forEach((b) => b.classList.remove("active"));
        document.querySelectorAll(".tab-panel").forEach((p) => p.classList.remove("active"));
        button.classList.add("active");
        el("tab-" + button.dataset.tab).classList.add("active");
      });
    });

    document.addEventListener("click", async (event) => {
      const target = event.target;
      if (!(target instanceof HTMLElement)) return;
      const copyID = target.getAttribute("data-copy");
      const copyValue = target.getAttribute("data-copy-value");
      const apiOpen = target.getAttribute("data-api-open");
      if (copyID) {
        const value = el(copyID).textContent || "";
        await navigator.clipboard.writeText(value);
        target.textContent = "Copied";
        setTimeout(() => target.textContent = copyID === "derivedEndpoint" ? "Copy endpoint" : "Copy default plugin", 900);
      }
      if (copyValue) {
        await navigator.clipboard.writeText(copyValue);
        target.textContent = "Copied";
        setTimeout(() => target.textContent = "Copy URL", 900);
      }
      if (apiOpen) {
        try {
          const payload = await fetchText(apiOpen);
          const blob = new Blob([payload], { type: "application/json" });
          const url = URL.createObjectURL(blob);
          window.open(url, "_blank", "noopener");
          setTimeout(() => URL.revokeObjectURL(url), 30000);
        } catch (err) {
          if (err.status === 401) showAuth(true, "This API route requires a ROAD token.");
          text("updatedAt", err.message || String(err));
        }
      }
    });

    el("refreshNow").addEventListener("click", refresh);
    el("refreshEvery").addEventListener("change", resetTimer);
    el("pauseToggle").addEventListener("click", () => {
      state.paused = !state.paused;
      text("pauseToggle", state.paused ? "Resume" : "Pause");
      if (!state.paused) refresh();
    });
    el("saveToken").addEventListener("click", () => {
      state.authToken = el("authToken").value.trim();
      state.authHeader = el("authHeader").value.trim() || "X-ROAD-Token";
      sessionStorage.setItem("road.control.token", state.authToken);
      sessionStorage.setItem("road.control.header", state.authHeader);
      refresh();
    });
    el("clearToken").addEventListener("click", () => {
      state.authToken = "";
      sessionStorage.removeItem("road.control.token");
      showAuth(false);
      refresh();
    });

    el("authToken").value = state.authToken;
    el("authHeader").value = state.authHeader;
    renderAPIRoutes();
    refresh();
    resetTimer();
  </script>
</body>
</html>
`
