<!--
SPDX-FileCopyrightText: 2026 tao3k team and Contributors
SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later
-->

# `@marlin/ui-sdk` — TypeScript + Vercel AI SDK Web Island Runtime

`@marlin/ui-sdk` bridges Marlin's native 120Hz MyGo GPU shell (Desktop macOS 26 / iOS 26 / Web WASM) with the TypeScript, React, shadcn/ui, Geist, and Vercel AI SDK (`data-*` Generative UI) ecosystem inside Layer-3 Sandboxed Web Islands (`~/.config/marlin/ui/islands/<id>/index.html`).

## Unified Design Token Parity

Every theme switch (`Doom One`, `Tokyo Night`, `Nord Frost`, `Doom One Light`) and Liquid Glass opacity adjustment in Marlin Desktop or iOS automatically broadcasts a `marlin:theme` event and updates CSS custom properties (`--marlin-*`, `--background`, `--foreground`, `--card`, `--primary`, `--border`, `--marlin-glass-opacity`).

## Quick Start

```ts
import { createMarlinIslandRuntime } from "@marlin/ui-sdk";

const dispose = createMarlinIslandRuntime({
  defaultTheme: "doom-one",
  onArtifact(artifact) {
    // Render Vercel AI SDK data-* Generative UI parts:
    // data-metrics, data-chart, data-table, data-workflow, data-mermaid, data-json
    console.log(artifact.dataType, artifact.title, artifact.parsed);
  },
  onVisibility(visible) {
    // Pause heavy WebGL/SVG loops when the Emacs buffer tab is hidden.
  },
});
```
