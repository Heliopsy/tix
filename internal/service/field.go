package service

import (
	"context"
	"strings"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// PutFieldDef defines or redefines a custom field on a project.
func (l *Local) PutFieldDef(ctx context.Context, projectRef string, in core.FieldDefInput) (*core.FieldDef, error) {
	actor, err := l.authorize(ctx, authz.ActionFieldWrite, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	key := strings.TrimSpace(in.Key)
	label := in.Label
	if label == "" {
		label = key
	}

	var out *core.FieldDef
	err = l.write(ctx, actor, func(m *mutation) error {
		p, err := lookupProject(ctx, m.tx, projectRef)
		if err != nil {
			return err
		}
		if _, err := l.authorize(ctx, authz.ActionFieldWrite, authz.Resource{ProjectID: p.ID}); err != nil {
			return err
		}
		def := &core.FieldDef{
			ProjectID:   p.ID,
			Key:         key,
			Label:       label,
			Type:        in.Type,
			Required:    in.Required,
			EnumOptions: in.EnumOptions,
			Default:     in.Default,
			Indexed:     in.Indexed,
			Position:    in.Position,
		}
		if in.Default != nil {
			value, err := coerceFieldValue(*def, in.Default)
			if err != nil {
				return err
			}
			def.Default = value
		}

		before, err := findFieldDef(ctx, m.tx, p.ID, key)
		if err != nil {
			return err
		}
		if before != nil {
			def.ID = before.ID
		}
		if err := m.tx.PutFieldDef(ctx, def); err != nil {
			return err
		}
		out = def
		return m.Record(auditFieldPut, core.EventFieldUpdated, "field_def", def.ID, p.ID, before, def,
			map[string]any{"key": def.Key, "project_id": p.ID})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListFieldDefs returns a project's custom field definitions in display order.
func (l *Local) ListFieldDefs(ctx context.Context, projectRef string) ([]core.FieldDef, error) {
	actor, err := l.authorize(ctx, authz.ActionProjectRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	out := []core.FieldDef{}
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		p, err := lookupProject(ctx, tx, projectRef)
		if err != nil {
			return err
		}
		if _, err := l.authorize(ctx, authz.ActionProjectRead, authz.Resource{ProjectID: p.ID}); err != nil {
			return err
		}
		found, err := tx.ListFieldDefs(ctx, p.ID)
		out = found
		return err
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteFieldDef removes a custom field definition from a project.
func (l *Local) DeleteFieldDef(ctx context.Context, projectRef, key string) error {
	actor, err := l.authorize(ctx, authz.ActionFieldWrite, authz.Resource{})
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		p, err := lookupProject(ctx, m.tx, projectRef)
		if err != nil {
			return err
		}
		if _, err := l.authorize(ctx, authz.ActionFieldWrite, authz.Resource{ProjectID: p.ID}); err != nil {
			return err
		}
		before, err := findFieldDef(ctx, m.tx, p.ID, key)
		if err != nil {
			return err
		}
		if before == nil {
			return core.NotFound("field %q on project %q", key, p.Key)
		}
		if err := m.tx.DeleteFieldDef(ctx, p.ID, key); err != nil {
			return err
		}
		return m.Record(auditFieldDelete, eventFieldDeleted, "field_def", before.ID, p.ID, before, nil,
			map[string]any{"key": key, "project_id": p.ID})
	})
}

// findFieldDef returns a project's definition for one key, or nil.
func findFieldDef(ctx context.Context, tx store.Tx, projectID, key string) (*core.FieldDef, error) {
	defs, err := tx.ListFieldDefs(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for i := range defs {
		if defs[i].Key == key {
			return &defs[i], nil
		}
	}
	return nil, nil
}
