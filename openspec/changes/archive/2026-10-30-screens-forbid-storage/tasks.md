# Tasks

## 1. The header reaches every screen

- [x] 1.1 `internal/web/render.go`: `screenCacheControl` constant, documented with both faults it
      closes and the back-forward cache it costs, set in `renderStatus` so every screen and every
      status code goes through it
- [x] 1.2 `internal/web/cacheheaders_test.go`: `TestEveryScreenForbidsStorage` lists the screens
      rather than discovering them, so a new screen without the header fails here;
      `TestARefusedScreenAlsoForbidsStorage` covers the non-200 render
- [x] 1.3 Assets left alone: `TestEveryAssetCarriesACacheValidator` still pins their ETag and
      `max-age`, which is the opposite rule and must not drift into this one
