# Tasks

## 1. Task numbering

- [x] 1.1 Raise a project's counter when a task is written under a number of its own (SQLite, PostgreSQL)
- [x] 1.2 Reserve the next number above both the counter and the highest number held
- [x] 1.3 Repair migration for counters behind their tasks, run under forced RLS on PostgreSQL
- [x] 1.4 `work.numbering_behind` in the status report and a `task-numbering` doctor check
- [x] 1.5 Tests: import into a fresh tenant then create, counter raise, backstop, repair migration, doctor

## 2. Request log

- [x] 2.1 The request record carries the resolved tenant
- [x] 2.2 Test the record of an authenticated request
