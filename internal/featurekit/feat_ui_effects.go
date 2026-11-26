// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

package featurekit

import (
	"github.com/tao3k/marlin-mygo/engine/features"
	"github.com/tao3k/marlin-mygo/engine/input"
)

// Governed hot-pluggable UI surface modules registered directly into
// marlin-mygo/engine/features.Registry (lazy-loaded & audited via GovernanceLedger).
const (
	FeatureUIGlass  features.FeatureID = "ui.glass"
	FeatureUIMotion features.FeatureID = "ui.motion"
)

func init() {
	RegisterCompiled(&uiEffectModule{
		meta: features.FeatureMeta{
			ID:             FeatureUIGlass,
			Name:           "Liquid Glass Surface",
			Description:    "Optional GPU frosted glass material (mygo/plugins/glass) + Oklab aurora backdrop",
			Category:       "surface",
			TrustTier:      features.Tier0HostNative,
			DefaultEnabled: true,
		},
		cmdName: "toggle-glass",
		cmdDesc: "Hot-plug Liquid Glass GPU surface module (SPC t v)",
	})
	RegisterCompiled(&uiEffectModule{
		meta: features.FeatureMeta{
			ID:             FeatureUIMotion,
			Name:           "120Hz Spring Motion",
			Description:    "Optional spring gauge interpolation, hover lift & 1Hz live clock tick",
			Category:       "surface",
			TrustTier:      features.Tier0HostNative,
			DefaultEnabled: true,
		},
		cmdName: "toggle-motion",
		cmdDesc: "Hot-plug 120Hz Spring Motion module (SPC t a)",
	})
}

type uiEffectModule struct {
	meta    features.FeatureMeta
	cmdName string
	cmdDesc string
}

func (m *uiEffectModule) Meta() features.FeatureMeta { return m.meta }

func (m *uiEffectModule) Register(host *features.Host) error {
	if host != nil && m.cmdName != "" {
		host.AddCommand(input.CommandEntry{
			Name:        m.cmdName,
			Description: m.cmdDesc,
		})
	}
	return nil
}
