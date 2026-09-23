// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// PutArtifact attaches structured worker output to a task.
func (l *Local) PutArtifact(ctx context.Context, ref core.TaskRef, in core.ArtifactInput) (*core.Artifact, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	if _, err := json.Marshal(in.Payload); err != nil {
		return nil, core.Invalid("artifact payload must be valid JSON")
	}
	actor, err := l.authorize(ctx, authz.ActionArtifactWrite, authz.Resource{})
	if err != nil {
		return nil, err
	}

	var out *core.Artifact
	err = l.write(ctx, actor, func(m *mutation) error {
		task, err := liveTask(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionArtifactWrite, task); err != nil {
			return err
		}
		a := &core.Artifact{
			TaskID:      task.ID,
			ActorID:     actor.ID,
			Kind:        in.Kind,
			Name:        in.Name,
			Payload:     in.Payload,
			ContentType: in.ContentType,
			Blob:        in.Blob,
		}
		if err := m.tx.PutArtifact(ctx, a); err != nil {
			return err
		}
		out = a
		return m.Record("artifact.put", core.EventArtifactAdded, "artifact", a.ID, task.ProjectID,
			nil, artifactSummary(a), map[string]any{"ref": task.Ref, "kind": string(a.Kind), "name": a.Name})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListArtifacts returns a task's artifacts oldest first.
func (l *Local) ListArtifacts(ctx context.Context, ref core.TaskRef) ([]core.Artifact, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out []core.Artifact
	err = l.read(ctx, actor, func(tx store.Tx) error {
		task, err := liveTask(ctx, tx, ref)
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionTaskRead, task); err != nil {
			return err
		}
		out, err = tx.ListArtifacts(ctx, task.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// artifactSummary describes an artifact for the audit log without its blob,
// which may be a megabyte of opaque bytes.
func artifactSummary(a *core.Artifact) map[string]any {
	return map[string]any{
		"id":           a.ID,
		"task_id":      a.TaskID,
		"kind":         string(a.Kind),
		"name":         a.Name,
		"payload":      a.Payload,
		"content_type": a.ContentType,
		"blob_bytes":   len(a.Blob),
	}
}
