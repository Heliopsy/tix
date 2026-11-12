# Design

- The manifest is rendered per request, not shipped as a static asset, because its name is the tenant's and
  its shortcut labels are in the reader's language. The link carries `crossorigin="use-credentials"` so the
  browser sends the session and language cookies with it; without them a manifest fetch is anonymous and
  says tix in the browser's language. It is sent `application/manifest+json` with `Cache-Control: no-cache`
  and is public: nothing in it is secret beyond the tenant's name, which the sign-in screen does not show.
- Icons: `tix.svg` with `sizes: any` and purpose `any`, and `tix-maskable.svg`, the same mark on a full-bleed
  square whose drawing sits inside the maskable safe zone, with purpose `maskable`. No PNGs: rasterising the
  SVG reproducibly would need a new dependency or a checked-in binary nobody can regenerate. Chromium and
  Safari accept an SVG icon for installation.
- The service worker caches nothing. Every page but the sign-in screen is authenticated, and a cached page or
  response is one more place a tenant's data outlives a sign-out on a shared machine. Caching only `/assets`
  would buy nothing offline without a cached page to use them, and the assets already revalidate cheaply by
  ETag. Its fetch handler returns without `respondWith`, so the browser fetches every request itself. A
  static test refuses a worker source that touches the Cache API, IndexedDB or `respondWith`.
- `/sw.js` is served from the root so its default scope is `/` without a `Service-Worker-Allowed` header, as
  `text/javascript` with `Cache-Control: no-cache`, so a changed worker is picked up on the next load.
- The share target is a GET that only redirects, so a crawler or a prefetch creates nothing. Its line is
  joined the way the app's `sharedLine` joins a share: the title leads unless the text already starts with
  it, and the URL follows unless the text already holds it. Newlines become spaces, since the box is one line.
  The redirect target, `/tasks?add=LINE`, is a GET too; the add is the ordinary quick add POST.
- `add` is dropped from the request before the task list is built, so the pager, the filter links and the
  live refresh do not carry it, and from the address a stay-after-add returns to, so the line is not put
  back in the box after it was added.
- The page has no content security policy header today, so there is no `worker-src` or `manifest-src` to
  open.
