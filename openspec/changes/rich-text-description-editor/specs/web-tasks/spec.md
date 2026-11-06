## ADDED Requirements

### Requirement: The description editor has Rich text and Markdown modes

With scripting, the description editor on the task screen and in the New task form SHALL offer Rich
text and Markdown modes. Markdown SHALL be the textarea with its Preview. Rich text SHALL edit the
same text formatted, with tools for bold, italic, strikethrough, headings, lists, checklists, quotes,
code, links and tables. Without scripting the textarea SHALL be all there is.

#### Scenario: Switching modes

- **GIVEN** a description with a table and a checklist
- **WHEN** a writer switches to Rich text and back to Markdown without editing
- **THEN** the textarea holds exactly the text it held before

### Requirement: Rich text submits Markdown

The textarea SHALL remain the form field in both modes, and Rich text SHALL write the Markdown it
serialises into it on every edit, so the form posts Markdown in the same field. Opening Rich text
without editing SHALL NOT change the field.

#### Scenario: Ticking a box in Rich text

- **WHEN** a writer ticks the second checklist item in Rich text and saves
- **THEN** the stored description has `- [x]` on that item and is otherwise the same text

### Requirement: A text Rich text would change stays in Markdown

Before opening a text, Rich text SHALL parse it, serialise it and compare the renderings. A text whose
rendering would change, or which holds a single-tilde strikethrough, SHALL stay in Markdown with a
notice saying why.

#### Scenario: A list item that opens with a code block

- **WHEN** a writer switches a description whose list item opens with a fenced code block to Rich text
- **THEN** the textarea stays, unchanged, with a notice that the text stays in Markdown

### Requirement: The chosen mode is remembered per browser

The chosen mode SHALL be kept in the browser's local storage and used for every description editor
the browser opens next. Markdown SHALL be the mode when nothing was chosen or storage is refused.

#### Scenario: Storage is refused

- **GIVEN** a browser that refuses local storage
- **WHEN** a task screen opens
- **THEN** the description editor is in Markdown mode

### Requirement: Rich text loads nothing a description names

Rich text SHALL NOT load an image a description names, and SHALL give a link an address only when it
is http, https or mailto. Raw HTML SHALL be shown as typed.

#### Scenario: An image in Rich text

- **WHEN** a description holding `![x](https://example.invalid/x.png)` opens in Rich text
- **THEN** no request is made for the image and its alt text is shown in its place

### Requirement: Mentions complete in Rich text

Typing `@` and part of a handle in Rich text SHALL offer the same completion list as the textarea,
chosen with the arrows and Enter or Tab and closed with Escape.

#### Scenario: Completing a handle

- **WHEN** a writer types `@to` in Rich text and presses Enter
- **THEN** the text reads `@tom` followed by a space, and the list closes
