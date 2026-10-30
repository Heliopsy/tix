# Tasks

## 1. Heading rhythm

- [x] 1.1 `internal/web/assets/app.css`: `--head-gap` and `--head-sub`; h1, lede, standfirst, header
      strip and task title bar written in them; the heading block owns the gap under it
- [x] 1.2 `internal/web/assets/app.css`: header controls centred on the heading's line at desktop width

## 2. One measure

- [x] 2.1 `internal/web/assets/app.css`: one `--measure-page`, the per-screen `:has()` measures removed

## 3. Task screen

- [x] 3.1 `internal/web/assets/app.css`: quick-add and filter controls at `--control-h`, Clear and the
      pager step on the control tokens, tighter rows, cards and toolbar spacing
- [x] 3.2 `internal/web/templates/tasks.html`, `internal/web/assets/icons.svg`: Add with details as a
      secondary button in the quick-add row opening a popover

## 4. Guards

- [x] 4.1 `internal/web/rhythm_test.go`: heading margins only in the rhythm tokens, the heading owns
      the gap, header controls centred; control rows take their height from the control rules; Add
      with details in the row; one page measure
