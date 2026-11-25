#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
"""One pass over the tix HTTP API, standard library only. See docs/examples.md."""

import json
import os
import time
import urllib.error
import urllib.parse
import urllib.request

# region setup
API = os.environ["TIX_URL"].rstrip("/") + "/api/v1"
TOKEN = os.environ["TIX_TOKEN"]


class APIError(Exception):
    def __init__(self, status, error):
        super().__init__(f"{status} {error.get('code')}: {error.get('message')}")
        self.status = status
        self.code = error.get("code")
        self.details = error.get("details") or {}


def call(method, path, body=None, query=None):
    url = API + path
    if query:
        url += "?" + urllib.parse.urlencode(query, doseq=True)
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Authorization", "Bearer " + TOKEN)
    if data is not None:
        req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req) as resp:
            return json.load(resp)
    except urllib.error.HTTPError as err:
        try:
            error = json.load(err)["error"]
        except (ValueError, KeyError):
            error = {"code": "http", "message": err.reason}
        raise APIError(err.code, error) from None
# endregion

# region whoami
me = call("GET", "/whoami")
print(f"signed in as {me['handle']} ({me['kind']})")
# endregion

# region cursor
cursor = call("GET", "/events/poll")["next_seq"]
# endregion

# region create-project
key = os.environ.get("TIX_PROJECT") or f"ex{int(time.time())}"
project = call("POST", "/projects", {"key": key, "name": "API examples"})
print(f"created project {project['key']}")
# endregion

# region create-task
task = call("POST", "/tasks", {"project_ref": key, "title": "Try the API"})
ref, created_version = task["ref"], task["version"]
print(f"created {ref} at version {created_version}")
# endregion

# region update-task
task = call("PATCH", f"/tasks/{ref}", {
    "title": "Try the tix API",
    "priority": 1,
    "due_at": "2030-01-15T17:00:00Z",
    "version": task["version"],
})
print(f"updated {ref} to version {task['version']}")
# endregion

# region conflict
change = {"title": "Try the tix HTTP API", "version": created_version}
try:
    task = call("PATCH", f"/tasks/{ref}", change)
except APIError as err:
    if err.status != 409 or err.code != "conflict":
        raise
    print(f"conflict: {ref} is at version {err.details.get('current_version')}, retrying")
    change["version"] = call("GET", f"/tasks/{ref}")["version"]
    task = call("PATCH", f"/tasks/{ref}", change)
print(f"retitled {ref} at version {task['version']}")
# endregion

# region transition
task = call("POST", f"/tasks/{ref}/transition", {"to": "doing", "version": task["version"]})
print(f"{ref} is now {task['status']}")
# endregion

# region comment
comment = call("POST", f"/tasks/{ref}/comments", {"body": "Started from the API example."})
print(f"commented {comment['id']}")
# endregion

# region list
page = call("GET", "/tasks", query={"project": key, "status": "doing", "priority": 1, "sort": "due_at"})
for t in page["items"]:
    print(f"listed {t['ref']} {t['status']} p{t['priority']} {t['title']}")
# endregion

# region events
events = call("GET", "/events/poll", query={
    "since_seq": cursor, "types": "task.*,comment.added", "limit": 10,
})
for e in events["events"]:
    print(f"event {e['seq']} {e['type']}")
# endregion
