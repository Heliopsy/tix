## ADDED Requirements

### Requirement: Project icons the terminal cannot draw

The terminal interface SHALL draw a built-in project icon as that symbol's fallback glyph and an
uploaded image as a fixed mark, wherever it draws a text icon, and the project view SHALL name the
kind of icon the project carries.

#### Scenario: Built-in icon in the project list

- **WHEN** a project whose icon is the built-in rocket is listed
- **THEN** its row starts with the rocket's fallback glyph

#### Scenario: Image in the project view

- **WHEN** the project view shows a project whose icon is an uploaded image
- **THEN** the icon line states that the icon is an uploaded image
