# Tasks

- [x] `/manifest.webmanifest` with tenant name, start_url, scope, standalone display, colours and icons
- [x] Launcher shortcuts Quick add, Inbox and My work, translated per request
- [x] Share target `GET /share` redirecting to the task list with the joined line in quick add
- [x] `/tasks?add=LINE` prefills, previews and focuses quick add; dropped from paging and the stay-after-add address
- [x] `/sw.js` service worker that caches nothing, `pwa.js` registering it
- [x] Manifest link and light and dark `theme-color` in the layout head
- [x] Tests: manifest content and type, share redirect and sign-in bounce, no task on GET, worker headers, worker caches nothing
- [x] docs/web-ui.md
