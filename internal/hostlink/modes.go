// SPDX-FileCopyrightText: 2026 tao3k team and Contributors
//
// SPDX-License-Identifier: Apache-2.0 AND LGPL-2.1-or-later

//go:build !js

package hostlink

import (
	"context"
	"fmt"
	"time"

	projectionv1 "github.com/tao3k/marlin-mygo/gen/marlin/projection/v1"
)

// ModeRegistryRecord is the wire-neutral projection of one Scheme `ui-modes`
// GraphRecordView (`ui.mode.major`, `ui.mode.minor`, `ui.mode.inner`,
// `ui.polymode`, `ui.buffer_rule`) streamed from `marlin-scheme-host`.
type ModeRegistryRecord struct {
	RecordID string
	Kind     string
	Title    string
	Facets   map[string]string
}

// FetchSchemeModeRecords queries Rust `ProjectionQueryService::QueryProjectGraph("modes:registry")`
// to hot-sync authoritative Major Modes, Minor Modes, Polymodes, and Buffer rules
// projected directly from Gerbil Scheme via native C-ABI FFI.
func (l *Link) FetchSchemeModeRecords(ctx context.Context) ([]ModeRegistryRecord, error) {
	if l == nil || l.Switcher == nil {
		return nil, fmt.Errorf("hostlink: no active gRPC switcher")
	}
	target := l.Switcher.CurrentTarget()
	if target == nil || target.Projection() == nil {
		return nil, fmt.Errorf("hostlink: no active projection target")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	reqCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	resp, err := target.Projection().QueryProjectGraph(reqCtx, &projectionv1.QueryProjectGraphRequest{
		ProjectId:        "default",
		Query:            "modes:registry",
		SecretVisibility: projectionv1.SecretVisibility_SECRET_VISIBILITY_UNSPECIFIED,
	})
	if err != nil {
		return nil, err
	}
	return DecodeModeRecords(resp.GetRecords()), nil
}

// DecodeModeRecords converts protobuf `GraphRecordView` slices into ModeRegistryRecords.
func DecodeModeRecords(records []*projectionv1.GraphRecordView) []ModeRegistryRecord {
	if len(records) == 0 {
		return nil
	}
	out := make([]ModeRegistryRecord, 0, len(records))
	for _, rec := range records {
		if rec == nil {
			continue
		}
		facets := make(map[string]string, len(rec.GetFacets()))
		for k, v := range rec.GetFacets() {
			facets[k] = v
		}
		out = append(out, ModeRegistryRecord{
			RecordID: rec.GetRecordId(),
			Kind:     rec.GetKind(),
			Title:    rec.GetTitle(),
			Facets:   facets,
		})
	}
	return out
}
