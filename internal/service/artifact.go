// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// PutArtifact attaches structured worker output to a task.
//
// Lease-bound, where AddComment and UpdateTask deliberately are not: an
// artifact is the holder's output, and one written by a worker that no longer
// holds the task displaces the current holder's result for every later reader.
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
		if err := requireLeaseToken(ctx, m.tx, task, in.LeaseToken, m.now); err != nil {
			return err
		}
		limits, err := l.artifactLimits(ctx, m.tx)
		if err != nil {
			return err
		}
		if err := limits.Check(in.ContentType, in.Blob); err != nil {
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

// ArtifactLimits reports the bounds a new artifact blob is held to here.
func (l *Local) ArtifactLimits(ctx context.Context) (*core.ArtifactLimits, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out core.ArtifactLimits
	err = l.read(ctx, actor, func(tx store.Tx) error {
		out, err = l.artifactLimits(ctx, tx)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// artifactLimits are the server's limits narrowed by the current tenant's.
func (l *Local) artifactLimits(ctx context.Context, tx store.Tx) (core.ArtifactLimits, error) {
	t, err := tx.GetTenant(ctx)
	if err != nil {
		return core.ArtifactLimits{}, err
	}
	return core.EffectiveArtifactLimits(l.artifactMaxBytes, l.artifactTypes, *t), nil
}

// tightenArtifactLimits applies a tenant's artifact settings, refusing any
// that would loosen the server's. Existing artifacts are never re-judged.
func (l *Local) tightenArtifactLimits(t *core.Tenant, in core.UpdateTenantInput) error {
	if in.ArtifactMaxBytes != nil {
		if n := *in.ArtifactMaxBytes; n > l.artifactMaxBytes {
			return core.Invalid("artifact size limit %s is above this server's limit of %s; "+
				"a tenant can only lower it", core.ByteSize(n), core.ByteSize(l.artifactMaxBytes)).
				WithDetail(core.DetailField, "artifact_max_bytes")
		}
		t.ArtifactMaxBytes = *in.ArtifactMaxBytes
	}
	if in.ArtifactTypes != nil {
		types, err := core.ParseArtifactTypes(*in.ArtifactTypes)
		if err != nil {
			return err
		}
		for _, p := range types {
			if !core.ArtifactTypeCovered(l.artifactTypes, p) {
				return core.Invalid("artifact type %q is not accepted by this server, which accepts: %s; "+
					"a tenant can only narrow the list", p, strings.Join(l.artifactTypes, ", ")).
					WithDetail(core.DetailField, "artifact_types")
			}
		}
		t.ArtifactTypes = types
	}
	return nil
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
