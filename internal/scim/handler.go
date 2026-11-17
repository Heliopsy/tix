// SPDX-License-Identifier: AGPL-3.0-or-later

// Package scim serves SCIM 2.0 provisioning (RFC 7643 and RFC 7644) under
// /scim/v2. It translates the protocol to core.Provisioner and holds no rule
// of its own: who may provision, how an account is matched and what a group
// does to a role are decided by the service.
package scim

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// Handler answers the SCIM endpoints.
type Handler struct {
	svc    core.Provisioner
	origin func(*http.Request) string
	mux    *http.ServeMux
}

// New returns a handler over the provisioner. origin gives the address a
// request reached the server at, which resource locations are built under.
func New(svc core.Provisioner, origin func(*http.Request) string) *Handler {
	h := &Handler{svc: svc, origin: origin, mux: http.NewServeMux()}
	p := core.SCIMPath
	h.mux.HandleFunc("GET "+p+"/ServiceProviderConfig", h.serviceProviderConfig)
	h.mux.HandleFunc("GET "+p+"/ResourceTypes", h.resourceTypes)
	h.mux.HandleFunc("GET "+p+"/ResourceTypes/{id}", h.resourceType)
	h.mux.HandleFunc("GET "+p+"/Schemas", h.schemas)
	h.mux.HandleFunc("GET "+p+"/Schemas/{id}", h.schema)
	h.mux.HandleFunc("GET "+p+"/Users", h.listUsers)
	h.mux.HandleFunc("POST "+p+"/Users", h.createUser)
	h.mux.HandleFunc("GET "+p+"/Users/{id}", h.getUser)
	h.mux.HandleFunc("PUT "+p+"/Users/{id}", h.replaceUser)
	h.mux.HandleFunc("PATCH "+p+"/Users/{id}", h.patchUser)
	h.mux.HandleFunc("DELETE "+p+"/Users/{id}", h.deleteUser)
	h.mux.HandleFunc("GET "+p+"/Groups", h.listGroups)
	h.mux.HandleFunc("POST "+p+"/Groups", h.createGroup)
	h.mux.HandleFunc("GET "+p+"/Groups/{id}", h.getGroup)
	h.mux.HandleFunc("PUT "+p+"/Groups/{id}", h.replaceGroup)
	h.mux.HandleFunc("PATCH "+p+"/Groups/{id}", h.patchGroup)
	h.mux.HandleFunc("DELETE "+p+"/Groups/{id}", h.deleteGroup)
	h.mux.HandleFunc(p+"/", h.unknown)
	return h
}

// ServeHTTP authenticates the caller as a provider and routes the request.
// No credential is a 401; one without scim:provision is a 403, whatever the
// tenant's switch says.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	actor, ok := core.ActorFrom(r.Context())
	if !ok {
		w.Header().Set("WWW-Authenticate", `Bearer realm="tix scim"`)
		writeError(w, core.Unauthenticated("a bearer token carrying %s is required", core.ScopeSCIMProvision))
		return
	}
	if actor.TokenID == "" || !actor.HasScope(core.ScopeSCIMProvision) {
		writeError(w, core.Forbidden("the SCIM endpoints need a token carrying %s", core.ScopeSCIMProvision))
		return
	}
	h.mux.ServeHTTP(w, r)
}

// base is the endpoint's absolute address for this request.
func (h *Handler) base(r *http.Request) string {
	origin := ""
	if h.origin != nil {
		origin = strings.TrimRight(h.origin(r), "/")
	}
	return origin + core.SCIMPath
}

// unknown answers a path or method SCIM does not define here.
func (h *Handler) unknown(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, core.SCIMPath+"/")
	switch strings.SplitN(rest, "/", 2)[0] {
	case "Users", "Groups", "ServiceProviderConfig", "ResourceTypes", "Schemas":
		writeError(w, &protoError{status: http.StatusMethodNotAllowed, detail: r.Method + " is not supported here"})
	default:
		writeError(w, core.NotFound("no SCIM endpoint at %s", r.URL.Path))
	}
}

// ready refuses a discovery request the provisioner would refuse.
func (h *Handler) ready(w http.ResponseWriter, r *http.Request) bool {
	if err := h.svc.SCIMReady(r.Context()); err != nil {
		writeError(w, err)
		return false
	}
	return true
}

func (h *Handler) serviceProviderConfig(w http.ResponseWriter, r *http.Request) {
	if h.ready(w, r) {
		writeJSON(w, http.StatusOK, spConfig(h.base(r)))
	}
}

