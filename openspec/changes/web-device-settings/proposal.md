# Settings search, where each setting is kept, start screen, reduce motion and browser notifications

## Why

The app's settings screen can be searched, labels every setting with where it is kept, chooses the screen
the app opens on, overrides the system's motion setting, and alerts the reader to new inbox items while
it is open. The web settings screen had none of these, and its lede still claimed every setting lived in
a cookie although half of them are kept with the account.

## What Changes

- A search box on Settings narrows the rows and sections to the words typed.
- Every setting carries On this device, Synced to your account or (on the Tenant screen) Whole tenant.
- Start screen (`tix_start`): where `/` and a sign-in without a link land: Projects, Tasks, My work or Inbox.
- Reduce motion (`tix_motion`): `data-motion="reduce"` on `<html>` turns off animation, transitions, page
  cross-fades and smooth scrolling as `prefers-reduced-motion` does.
- Browser notifications for new inbox items while a tab is open, opt-in from Settings with the browser's
  permission prompt, with a switch hiding what they say. Kept in browser storage.
