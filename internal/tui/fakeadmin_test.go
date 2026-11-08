// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"
	"sync"

	"github.com/heliopsy/tix/internal/core"
)

// adminFake is the fake the server and global admin views are driven against.
// It embeds the shared fake and answers the administration reads and writes
// itself, recording each call, so those views' tests touch no shared fixture.
type adminFake struct {
	*fakeService
	amu sync.Mutex

	status    *core.StatusReport
	statusErr error
	conns     *core.ConnectionList
	connErr   error
	ended     []string
	statusN   int

	allTenants []core.Tenant
	admins     []core.User
	adminsN    int
	users      map[string][]core.TenantUser
	logs       map[string][]core.AccountLogEntry

	createdTenants []core.CreateTenantInput
	credentials    []core.InitialCredential
	deletedTenants []string
	granted        []string
	revoked        []string
	createdUsers   []userCall
	addedMembers   []memberCall
	updatedUsers   []updateCall
	deletedUsers   [][2]string
	logAsked       []string
}

type userCall struct {
	tenant string
	in     core.CreateUserInput
}

type memberCall struct {
	tenant string
	in     core.AddTenantMemberInput
}

type updateCall struct {
	tenant, id string
	in         core.UpdateUserInput
}

func newAdminFake() *adminFake {
	return &adminFake{fakeService: newFakeService(),
		users: map[string][]core.TenantUser{}, logs: map[string][]core.AccountLogEntry{}}
}

func (f *adminFake) Status(context.Context) (*core.StatusReport, error) {
	f.amu.Lock()
	defer f.amu.Unlock()
	f.statusN++
	return f.status, f.statusErr
}

func (f *adminFake) ListConnections(context.Context) (*core.ConnectionList, error) {
	f.amu.Lock()
	defer f.amu.Unlock()
	return f.conns, f.connErr
}

func (f *adminFake) EndConnection(_ context.Context, id string) error {
	f.amu.Lock()
	defer f.amu.Unlock()
	f.ended = append(f.ended, id)
	return nil
}

func (f *adminFake) ListTenants(context.Context, core.Page) ([]core.Tenant, string, error) {
	f.amu.Lock()
	defer f.amu.Unlock()
	return f.allTenants, "", nil
}

func (f *adminFake) ListGlobalAdmins(context.Context) ([]core.User, error) {
	f.amu.Lock()
	defer f.amu.Unlock()
	f.adminsN++
	return f.admins, nil
}

func (f *adminFake) CreateTenant(_ context.Context, in core.CreateTenantInput) (*core.CreatedTenant, error) {
	f.amu.Lock()
	defer f.amu.Unlock()
	f.createdTenants = append(f.createdTenants, in)
	f.allTenants = append(f.allTenants, core.Tenant{ID: "t-" + in.Key, Key: in.Key, Name: in.Name})
	return &core.CreatedTenant{Tenant: core.Tenant{Key: in.Key}, InitialCredentials: f.credentials}, nil
}

func (f *adminFake) DeleteTenant(_ context.Context, ref string) error {
	f.amu.Lock()
	defer f.amu.Unlock()
	f.deletedTenants = append(f.deletedTenants, ref)
	return nil
}

func (f *adminFake) GrantGlobalAdmin(_ context.Context, in core.GlobalAdminInput) (*core.User, error) {
	f.amu.Lock()
	defer f.amu.Unlock()
	f.granted = append(f.granted, in.Email)
	return &core.User{Email: in.Email}, nil
}

func (f *adminFake) RevokeGlobalAdmin(_ context.Context, email string) error {
	f.amu.Lock()
	defer f.amu.Unlock()
	f.revoked = append(f.revoked, email)
	return nil
}

func (f *adminFake) ListTenantUsers(_ context.Context, tenant string, _ core.Page) ([]core.TenantUser, string, error) {
	f.amu.Lock()
	defer f.amu.Unlock()
	return f.users[tenant], "", nil
}

func (f *adminFake) CreateTenantUser(_ context.Context, tenant string, in core.CreateUserInput) (*core.User, error) {
	f.amu.Lock()
	defer f.amu.Unlock()
	f.createdUsers = append(f.createdUsers, userCall{tenant, in})
	return &core.User{Email: in.Email}, nil
}

func (f *adminFake) AddTenantMember(_ context.Context, tenant string, in core.AddTenantMemberInput) (*core.User, error) {
	f.amu.Lock()
	defer f.amu.Unlock()
	f.addedMembers = append(f.addedMembers, memberCall{tenant, in})
	return &core.User{Email: in.Email}, nil
}

func (f *adminFake) UpdateTenantUser(_ context.Context, tenant, id string, in core.UpdateUserInput) (*core.User, error) {
	f.amu.Lock()
	defer f.amu.Unlock()
	f.updatedUsers = append(f.updatedUsers, updateCall{tenant, id, in})
	return &core.User{ID: id}, nil
}

func (f *adminFake) DeleteTenantUser(_ context.Context, tenant, id string) error {
	f.amu.Lock()
	defer f.amu.Unlock()
	f.deletedUsers = append(f.deletedUsers, [2]string{tenant, id})
	return nil
}

func (f *adminFake) GetAccountLogFor(_ context.Context, email string, _ core.Page) ([]core.AccountLogEntry, string, error) {
	f.amu.Lock()
	defer f.amu.Unlock()
	f.logAsked = append(f.logAsked, email)
	entries, ok := f.logs[email]
	if !ok {
		return nil, "", core.NotFound("no account has the address %s", email)
	}
	return entries, "", nil
}