func (h *Handler) resourceTypes(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w, r) {
		return
	}
	types := resourceTypes(h.base(r))
	writeJSON(w, http.StatusOK, ListResponse[ResourceType]{Schemas: []string{SchemaListResponse},
		TotalResults: len(types), StartIndex: 1, ItemsPerPage: len(types), Resources: types})
}

func (h *Handler) resourceType(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w, r) {
		return
	}
	for _, t := range resourceTypes(h.base(r)) {
		if t.ID == r.PathValue("id") {
			writeJSON(w, http.StatusOK, t)
			return
		}
	}
	writeError(w, core.NotFound("no resource type %q", r.PathValue("id")))
}

func (h *Handler) schemas(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w, r) {
		return
	}
	all := schemas(h.base(r))
	writeJSON(w, http.StatusOK, ListResponse[Schema]{Schemas: []string{SchemaListResponse},
		TotalResults: len(all), StartIndex: 1, ItemsPerPage: len(all), Resources: all})
}

func (h *Handler) schema(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w, r) {
		return
	}
	for _, s := range schemas(h.base(r)) {
		if s.ID == r.PathValue("id") {
			writeJSON(w, http.StatusOK, s)
			return
		}
	}
	writeError(w, core.NotFound("no schema %q", r.PathValue("id")))
}

// query reads filter, startIndex, count and excludedAttributes.
func query(r *http.Request, kind resource) (core.SCIMQuery, error) {
	v := r.URL.Query()
	q := core.SCIMQuery{StartIndex: 1, Count: DefaultCount}
	if raw := strings.TrimSpace(v.Get("filter")); raw != "" {
		conds, err := parseFilter(raw, kind)
		if err != nil {
			return q, err
		}
		q.Conditions = conds
	}
	if raw := v.Get("startIndex"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return q, badRequest(TypeInvalidValue, "startIndex must be a number")
		}
		q.StartIndex = max(n, 1)
	}
	if raw := v.Get("count"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return q, badRequest(TypeInvalidValue, "count must be a number")
		}
		q.Count = min(max(n, 0), MaxResults)
	}
	q.WithoutMembers = excludesMembers(r)
	return q, nil
}

// excludesMembers reports whether the request leaves out group members.
func excludesMembers(r *http.Request) bool {
	for _, a := range strings.Split(r.URL.Query().Get("excludedAttributes"), ",") {
		if strings.EqualFold(strings.TrimSpace(a), "members") {
			return true
		}
	}
	return false
}

// readBody decodes a JSON body sent as application/scim+json or JSON.
func readBody(r *http.Request, v any) error {
	if ct := r.Header.Get(wire.HeaderContentType); ct != "" {
		media, _, err := mime.ParseMediaType(ct)
		if err != nil || (media != wire.ContentSCIM && media != wire.ContentJSON) {
			return &protoError{status: http.StatusUnsupportedMediaType,
				detail: "the body must be " + wire.ContentSCIM}
		}
	}
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		var tooLarge *http.MaxBytesError
		switch {
		case errors.As(err, &tooLarge):
			return &protoError{status: http.StatusRequestEntityTooLarge, detail: "the body is too large"}
		case errors.Is(err, io.EOF):
			return badRequest(TypeInvalidSyntax, "a body is required")
		}
		return badRequest(TypeInvalidSyntax, "the body is not valid JSON: "+err.Error())
	}
	return nil
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	q, err := query(r, resourceUser)
	if err != nil {
		writeError(w, err)
		return
	}
	users, total, err := h.svc.SCIMListUsers(r.Context(), q)
	if err != nil {
		writeError(w, err)
		return
	}
	base := h.base(r)
	out := make([]User, 0, len(users))
	for _, u := range users {
		out = append(out, userOut(u, base))
	}
	writeJSON(w, http.StatusOK, ListResponse[User]{Schemas: []string{SchemaListResponse},
		TotalResults: total, StartIndex: q.StartIndex, ItemsPerPage: len(out), Resources: out})
}

// writeUser answers with one user.
func (h *Handler) writeUser(w http.ResponseWriter, r *http.Request, status int, u *core.SCIMUser) {
	out := userOut(*u, h.base(r))
	if status == http.StatusCreated {
		w.Header().Set("Location", out.Meta.Location)
	}
	writeJSON(w, status, out)
}

