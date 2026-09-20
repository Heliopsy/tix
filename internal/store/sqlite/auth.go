package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/id"
	sqlb "github.com/thereisnotime/tix/internal/store/sql"
)

var actorColumns = []string{"id", "tenant_id", "kind", "handle", "display_name"}

func scanActor(s scanner) (core.Actor, error) {
	var a core.Actor
	if err := s.Scan(&a.ID, &a.TenantID, &a.Kind, &a.Handle, &a.DisplayName); err != nil {
		return core.Actor{}, mapErr(err, "scanning actor")
	}
	return a, nil
}

// CreateActor inserts an actor into this tenant.
func (t *tx) CreateActor(ctx context.Context, a *core.Actor) error {
	if a.ID == "" {
		a.ID = id.New()
	}
	a.TenantID = t.scope.TenantID
	ins := t.insert("actors").
		Set("id", a.ID).
		Set("kind", string(a.Kind)).
		Set("handle", a.Handle).
		Set("display_name", a.DisplayName).
		Set("created_at", t.now())
	_, err := t.execInsert(ctx, ins, "creating actor %q", a.Handle)
	return err
}

// GetActor returns an actor by identifier.
func (t *tx) GetActor(ctx context.Context, actorID string) (*core.Actor, error) {
	return t.actorWhere(ctx, "id = ?", actorID, "actor %q", actorID)
}

// GetActorByHandle returns an actor by its tenant-unique handle.
func (t *tx) GetActorByHandle(ctx context.Context, handle string) (*core.Actor, error) {
	return t.actorWhere(ctx, "handle = ?", handle, "actor %q", handle)
}

func (t *tx) actorWhere(ctx context.Context, cond string, arg any, what string, whatArgs ...any) (*core.Actor, error) {
	b := t.builder("actors").Select(actorColumns...).Where(cond, arg).Limit(1)
	q, args := b.SelectQuery()
	a, err := scanActor(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound(what, whatArgs...)
		}
		return nil, err
	}
	return &a, nil
}

// ListActors returns this tenant's actors, keyset paginated.
func (t *tx) ListActors(ctx context.Context, page core.Page) ([]core.Actor, error) {
	spec, err := resolvePage(page, "created_at", map[string]string{
		"created_at": "created_at",
		"handle":     "handle",
	})
	if err != nil {
		return nil, err
	}
	b := spec.apply(t.builder("actors").Select(actorColumns...), "id")
	rows, err := t.query(ctx, b, "listing actors")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.Actor{}
	for rows.Next() {
		v, err := scanActor(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "listing actors")
}

var userColumns = []string{
	"id", "email", "display_name", "created_at", "updated_at", "disabled_at",
	"sso_provider", "sso_subject",
}

func scanUser(s scanner) (core.User, error) {
	var (
		u        core.User
		created  sql.NullString
		updated  sql.NullString
		disabled sql.NullString
	)
	if err := s.Scan(&u.ID, &u.Email, &u.DisplayName, &created, &updated, &disabled,
		&u.SSOProvider, &u.SSOSubject); err != nil {
		return core.User{}, mapErr(err, "scanning user")
	}
	var err error
	if u.CreatedAt, err = sqlb.ScanTime(created); err != nil {
		return core.User{}, err
	}
	if u.UpdatedAt, err = sqlb.ScanTime(updated); err != nil {
		return core.User{}, err
	}
	if u.DisabledAt, err = sqlb.ScanNullTime(disabled); err != nil {
		return core.User{}, err
	}
	return u, nil
}

// CreateUser inserts a user. Users are global, not tenant-owned.
func (t *tx) CreateUser(ctx context.Context, u *core.User, passwordHash string) error {
	if u.ID == "" {
		u.ID = id.New()
	}
	now := t.store.clock.Now()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	u.UpdatedAt = now

	ins := t.insert("users").
		Set("id", u.ID).
		Set("email", u.Email).
		Set("password_hash", passwordHash).
		Set("display_name", u.DisplayName).
		Set("created_at", sqlb.TimeText(u.CreatedAt)).
		Set("updated_at", sqlb.TimeText(u.UpdatedAt)).
		Set("disabled_at", sqlb.NullTimeText(u.DisabledAt)).
		Set("sso_provider", u.SSOProvider).
		Set("sso_subject", u.SSOSubject)
	_, err := t.execInsert(ctx, ins, "creating user %q", u.Email)
	return err
}

// GetUser returns a user by identifier.
func (t *tx) GetUser(ctx context.Context, userID string) (*core.User, error) {
	b := t.builder("users").Select(userColumns...).Where("id = ?", userID).Limit(1)
	q, args := b.SelectQuery()
	u, err := scanUser(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("user %q", userID)
		}
		return nil, err
	}
	return &u, nil
}

