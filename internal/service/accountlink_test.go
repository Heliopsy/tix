// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// seedLinkedPerson writes an account and an admin actor of the fixture's
// tenant linked to it, with an actor identifier that is not the account's.
// Every path that once found the account by the actor's identifier finds
// nothing for this person, which is what makes it the guard for the link.
func seedLinkedPerson(t *testing.T, l *Local, scope core.TenantScope, email string) (*core.User, *core.Actor) {
	t.Helper()
	ctx := context.Background()
	hash, err := l.hasher.Hash(testPassword)
	if err != nil {
		t.Fatalf("hashing: %v", err)
	}
	user := &core.User{ID: l.ids.New(), Email: email}
	actor := &core.Actor{ID: l.ids.New(), UserID: user.ID, Kind: core.ActorUser, Handle: "linked"}
	if user.ID == actor.ID {
		t.Fatal("the account and the actor were given one identifier; the guard needs two")
	}
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		if err := tx.CreateUser(ctx, user, hash); err != nil {
			return err
		}
		if err := tx.CreateActor(ctx, actor); err != nil {
			return err
		}
		if err := tx.AddMember(ctx, &core.Membership{ActorID: actor.ID, Role: core.RoleAdmin}); err != nil {
			return err
		}
		return tx.SetUserGlobalAdmin(ctx, user.ID, true)
	}); err != nil {
		t.Fatalf("seeding %s: %v", email, err)
	}
	return user, &core.Actor{ID: actor.ID, TenantID: scope.TenantID, Kind: core.ActorUser,
		Handle: actor.Handle, Role: core.RoleAdmin}
}

func TestAnActorWhoseIdentifierIsNotItsAccountsSignsIn(t *testing.T) {
	l, clk, scope, admin := newTOTPLocal(t)
	user, person := seedLinkedPerson(t, l, scope, "linked@example.com")

	session, err := signIn(l, scope, user.Email, testPassword)
	if err != nil || session == nil {
		t.Fatalf("password sign-in = %+v, %v; want a session", session, err)
	}
	if session.ActorID != person.ID {
		t.Fatalf("session acts as %q, want the person's actor %q", session.ActorID, person.ID)
	}

	secret, _ := enrol(t, l, clk, person)
	if got := storedTOTP(t, l, scope, user.ID); got == nil || !got.Confirmed() {
		t.Fatalf("enrolment stored against the account = %+v, want a confirmed one", got)
	}
	res, err := l.Login(loginContext(scope), core.LoginInput{
		Email: user.Email, Password: testPassword, Surface: core.SurfaceCLI,
	})
	if err != nil || res.Challenge == nil {
		t.Fatalf("password step with a factor enrolled = %+v, %v; want a challenge", res, err)
	}
	clk.Advance(30 * time.Second)
	done, err := l.Login(loginContext(scope), core.LoginInput{
		Surface: core.SurfaceCLI, Ticket: res.Challenge.Ticket, Code: codeFor(t, secret, clk),
	})
	if err != nil || done.Session == nil || done.Session.ActorID != person.ID {
		t.Fatalf("code step = %+v, %v; want a session for %q", done, err, person.ID)
	}

	s := newSigner(t)
	enrolKeyFor(t, l, admin, person.ID, s)
	ctx := sshLoginContext(scope)
	ssh, err := l.SSHLogin(ctx, signedAnswer(t, l, ctx, s, ""))
	if err != nil || ssh.Challenge == nil {
		t.Fatalf("SSH sign-in of an enrolled person = %+v, %v; want a code challenge", ssh, err)
	}
	clk.Advance(30 * time.Second)
	done, err = l.Login(loginContext(scope), core.LoginInput{
		Surface: core.SurfaceCLI, Ticket: ssh.Challenge.Ticket, Code: codeFor(t, secret, clk),
	})
	if err != nil || done.Session == nil || done.Session.ActorID != person.ID {
		t.Fatalf("SSH code step = %+v, %v; want a session for %q", done, err, person.ID)
	}
}

