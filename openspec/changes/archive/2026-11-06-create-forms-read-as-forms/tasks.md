# Tasks

## 1. Refusals name their field

- [x] 1.1 `internal/core`: `OnField`; project and task input validation names its fields
- [x] 1.2 `internal/service`: task create and project create and update attribute their refusals

## 2. Add with details

- [x] 2.1 `internal/web/tasks.go`: due date, assignee, parent and dependencies are read; a refusal re-renders the list with the form in place
- [x] 2.2 `templates/tasks.html`: title first and focused, essentials beside it, More options, required mark, actions bottom right
- [x] 2.3 the filtered project is preselected in both add forms

## 3. Projects

- [x] 3.1 `templates/projects.html`: New project in the header, name first, key suggested, Appearance group
- [x] 3.2 `templates/board.html`: project settings in the same layout
- [x] 3.3 `internal/web/projects.go`: create and update refusals re-render in place

## 4. Shared

- [x] 4.1 `assets/forms.js`: Ctrl/Cmd+Enter submits, Escape cancels, key suggestion and preview
- [x] 4.2 `assets/app.css`: the form layout, two columns on a wide container
- [x] 4.3 `internal/i18n`: Bulgarian for every new string
