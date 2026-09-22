// Package smoke drives the compiled tix binary end to end.
//
// It holds no production code and it is not a second test suite. Everything
// here runs the shipped artifact as a subprocess and asserts on what a person
// would actually see, because the bugs this suite exists for were invisible to
// the unit tests that covered the same code: a listener that refused the
// zero-configuration store after creating it, and a listing whose service was
// correct while its rendering printed "<nil>", raw Go timestamps and an
// eighty-character column.
//
// internal/integration is the neighbouring suite and a different thing: it
// assembles the server from Go and calls it in process, so it can never see a
// wiring mistake that lives in cmd/ or a rendering mistake that only the
// terminal shows. Smoke builds the binary instead.
//
// The tests carry the "smoke" build tag so `go test ./...` stays a unit gate.
// Run them with `just smoke`.
package smoke
