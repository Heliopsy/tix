// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
)

func TestEveryServiceOperationHasAWebBindingOrAnExemption(t *testing.T) {
	t.Parallel()
	bound := map[string]bool{}
	for _, binding := range web.Bindings() {
		for _, name := range binding.Services {
			bound[name] = true
		}
	}
	exempt := map[string]string{}
	for _, exemption := range web.Exemptions() {
		exempt[exemption.Service] = exemption.Reason
	}

	iface := reflect.TypeOf((*core.Service)(nil)).Elem()
	for i := range iface.NumMethod() {
		name := iface.Method(i).Name
		if bound[name] {
			continue
		}
		reason, ok := exempt[name]
		if !ok {
			t.Errorf("service operation %q has no web binding and no exemption", name)
			continue
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("service operation %q is exempted without a justification", name)
		}
	}
}

func TestExemptionsNameRealOperations(t *testing.T) {
	t.Parallel()
	iface := reflect.TypeOf((*core.Service)(nil)).Elem()
	known := map[string]bool{}
	for i := range iface.NumMethod() {
		known[iface.Method(i).Name] = true
	}
	for _, exemption := range web.Exemptions() {
		if !known[exemption.Service] {
			t.Errorf("exemption names %q, which the service does not declare", exemption.Service)
		}
		if strings.TrimSpace(exemption.Reason) == "" {
			t.Errorf("exemption for %q carries no justification", exemption.Service)
		}
	}
}

func TestEveryBindingResolvesAgainstTheServedMux(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	handler := web.Handler(f.svc)
	for _, binding := range web.Bindings() {
		path := concretePath(binding.Pattern)
		req := httptest.NewRequest(binding.Method, path, nil)
		matched, pattern := patternFor(handler, req)
		if !matched {
			t.Errorf("binding %s %s resolves to no route", binding.Method, binding.Pattern)
			continue
		}
		if !strings.HasSuffix(pattern, binding.Pattern) {
			t.Errorf("binding %s %s resolved to %q", binding.Method, binding.Pattern, pattern)
		}
	}
}

// patternFor reports which registered pattern serves a request.
func patternFor(handler http.Handler, req *http.Request) (bool, string) {
	mux, ok := handler.(interface {
		Handler(*http.Request) (http.Handler, string)
	})
	if ok {
		_, pattern := mux.Handler(req)
		return pattern != "", pattern
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder.Code != http.StatusNotFound, req.Pattern
}

// concretePath replaces a pattern's wildcards with usable values.
func concretePath(pattern string) string {
	path := strings.ReplaceAll(pattern, "{key}", "infra")
	return strings.ReplaceAll(path, "{ref}", "infra-1")
}

func TestEveryBindingNamesAnEmbeddedTemplate(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for _, binding := range web.Bindings() {
		if binding.Template == "" {
			continue
		}
		if !web.HasTemplate(binding.Template) {
			t.Errorf("binding %s %s names missing template %q",
				binding.Method, binding.Pattern, binding.Template)
		}
	}
	if web.HasTemplate("absent.html") {
		t.Fatalf("a template that is not embedded reported as present")
	}
	if page := b.page("/projects"); page == "" {
		t.Fatalf("the embedded templates render nothing")
	}
}

func TestBindingsAreWellFormed(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, binding := range web.Bindings() {
		key := binding.Method + " " + binding.Pattern
		if seen[key] {
			t.Errorf("binding %q is declared twice", key)
		}
		seen[key] = true
		if !strings.HasPrefix(binding.Pattern, "/") {
			t.Errorf("binding %q has no absolute pattern", key)
		}
		if binding.Method == http.MethodGet && binding.Pattern != web.RouteRoot &&
			binding.Template == "" {
			t.Errorf("screen %q names no template", key)
		}
	}
}
