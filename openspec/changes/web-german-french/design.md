# Design

## Catalogs

Each `bg_<area>.go` gets a `de_<area>.go` and `fr_<area>.go` registering the same keys. Values translate the
English key. Terms the app also shows (task, project, inbox, snooze, tenant, token, workflow) use the app's
`.arb` wording, so a reader sees one vocabulary on both.

## Complete and partial languages

`i18n.Complete` names the languages whose catalog must hold every key; today that is Bulgarian, which every
feature already translates. `i18n.Coverage` (0.95) is the share of keys any other shipped language must
hold. `TestEveryLanguageCoversEveryKey` applies both, and still fails on an entry nothing asks for and on a
translation that drops a verb or a tag.

The trade-off: a feature PR keeps translating to Bulgarian only, and German or French text it adds reads in
English until someone translates it. The floor is what turns a slow drift into a test failure.
