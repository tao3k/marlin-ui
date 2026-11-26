// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package frame_test

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/tao3k/marlin-mygo/engine/search"
	"github.com/tao3k/marlin-mygo/runtime/mygoshell"

	"github.com/tao3k/marlin-ui/internal/appconfig"
	"github.com/tao3k/marlin-ui/internal/emacs/buffer"
	"github.com/tao3k/marlin-ui/internal/emacs/frame"
)

func TestConversationCollapsibleThinkingFoldedByDefaultAndMarkdownRender(t *testing.T) {
	adaptive := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle: "Marlin",
		FormFactor: mygoshell.FormFactorDesktop,
	})
	tr := adaptive.Desktop.Transcript
	tr.AppendUser("Explain the 3-layer UI architecture")
	tr.AppendThinking("Inspect architecture", "Check Layer 1 Rust gRPC and Layer 3 WebIsland.", true)
	tr.UpsertToolCall(mygoshell.ToolCallCardView{
		CallID:    "call-1",
		ToolName:  "workspace_search",
		Phase:     "COMPLETED",
		Title:     "workspace_search",
		Summary:   "Matched 4 files in crates/",
		ReceiptID: "rcpt-42",
	}, true)
	tr.BeginAssistantTurn(1)
	tr.ApplyCheckpoint(1, "ev-1", "<think>First inspect Layer 1 Rust gRPC and Layer 2 MyGo GPU.</think>\n## Three Layers\n\nSee @cite:crates/ui_grpc.rs:L1896.\n\n```go\nfmt.Println(\"hello\")\n```\n\n```mermaid\ngraph LR\n  Rust-->MyGo\n```")
	tr.CompleteTurn(1, "COMPLETED")

	modelCfg, err := appconfig.ParseModelConfigTOML(`
provider = "deepseek"
model = "deepseek-flash"
protocol = "anthropic"
credential_env = "ANTHROPIC_API_KEY"
[anthropic]
thinking = { type = "adaptive" }
effort = "low"
[models.deepseek-v4-pro]
provider = "deepseek"
model = "deepseek-v4-pro"
protocol = "anthropic"
`)
	if err != nil {
		t.Fatalf("parse model config: %v", err)
	}
	modelCfg.Loaded = true
	modelCfg.SourcePath = "/tmp/example.toml"
	modelCfg.SourceOrigin = "flag"

	f := frame.New(frame.Config{
		Adaptive: adaptive,
		AppConfig: appconfig.Config{
			SessionID: "sess-conv-test",
			ModelCfg:  modelCfg,
		},
	})
	defer f.Close()
	f.SelectBuffer(buffer.Conversation)

	tt := ui.NewTester(f.View, 1280, 820)
	tt.Frame()

	// Thinking block must be rendered as a collapsible header and FOLDED by default.
	if !tt.HasText("Thought Process") {
		t.Fatal("expected collapsible Thought Process drawer header in *conversation*")
	}
	if tt.HasText("First inspect Layer 1 Rust gRPC") {
		t.Fatal("expected thinking trace body to be FOLDED by default")
	}
	// Header bar must show loaded TOML model route.
	if !tt.HasText("deepseek/deepseek-flash") {
		t.Fatal("expected loaded TOML route deepseek/deepseek-flash in conversation header")
	}
	// Contiguous Thinking + Tool steps collapse into a single Execution Trace stepper pill.
	if !tt.HasText("Execution Trace") || !tt.HasText("1 receipt(s)") {
		t.Fatal("expected grouped Execution Trace stepper pill with receipt summary")
	}
	// Luxury editorial CodeBlock header + inline @cite source pill strip must appear.
	if !tt.HasText("GO") || !tt.HasText("[1] crates/ui_grpc.rs:L1896") {
		t.Fatal("expected editorial CodeBlock language header and inline source citation pill")
	}
	// Vercel AI SDK Generative UI PartData card and Composer action strip must appear.
	if !tt.HasText("AI SDK · data-mermaid") || !tt.HasText("Deep Think") {
		t.Fatal("expected Generative UI data-mermaid card and Deep Think composer strip")
	}

	// Clicking Preview Stage opens the right-side Generative Artifact Split Stage.
	if err := tt.Click("Preview Stage"); err != nil {
		t.Fatalf("click Preview Stage: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Generative UI Artifact Preview") {
		t.Fatal("expected right-side Generative Artifact Split Stage to open")
	}
	if err := tt.Click("Close"); err != nil {
		t.Fatalf("click Close artifact stage: %v", err)
	}
	tt.Frame()
	if tt.HasText("Generative UI Artifact Preview") {
		t.Fatal("expected Close button to dismiss right-side Artifact Split Stage")
	}

	// Clicking the Thought Process header expands the reasoning body.
	if err := tt.Click("▸ Thought Process"); err != nil {
		t.Fatalf("click Thought Process header: %v", err)
	}
	tt.Frame()
	if !tt.HasText("First inspect Layer 1 Rust gRPC") {
		t.Fatal("expected clicking Thought Process header to expand reasoning trace")
	}

	// Switch to *Models* -> governed by Doom right-side Popup Rule without disturbing *conversation*!
	f.SelectBuffer(buffer.Models)
	tt.Frame()
	if !tt.HasText("POPUP · *Models*") || !tt.HasText("deepseek-v4-pro") || !tt.HasText("deepseek-flash") {
		t.Fatal("expected *Models* to render inside non-destructive Doom Popup Drawer with TOML routes")
	}
	if !tt.HasText("Thought Process") {
		t.Fatal("expected main *conversation* buffer to remain visible beside *Models* popup drawer")
	}
	// Dismiss popup cleanly and verify perspective workspace isolation.
	if err := tt.Click("Dismiss"); err != nil {
		t.Fatalf("click Dismiss popup: %v", err)
	}
	tt.Frame()
	if tt.HasText("POPUP · *Models*") {
		t.Fatal("expected Dismiss to close *Models* popup drawer cleanly")
	}

	// Create isolated second workspace "review" -> tabs & buffers stay isolated from "main".
	f.CreateWorkspace("review", "/repo/marlin-ui")
	f.SelectBuffer(buffer.Packages)
	tt.Frame()
	if !tt.HasText("2:review") || !tt.HasText("[review]") {
		t.Fatal("expected active workspace pill 2:review in tab bar and [review] in modeline")
	}
	f.SwitchWorkspace("main")
	tt.Frame()
	if !tt.HasText("[main]") {
		t.Fatal("expected switching back to main workspace to restore [main] perspective")
	}
}

