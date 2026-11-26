// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package theme_test

import (
	"testing"

	"github.com/tao3k/marlin-mygo/pkg/schema"
	"github.com/tao3k/marlin-ui/internal/emacs/theme"
)

func TestDoomEmacsThemeCatalogAndGRPCProjection(t *testing.T) {
	cat := theme.Catalog()
	if len(cat) != 3 {
		t.Fatalf("expected 3 Doom Emacs themes, got %d", len(cat))
	}
	if def := theme.ByID("unknown"); def.ID != theme.DefaultThemeID {
		t.Fatalf("expected fallback to %q, got %q", theme.DefaultThemeID, def.ID)
	}
	if next := theme.Next("doom-one"); next.ID != "pi" {
		t.Fatalf("expected Next(doom-one)=pi, got %q", next.ID)
	}
	if next := theme.Next("pi"); next.ID != "doom-paper" || !next.IsLight {
		t.Fatalf("expected Next(pi)=doom-paper (light), got %+v", next)
	}
	proj := theme.FromUITheme("grpc-doom", "grpc-doom", schema.DefaultUITheme())
	if proj.ID != "grpc-doom" || proj.IsLight {
		t.Fatalf("unexpected gRPC theme projection: %+v", proj)
	}
}