// GetUserByEmail returns a user and the stored password hash.
func (t *tx) GetUserByEmail(ctx context.Context, email string) (*core.User, string, error) {
	cols := append(append([]string{}, userColumns...), "password_hash")
	b := t.builder("users").Select(cols...).Where("email = ?", email).Limit(1)
	q, args := b.SelectQuery()

	var (
		u        core.User
		created  sql.NullString
		updated  sql.NullString
		disabled sql.NullString
		hash     string
	)
	err := t.ex.QueryRowContext(ctx, q, args...).Scan(&u.ID, &u.Email, &u.DisplayName,
		&created, &updated, &disabled, &u.SSOProvider, &u.SSOSubject, &hash)
	if err != nil {
		if mapped := mapErr(err, "reading user %q", email); core.IsKind(mapped, core.KindNotFound) {
			return nil, "", core.NotFound("user %q", email)
		}
		return nil, "", mapErr(err, "reading user %q", email)
	}
	if u.CreatedAt, err = sqlb.ScanTime(created); err != nil {
		return nil, "", err
	}
	if u.UpdatedAt, err = sqlb.ScanTime(updated); err != nil {
		return nil, "", err
	}
	if u.DisabledAt, err = sqlb.ScanNullTime(disabled); err != nil {
		return nil, "", err
	}
	return &u, hash, nil
}

// ListUsers returns users, keyset paginated.
func (t *tx) ListUsers(ctx context.Context, page core.Page) ([]core.User, error) {
	spec, err := resolvePage(page, "created_at", map[string]string{
		"created_at": "created_at",
		"email":      "email",
	})
	if err != nil {
		return nil, err
	}
	b := spec.apply(t.builder("users").Select(userColumns...), "id")
	rows, err := t.query(ctx, b, "listing users")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.User{}
	for rows.Next() {
		v, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "listing users")
}

// UpdateUser writes a user's mutable fields, replacing the password hash when given.
func (t *tx) UpdateUser(ctx context.Context, u *core.User, passwordHash string) error {
	u.UpdatedAt = t.store.clock.Now()
	b := t.builder("users").
		Where("id = ?", u.ID).
		Set("email", u.Email).
		Set("display_name", u.DisplayName).
		Set("updated_at", sqlb.TimeText(u.UpdatedAt)).
		Set("disabled_at", sqlb.NullTimeText(u.DisabledAt)).
		Set("sso_provider", u.SSOProvider).
		Set("sso_subject", u.SSOSubject)
	if passwordHash != "" {
		b.Set("password_hash", passwordHash)
	}
	n, err := t.execUpdate(ctx, b, "updating user %q", u.ID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("user %q", u.ID)
	}
	return nil
}

// DeleteUser removes a user.
func (t *tx) DeleteUser(ctx context.Context, userID string) error {
	b := t.builder("users").Where("id = ?", userID)
	n, err := t.execDelete(ctx, b, "deleting user %q", userID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("user %q", userID)
	}
	return nil
}

// CreateSession stores a session token hash for an actor.
func (t *tx) CreateSession(ctx context.Context, actorID, tokenHash string, expiresAt time.Time) error {
	ins := t.insert("sessions").
		Set("id", id.New()).
		Set("actor_id", actorID).
		Set("token_hash", tokenHash).
		Set("created_at", t.now()).
		Set("expires_at", sqlb.TimeText(expiresAt))
	_, err := t.execInsert(ctx, ins, "creating session for actor %q", actorID)
	return err
}

// GetSessionByHash returns the actor and expiry behind a session token hash.
func (t *tx) GetSessionByHash(ctx context.Context, tokenHash string) (string, time.Time, error) {
	b := t.builder("sessions").
		Select("actor_id", "expires_at").
		Where("token_hash = ?", tokenHash).
		Limit(1)
	q, args := b.SelectQuery()

	var (
		actorID string
		expires sql.NullString
	)
	if err := t.ex.QueryRowContext(ctx, q, args...).Scan(&actorID, &expires); err != nil {
		if mapped := mapErr(err, "reading session"); core.IsKind(mapped, core.KindNotFound) {
			return "", time.Time{}, core.NotFound("session not found")
		}
		return "", time.Time{}, mapErr(err, "reading session")
	}
	at, err := sqlb.ScanTime(expires)
	if err != nil {
		return "", time.Time{}, err
	}
	return actorID, at, nil
}

// DeleteSession removes one session.
func (t *tx) DeleteSession(ctx context.Context, tokenHash string) error {
	b := t.builder("sessions").Where("token_hash = ?", tokenHash)
	n, err := t.execDelete(ctx, b, "deleting session")
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("session not found")
	}
	return nil
}

