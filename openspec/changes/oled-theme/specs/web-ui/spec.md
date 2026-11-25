## ADDED Requirements

### Requirement: An OLED colour scheme is offered

The sidebar's colour scheme switch SHALL offer an OLED scheme whose page background is pure black and
whose surfaces are near black, so a raised element is told apart by its border rather than a grey fill.
It SHALL be recorded in the same cookie as the other schemes and rendered as the root element's
`data-theme`.

#### Scenario: Choosing OLED

- **WHEN** a reader presses the switch until it names OLED
- **THEN** the cookie records `oled`, the root element carries `data-theme="oled"`, and the page
  background is `#000`

### Requirement: System can follow the device into OLED

The switch SHALL also offer System, OLED when dark, which draws the light scheme while the device asks
for light and the OLED scheme while it asks for dark. Plain System SHALL remain the default and SHALL
keep mapping to the light and dark schemes.

#### Scenario: A dark device with System, OLED when dark

- **GIVEN** a browser that chose System, OLED when dark
- **WHEN** its device asks for a dark scheme
- **THEN** the page is drawn in the OLED scheme

#### Scenario: A light device with System, OLED when dark

- **GIVEN** a browser that chose System, OLED when dark
- **WHEN** its device asks for a light scheme
- **THEN** the page is drawn in the light scheme

### Requirement: The OLED scheme keeps every dark token and the contrast floor

The OLED scheme SHALL define a value for every colour token the dark scheme defines. Its text tokens,
state colours and accents SHALL clear 4.5:1 on its page and surface, the text on its soft tints SHALL
clear 4.5:1, and its strong line SHALL clear 3:1 on its page and surface.

#### Scenario: A token added to dark

- **WHEN** a colour token is added to the dark scheme and not to OLED
- **THEN** the stylesheet test fails

### Requirement: Colour-coded elements stay legible under OLED

A tenant's dark accent, the project palette and a custom project colour's shade SHALL apply under the
OLED scheme, each clearing 4.5:1 on its page and surface as under the other schemes.

#### Scenario: A custom project colour under OLED

- **WHEN** a project with a custom colour is drawn under the OLED scheme
- **THEN** it is drawn in a shade that clears 4.5:1 on the OLED page and surface
