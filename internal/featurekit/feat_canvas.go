// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build marlin_feat_canvas || marlin_full

package featurekit

import "github.com/tao3k/marlin-mygo/features/canvas"

func init() {
	RegisterCompiled(canvas.NewModule(nil))
}
