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
	tr.BeginAssistantTurn(1)
	tr.ApplyCheckpoint(1, "ev-1", "<think>First inspect Layer 1 Rust gRPC and Layer 2 MyGo GPU.</think>\n## Three Layers\n\n- **Layer 1**: Rust gRPC\n- **Layer 2**: Pure-Go MyGo")
	tr.CompleteTurn(1, "COMPLETED")
	tr.UpsertToolCall(mygoshell.ToolCallCardView{
		CallID:    "call-1",
		ToolName:  "workspace_search",
		Phase:     "COMPLETED",
		Title:     "workspace_search",
		Summary:   "Matched 4 files in crates/",
		ReceiptID: "rcpt-42",
	}, true)

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
	// Tool call card must appear with receipt badge.
	if !tt.HasText("workspace_search") || !tt.HasText("receipt:rcpt-42") {
		t.Fatal("expected structured tool call card with receipt badge")
	}

	// Clicking the Thought Process header expands the reasoning body.
	if err := tt.Click("▸ Thought Process"); err != nil {
		t.Fatalf("click Thought Process header: %v", err)
	}
	tt.Frame()
	if !tt.HasText("First inspect Layer 1 Rust gRPC") {
		t.Fatal("expected clicking Thought Process header to expand reasoning trace")
	}

	// Switch to *Models* buffer and verify all TOML routes render cleanly.
	f.SelectBuffer(buffer.Models)
	tt.Frame()
	if !tt.HasText("deepseek-v4-pro") || !tt.HasText("deepseek-flash") {
		t.Fatal("expected *Models* buffer to list both default and named TOML routes")
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
