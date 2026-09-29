## ADDED Requirements

### Requirement: Reachability is one answer the whole product shares

The domain SHALL provide, as a pure function of a workflow definition and a starting state, the routes
that reach every other state: the adjacent ones as a single hop and the ones reachable only through
another state as the sequence of states they pass through.

The walk SHALL be breadth first, so an adjacent state is never offered as a detour, and SHALL terminate
on a workflow containing cycles. It SHALL drop an edge naming a state the definition does not declare,
and SHALL bound both the length of a route and the number of equally short routes it enumerates to one
state.

No surface SHALL compute reachability for itself.

#### Scenario: A state reachable only through another is offered as a route

- **WHEN** the workflow reaches a state only by passing through another
- **THEN** that state is offered, named with every state the route passes through and the number of
  moves it takes

#### Scenario: An adjacent state is offered as one hop

- **WHEN** a state is reachable directly
- **THEN** it is offered as a single hop and never as a longer route to the same place

#### Scenario: A cycle does not walk forever

- **WHEN** the workflow contains a cycle, such as todo to doing and doing back to todo
- **THEN** the walk terminates and offers no state twice by the same route

### Requirement: A route is applied one ordinary transition per hop

The service SHALL apply a route by performing each hop as an ordinary transition, in order. Each hop
SHALL write its own domain rows, its own audit entry and its own outbox event in one transaction, exactly
as making those moves separately would.

A route of N hops SHALL write exactly N transition audit entries and emit exactly N transition events, in
the order the hops were applied, each naming the state it left and the state it entered. It SHALL write
nothing for a hop that did not happen.

#### Scenario: Three hops are three entries

- **WHEN** a route through three states is applied
- **THEN** the trail holds exactly three transition entries, in order, each naming its own from and to
- **AND** exactly three transition events are emitted, naming the same states in the same order

#### Scenario: One hop is one entry

- **WHEN** a route of a single state is applied
- **THEN** the trail holds exactly one transition entry, and no more

#### Scenario: A subscriber sees the journey

- **WHEN** a route through another state is applied and a subscriber is watching the stream
- **THEN** the subscriber sees one event per hop, and never a single event jumping to the last state

### Requirement: A route that stops part way reports where it stopped

When a hop after the first is refused, the service SHALL report a result rather than an error. The result
SHALL name the hops that were applied, the state it could not reach and why, and SHALL leave the task in
the state the last successful hop reached.

Exactly the hops that happened SHALL be written to the trail and the event stream, and none of the hops
that did not.

A route refused on its first hop SHALL be an error, because nothing happened.

#### Scenario: The middle hop is refused

- **WHEN** the second of three hops is refused by the workflow
- **THEN** the caller is told the task moved to the first state and then stopped, naming the state it
  could not reach
- **AND** the trail and the event stream hold exactly one entry and one event, for the hop that happened

#### Scenario: The first hop is refused

- **WHEN** the first hop of a route is refused
- **THEN** the call fails, and nothing is written to the trail or the event stream

### Requirement: An ambiguous destination is refused rather than decided

When a caller names a destination rather than a route, and two or more routes of the same length reach
it, the service SHALL refuse. It SHALL NOT choose one, including by any tie-break of its own.

The refusal SHALL name at least two of the competing routes and SHALL say that the caller is to name the
route they want.

A caller that names the route explicitly SHALL be served.

#### Scenario: Two equal routes are refused

- **WHEN** a destination is reachable by two routes of the same length and only the destination is named
- **THEN** the call is refused as invalid, the refusal names both routes, and nothing is written

#### Scenario: Naming the route resolves it

- **WHEN** the caller names one of those routes explicitly
- **THEN** the route is applied, hop by hop, and the trail records the states that route passed through

### Requirement: Every surface offers routes

The terminal interface, the command line, the HTTP API and the browser SHALL each offer moves to states
reachable through another state, not only to adjacent ones.

The terminal's transition picker SHALL list the adjacent states before the routes, so a single hop costs
the same one keystroke it costs today, and SHALL name the states a route passes through and how many
moves it is before it is applied.

The command line SHALL offer a switch on the move command that finds the route, prints it before acting,
and honours the existing dry-run switch by printing the route and writing nothing. Its machine-readable
output SHALL report the route found and the hops applied.

The HTTP API SHALL accept a route additively, leaving the single-state form working unchanged, and SHALL
use the existing error envelope.

The browser's task detail screen and its board SHALL offer what its task list already offers. Dragging a
card between columns SHALL remain a single hop, because a drop names a destination and no journey.

#### Scenario: The terminal offers a route

- **WHEN** the transition picker is opened on a task whose workflow reaches a state only through another
- **THEN** that state appears in the picker, named with the states it passes through and its hop count
- **AND** the adjacent states are numbered first, so a direct move is still one digit

#### Scenario: The command line prints the route before acting

- **WHEN** the move command is asked for a status reachable only through another state, with the switch
- **THEN** the route is printed, and then applied one transition per hop

#### Scenario: A dry run writes nothing

- **WHEN** the same command is given the dry-run switch
- **THEN** the route is printed and reported as planned, and the task's state and trail are unchanged

#### Scenario: The single-state API form is unchanged

- **WHEN** a client posts a single target status to the existing transition endpoint
- **THEN** it behaves exactly as before, with no route semantics applied

### Requirement: Multi-hop is an operation the capability registry names

The capability registry SHALL declare applying a route as an operation in its own right, with its own
bindings on the command line, the HTTP API, the browser and the terminal.

It SHALL NOT be recorded only as a capability of the single transition operation, because the registry
asserts that bindings exist and cannot see a difference in what bound surfaces offer.

#### Scenario: A surface that does not offer routes fails the build

- **WHEN** a surface binds no route operation and records no exemption for it
- **THEN** the parity tests fail
