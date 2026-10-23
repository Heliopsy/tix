## ADDED Requirements

### Requirement: A live screen recovers its connection, and says when it has not

Every screen that updates from the event stream, the activity feed included,
SHALL share one connection mechanism.

A connection that drops SHALL be reopened, on a delay that grows with each
failed attempt, is bounded, and is jittered so that a server returning does
not meet every open browser at once. A successful connection SHALL reset the
delay. A hidden page SHALL NOT retry; returning to it SHALL reconnect at once
and re-read what it missed.

A resubscription SHALL carry the sequence of the last event the screen saw, so
the server replays the gap. Where that cursor is no longer retained, the
screen SHALL subscribe afresh and re-fetch its fragment whole rather than
leaving a gap nothing fills.

Where no connection can be established at all, the screen SHALL poll its
fragment on a slow interval, and SHALL stop polling as soon as a connection
succeeds.

While a screen is not receiving events it SHALL say so on screen, naming the
consequence: that what is shown may be out of date.

#### Scenario: A dropped connection is reopened

- **GIVEN** a screen receiving events
- **WHEN** the connection drops
- **THEN** it is reopened after a bounded, growing, jittered delay, and updates resume

#### Scenario: Nothing is lost across the gap

- **GIVEN** a screen that has seen events up to a known sequence
- **WHEN** it reconnects
- **THEN** it resubscribes from that sequence and the changes made while it was down are shown

#### Scenario: A cursor the server cannot honour

- **GIVEN** a screen resuming from a sequence that has been retained out of the log
- **WHEN** the server refuses the cursor
- **THEN** the screen subscribes from the present and re-fetches its fragment, rather than failing silently

#### Scenario: A sleeping laptop does not storm the server

- **GIVEN** a page hidden while its connection is down
- **WHEN** it is hidden
- **THEN** it does not retry, and on becoming visible it reconnects at once and re-reads

#### Scenario: No socket available

- **GIVEN** a reader behind an intermediary that will not carry a WebSocket
- **WHEN** the screen cannot open one
- **THEN** it polls its fragment on a slow interval and stops as soon as a socket succeeds

#### Scenario: The reader is told the view may be stale

- **WHEN** a screen is not receiving events
- **THEN** it shows a notice saying the view may be out of date, and removes it when the stream is live again

### Requirement: An update is drawn at once and a burst is grouped

The first event in a quiet period SHALL be rendered without waiting. Further
events arriving within the screen's coalescing window SHALL be grouped into
one further render, and SHALL NOT postpone it indefinitely: a bounded maximum
wait SHALL apply however long the stream keeps arriving.

#### Scenario: A single change is not delayed

- **WHEN** one event arrives on an idle screen
- **THEN** the screen re-renders immediately rather than after the coalescing window

#### Scenario: A multi-hop transition settles once

- **WHEN** a transition writes several hops milliseconds apart
- **THEN** the screen renders on the first and once more when they have all landed, not once per hop

#### Scenario: Continuous change still redraws

- **WHEN** events keep arriving faster than the coalescing window
- **THEN** the screen still redraws within the bounded maximum wait

## MODIFIED Requirements

### Requirement: Live activity feed

The web UI SHALL provide an activity feed showing recent events for the tenant, updating as new events arrive when JavaScript is available.

The feed SHALL use the same connection mechanism as every other live screen: the same reconnect, the same resume, the same fallback and the same notice when it is not live.

#### Scenario: Feed shows recent activity

- **WHEN** a user opens the activity feed
- **THEN** recent events for the tenant are listed in reverse chronological order

#### Scenario: Feed updates live

- **WHEN** another user changes a task while the feed is open with JavaScript enabled
- **THEN** the change appears in the feed without the user reloading the page

#### Scenario: Feed is readable without JavaScript

- **WHEN** the feed is opened with JavaScript disabled
- **THEN** recent events are still listed, refreshed on page load

#### Scenario: A feed that has stopped receiving says so

- **WHEN** the feed's connection drops and cannot immediately be reopened
- **THEN** the feed says the view may be out of date, and reconnects
