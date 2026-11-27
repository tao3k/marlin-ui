// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

// Hierarchical OpenSysML v2 architecture for internal/emacs/*.
// Every Go package under internal/emacs/ maps 1:1 to a declared structural part
// here, aligned with GNU Emacs concepts (frame, window, buffer, minibuffer,
// mode-line, tab-bar) and marlin-agent-core's Scheme TUI modules (scheme/ui-*).
package MarlinEmacsWorkbench {
    private import MarlinUIArchitecture::*;

    doc /* Hierarchical decomposition of Marlin UI's Doom Emacs desktop workbench.
           Separates visual faces (theme), modal key state (meow), top tab strip
           (tabs), bottom status strip (modeline), interactive completion and
           echo area (minibuffer), major-mode buffer views (buffer), window
           viewport + Tier-0 gate container (window), and top-level frame
           composition + live marlin.ui.v1 gRPC synchronization (frame). */

    package actual {
        part <'MUI-EMACS-A01'> emacsFrame {
            doc /* internal/emacs/frame: top-level Emacs Frame owner. Composes the
                   strict 4-region vertical layout (Centaur-Tabs, Window,
                   Doom Modeline, Minibuffer & Echo Area), synchronizes live
                   marlin.ui.v1 UIWorkbenchSnapshot (theme, model label, session
                   label, commands), and exposes ui.View to cmd/marlin-desktop. */

            part <'MUI-EMACS-A02'> emacsTheme {
                doc /* internal/emacs/theme: embedded CSS theme packs
                       (nord-bento default, doom-one, doom-tokyo-night,
                       doom-one-light) compiled through marlin-mygo/runtime/style
                       against MarlinStyleContract(), cascaded with governed
                       user overrides (~/.config/marlin/ui/), live schema.UITheme
                       projection, Orderless match highlighting, and tactile primitives. */
            }

            part <'MUI-EMACS-A03'> emacsMeow {
                doc /* internal/emacs/meow: Meow modal editing state machine
                       (NORMAL default on Doom Dashboard, INSERT for typing) and
                       SPC Hydra leader prefix routing (SPC, SPC b, SPC t, SPC m),
                       plus universal Emacs/OS shortcuts. */
            }

            part <'MUI-EMACS-A04'> emacsTabs {
                doc /* internal/emacs/tabs: authentic sort-tab strip (30px height,
                       seamless dark bar, soft-rounded warm active tab pill
                       #504945/#fbf1c7, quiet muted inactive tabs separated by |). */
            }

            part <'MUI-EMACS-A05'> emacsWindow {
                doc /* internal/emacs/window: full-bleed primary Emacs Window
                       viewport occupying 90%+ of frame height, delegating to the
                       focused major-mode buffer or compiled feature renderer,
                       plus inline Tier-0 GATE approval strip when pending. */
            }

            part <'MUI-EMACS-A06'> emacsBuffer {
                doc /* internal/emacs/buffer: categorized Emacs buffer catalog
                       and major-mode renderers:
                       - registry.go: BufferID definitions & major-mode router
                       - conversation.go: *conversation* (classic Doom ASCII
                         Dashboard + clean monospace stream, zero chat bubbles)
                       - tabulated.go: tabulated-list-mode pages (*Sessions*,
                         *Targets*, *Processes*, *Models*, *Messages*, *Help*)
                       - customize.go: *Settings* Emacs customize buffer. */
            }

            part <'MUI-EMACS-A07'> emacsModeline {
                doc /* internal/emacs/modeline: complete Doom Modeline (28px,
                       borderless matte bar, 3px left state bar, circular Meow
                       modal badge, and 'dashboard / 'main / 'helm layout presets
                       mirroring doom-modeline.el). */
            }

            part <'MUI-EMACS-A08'> emacsMinibuffer {
                doc /* internal/emacs/minibuffer: single-line Emacs Minibuffer
                       and quiet Echo Area (32px) with expandable Vertico
                       completion (M-x merging live marlin.ui.v1 commands,
                       switch-buffer, switch-themes), 120Hz Helm candidate list,
                       and integrated multi-column Hydra popup host. */
            }

            part <'MUI-EMACS-A09'> emacsHydra {
                doc /* internal/emacs/hydra: column-grouped Doom Leader / Hydra
                       and Which-Key Minibuffer popup (mirroring scheme/ui-hydra),
                       supporting nested prefix hydras (+buffers, +themes,
                       +session) and repeatable (Exit=false) vs exit heads. */
            }

            part <'MUI-EMACS-A10'> emacsWidgets {
                doc /* Consumer binding to upstream marlin-mygo/runtime/{style,kit,widgets}
                       powering *Dashboard* (Tab 1) and the auto-hide floating left
                       EdgeRail with fluid edge-to-edge tiling, flat macOS NSVisualEffectView
                       frosted glass, native SF Symbols, GPU RingGauges & ThinkingOrb,
                       W3C Design Tokens (tokens.json with oklch), @marlin/ui-kit
                       TS/JSON declarative widgets & custom sidebars, and lazy-loaded
                       Hybrid GPU + Sandboxed WebView islands (ui.webview BPMN Studio). */
            }

            part <'MUI-EMACS-A11'> emacsWorkspace {
                doc /* internal/emacs/workspace: Doom Emacs-inspired perspective
                       workspace isolation (persp-mode / :ui workspaces) and
                       declarative popup window rule engine (set-popup-rule!),
                       mirrored 1:1 with scheme/ui-workspaces/. */
            }

            part <'MUI-EMACS-A12'> emacsModes {
                doc /* internal/emacs/modes: Emacs-authentic Major Mode inheritance,
                       orthogonal Minor Modes, Mode Hooks, display-buffer-alist
                       policies, and modern zero-indirect-buffer Span-driven Polymode
                       (Host Mode + Inner Modes) runtime mirrored from scheme/ui-modes/. */
            }
        }

        dependency frameUsesTheme from emacsFrame to emacsFrame::emacsTheme;
        dependency frameUsesMeow from emacsFrame to emacsFrame::emacsMeow;
        dependency frameUsesModes from emacsFrame to emacsFrame::emacsModes;
        dependency frameUsesWidgets from emacsFrame to emacsFrame::emacsWidgets;
        dependency frameUsesWorkspace from emacsFrame to emacsFrame::emacsWorkspace;
        dependency windowUsesWorkspace from emacsFrame::emacsWindow to emacsFrame::emacsWorkspace;
        dependency workspaceUsesBuffer from emacsFrame::emacsWorkspace to emacsFrame::emacsBuffer;
        dependency frameUsesHydra from emacsFrame to emacsFrame::emacsHydra;
        dependency frameUsesTabs from emacsFrame to emacsFrame::emacsTabs;
        dependency frameUsesWindow from emacsFrame to emacsFrame::emacsWindow;
        dependency frameUsesModeline from emacsFrame to emacsFrame::emacsModeline;
        dependency frameUsesMinibuffer from emacsFrame to emacsFrame::emacsMinibuffer;
        dependency windowUsesBuffer from emacsFrame::emacsWindow to emacsFrame::emacsBuffer;
        dependency bufferUsesWidgets from emacsFrame::emacsBuffer to emacsFrame::emacsWidgets;
        dependency widgetsUseTheme from emacsFrame::emacsWidgets to emacsFrame::emacsTheme;
        dependency tabsUsesTheme from emacsFrame::emacsTabs to emacsFrame::emacsTheme;
        dependency modelineUsesTheme from emacsFrame::emacsModeline to emacsFrame::emacsTheme;
        dependency minibufferUsesTheme from emacsFrame::emacsMinibuffer to emacsFrame::emacsTheme;
        dependency minibufferUsesHydra from emacsFrame::emacsMinibuffer to emacsFrame::emacsHydra;
        dependency hydraUsesTheme from emacsFrame::emacsHydra to emacsFrame::emacsTheme;
        dependency bufferUsesTheme from emacsFrame::emacsBuffer to emacsFrame::emacsTheme;
        dependency frameConsumesHostlink from emacsFrame to MarlinUIArchitecture::actual::hostlinkTransport;
        dependency frameConsumesEngine from emacsFrame to MarlinUIArchitecture::actual::marlinMyGoEngine;
    }

    package target {
        part <'MUI-EMACS-T01'> emacsMultiWindow {
            doc /* Target: binary split Emacs Window tree (C-x 2 / C-x 3 / C-x 0
                   / C-x o) projecting independent active/inactive Doom Modelines
                   per split pane while sharing one bottom Minibuffer & Echo Area. */
        }

        part <'MUI-EMACS-T02'> emacsOrgBufferMode {
            doc /* Target: native Org-mode buffer renderer with heading folding,
                   property drawers, and live Shared Org-as-Context citations
                   projected from marlin-agent-core. */
        }

        part <'MUI-EMACS-T03'> emacsGrpcKeymapSync {
            doc /* Target: dynamically project live Meow/Hydra key-trie bindings
                   and buffer list mutations streamed over marlin.ui.v1 gRPC
                   directly into *Help* and Keypad Which-Key hints. */
        }

        dependency multiWindowExtendsWindow from emacsMultiWindow to actual::emacsFrame::emacsWindow;
        dependency orgModeExtendsBuffer from emacsOrgBufferMode to actual::emacsFrame::emacsBuffer;
        dependency keymapSyncExtendsMeow from emacsGrpcKeymapSync to actual::emacsFrame::emacsMeow;
    }

    package requirements {
        requirement <'MUI-EMACS-R01'> strictEmacsHierarchy {
            doc /* Every Emacs subsystem (theme, meow, tabs, window, buffer,
                   modeline, minibuffer, frame) lives in its own focused package
                   under internal/emacs/ with acyclic dependencies and each Go
                   file <= 1,000 lines. */
        }

        requirement <'MUI-EMACS-R02'> doomEmacsVisualConvergence {
            doc /* Zero web-app sidebars, zero floating modal popups, zero harsh
                   wireframe grids. Legible 13.5-14px monospace typography with
                   soft-rounded tactile controls (Radius 4-6px), sort-tab top bar,
                   full doom-modeline presets, and SPC Hydra popup matching Doom Emacs. */
        }

        requirement <'MUI-EMACS-R03'> liveGrpcProjection {
            doc /* Admitted marlin.ui.v1 UIWorkbenchSnapshot (theme, model label,
                   session label) and admitted CommandEntry items project live
                   into the Doom Modeline, *Models*, and Vertico M-x completion. */
        }

        requirement <'MUI-EMACS-R04'> modalKeyAndMinibufferDeterminism {
            doc /* Meow INSERT/NORMAL/KEYPAD transitions and Minibuffer
                   Prompt/Vertico/Helm transitions are deterministic, testable
                   headlessly, and always cancellable via C-g or ESC. */
        }

        requirement <'MUI-EMACS-R05'> bufferOrientedSettings {
            doc /* Configuration, remote federation targets, OS Keychain Vault
                   auth/OIDC, and compiled features live inside the *Settings*
                   Emacs customize buffer rather than modal web overlays. */
        }

        requirement <'MUI-EMACS-R06'> governedStylesheetContract {
            doc /* User UI theming lives strictly under ~/.config/marlin/ui/
                   with a typed directory contract (manifest.toml with
                   schema_version = marlin.ui.theme.v1, named theme packs under
                   themes/<id>/{theme.toml, style.css}, and bounded override
                   theme.css). Every stylesheet is admitted fail-closed through
                   marlin-mygo/runtime/style against MarlinStyleContract()
                   allowlisted semantic classes and visual properties, supporting
                   live hot-reload via SPC t r / M-x reload-theme without
                   corrupting the active frame on syntax or contract errors. */
        }

        requirement <'MUI-EMACS-R07'> extensibleBentoWidgetDesktop {
            doc /* Tab 1 (*Dashboard*) renders an EWW-inspired Bento widget grid
                   bound to live Marlin signals (tasks, agents, sessions, gates,
                   host health, clock) while Tab 2 (*conversation*) preserves
                   the Emacs conversation buffer. Users author custom desktop
                   widgets in either TOML (widget.toml / dashboard.toml) or
                   S-expression (layout.sexp / dashboard.sexp) under
                   ~/.config/marlin/ui/, and may toggle borderless immersive
                   Zen Mode via SPC t z / M-x zen-mode. */
        }
    }

    allocation ownHierarchy allocate requirements::strictEmacsHierarchy to actual::emacsFrame;
    allocation ownVisuals allocate requirements::doomEmacsVisualConvergence to actual::emacsFrame.emacsTheme;
    allocation ownGrpcSync allocate requirements::liveGrpcProjection to actual::emacsFrame.emacsModeline;
    allocation ownGrpcCommands allocate requirements::liveGrpcProjection to actual::emacsFrame.emacsMinibuffer;
    allocation ownModalDeterminism allocate requirements::modalKeyAndMinibufferDeterminism to actual::emacsFrame.emacsMeow;
    allocation ownCustomizeBuffer allocate requirements::bufferOrientedSettings to actual::emacsFrame.emacsBuffer;
    allocation ownStylesheetContract allocate requirements::governedStylesheetContract to actual::emacsFrame.emacsTheme;
    allocation ownBentoWidgets allocate requirements::extensibleBentoWidgetDesktop to actual::emacsFrame.emacsWidgets;
}
