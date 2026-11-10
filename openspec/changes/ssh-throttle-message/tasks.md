# Tasks

- [x] `limiter.take` returns the wait until the next token
- [x] Record the throttle decision per connection and refuse the session with the wait
- [x] Tests: the wait, the message, a throttled connection authenticates and is told why with a nonzero exit
- [x] Rate limit tests in source and proxy read the refusal from the session
- [x] docs/deployment.md: the throttle, its defaults and what a client sees
- [x] docs/tenancy.md: a global admin is not a tenant member; a tenant with no members
