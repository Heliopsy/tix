# Tasks

- [x] Take the tenant's live cursor in `pumps.start` before it returns, and pass it to the tailer as the
      subscription filter's `since_seq`.
- [x] Fail the start, releasing the tenant slot, when the cursor cannot be read.
- [x] Add `TestPumpTakesItsLiveCursorBeforeStartReturns`, driving a real WebSocket subscriber and asserting
      an event committed once the connection is registered is delivered.
- [x] Record the connect-path serialisation cost where the cursor is taken.
- [x] Correct the SQLite tailer wake-up mechanism in the tix-v1 design note.
