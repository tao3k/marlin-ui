// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js && !marlin_minimal

package featurekit

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/tao3k/marlin-mygo/runtime/kit"
)

func hexRGB(c ui.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// appleGeistArtifactStageHTML builds Marlin's zero-CDN, strict-CSP, Apple
// Liquid Glass × Vercel Geist Layer-3 Artifact Stage for rendering Vercel AI
// SDK `data-*` Generative UI parts (data-metrics, data-chart, data-table,
// data-workflow, data-mermaid, data-json) with live Doom One / Tokyo Night
// token parity via the @marlin/ui-sdk marlin:theme / marlin:action bridge.
func appleGeistArtifactStageHTML(pal kit.Palette) string {
	vars := fmt.Sprintf(`:root {
  --marlin-canvas: %s;
  --marlin-surface: %s;
  --marlin-bar: %s;
  --marlin-ink: %s;
  --marlin-muted: %s;
  --marlin-accent: %s;
  --marlin-info: %s;
  --marlin-success: %s;
  --marlin-warning: %s;
  --marlin-danger: %s;
  --marlin-glass-opacity: 0.72;
}`, hexRGB(pal.Canvas), hexRGB(pal.Surface), hexRGB(pal.Bar), hexRGB(pal.Ink), hexRGB(pal.Muted),
		hexRGB(pal.Accent), hexRGB(pal.Info), hexRGB(pal.Success), hexRGB(pal.Warning), hexRGB(pal.Danger))

	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8" />
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline';" />
<meta name="viewport" content="width=device-width,initial-scale=1" />
<title>Marlin Generative Artifact Stage</title>
<style>
`)
	b.WriteString(vars)
	b.WriteString(`
:root {
  --background: transparent;
  --foreground: var(--marlin-ink);
  --card: color-mix(in srgb, var(--marlin-surface) 62%, transparent);
  --primary: var(--marlin-accent);
  --muted-foreground: var(--marlin-muted);
  --border: color-mix(in srgb, var(--marlin-ink) 13%, transparent);
}
* { box-sizing: border-box; }
html, body {
  margin: 0; padding: 0; width: 100%; height: 100%;
  background: transparent; color: var(--foreground);
  font: 13px/1.52 -apple-system, BlinkMacSystemFont, "SF Pro Text", "Geist", "Inter", system-ui, sans-serif;
  font-variant-numeric: tabular-nums;
  -webkit-font-smoothing: antialiased;
  overflow: hidden;
}
.stage { display: flex; flex-direction: column; width: 100%; height: 100%; padding: 14px; gap: 12px; }
.topbar {
  display: flex; align-items: center; justify-content: space-between; gap: 10px;
  padding: 9px 14px; border-radius: 14px;
  background: color-mix(in srgb, var(--marlin-bar) 56%, transparent);
  backdrop-filter: blur(18px) saturate(140%);
  border: 1px solid var(--border);
}
.topbar-left { display: flex; align-items: center; gap: 8px; min-width: 0; }
.pill {
  font: 600 11px/1 "SF Mono", ui-monospace, Menlo, monospace;
  padding: 4px 9px; border-radius: 999px; white-space: nowrap;
  color: var(--primary);
  background: color-mix(in srgb, var(--primary) 16%, transparent);
  border: 1px solid color-mix(in srgb, var(--primary) 34%, transparent);
}
.title { font-weight: 600; font-size: 13px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.copy-btn {
  appearance: none; cursor: pointer; font: 500 11.5px/1 inherit;
  color: var(--foreground); padding: 6px 11px; border-radius: 999px;
  background: color-mix(in srgb, var(--marlin-surface) 70%, transparent);
  border: 1px solid var(--border); transition: transform .14s ease, border-color .14s ease;
}
.copy-btn:hover { border-color: color-mix(in srgb, var(--primary) 55%, transparent); }
.copy-btn:active { transform: scale(0.96); }
.body {
  flex: 1; min-height: 0; overflow: auto; padding: 16px; border-radius: 18px;
  background: var(--card); backdrop-filter: blur(22px) saturate(135%);
  border: 1px solid var(--border);
}
.kpi-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(136px, 1fr)); gap: 10px; }
.kpi {
  padding: 13px 14px; border-radius: 14px;
  background: color-mix(in srgb, var(--marlin-bar) 54%, transparent);
  border: 1px solid var(--border);
}
.kpi-lbl { font-size: 11px; color: var(--muted-foreground); letter-spacing: .02em; }
.kpi-val { font-size: 22px; font-weight: 700; margin-top: 4px; color: var(--foreground); }
.kpi-delta { font: 600 11px/1 "SF Mono", ui-monospace, monospace; margin-top: 6px; color: var(--marlin-success); }
.chart-wrap { display: flex; flex-direction: column; gap: 10px; }
.bar-row { display: grid; grid-template-columns: 110px 1fr 56px; align-items: center; gap: 10px; }
.bar-lbl { font-size: 12px; color: var(--muted-foreground); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.bar-track { height: 10px; border-radius: 999px; background: color-mix(in srgb, var(--marlin-bar) 75%, transparent); overflow: hidden; }
.bar-fill { height: 100%; border-radius: 999px; background: linear-gradient(90deg, var(--marlin-accent), var(--marlin-info)); }
.bar-num { font: 600 11.5px/1 "SF Mono", ui-monospace, monospace; text-align: right; color: var(--foreground); }
table { width: 100%; border-collapse: collapse; font-size: 12.5px; }
th, td { text-align: left; padding: 8px 10px; border-bottom: 1px solid var(--border); }
th { font-size: 11px; text-transform: uppercase; letter-spacing: .04em; color: var(--muted-foreground); font-weight: 600; }
pre {
  margin: 0; font: 12px/1.55 "SF Mono", ui-monospace, Menlo, monospace;
  color: var(--foreground); white-space: pre-wrap; word-break: break-word;
}
.empty { color: var(--muted-foreground); display: flex; align-items: center; justify-content: center; height: 100%; }
</style>
</head>
<body>
<div class="stage">
  <div class="topbar">
    <div class="topbar-left">
      <span class="pill" id="badge">AI SDK · ready</span>
      <span class="title" id="title">Generative Artifact Stage</span>
    </div>
    <button class="copy-btn" id="copyBtn" type="button">Copy JSON</button>
  </div>
  <div class="body" id="body"><div class="empty">Waiting for Vercel AI SDK data-* Generative UI payload…</div></div>
</div>
<script>
(() => {
  const badge = document.getElementById("badge");
  const titleEl = document.getElementById("title");
  const bodyEl = document.getElementById("body");
  const copyBtn = document.getElementById("copyBtn");
  let lastRaw = "";

  function esc(s) {
    return String(s).replace(/[&<>"]/g, (ch) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[ch]));
  }

  function renderMetrics(items) {
    bodyEl.innerHTML = '<div class="kpi-grid">' + items.map((it) => {
      const tone = it.tone ? 'var(--marlin-' + esc(it.tone) + ')' : 'var(--marlin-accent)';
      const delta = it.delta ? '<div class="kpi-delta" style="color:' + tone + '">' + esc(it.delta) + '</div>' : '';
      return '<div class="kpi"><div class="kpi-lbl">' + esc(it.label ?? '') + '</div><div class="kpi-val">' + esc(it.value ?? '') + '</div>' + delta + '</div>';
    }).join('') + '</div>';
  }

  function renderChart(series) {
    const maxV = Math.max(1, ...series.map((d) => Number(d.value) || 0));
    bodyEl.innerHTML = '<div class="chart-wrap">' + series.map((d) => {
      const v = Number(d.value) || 0;
      const pct = Math.max(2, Math.round((v / maxV) * 100));
      return '<div class="bar-row"><div class="bar-lbl">' + esc(d.label ?? '') + '</div><div class="bar-track"><div class="bar-fill" style="width:' + pct + '%"></div></div><div class="bar-num">' + esc(v) + '</div></div>';
    }).join('') + '</div>';
  }

  function renderTable(tbl) {
    const cols = Array.isArray(tbl.columns) ? tbl.columns : [];
    const rows = Array.isArray(tbl.rows) ? tbl.rows : [];
    const head = '<thead><tr>' + cols.map((c) => '<th>' + esc(c) + '</th>').join('') + '</tr></thead>';
    const body = '<tbody>' + rows.map((r) => '<tr>' + r.map((v) => '<td>' + esc(v ?? '') + '</td>').join('') + '</tr>').join('') + '</tbody>';
    bodyEl.innerHTML = '<table>' + head + body + '</table>';
  }

  function loadArtifact(env) {
    const dt = String(env.dataType || "data-artifact");
    badge.textContent = "AI SDK · " + dt;
    titleEl.textContent = String(env.title || dt);
    lastRaw = String(env.payload || "");
    try {
      const parsed = JSON.parse(lastRaw);
      if (dt === "data-metrics" && Array.isArray(parsed)) { renderMetrics(parsed); return; }
      if (dt === "data-chart" && Array.isArray(parsed)) { renderChart(parsed); return; }
      if (dt === "data-table" && parsed && Array.isArray(parsed.columns)) { renderTable(parsed); return; }
      bodyEl.innerHTML = '<pre>' + esc(JSON.stringify(parsed, null, 2)) + '</pre>';
    } catch {
      bodyEl.innerHTML = '<pre>' + esc(lastRaw) + '</pre>';
    }
  }

  copyBtn.addEventListener("click", () => {
    if (!lastRaw || !navigator.clipboard) return;
    navigator.clipboard.writeText(lastRaw).then(() => {
      copyBtn.textContent = "Copied";
      setTimeout(() => { copyBtn.textContent = "Copy JSON"; }, 1200);
    }).catch(() => {});
  });

  window.addEventListener("marlin:action", (ev) => {
    const d = ev.detail;
    if (d && d.event === "artifact:load" && d.payload) loadArtifact(d.payload);
  });
})();
</script>
</body>
</html>`)
	return b.String()
}