func TestAnActorWhoseIdentifierIsNotItsAccountsIsAGlobalAdmin(t *testing.T) {
	l, _, scope, _ := newLocal(t)
	_, person := seedLinkedPerson(t, l, scope, "linked@example.com")
	ctx := authContext(person)

	who, err := l.WhoAmI(ctx)
	if err != nil || !who.GlobalAdmin {
		t.Fatalf("WhoAmI = %+v, %v; want a global admin", who, err)
	}
	issued, err := l.CreateToken(ctx, core.CreateTokenInput{
		Name: "root", Scopes: []core.Scope{core.ScopeAll, core.ScopeGlobalAdmin},
	})
	if err != nil {
		t.Fatalf("minting a global:admin token for the person = %v, want it issued", err)
	}
	if issued.ActorID != person.ID {
		t.Errorf("token acts as %q, want %q", issued.ActorID, person.ID)
	}
}

func TestAnActorWhoseIdentifierIsNotItsAccountsIsAdministered(t *testing.T) {
	l, clk, scope, admin := newTOTPLocal(t)
	user, person := seedLinkedPerson(t, l, scope, "linked@example.com")
	ctx := authContext(admin)

	got, err := l.GetUser(ctx, person.ID)
	if err != nil || got.Email != user.Email || got.ID != person.ID {
		t.Fatalf("GetUser(actor) = %+v, %v; want %s under the actor's identifier", got, err, user.Email)
	}
	if _, err := l.GetUser(ctx, user.ID); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("GetUser(account identifier) = %v, want not found: a tenant names its users by actor", err)
	}

	listed, _, err := l.ListUsers(ctx, core.Page{Limit: 50})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	found := false
	for _, u := range listed {
		if u.Email == user.Email {
			found = u.ID == person.ID
		}
	}
	if !found {
		t.Fatalf("ListUsers = %+v, want %s listed under %q", listed, user.Email, person.ID)
	}

	name := "Linked Person"
	updated, err := l.UpdateUser(ctx, person.ID, core.UpdateUserInput{DisplayName: &name})
	if err != nil || updated.DisplayName != name || updated.ID != person.ID {
		t.Fatalf("UpdateUser = %+v, %v; want the name set under the actor's identifier", updated, err)
	}
	role := core.RoleMember
	if _, err := l.UpdateUser(ctx, person.ID, core.UpdateUserInput{Role: &role}); err != nil {
		t.Fatalf("changing the role: %v", err)
	}
	var member *core.Membership
	if err := l.read(context.Background(), admin, func(tx store.Tx) error {
		member, err = tx.GetMember(context.Background(), person.ID)
		return err
	}); err != nil || member.Role != core.RoleMember {
		t.Fatalf("membership after the role change = %+v, %v; want member", member, err)
	}

	enrol(t, l, clk, person)
	if err := l.ClearUserTOTP(ctx, person.ID); err != nil {
		t.Fatalf("ClearUserTOTP(actor) = %v, want the account's factor cleared", err)
	}
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		_, err := tx.GetUserTOTP(context.Background(), user.ID)
		return err
	}); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("reading the enrolment after the clear = %v, want not found", err)
	}

	if err := l.DeleteUser(ctx, person.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if _, err := signIn(l, scope, user.Email, testPassword); err == nil {
		t.Fatal("a deleted person still signs in")
	}
}

func TestAnActorWhoseIdentifierIsNotItsAccountsKeepsItsDatePreference(t *testing.T) {
	l, _, scope, _ := newLocal(t)
	_, person := seedLinkedPerson(t, l, scope, "linked@example.com")
	zone := "Asia/Tokyo"

	if _, err := l.SetTimePrefs(authContext(person), core.TimePrefsInput{Timezone: &zone}); err != nil {
		t.Fatalf("SetTimePrefs = %v, want the person's preference stored", err)
	}
	got, err := l.GetTimeSettings(authContext(person))
	if err != nil || !got.Personal || got.User.Timezone != zone {
		t.Fatalf("GetTimeSettings = %+v, %v; want the person's own %s", got, err, zone)
	}
}

func TestAnActorWhoseIdentifierIsNotItsAccountsCountsAsAnEnrolledAdmin(t *testing.T) {
	l, clk, scope, admin := newTOTPLocal(t)
	_, person := seedLinkedPerson(t, l, scope, "linked@example.com")
	enrol(t, l, clk, person)

	report, err := l.Status(authContext(admin))
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if report.SecondFactor.EnrolledAdmins != 1 {
		t.Fatalf("enrolled admins = %d, want the linked person counted", report.SecondFactor.EnrolledAdmins)
	}
}
