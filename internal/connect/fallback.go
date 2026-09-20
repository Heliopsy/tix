package connect

import (
	"context"
	"io"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/service"
)

// localService completes the service contract while snapshot transfer and
// external sync are still to be built.
//
// Only the methods genuinely absent from service.Local are declared below. A
// method that Local later grows will collide with its placeholder here rather
// than being silently shadowed, so removing a placeholder is a deliberate step
// and a regression in Local surfaces as a build failure.
type localService struct {
	*service.Local
	pending
}

// pending holds the operations this build cannot perform yet.
type pending struct{}

func absent(area string) *core.Error {
	return core.Internal("%s is not available in this build", area)
}

// ExportTo is not available in this build.
func (pending) ExportTo(context.Context, core.ExportInput, io.Writer) error {
	return absent("snapshot export")
}

// ImportFrom is not available in this build.
func (pending) ImportFrom(context.Context, io.Reader, core.ImportInput) (*core.ImportResult, error) {
	return nil, absent("snapshot import")
}

// PutSyncSource is not available in this build.
func (pending) PutSyncSource(context.Context, core.SyncSourceInput) (*core.SyncSource, error) {
	return nil, absent("external sync")
}

// ListSyncSources is not available in this build.
func (pending) ListSyncSources(context.Context) ([]core.SyncSource, error) {
	return nil, absent("external sync")
}

// DeleteSyncSource is not available in this build.
func (pending) DeleteSyncSource(context.Context, string) error {
	return absent("external sync")
}

// RunSync is not available in this build.
func (pending) RunSync(context.Context, core.RunSyncInput) (*core.SyncResult, error) {
	return nil, absent("external sync")
}
