## ADDED Requirements

### Requirement: The selected row pulses

The terminal interface SHALL animate the selected row by alternating the emphasis it is drawn with, rather
than by using the terminal's blink attribute, so that every terminal able to draw colour draws the same
animation. No other element of the interface SHALL animate.

The selection SHALL remain unambiguous at every point in the cycle: the selection marker and the
selection's colour SHALL be drawn in every frame, and only the emphasis SHALL vary. A reader looking at
the screen at any moment SHALL be able to tell which row is selected.

#### Scenario: The selected row changes emphasis

- **WHEN** consecutive phases of the pulse are drawn
- **THEN** the selected row is rendered differently in each

#### Scenario: The marker survives both phases

- **WHEN** the selected row is drawn at either phase of the pulse
- **THEN** it carries the selection marker and is rendered differently from an unselected row

#### Scenario: Nothing else moves

- **WHEN** the interface is idle at the keyboard but connected
- **THEN** only the selected row's emphasis changes, and no other element animates

### Requirement: An idle session animates nothing

The interface SHALL stop the pulse after a bounded period with no input, and SHALL produce no further
frames until input arrives. It SHALL start the pulse again on the next keystroke.

A stopped pulse SHALL rest on the emphasised phase, which is the same rendering a session with motion
turned off draws, so a session that has gone quiet cannot be mistaken for one that has stopped responding.

#### Scenario: The frames stop

- **WHEN** the idle period passes with no keystroke
- **THEN** the interface produces no further frames until a key arrives

#### Scenario: A phase arriving after the pause changes nothing

- **WHEN** a phase of the pulse is delivered after the idle pause has taken effect
- **THEN** the frame is unchanged and no further phase is scheduled

#### Scenario: A keystroke starts it again

- **WHEN** a key arrives after the idle pause has stopped the pulse
- **THEN** the pulse resumes

### Requirement: A terminal that cannot show the pulse gets a static selection

The interface SHALL animate only when it is drawing in colour. A session whose colour is suppressed, by
`NO_COLOR`, by `TIX_NO_COLOR`, by `output.color = never` or by a destination that is not a terminal, SHALL
draw a static selection and SHALL produce no animation frames at all.

#### Scenario: A colourless session never pulses

- **WHEN** a session is drawing without colour
- **THEN** no phase of the pulse is ever scheduled and the selected row is drawn the same in every frame

### Requirement: Motion is a display preference on the settings screen

The settings screen SHALL offer motion as a display preference alongside the keybinding scheme, the time
format, the timezone and the colour mode. It SHALL be on unless it is turned off, including in a session
given no preferences at all.

The row SHALL name the configuration key it is written to, SHALL say what the chosen value means on this
terminal, SHALL apply the change to the frame it is read in, SHALL write the change down at once, and SHALL
report the configuration layer supplying the value when a layer above the file supplies it. A session with
nowhere to write SHALL keep the row usable and say the choice lasts only for the session.

#### Scenario: Motion is on by default

- **WHEN** a session is opened with nothing configured
- **THEN** the selected row pulses

#### Scenario: The row is written down when it is stepped

- **WHEN** the motion row is stepped to another value
- **THEN** the interface adopts it for the open frame and writes it to the configuration file

#### Scenario: Turning motion off stops the animation

- **WHEN** the motion row is stepped to off
- **THEN** no further phase of the pulse is scheduled

#### Scenario: The row says what it means on this terminal

- **WHEN** the motion row is read on a session that is drawing without colour
- **THEN** it says that nothing will pulse, rather than that motion is running
