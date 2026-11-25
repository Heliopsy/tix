# Tasks

## Palette

- [x] `GET /palette/templates`, bound as a file on `ListTemplates`
- [x] Embedded colour schemes, in the switch's order, and task actions with the control each drives
- [x] palette.js: Task, Templates, Switch to and Colour scheme groups, and what choosing each does
- [x] A tenant-wide template's task goes in the project on screen, or opens the Use form without one

## Tests

- [x] Every task action names a control the task screen draws, on its page and in the dialog
- [x] A viewer is offered no action that changes the task; the listing draws no task header
- [x] Every colour scheme is embedded in order, and the switch carries the field the palette fills
- [x] `/palette/templates` names each template, its title and scope, and needs a session
- [x] The switcher dialog carries the form the palette sends for each tenant it can switch to
- [x] jstest: task actions only with a task on screen, templates, tenants and schemes once typed

## Surfaces

- [x] Bulgarian strings
- [x] `docs/web-ui.md`
