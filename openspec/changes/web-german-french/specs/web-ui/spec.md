## MODIFIED Requirements

### Requirement: Screens are drawn in the language the browser asks for

The browser interface SHALL negotiate a language from the request's `Accept-Language` header, choosing
the highest-weighted language it ships, matching an exact tag before its base language, and SHALL draw
English when the header names none it ships. Text a language's catalog lacks SHALL be drawn in
English. The interface SHALL ship English, Bulgarian, German and French.

#### Scenario: A language the build does not ship

- **WHEN** a browser sends `Accept-Language: es-ES,it;q=0.8`
- **THEN** the screen is drawn in English with `lang="en"`

#### Scenario: A German browser

- **WHEN** a browser sends `Accept-Language: de-DE,de;q=0.9,en;q=0.5` and opens the task list
- **THEN** the root says `lang="de"` and the navigation reads Aufgaben

## ADDED Requirements

### Requirement: German and French may trail Bulgarian by a bounded share

The Bulgarian catalog SHALL hold every key the interface asks for. The German and French catalogs SHALL
each hold at least 95% of those keys, and a key either lacks SHALL be drawn in English.

#### Scenario: A new string not yet translated to French

- **WHEN** a new screen adds a string with a Bulgarian entry and no French one
- **THEN** a French reader sees that string in English and the rest of the screen in French
