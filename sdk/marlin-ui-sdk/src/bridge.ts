// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

import { applyMarlinThemeTokens, type MarlinThemeTokens } from "./tokens";

/**
 * Typed Vercel AI SDK v5 DataUIPart payloads streamed from Marlin Core
 * (Rust marlin-agent-core -> Go hostlink -> Layer-3 Sandboxed Web Island).
 */
export type MarlinDataPartType =
  | "data-metrics"
  | "data-chart"
  | "data-table"
  | "data-workflow"
  | "data-mermaid"
  | "data-json"
  | `data-${string}`;

export interface MarlinArtifactEnvelope<T = unknown> {
  readonly dataType: MarlinDataPartType;
  readonly title: string;
  readonly payload: string;
  readonly parsed?: T;
}

export interface MarlinMetricsItem {
  readonly label: string;
  readonly value: string | number;
  readonly delta?: string;
  readonly tone?: "accent" | "info" | "success" | "warning" | "danger";
}

export interface MarlinChartSeries {
  readonly label: string;
  readonly value: number;
}

export interface MarlinTablePayload {
  readonly columns: readonly string[];
  readonly rows: readonly (readonly (string | number | boolean | null)[])[];
}

export interface MarlinActionEvent<T = unknown> {
  readonly event: string;
  readonly payload: T;
}

export interface MarlinVisibilityEvent {
  readonly visible: boolean;
}

export type Unsubscribe = () => void;

export function subscribeMarlinTheme(
  onTheme: (tokens: Partial<MarlinThemeTokens>) => void,
): Unsubscribe {
  const handler = (ev: Event) => {
    const detail = (ev as CustomEvent<Partial<MarlinThemeTokens>>).detail;
    if (!detail) return;
    applyMarlinThemeTokens(detail);
    onTheme(detail);
  };
  window.addEventListener("marlin:theme", handler);
  return () => window.removeEventListener("marlin:theme", handler);
}

export function subscribeMarlinVisibility(
  onVisibility: (visible: boolean) => void,
): Unsubscribe {
  const handler = (ev: Event) => {
    const detail = (ev as CustomEvent<MarlinVisibilityEvent>).detail;
    if (detail && typeof detail.visible === "boolean") {
      onVisibility(detail.visible);
    }
  };
  window.addEventListener("marlin:visibility", handler);
  return () => window.removeEventListener("marlin:visibility", handler);
}

export function subscribeMarlinAction<T = unknown>(
  onAction: (action: MarlinActionEvent<T>) => void,
): Unsubscribe {
  const handler = (ev: Event) => {
    const detail = (ev as CustomEvent<MarlinActionEvent<T>>).detail;
    if (detail && typeof detail.event === "string") {
      onAction(detail);
    }
  };
  window.addEventListener("marlin:action", handler);
  return () => window.removeEventListener("marlin:action", handler);
}

export function subscribeArtifactLoad<T = unknown>(
  onArtifact: (artifact: MarlinArtifactEnvelope<T>) => void,
): Unsubscribe {
  return subscribeMarlinAction<MarlinArtifactEnvelope<T>>((action) => {
    if (action.event !== "artifact:load" || !action.payload) return;
    const env = action.payload;
    let parsed: T | undefined;
    if (typeof env.payload === "string" && env.payload.trim() !== "") {
      try {
        parsed = JSON.parse(env.payload) as T;
      } catch {
        parsed = undefined;
      }
    }
    onArtifact({ ...env, parsed });
  });
}
