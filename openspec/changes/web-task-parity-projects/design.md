# Design

One `ListTasks` call with `Tally` set and the tenant's unfinished statuses as the filter, page size 1.
The tally already counts the selection per project and status, which is what the task list's heading
uses, so the screen adds one query whatever the number of projects. A failed read draws no counts
rather than failing the screen: the count is a courtesy beside the name.

The count is quiet text beside the key, inside the row's link, not a control of its own.
