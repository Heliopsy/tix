# Tasks

- [x] Record a 5xx written after the client left, or no answer at all, as 499 in `logRequest`
- [x] Keep the server timeout at 504 and a cancellation the client did not cause at 500
- [x] Skip the error log and the error page in the browser interface for a browser that left
- [x] Tests: abandoned long poll logs 499 at info, service failures stay 500, timeout stays 504, a dropped
  WebSocket logs no error
- [x] Document the request line and 499 in `docs/deployment.md` and `docs/api.md`
