// Package integration exercises tix across transports. It holds no production
// code: the tests here assemble the real server, store and service so that a
// regression turning the durable outbox into an in-memory bus fails loudly.
package integration