func (h *Handler) getUser(w http.ResponseWriter, r *http.Request) {
	u, err := h.svc.SCIMGetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	h.writeUser(w, r, http.StatusOK, u)
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	var in userIn
	if err := readBody(r, &in); err != nil {
		writeError(w, err)
		return
	}
	u, err := in.toUser()
	if err != nil {
		writeError(w, err)
		return
	}
	created, err := h.svc.SCIMCreateUser(r.Context(), u)
	if err != nil {
		writeError(w, err)
		return
	}
	h.writeUser(w, r, http.StatusCreated, created)
}

func (h *Handler) replaceUser(w http.ResponseWriter, r *http.Request) {
	var in userIn
	if err := readBody(r, &in); err != nil {
		writeError(w, err)
		return
	}
	want, err := in.toUser()
	if err != nil {
		writeError(w, err)
		return
	}
	u, err := h.svc.SCIMUpdateUser(r.Context(), r.PathValue("id"), func(u *core.SCIMUser) error {
		email := want.Email
		if email == "" {
			email = u.Email
		}
		want.ID, want.Groups, want.Email = u.ID, u.Groups, email
		*u = want
		return nil
	})
	if err != nil {
		writeError(w, err)
		return
	}
	h.writeUser(w, r, http.StatusOK, u)
}

func (h *Handler) patchUser(w http.ResponseWriter, r *http.Request) {
	var req PatchRequest
	if err := readBody(r, &req); err != nil {
		writeError(w, err)
		return
	}
	u, err := h.svc.SCIMUpdateUser(r.Context(), r.PathValue("id"), func(u *core.SCIMUser) error {
		return applyUserPatch(u, req)
	})
	if err != nil {
		writeError(w, err)
		return
	}
	h.writeUser(w, r, http.StatusOK, u)
}

func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.SCIMDeleteUser(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listGroups(w http.ResponseWriter, r *http.Request) {
	q, err := query(r, resourceGroup)
	if err != nil {
		writeError(w, err)
		return
	}
	groups, total, err := h.svc.SCIMListGroups(r.Context(), q)
	if err != nil {
		writeError(w, err)
		return
	}
	base := h.base(r)
	out := make([]Group, 0, len(groups))
	for _, g := range groups {
		out = append(out, groupOut(g, base))
	}
	writeJSON(w, http.StatusOK, ListResponse[Group]{Schemas: []string{SchemaListResponse},
		TotalResults: total, StartIndex: q.StartIndex, ItemsPerPage: len(out), Resources: out})
}

// writeGroup answers with one group.
func (h *Handler) writeGroup(w http.ResponseWriter, r *http.Request, status int, g *core.SCIMGroup) {
	out := groupOut(*g, h.base(r))
	if status == http.StatusCreated {
		w.Header().Set("Location", out.Meta.Location)
	}
	writeJSON(w, status, out)
}

func (h *Handler) getGroup(w http.ResponseWriter, r *http.Request) {
	g, err := h.svc.SCIMGetGroup(r.Context(), r.PathValue("id"), !excludesMembers(r))
	if err != nil {
		writeError(w, err)
		return
	}
	h.writeGroup(w, r, http.StatusOK, g)
}

func (h *Handler) createGroup(w http.ResponseWriter, r *http.Request) {
	var in groupIn
	if err := readBody(r, &in); err != nil {
		writeError(w, err)
		return
	}
	g, err := h.svc.SCIMCreateGroup(r.Context(), in.toGroup())
	if err != nil {
		writeError(w, err)
		return
	}
	h.writeGroup(w, r, http.StatusCreated, g)
}

func (h *Handler) replaceGroup(w http.ResponseWriter, r *http.Request) {
	var in groupIn
	if err := readBody(r, &in); err != nil {
		writeError(w, err)
		return
	}
	want := in.toGroup()
	g, err := h.svc.SCIMUpdateGroup(r.Context(), r.PathValue("id"), func(g *core.SCIMGroup) error {
		want.ID = g.ID
		*g = want
		return nil
	})
	if err != nil {
		writeError(w, err)
		return
	}
	h.writeGroup(w, r, http.StatusOK, g)
}

func (h *Handler) patchGroup(w http.ResponseWriter, r *http.Request) {
	var req PatchRequest
	if err := readBody(r, &req); err != nil {
		writeError(w, err)
		return
	}
	g, err := h.svc.SCIMUpdateGroup(r.Context(), r.PathValue("id"), func(g *core.SCIMGroup) error {
		return applyGroupPatch(g, req)
	})
	if err != nil {
		writeError(w, err)
		return
	}
	h.writeGroup(w, r, http.StatusOK, g)
}

func (h *Handler) deleteGroup(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.SCIMDeleteGroup(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
