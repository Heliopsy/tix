# Tasks

## 1. The sign-in screen is composed

- [x] 1.1 `internal/web/templates/login.html`: a brand lockup above the card, built from the shipped
      `assets/tix.svg` and `.Brand.Title` rather than from a new mark, and the card carries an
      `authcard` class so the new rules reach it and nothing else
- [x] 1.2 `internal/web/assets/app.css`: one block appended at the end of the sheet behind a
      `/* ===== Sign-in screen redesign` comment, holding the column, the lockup, the card, the
      glow, the field labels, the submit and the secondary action
- [x] 1.3 Every rule scoped to `.authcard` or `.auth:has(.authcard)`, so `error.html`, which shares
      the shell-less `.auth` layout and carries no card, keeps its own measure and heading size

## 2. The glow resolves the tenant accent in every scheme

- [x] 2.1 A 749px circle centred on the card, `radial-gradient` in `color-mix(... var(--accent) ...)`,
      softened by its stops rather than by `filter: blur()`, painted at `z-index: -1`
- [x] 2.2 Lower alphas for `[data-theme="dark"]`, `[data-theme="dim"]` and the
      `prefers-color-scheme: dark` mirror, because those schemes take the bright end of the ramp
- [x] 2.3 `--auth-glow: min(749px, calc(100vw - var(--gutter) * 2))`, because a 749px absolutely
      positioned child adds scrollable width that `body { overflow-x: hidden }` does not clip:
      measured at 390px, the page scrolled 180px sideways before the cap

## 3. The guards read the rules they name

- [x] 3.1 `internal/web/loginlook_test.go`: `TestSignInCardCarriesTheAccentGlow` reads the
      `.auth .authcard::before` rule by name and checks the card carries the class the rule needs;
      `TestSignInSubmitIsPaintedInTokens` reads the submit rule;
      `TestSignInRedesignHardCodesNoColour` reads the delimited block and fails on a hex literal
- [x] 3.2 Each guard watched fail against a real mutation and restored byte-exact: the `authcard`
      class removed from the template, `var(--accent)` replaced with a literal in both glow stops,
      and `background: var(--ink)` swapped for `var(--accent)` on the submit

## 4. Verified in the browser, not in the stylesheet

- [x] 4.1 A binary built and served against a scratch database, screenshotted at 1280px in `light`,
      `dark` and `dim`, and at 390px with the disclosure open
- [x] 4.2 A non-default accent confirmed to reach the glow
- [x] 4.3 Sign-in completed with no scripting at all, by posting the form with the CSRF field and
      following the redirect
