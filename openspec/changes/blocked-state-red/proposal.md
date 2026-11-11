# Blocked reads red in the browser

## Why

The terminal and the TUI draw the blocked category in red. The web interface drew it amber, the same
hue as a soon-due date and an expiring lease, so a blocked task on the board looked like a warning
rather than a stop. The board column heading carried no state colour at all.

## What Changes

- The blocked category's chip, badge, status tint and statistics colour use the danger red.
- A board column heading is written in its state category's colour.
- No data change. States carry a category, not a colour, so every workflow whose blocked state has
  `category: blocked`, the default one and the built-in templates included, turns red on upgrade.
