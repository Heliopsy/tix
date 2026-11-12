# German and French in the browser

## Why

The tix-mobile app ships English, Bulgarian, German and French. The browser interface ships English and
Bulgarian only, so a German or French reader gets English on the web and their own language in the app,
with two different sets of words for the same things.

## What Changes

- German (`de`) and French (`fr`) catalogs under `internal/i18n`, one file per area beside the Bulgarian
  ones, worded after the app's `app_de.arb` and `app_fr.arb` for the terms the two share.
- `Accept-Language` negotiation picks them, and the settings Language row lists Deutsch and Français.
- Bulgarian stays a complete catalog. German and French may trail new strings, which read in English,
  but must hold at least 95% of the keys, so a new screen does not wait on three translations and a
  catalog cannot quietly rot.
- No service, store or API change.