// DeleteExpiredSessions removes sessions that have passed their expiry.
func (t *tx) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	b := t.builder("sessions").Where("expires_at <= ?", sqlb.TimeText(now))
	return t.execDelete(ctx, b, "deleting expired sessions")
}

var tokenColumns = []string{
	"id", "tenant_id", "actor_id", "name", "scopes", "project_id",
	"created_at", "expires_at", "last_used_at", "revoked_at",
}

func scanToken(s scanner) (core.APIToken, error) {
	var (
		tk       core.APIToken
		scopes   string
		project  sql.NullString
		created  sql.NullString
		expires  sql.NullString
		lastUsed sql.NullString
		revoked  sql.NullString
	)
	if err := s.Scan(&tk.ID, &tk.TenantID, &tk.ActorID, &tk.Name, &scopes, &project,
		&created, &expires, &lastUsed, &revoked); err != nil {
		return core.APIToken{}, mapErr(err, "scanning api token")
	}
	tk.ProjectID = sqlb.Text(project)
	if err := sqlb.ParseJSON(scopes, &tk.Scopes); err != nil {
		return core.APIToken{}, core.Internal("decoding scopes of token %q", tk.ID).Wrap(err)
	}
	var err error
	if tk.CreatedAt, err = sqlb.ScanTime(created); err != nil {
		return core.APIToken{}, err
	}
	if tk.ExpiresAt, err = sqlb.ScanNullTime(expires); err != nil {
		return core.APIToken{}, err
	}
	if tk.LastUsedAt, err = sqlb.ScanNullTime(lastUsed); err != nil {
		return core.APIToken{}, err
	}
	if tk.RevokedAt, err = sqlb.ScanNullTime(revoked); err != nil {
		return core.APIToken{}, err
	}
	return tk, nil
}

// CreateToken stores an API token's metadata and its hash.
func (t *tx) CreateToken(ctx context.Context, tk *core.APIToken, tokenHash string) error {
	if tk.ID == "" {
		tk.ID = id.New()
	}
	tk.TenantID = t.scope.TenantID
	if tk.CreatedAt.IsZero() {
		tk.CreatedAt = t.store.clock.Now()
	}
	scopes, err := sqlb.JSONText(tk.Scopes, "[]")
	if err != nil {
		return core.Internal("encoding scopes of token %q", tk.Name).Wrap(err)
	}
	ins := t.insert("api_tokens").
		Set("id", tk.ID).
		Set("actor_id", tk.ActorID).
		Set("name", tk.Name).
		Set("token_hash", tokenHash).
		Set("scopes", scopes).
		Set("project_id", sqlb.NullText(tk.ProjectID)).
		Set("created_at", sqlb.TimeText(tk.CreatedAt)).
		Set("expires_at", sqlb.NullTimeText(tk.ExpiresAt)).
		Set("last_used_at", sqlb.NullTimeText(tk.LastUsedAt)).
		Set("revoked_at", sqlb.NullTimeText(tk.RevokedAt))
	_, err = t.execInsert(ctx, ins, "creating api token %q", tk.Name)
	return err
}

// GetTokenByHash returns the token behind a hash.
func (t *tx) GetTokenByHash(ctx context.Context, tokenHash string) (*core.APIToken, error) {
	b := t.builder("api_tokens").Select(tokenColumns...).Where("token_hash = ?", tokenHash).Limit(1)
	q, args := b.SelectQuery()
	tk, err := scanToken(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("api token not found")
		}
		return nil, err
	}
	return &tk, nil
}

// ListTokens returns an actor's tokens.
func (t *tx) ListTokens(ctx context.Context, actorID string) ([]core.APIToken, error) {
	b := t.builder("api_tokens").
		Select(tokenColumns...).
		Where("actor_id = ?", actorID).
		OrderBy("created_at", core.Ascending).
		OrderBy("id", core.Ascending)
	rows, err := t.query(ctx, b, "listing api tokens")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.APIToken{}
	for rows.Next() {
		v, err := scanToken(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "listing api tokens")
}

// RevokeToken marks a token unusable from the given instant.
func (t *tx) RevokeToken(ctx context.Context, tokenID string, at time.Time) error {
	b := t.builder("api_tokens").
		Where("id = ?", tokenID).
		Set("revoked_at", sqlb.TimeText(at))
	n, err := t.execUpdate(ctx, b, "revoking api token %q", tokenID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("api token %q", tokenID)
	}
	return nil
}

// TouchToken records the last time a token authenticated.
func (t *tx) TouchToken(ctx context.Context, tokenID string, at time.Time) error {
	b := t.builder("api_tokens").
		Where("id = ?", tokenID).
		Set("last_used_at", sqlb.TimeText(at))
	n, err := t.execUpdate(ctx, b, "touching api token %q", tokenID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("api token %q", tokenID)
	}
	return nil
}
