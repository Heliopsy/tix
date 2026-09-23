// SPDX-License-Identifier: AGPL-3.0-or-later

package integration

import (
	"sort"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/capability"
)

// matrixExemptions records the core.Service methods the transport-equivalence
// table does not exercise, each with the reason it does not. An operation is
// either driven by a scenario or named here; there is no third state, so the
// gap between the contract and the table is visible in source rather than
// inferred by counting.
//
// "Not written yet" is a legitimate entry. An entry that says nothing is not.
var matrixExemptions = map[string]string{
	"Close": "releases the transport's own resources rather than performing a " +
		"tenant operation: service.Local closes a database pool and client.Client " +
		"closes idle sockets and live event streams, so the two are not comparable " +
		"outcomes and every scenario already exercises both through the harness cleanup",
}

// TestEveryOperationIsCoveredOrExempt is the guard the matrix lacked. The
// capability registry is the same list internal/capability/parity_test.go
// reflects over to require CLI, HTTP and Web bindings; requiring a
// transport-equivalence scenario from that list too is what stops a new
// operation from shipping with three bindings and no proof the two transports
// agree about it.
func TestEveryOperationIsCoveredOrExempt(t *testing.T) {
	t.Parallel()

	known := map[string]bool{}
	for _, op := range capability.Operations() {
		known[op.Method] = true
	}

	covered := map[string][]string{}
	exempted := map[string]string{}
	for method, reason := range matrixExemptions {
		exempted[method] = reason
	}

	for _, sc := range serviceScenarios {
		switch {
		case sc.exempt != "" && len(sc.covers) == 0:
			t.Errorf("scenario %q is exempt but names no operation, so the exemption "+
				"cannot be attributed to anything", sc.name)
		case sc.exempt != "":
			for _, m := range sc.covers {
				exempted[m] = sc.exempt
			}
		case len(sc.covers) == 0:
			t.Errorf("scenario %q names no operation in covers, so it cannot count "+
				"towards coverage", sc.name)
		default:
			for _, m := range sc.covers {
				covered[m] = append(covered[m], sc.name)
			}
		}
	}

	for method := range covered {
		if !known[method] {
			t.Errorf("a scenario covers %q, which the capability registry does not declare; "+
				"the name is stale or misspelt", method)
		}
	}
	for method, reason := range exempted {
		if !known[method] {
			t.Errorf("exemption names %q, which the capability registry does not declare; "+
				"the name is stale or misspelt", method)
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("%s: the matrix exemption carries no reason", method)
		}
		if scenarios, both := covered[method]; both {
			t.Errorf("%s: exempted from the matrix and also covered by %v; "+
				"delete the exemption", method, scenarios)
		}
	}

	var missing []string
	for _, op := range capability.Operations() {
		if len(covered[op.Method]) > 0 {
			continue
		}
		if _, ok := exempted[op.Method]; ok {
			continue
		}
		missing = append(missing, op.Method)
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("%d of %d operations are neither exercised by a transport-equivalence "+
			"scenario nor exempt: %s\n"+
			"Add a scenario to serviceScenarios in service_matrix_test.go naming the "+
			"operation in covers, or add it to matrixExemptions with the reason it "+
			"cannot be compared across transports.",
			len(missing), len(capability.Operations()), strings.Join(missing, ", "))
	}
}

// TestMatrixCoverageIsReported prints the standing of the table so a change to
// it is legible in the log rather than only in a diff.
func TestMatrixCoverageIsReported(t *testing.T) {
	t.Parallel()
	covered := map[string]bool{}
	for _, sc := range serviceScenarios {
		if sc.exempt != "" {
			continue
		}
		for _, m := range sc.covers {
			covered[m] = true
		}
	}
	total := len(capability.Operations())
	t.Logf("transport equivalence: %d scenarios cover %d of %d operations, %d exempt",
		len(serviceScenarios), len(covered), total, total-len(covered))
}
