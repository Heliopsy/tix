## ADDED Requirements

### Requirement: The position control says how many pages there are

The shared position control SHALL state the page on screen, how many pages the listing has and how
many items it holds, as "Page N of M · T items" in the listing's own noun. When the count was capped
it SHALL write the page count and the total as a floor, such as "400+".

#### Scenario: A listing of three pages

- **GIVEN** a listing holding 60 items shown 25 to a page
- **WHEN** a reader opens its second page
- **THEN** the control reads "Page 2 of 3 · 60" followed by the listing's noun

#### Scenario: A capped count

- **GIVEN** a listing holding more items than the count cap
- **WHEN** a reader opens it
- **THEN** the control writes the page count with a trailing plus sign

### Requirement: The position control links pages by number

The control SHALL draw a link to the first page, the last page when the count is exact, and the
pages beside the current one, with a gap marker for each run of pages it leaves out, plus First,
Previous, Next and Last. The current page SHALL be marked with `aria-current="page"` and not linked.

#### Scenario: A middle page of eighteen

- **GIVEN** a listing of eighteen pages
- **WHEN** a reader is on page 5
- **THEN** the numbered links read 1, a gap, 4, 5, 6, a gap and 18, and 5 is not a link

### Requirement: A reader can type the page to go to

The control SHALL carry a "Go to page" number field in a GET form whose hidden fields carry the
listing's own parameters, so it works without scripting and keeps the filter. A number past the last
page SHALL open the last page.

#### Scenario: Going to page 7 without scripting

- **GIVEN** a filtered listing of ten pages
- **WHEN** a reader without scripting types 7 and submits the form
- **THEN** the seventh page of the same filtered listing is drawn

#### Scenario: A page past the end

- **WHEN** a reader asks for page 99 of a listing of ten pages
- **THEN** the tenth page is drawn and the control says it is page 10

### Requirement: A numbered page is an address

The control's links SHALL address a page by `page=N` and no cursor. A link that carries a cursor SHALL
still open the page that cursor names.

#### Scenario: A page link is shareable

- **WHEN** a reader follows the link to page 3
- **THEN** the address carries `page=3` and no cursor, and opening it elsewhere draws the same page

#### Scenario: An old cursor link

- **WHEN** a reader opens a link carrying a cursor issued before numbered pages
- **THEN** the page that cursor names is drawn
