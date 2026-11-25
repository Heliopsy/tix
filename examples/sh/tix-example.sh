#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# One pass over the tix HTTP API with curl and jq. See docs/examples.md.
set -eu

# region setup
API="${TIX_URL%/}/api/v1"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
# The token goes in a header file so it never shows up in the process list.
(umask 077 && printf 'Authorization: Bearer %s\n' "$TIX_TOKEN" >"$tmp/auth")

# call METHOD PATH [JSON]: leaves the body in $tmp/body and the status in $STATUS,
# and fails on anything but a 2xx.
call() {
	set -- "$1" "$2" "${3-}"
	if [ -n "$3" ]; then
		STATUS=$(curl -sS -o "$tmp/body" -w '%{http_code}' -X "$1" -H @"$tmp/auth" \
			-H 'Content-Type: application/json' --data "$3" "$API$2")
	else
		STATUS=$(curl -sS -o "$tmp/body" -w '%{http_code}' -X "$1" -H @"$tmp/auth" "$API$2")
	fi
	[ "$STATUS" -lt 300 ]
}

field() { jq -r "$1" "$tmp/body"; }

fail() {
	echo "$STATUS $(field '.error.code // "http"'): $(field '.error.message // empty')" >&2
	exit 1
}
# endregion

# region whoami
call GET /whoami || fail
echo "signed in as $(field .handle) ($(field .kind))"
# endregion

# region cursor
call GET /events/poll || fail
cursor=$(field .next_seq)
# endregion

# region create-project
key=${TIX_PROJECT:-ex$(date +%s)}
call POST /projects "$(jq -n --arg key "$key" '{key: $key, name: "API examples"}')" || fail
echo "created project $(field .key)"
# endregion

# region create-task
call POST /tasks "$(jq -n --arg key "$key" '{project_ref: $key, title: "Try the API"}')" || fail
ref=$(field .ref)
created_version=$(field .version)
echo "created $ref at version $created_version"
# endregion

# region update-task
call PATCH "/tasks/$ref" "$(jq -n --argjson v "$created_version" \
	'{title: "Try the tix API", priority: 1, due_at: "2030-01-15T17:00:00Z", version: $v}')" || fail
echo "updated $ref to version $(field .version)"
# endregion

# region conflict
retitle() { jq -n --argjson v "$1" '{title: "Try the tix HTTP API", version: $v}'; }
if ! call PATCH "/tasks/$ref" "$(retitle "$created_version")"; then
	[ "$STATUS" = 409 ] && [ "$(field .error.code)" = conflict ] || fail
	echo "conflict: $ref is at version $(field .error.details.current_version), retrying"
	call GET "/tasks/$ref" || fail
	call PATCH "/tasks/$ref" "$(retitle "$(field .version)")" || fail
fi
echo "retitled $ref at version $(field .version)"
# endregion

# region transition
call POST "/tasks/$ref/transition" "$(jq -n --argjson v "$(field .version)" '{to: "doing", version: $v}')" || fail
echo "$ref is now $(field .status)"
# endregion

# region comment
call POST "/tasks/$ref/comments" '{"body": "Started from the API example."}' || fail
echo "commented $(field .id)"
# endregion

# region list
call GET "/tasks?project=$key&status=doing&priority=1&sort=due_at" || fail
field '.items[] | "listed \(.ref) \(.status) p\(.priority) \(.title)"'
# endregion

# region events
call GET "/events/poll?since_seq=$cursor&types=task.*,comment.added&limit=10" || fail
field '.events[] | "event \(.seq) \(.type)"'
# endregion
