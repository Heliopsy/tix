# Tasks

- [x] Move the release watch, status, comparison, link stripping, upgrade methods and changelog selection to `internal/release`, with their tests
- [x] Web reads the shared watch through `web.WithReleaseWatch`, keeping its own when none is given
- [x] `GET /api/v1/release` in `internal/httpapi`, a `capability.Endpoints` entry with the `WhoAmI` authority
- [x] `internal/serving` builds one watch and hands it to the web and the API
- [x] Handler tests: every status, notes and methods only when newer, the changelog selection, authentication
- [x] `just openapi`
- [x] docs/api.md, docs/web-ui.md
