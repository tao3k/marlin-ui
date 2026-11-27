// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

/**
 * Canonical Marlin UI theme tokens mirroring internal/emacs/theme CSS packs
 * (Doom One, Tokyo Night, Nord Frost, Doom One Light) and projecting them onto
 * both --marlin-* and shadcn/ui / Vercel Geist CSS custom properties.
 */
export type MarlinThemeID =
  | "doom-one"
  | "doom-tokyo-night"
  | "nord-bento"
  | "doom-one-light";

export interface MarlinThemeTokens {
  readonly id: MarlinThemeID;
  readonly name: string;
  readonly isLight: boolean;
  readonly canvas: string;
  readonly bar: string;
  readonly surface: string;
  readonly hover: string;
  readonly border: string;
  readonly ink: string;
  readonly muted: string;
  readonly accent: string;
  readonly info: string;
  readonly success: string;
  readonly warning: string;
  readonly danger: string;
  readonly glassOpacity: number;
}

export const MARLIN_THEME_PRESETS: Record<MarlinThemeID, MarlinThemeTokens> = {
  "doom-one": {
    id: "doom-one",
    name: "Doom One",
    isLight: false,
    canvas: "#282c34",
    bar: "#21242b",
    surface: "#2d333f",
    hover: "#363d4c",
    border: "#424959",
    ink: "#d5dce6",
    muted: "#8b94a6",
    accent: "#51afef",
    info: "#46d9ff",
    success: "#98be65",
    warning: "#ecbe7b",
    danger: "#ff6c6b",
    glassOpacity: 0.75,
  },
  "doom-tokyo-night": {
    id: "doom-tokyo-night",
    name: "Tokyo Night",
    isLight: false,
    canvas: "#1a1b26",
    bar: "#16161e",
    surface: "#24283b",
    hover: "#2f3549",
    border: "#3b4261",
    ink: "#c0caf5",
    muted: "#828bb8",
    accent: "#7aa2f7",
    info: "#7dcfff",
    success: "#9ece6a",
    warning: "#e0af68",
    danger: "#f7768e",
    glassOpacity: 0.75,
  },
  "nord-bento": {
    id: "nord-bento",
    name: "Nord Frost",
    isLight: false,
    canvas: "#1e222a",
    bar: "#191d24",
    surface: "#2b303c",
    hover: "#353c4b",
    border: "#3d4556",
    ink: "#e5e9f0",
    muted: "#8c96a8",
    accent: "#88c0d0",
    info: "#81a1c1",
    success: "#a3be8c",
    warning: "#ebcb8b",
    danger: "#bf616a",
    glassOpacity: 0.75,
  },
  "doom-one-light": {
    id: "doom-one-light",
    name: "Doom One Light",
    isLight: true,
    canvas: "#fafafa",
    bar: "#f0f0f0",
    surface: "#e7e9ee",
    hover: "#dfe2e9",
    border: "#cfd4de",
    ink: "#383a42",
    muted: "#7a7f8c",
    accent: "#4078f2",
    info: "#0184bc",
    success: "#50a14f",
    warning: "#c18401",
    danger: "#e45649",
    glassOpacity: 0.82,
  },
};

/**
 * Projects MarlinThemeTokens into CSS custom properties on target (defaults to
 * document.documentElement) so shadcn/ui, Tailwind v4, Vercel Geist, and Apple
 * Liquid Glass Web Island components share 1:1 visual parity with native GPU UI.
 */
export function applyMarlinThemeTokens(
  tokens: Partial<MarlinThemeTokens>,
  target: HTMLElement = document.documentElement,
): void {
  const s = target.style;
  if (tokens.canvas) s.setProperty("--marlin-canvas", tokens.canvas);
  if (tokens.bar) s.setProperty("--marlin-bar", tokens.bar);
  if (tokens.surface) {
    s.setProperty("--marlin-surface", tokens.surface);
    s.setProperty("--card", tokens.surface);
  }
  if (tokens.hover) s.setProperty("--marlin-hover", tokens.hover);
  if (tokens.border) {
    s.setProperty("--marlin-border", tokens.border);
    s.setProperty("--border", tokens.border);
  }
  if (tokens.ink) {
    s.setProperty("--marlin-ink", tokens.ink);
    s.setProperty("--foreground", tokens.ink);
  }
  if (tokens.muted) {
    s.setProperty("--marlin-muted", tokens.muted);
    s.setProperty("--muted-foreground", tokens.muted);
  }
  if (tokens.accent) {
    s.setProperty("--marlin-accent", tokens.accent);
    s.setProperty("--primary", tokens.accent);
    s.setProperty("--ring", tokens.accent);
  }
  if (tokens.info) s.setProperty("--marlin-info", tokens.info);
  if (tokens.success) s.setProperty("--marlin-success", tokens.success);
  if (tokens.warning) s.setProperty("--marlin-warning", tokens.warning);
  if (tokens.danger) {
    s.setProperty("--marlin-danger", tokens.danger);
    s.setProperty("--destructive", tokens.danger);
  }
  if (typeof tokens.glassOpacity === "number" && tokens.glassOpacity > 0) {
    s.setProperty("--marlin-glass-opacity", String(tokens.glassOpacity));
  }
  if (typeof tokens.isLight === "boolean") {
    target.dataset.marlinColorScheme = tokens.isLight ? "light" : "dark";
    target.classList.toggle("dark", !tokens.isLight);
  }
}
