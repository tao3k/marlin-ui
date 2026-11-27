// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

export * from "./tokens";
export * from "./bridge";

import {
  subscribeArtifactLoad,
  subscribeMarlinTheme,
  subscribeMarlinVisibility,
  type MarlinArtifactEnvelope,
  type Unsubscribe,
} from "./bridge";
import {
  MARLIN_THEME_PRESETS,
  applyMarlinThemeTokens,
  type MarlinThemeID,
  type MarlinThemeTokens,
} from "./tokens";

export interface MarlinIslandRuntimeOptions {
  readonly defaultTheme?: MarlinThemeID;
  readonly onTheme?: (tokens: Partial<MarlinThemeTokens>) => void;
  readonly onVisibility?: (visible: boolean) => void;
  readonly onArtifact?: (artifact: MarlinArtifactEnvelope) => void;
}

/**
 * Bootstraps a Layer-3 Marlin Web Island (React / Vercel AI SDK / shadcn/ui /
 * Geist) with live Doom One & Tokyo Night Liquid Glass theme parity and
 * focus-lifecycle animation pausing.
 */
export function createMarlinIslandRuntime(
  options: MarlinIslandRuntimeOptions = {},
): Unsubscribe {
  const initial = MARLIN_THEME_PRESETS[options.defaultTheme ?? "doom-one"];
  applyMarlinThemeTokens(initial);

  const unsubs: Unsubscribe[] = [
    subscribeMarlinTheme((t) => options.onTheme?.(t)),
    subscribeMarlinVisibility((visible) => {
      document.documentElement.dataset.marlinVisible = visible ? "1" : "0";
      options.onVisibility?.(visible);
    }),
    subscribeArtifactLoad((art) => options.onArtifact?.(art)),
  ];

  return () => {
    for (const stop of unsubs) stop();
  };
}