func TestHelmRustParityPresetsAndConsultChipBar(t *testing.T) {
	adaptive := mygoshell.NewAdaptiveShell(mygoshell.AdaptiveShellConfig{
		BrandTitle: "Marlin",
		FormFactor: mygoshell.FormFactorDesktop,
	})
	f := frame.New(frame.Config{
		Adaptive:  adaptive,
		AppConfig: appconfig.Config{SessionID: "sess-helm-test"},
	})
	defer f.Close()

	f.OpenHelm()
	gen := adaptive.Desktop.HelmSearch.SetQuery("@lang:rust @sym:HelmEngine #graph !test")
	adaptive.Desktop.HelmSearch.Engine.IngestBatch(gen, search.SourceMeta{
		ID:       "rust-graph",
		Label:    "Workspace Search",
		Priority: 2,
	}, []search.Candidate{
		{
			ID:            "rust:rec-1",
			SourceID:      "rust-graph",
			Title:         "HelmEngine graph query",
			Subtitle:      "symbol · #a1b2c3d4",
			Location:      "crates/marlin-agent-search-engine/src/lib.rs:L42",
			Snippet:       "Converged Rust workspace search engine",
			Score:         95,
			PayloadKind:   "file",
			PayloadTarget: "crates/marlin-agent-search-engine/src/lib.rs:L42",
		},
	})
	if !adaptive.Desktop.HelmSearch.Engine.PreviewOpen() {
		adaptive.Desktop.HelmSearch.Engine.TogglePreview()
	}

	tt := ui.NewTester(f.View, 1280, 820)
	tt.Frame()

	for _, want := range []string{"All", "Search", "DSL", "Buffers", "Commands", "Attach @cite", "Dispatch Agent", "HelmEngine graph query"} {
		if !tt.HasText(want) {
			t.Fatalf("expected Helm Rust-parity UI to contain %q", want)
		}
	}
}
