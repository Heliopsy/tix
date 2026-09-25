## ADDED Requirements

### Requirement: Display preferences in the terminal interface

The terminal interface SHALL offer a settings view carrying the display preferences a reader owns: the
keybinding scheme, the time format, the timezone and the colour mode. Each SHALL name the configuration
key it is written to, and SHALL show what choosing a value would mean, rendered by the renderer that will
render it, so an example can never claim a layout or a zone the interface does not produce.

A value the build does not ship, arriving from a configuration file written elsewhere, SHALL remain on
offer rather than being dropped, so stepping through the values cannot silently discard it.

#### Scenario: Every preference names its key

- **WHEN** the settings view is open
- **THEN** each preference row names the configuration key its value is written to

#### Scenario: An example is rendered in the chosen zone

- **WHEN** the timezone row is stepped from one zone to another
- **THEN** the example beside it renders the same instant in the newly chosen zone

#### Scenario: An example is rendered in the chosen layout

- **WHEN** the time format row is stepped from one layout to another
- **THEN** the example beside it renders in the newly chosen layout

#### Scenario: Stepping wraps rather than stopping

- **WHEN** a preference is stepped past either end of its values
- **THEN** it continues from the other end

#### Scenario: A configured value this build does not ship is kept

- **WHEN** a preference holds a value that is not one this build offers
- **THEN** that value is still among the values the row steps through

### Requirement: A display preference chosen in the terminal interface persists

A preference changed in the settings view SHALL take effect in the frame it is read in and SHALL be
written to the reader's configuration file in the same act, with no separate save. The interface SHALL
state where the change was written.

The write SHALL edit the configuration file in place, carrying forward the keys it already held and
writing only the keys that changed, so choosing a display preference never pins a value another layer was
supplying.

A session with no configuration file to write SHALL say that its choices last only for the session,
rather than offering a choice it cannot keep. A write that fails SHALL be reported, naming the failure.

#### Scenario: A change is written down at once

- **WHEN** a preference is stepped to a new value
- **THEN** the interface adopts it, writes it to the configuration file, and says which file it wrote

#### Scenario: The choice survives a restart

- **WHEN** the interface is closed and opened again
- **THEN** the preference chosen in the previous run is the one in force

#### Scenario: Writing one preference pins nothing else

- **WHEN** a display preference is written to a configuration file
- **THEN** no key that the file did not already carry, and that the change did not alter, is written to it

#### Scenario: A session that cannot write says so

- **WHEN** the session was opened with no way to write a configuration file
- **THEN** the screen states that the choices last until the interface is quit, and a change says the same

#### Scenario: A failed write is reported

- **WHEN** writing the configuration file fails
- **THEN** the interface says the change was not saved and names the failure

#### Scenario: A value the build cannot render is refused

- **WHEN** a preference would be set to a value this build cannot resolve
- **THEN** the value is not adopted and the refusal names it

### Requirement: The settings view states which layer supplies each preference

Where a preference's value arrives from a configuration layer above the file, the settings view SHALL say
so on that preference's own row, and SHALL state that the layer still decides after a restart. A value
arriving from the file or from the built-in defaults SHALL carry no such warning.

#### Scenario: An environment variable is announced

- **WHEN** a preference's value is supplied by an environment variable
- **THEN** that preference's row says the environment layer supplies it and still wins after a restart

#### Scenario: A file value carries no warning

- **WHEN** a preference's value comes from the configuration file or from the defaults
- **THEN** that preference's row carries no warning about another layer

### Requirement: The settings view states what the session is connected to

The settings view SHALL state the target this run resolved, the tenant, the actor, the build and the
configuration file. The target SHALL be stated with its secrets redacted, so a connection string carrying
a password is never drawn on a screen. A fact the session was not told SHALL say so rather than rendering
blank, which reads as a value that failed to load.

The tenant and the actor SHALL be the ones in force, so a session that has switched tenant reports the
tenant it switched to.

The view SHALL name the command that answers the same question across every configuration key, and SHALL
NOT offer to change the target, the credentials or any other deployment setting: a session is already
connected through the target it resolved and could not act on a new one.

#### Scenario: The connection is named

- **WHEN** the settings view is open
- **THEN** it states the target, the tenant, the actor, the build and the configuration file

#### Scenario: A secret in the target is redacted

- **WHEN** the resolved target is a connection string carrying a password
- **THEN** the password does not appear on the screen

#### Scenario: The tenant follows a switch

- **WHEN** the session has switched to another tenant
- **THEN** the settings view names the tenant the session switched to

#### Scenario: A fact the session was not told

- **WHEN** a fact was never supplied to the session
- **THEN** the row says so rather than rendering empty

#### Scenario: The rest of the configuration is pointed at, not duplicated

- **WHEN** the settings view is open
- **THEN** it names `tix config show --sources` and offers no control over the target or the credentials

### Requirement: The whole settings view is reachable on a short terminal

The settings view SHALL be navigable to its last line on a terminal too short to show it at once, and
SHALL never draw more lines than the frame has. The keys that step a preference's values SHALL be
described as doing that, rather than by the meaning they carry on the board.

#### Scenario: The bottom of the screen is reachable

- **WHEN** the terminal is too short to show the whole settings view and the reader walks down
- **THEN** the last line of the view comes into the window

#### Scenario: The body never overflows the frame

- **WHEN** the settings view is drawn into a frame shorter than the view
- **THEN** it draws no more lines than the frame holds and says how many are off screen

#### Scenario: The footer describes the settings view

- **WHEN** the settings view is open
- **THEN** its footer describes the value keys as stepping a setting's values, not as moving between columns
