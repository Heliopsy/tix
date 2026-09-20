package connect

import (
	"context"
	"io"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/service"
)

// localService completes the service contract while parts of the local
// implementation are still being built. The fallback sits one level deeper
// than the embedded implementation, so any method service.Local grows shadows
// the placeholder automatically and this file needs no edit.
type localService struct {
	*service.Local
	fallback
}

type fallback struct{ unimplemented }

// unimplemented reports that an operation is absent from this build.
type unimplemented struct{}

func absent(name string) *core.Error {
	return core.Internal("%s is not available in this build", name)
}

// CreateUser is not available in this build.
func (unimplemented) CreateUser(context.Context, core.CreateUserInput) (*core.User, error) {
	return nil, absent("user management")
}

// GetUser is not available in this build.
func (unimplemented) GetUser(context.Context, string) (*core.User, error) {
	return nil, absent("user management")
}

// ListUsers is not available in this build.
func (unimplemented) ListUsers(context.Context, core.Page) ([]core.User, string, error) {
	return nil, "", absent("user management")
}

// UpdateUser is not available in this build.
func (unimplemented) UpdateUser(context.Context, string, core.UpdateUserInput) (*core.User, error) {
	return nil, absent("user management")
}

// DeleteUser is not available in this build.
func (unimplemented) DeleteUser(context.Context, string) error { return absent("user management") }

// Login is not available in this build.
func (unimplemented) Login(context.Context, string, string) (*core.Session, error) {
	return nil, absent("login")
}

// Logout is not available in this build.
func (unimplemented) Logout(context.Context) error { return absent("logout") }

// CreateToken is not available in this build.
func (unimplemented) CreateToken(context.Context, core.CreateTokenInput) (*core.IssuedToken, error) {
	return nil, absent("token management")
}

// ListTokens is not available in this build.
func (unimplemented) ListTokens(context.Context, string) ([]core.APIToken, error) {
	return nil, absent("token management")
}

// RevokeToken is not available in this build.
func (unimplemented) RevokeToken(context.Context, string) error {
	return absent("token management")
}

// PutWebhook is not available in this build.
func (unimplemented) PutWebhook(context.Context, core.WebhookInput) (*core.WebhookEndpoint, error) {
	return nil, absent("webhooks")
}

// ListWebhooks is not available in this build.
func (unimplemented) ListWebhooks(context.Context) ([]core.WebhookEndpoint, error) {
	return nil, absent("webhooks")
}

// DeleteWebhook is not available in this build.
func (unimplemented) DeleteWebhook(context.Context, string) error { return absent("webhooks") }

// ListDeliveries is not available in this build.
func (unimplemented) ListDeliveries(context.Context, core.DeliveryFilter) ([]core.WebhookDelivery, string, error) {
	return nil, "", absent("webhooks")
}

// RedeliverWebhook is not available in this build.
func (unimplemented) RedeliverWebhook(context.Context, string) error { return absent("webhooks") }

// ExportTo is not available in this build.
func (unimplemented) ExportTo(context.Context, core.ExportInput, io.Writer) error {
	return absent("export")
}

// ImportFrom is not available in this build.
func (unimplemented) ImportFrom(context.Context, io.Reader, core.ImportInput) (*core.ImportResult, error) {
	return nil, absent("import")
}

// PutSyncSource is not available in this build.
func (unimplemented) PutSyncSource(context.Context, core.SyncSourceInput) (*core.SyncSource, error) {
	return nil, absent("external sync")
}

// ListSyncSources is not available in this build.
func (unimplemented) ListSyncSources(context.Context) ([]core.SyncSource, error) {
	return nil, absent("external sync")
}

// DeleteSyncSource is not available in this build.
func (unimplemented) DeleteSyncSource(context.Context, string) error {
	return absent("external sync")
}

// RunSync is not available in this build.
func (unimplemented) RunSync(context.Context, core.RunSyncInput) (*core.SyncResult, error) {
	return nil, absent("external sync")
}
