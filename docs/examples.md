# API examples

The same client written four times: a POSIX shell script with curl, Python 3, TypeScript on Node and Go. Each
one signs in with an API token, creates a project and a task, edits the task, recovers from a stale version,
moves it to another status, comments on it, lists tasks through a filter and reads back the events it caused.
Nothing beyond the language's own runtime is needed, apart from `jq` for the shell script.

| Language | File | Run it with |
| --- | --- | --- |
| curl | [examples/sh/tix-example.sh](../examples/sh/tix-example.sh) | `sh examples/sh/tix-example.sh` (needs curl and jq) |
| Python | [examples/python/tix_example.py](../examples/python/tix_example.py) | `python3 examples/python/tix_example.py` |
| TypeScript | [examples/typescript/tix-example.ts](../examples/typescript/tix-example.ts) | `node --experimental-strip-types examples/typescript/tix-example.ts` (Node 22.6 or later; 23.6 and later need no flag) |
| Go | [examples/go/main.go](../examples/go/main.go) | `cd examples/go && go run .` (its own module, standard library only) |

Every snippet below is a region of those files, marked `region NAME` and `endregion`. `just docs-check` fails
when a snippet on this page differs from its file, and the smoke suite (`just smoke`) runs all four programs
against a scratch server, so what is shown here is what was run. [api.md](api.md) is the reference for every
route used.

## Running them

Point the programs at a server and a token through the environment:

```sh
export TIX_URL=http://127.0.0.1:8080
export TIX_TOKEN=$(tix token create api-example -o json \
  --scope project:write --scope task:read --scope task:write \
  --scope task:transition --scope comment:write --scope event:subscribe | jq -r .token)
```

Those six scopes are exactly what the walk needs; `GET /whoami` takes none. `TIX_PROJECT` names the project
key to create, and defaults to `ex` followed by the current Unix time so a second run does not collide with
the first. Try them against a scratch server, never your real store:

```sh
tix --db /tmp/tix-examples.db serve --listen 127.0.0.1:8080
```

Each program prints one line per step and stops at the first error with the status, the error code and the
message. None of them prints the token.

## The walk

Each step shows the shell script, then Python, then TypeScript, then Go.

### Setting up a client

Every request carries `Authorization: Bearer $TIX_TOKEN` under `/api/v1`, and a body is JSON with
`Content-Type: application/json`. Any status of 300 or more is turned into an error carrying the envelope
every route answers with, `{"error":{"code","message","details"}}`; see [Errors](api.md#errors). The shell
script writes the token to a header file so it never appears on a command line.

<!-- example: examples/sh/tix-example.sh#setup -->

```sh
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
```

<!-- example: examples/python/tix_example.py#setup -->

```python
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
```

<!-- example: examples/typescript/tix-example.ts#setup -->

```ts
const API = `${process.env.TIX_URL!.replace(/\/+$/, "")}/api/v1`;
const TOKEN = process.env.TIX_TOKEN!;

type Task = { ref: string; title: string; status: string; priority: number; version: number };
type TixError = { code: string; message: string; details?: Record<string, unknown> };

class APIError extends Error {
  status: number;
  code: string;
  details: Record<string, unknown>;
  constructor(status: number, error: TixError) {
    super(`${status} ${error.code}: ${error.message}`);
    this.status = status;
    this.code = error.code;
    this.details = error.details ?? {};
  }
}

async function call<T>(method: string, path: string, body?: unknown, query?: Record<string, string | number>): Promise<T> {
  const url = new URL(API + path);
  for (const [k, v] of Object.entries(query ?? {})) url.searchParams.set(k, String(v));
  const headers: Record<string, string> = { Authorization: `Bearer ${TOKEN}` };
  if (body !== undefined) headers["Content-Type"] = "application/json";
  const res = await fetch(url, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
  if (!res.ok) {
    const error = await res.json().then((b) => b.error, () => undefined);
    throw new APIError(res.status, error ?? { code: "http", message: res.statusText });
  }
  return res.json() as Promise<T>;
}
```

<!-- example: examples/go/main.go#setup -->

```go
var (
    api   = strings.TrimRight(os.Getenv("TIX_URL"), "/") + "/api/v1"
    token = os.Getenv("TIX_TOKEN")
)

// APIError is the error envelope every tix route answers with.
type APIError struct {
    Status  int            `json:"-"`
    Code    string         `json:"code"`
    Message string         `json:"message"`
    Details map[string]any `json:"details"`
}

func (e *APIError) Error() string { return fmt.Sprintf("%d %s: %s", e.Status, e.Code, e.Message) }

// Task holds the task fields this example reads.
type Task struct {
    Ref      string `json:"ref"`
    Title    string `json:"title"`
    Status   string `json:"status"`
    Priority int    `json:"priority"`
    Version  int    `json:"version"`
}

func call(method, path string, query url.Values, body, out any) error {
    u := api + path
    if len(query) > 0 {
        u += "?" + query.Encode()
    }
    var payload io.Reader
    if body != nil {
        b, err := json.Marshal(body)
        if err != nil {
            return err
        }
        payload = bytes.NewReader(b)
    }
    req, err := http.NewRequest(method, u, payload)
    if err != nil {
        return err
    }
    req.Header.Set("Authorization", "Bearer "+token)
    if body != nil {
        req.Header.Set("Content-Type", "application/json")
    }
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer func() { _ = resp.Body.Close() }()
    if resp.StatusCode >= 300 {
        var envelope struct{ Error *APIError }
        if json.NewDecoder(resp.Body).Decode(&envelope) != nil || envelope.Error == nil {
            envelope.Error = &APIError{Code: "http", Message: resp.Status}
        }
        envelope.Error.Status = resp.StatusCode
        return envelope.Error
    }
    return json.NewDecoder(resp.Body).Decode(out)
}
```

### Checking the token

`GET /whoami` answers who the credential acts as. A missing or wrong token is `401`, a token without a scope
a route needs is `403`.

<!-- example: examples/sh/tix-example.sh#whoami -->

```sh
call GET /whoami || fail
echo "signed in as $(field .handle) ($(field .kind))"
```

<!-- example: examples/python/tix_example.py#whoami -->

```python
me = call("GET", "/whoami")
print(f"signed in as {me['handle']} ({me['kind']})")
```

<!-- example: examples/typescript/tix-example.ts#whoami -->

```ts
const me = await call<{ handle: string; kind: string }>("GET", "/whoami");
console.log(`signed in as ${me.handle} (${me.kind})`);
```

<!-- example: examples/go/main.go#whoami -->

```go
var me struct{ Handle, Kind string }
if err := call("GET", "/whoami", nil, nil, &me); err != nil {
    return err
}
fmt.Printf("signed in as %s (%s)\n", me.Handle, me.Kind)
```

### Remembering where the event log is

`GET /events/poll` without `since_seq` returns no events and the current cursor, `next_seq`. Keeping it now
means the last step can read exactly the events this run causes. See
[Following events over HTTP](api.md#following-events-over-http).

<!-- example: examples/sh/tix-example.sh#cursor -->

```sh
call GET /events/poll || fail
cursor=$(field .next_seq)
```

<!-- example: examples/python/tix_example.py#cursor -->

```python
cursor = call("GET", "/events/poll")["next_seq"]
```

<!-- example: examples/typescript/tix-example.ts#cursor -->

```ts
const { next_seq: cursor } = await call<{ next_seq: number }>("GET", "/events/poll");
```

<!-- example: examples/go/main.go#cursor -->

```go
var start struct {
    NextSeq int64 `json:"next_seq"`
}
if err := call("GET", "/events/poll", nil, nil, &start); err != nil {
    return err
}
```

### Creating a project

`POST /projects` with a `key` and a `name`. The key becomes the prefix of the project's task references. A
key already in use is `409 conflict`.

<!-- example: examples/sh/tix-example.sh#create-project -->

```sh
key=${TIX_PROJECT:-ex$(date +%s)}
call POST /projects "$(jq -n --arg key "$key" '{key: $key, name: "API examples"}')" || fail
echo "created project $(field .key)"
```

<!-- example: examples/python/tix_example.py#create-project -->

```python
key = os.environ.get("TIX_PROJECT") or f"ex{int(time.time())}"
project = call("POST", "/projects", {"key": key, "name": "API examples"})
print(f"created project {project['key']}")
```

<!-- example: examples/typescript/tix-example.ts#create-project -->

```ts
const key = process.env.TIX_PROJECT || `ex${Math.floor(Date.now() / 1000)}`;
const project = await call<{ key: string }>("POST", "/projects", { key, name: "API examples" });
console.log(`created project ${project.key}`);
```

<!-- example: examples/go/main.go#create-project -->

```go
key := os.Getenv("TIX_PROJECT")
if key == "" {
    key = fmt.Sprintf("ex%d", time.Now().Unix())
}
var project struct{ Key string }
if err := call("POST", "/projects", nil, map[string]any{"key": key, "name": "API examples"}, &project); err != nil {
    return err
}
fmt.Printf("created project %s\n", project.Key)
```

### Creating a task

`POST /tasks` names the project in `project_ref`. The answer is the task, with its reference (`ex123-1`) and
its `version`, which starts at 1 and goes up with every change.

<!-- example: examples/sh/tix-example.sh#create-task -->

```sh
call POST /tasks "$(jq -n --arg key "$key" '{project_ref: $key, title: "Try the API"}')" || fail
ref=$(field .ref)
created_version=$(field .version)
echo "created $ref at version $created_version"
```

<!-- example: examples/python/tix_example.py#create-task -->

```python
task = call("POST", "/tasks", {"project_ref": key, "title": "Try the API"})
ref, created_version = task["ref"], task["version"]
print(f"created {ref} at version {created_version}")
```

<!-- example: examples/typescript/tix-example.ts#create-task -->

```ts
let task = await call<Task>("POST", "/tasks", { project_ref: key, title: "Try the API" });
const { ref, version: createdVersion } = task;
console.log(`created ${ref} at version ${createdVersion}`);
```

<!-- example: examples/go/main.go#create-task -->

```go
var task Task
if err := call("POST", "/tasks", nil, map[string]any{"project_ref": key, "title": "Try the API"}, &task); err != nil {
    return err
}
ref, createdVersion := task.Ref, task.Version
fmt.Printf("created %s at version %d\n", ref, createdVersion)
```

### Updating it

`PATCH /tasks/REF` changes only the fields the body names. Sending the `version` you last read makes the
update conditional: the server refuses it if somebody changed the task in between. See
[Updating a task](api.md#updating-a-task).

<!-- example: examples/sh/tix-example.sh#update-task -->

```sh
call PATCH "/tasks/$ref" "$(jq -n --argjson v "$created_version" \
    '{title: "Try the tix API", priority: 1, due_at: "2030-01-15T17:00:00Z", version: $v}')" || fail
echo "updated $ref to version $(field .version)"
```

<!-- example: examples/python/tix_example.py#update-task -->

```python
task = call("PATCH", f"/tasks/{ref}", {
    "title": "Try the tix API",
    "priority": 1,
    "due_at": "2030-01-15T17:00:00Z",
    "version": task["version"],
})
print(f"updated {ref} to version {task['version']}")
```

<!-- example: examples/typescript/tix-example.ts#update-task -->

```ts
task = await call<Task>("PATCH", `/tasks/${ref}`, {
  title: "Try the tix API",
  priority: 1,
  due_at: "2030-01-15T17:00:00Z",
  version: task.version,
});
console.log(`updated ${ref} to version ${task.version}`);
```

<!-- example: examples/go/main.go#update-task -->

```go
update := map[string]any{
    "title":    "Try the tix API",
    "priority": 1,
    "due_at":   "2030-01-15T17:00:00Z",
    "version":  task.Version,
}
if err := call("PATCH", "/tasks/"+ref, nil, update, &task); err != nil {
    return err
}
fmt.Printf("updated %s to version %d\n", ref, task.Version)
```

### Handling a conflict

tix does optimistic locking with that `version` field, not with `If-Match`, so a stale write is `409` with
code `conflict` rather than `412`, and `details.current_version` says where the task is now. To show the
recovery, this step deliberately sends the version the task was created at, which the update above made
stale. The right answer to a conflict is to read the task again, decide whether the change still makes
sense, and send it with the fresh version. A blind retry with no version at all would overwrite whatever
the other writer did.

Check the code as well as the status: `409` also covers a key that is taken and an `Idempotency-Key` whose
first request is still running (see [Retrying safely](api.md#retrying-safely)), and neither of those is fixed
by reading the task again.

<!-- example: examples/sh/tix-example.sh#conflict -->

```sh
retitle() { jq -n --argjson v "$1" '{title: "Try the tix HTTP API", version: $v}'; }
if ! call PATCH "/tasks/$ref" "$(retitle "$created_version")"; then
    [ "$STATUS" = 409 ] && [ "$(field .error.code)" = conflict ] || fail
    echo "conflict: $ref is at version $(field .error.details.current_version), retrying"
    call GET "/tasks/$ref" || fail
    call PATCH "/tasks/$ref" "$(retitle "$(field .version)")" || fail
fi
echo "retitled $ref at version $(field .version)"
```

<!-- example: examples/python/tix_example.py#conflict -->

```python
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
```

<!-- example: examples/typescript/tix-example.ts#conflict -->

```ts
const change = { title: "Try the tix HTTP API", version: createdVersion };
try {
  task = await call<Task>("PATCH", `/tasks/${ref}`, change);
} catch (err) {
  if (!(err instanceof APIError) || err.status !== 409 || err.code !== "conflict") throw err;
  console.log(`conflict: ${ref} is at version ${err.details.current_version}, retrying`);
  change.version = (await call<Task>("GET", `/tasks/${ref}`)).version;
  task = await call<Task>("PATCH", `/tasks/${ref}`, change);
}
console.log(`retitled ${ref} at version ${task.version}`);
```

<!-- example: examples/go/main.go#conflict -->

```go
change := map[string]any{"title": "Try the tix HTTP API", "version": createdVersion}
err := call("PATCH", "/tasks/"+ref, nil, change, &task)
var apiErr *APIError
if errors.As(err, &apiErr) && apiErr.Status == http.StatusConflict && apiErr.Code == "conflict" {
    fmt.Printf("conflict: %s is at version %v, retrying\n", ref, apiErr.Details["current_version"])
    if err := call("GET", "/tasks/"+ref, nil, nil, &task); err != nil {
        return err
    }
    change["version"] = task.Version
    err = call("PATCH", "/tasks/"+ref, nil, change, &task)
}
if err != nil {
    return err
}
fmt.Printf("retitled %s at version %d\n", ref, task.Version)
```

### Changing its status

`POST /tasks/REF/transition` with `to`, the target state. The builtin workflow goes from `todo` to `doing`,
then `done`; a move the workflow does not allow is `422 precondition_failed`. It takes `version` the same way
the update does.

<!-- example: examples/sh/tix-example.sh#transition -->

```sh
call POST "/tasks/$ref/transition" "$(jq -n --argjson v "$(field .version)" '{to: "doing", version: $v}')" || fail
echo "$ref is now $(field .status)"
```

<!-- example: examples/python/tix_example.py#transition -->

```python
task = call("POST", f"/tasks/{ref}/transition", {"to": "doing", "version": task["version"]})
print(f"{ref} is now {task['status']}")
```

<!-- example: examples/typescript/tix-example.ts#transition -->

```ts
task = await call<Task>("POST", `/tasks/${ref}/transition`, { to: "doing", version: task.version });
console.log(`${ref} is now ${task.status}`);
```

<!-- example: examples/go/main.go#transition -->

```go
move := map[string]any{"to": "doing", "version": task.Version}
if err := call("POST", "/tasks/"+ref+"/transition", nil, move, &task); err != nil {
    return err
}
fmt.Printf("%s is now %s\n", ref, task.Status)
```

### Adding a comment

`POST /tasks/REF/comments` with a Markdown `body` answers `201` and the comment.

<!-- example: examples/sh/tix-example.sh#comment -->

```sh
call POST "/tasks/$ref/comments" '{"body": "Started from the API example."}' || fail
echo "commented $(field .id)"
```

<!-- example: examples/python/tix_example.py#comment -->

```python
comment = call("POST", f"/tasks/{ref}/comments", {"body": "Started from the API example."})
print(f"commented {comment['id']}")
```

<!-- example: examples/typescript/tix-example.ts#comment -->

```ts
const comment = await call<{ id: string }>("POST", `/tasks/${ref}/comments`, { body: "Started from the API example." });
console.log(`commented ${comment.id}`);
```

<!-- example: examples/go/main.go#comment -->

```go
var comment struct{ ID string }
if err := call("POST", "/tasks/"+ref+"/comments", nil, map[string]any{"body": "Started from the API example."}, &comment); err != nil {
    return err
}
fmt.Printf("commented %s\n", comment.ID)
```

### Listing with a filter

`GET /tasks` takes its filter as query parameters, each one repeatable: here the project, the `doing` state
and priority 1, sorted by due date. The answer is a keyset page, `{"items":[...],"next_cursor":"..."}`; pass
`next_cursor` back as `cursor` for the next page. See [Listing and pagination](api.md#listing-and-pagination)
and [Filtering tasks](api.md#filtering-tasks).

<!-- example: examples/sh/tix-example.sh#list -->

```sh
call GET "/tasks?project=$key&status=doing&priority=1&sort=due_at" || fail
field '.items[] | "listed \(.ref) \(.status) p\(.priority) \(.title)"'
```

<!-- example: examples/python/tix_example.py#list -->

```python
page = call("GET", "/tasks", query={"project": key, "status": "doing", "priority": 1, "sort": "due_at"})
for t in page["items"]:
    print(f"listed {t['ref']} {t['status']} p{t['priority']} {t['title']}")
```

<!-- example: examples/typescript/tix-example.ts#list -->

```ts
const page = await call<{ items: Task[] }>("GET", "/tasks", undefined, {
  project: key, status: "doing", priority: 1, sort: "due_at",
});
for (const t of page.items) console.log(`listed ${t.ref} ${t.status} p${t.priority} ${t.title}`);
```

<!-- example: examples/go/main.go#list -->

```go
filter := url.Values{"project": {key}, "status": {"doing"}, "priority": {"1"}, "sort": {"due_at"}}
var page struct{ Items []Task }
if err := call("GET", "/tasks", filter, nil, &page); err != nil {
    return err
}
for _, t := range page.Items {
    fmt.Printf("listed %s %s p%d %s\n", t.Ref, t.Status, t.Priority, t.Title)
}
```

### Reading the events

`GET /events/poll` again, from the cursor kept at the start and narrowed to task and comment events. This run
caused `task.created`, two `task.updated`, `task.transitioned` and `comment.added`; anything else that
happened in the tenant meanwhile and matches the filter comes back too. Pass `next_seq` back as `since_seq`
to keep following, and add `wait=25s` to hold the request open until something happens. A client that can
hold a WebSocket uses `GET /events` instead, from the same cursor; see [The event stream](api.md#the-event-stream).

<!-- example: examples/sh/tix-example.sh#events -->

```sh
call GET "/events/poll?since_seq=$cursor&types=task.*,comment.added&limit=10" || fail
field '.events[] | "event \(.seq) \(.type)"'
```

<!-- example: examples/python/tix_example.py#events -->

```python
events = call("GET", "/events/poll", query={
    "since_seq": cursor, "types": "task.*,comment.added", "limit": 10,
})
for e in events["events"]:
    print(f"event {e['seq']} {e['type']}")
```

<!-- example: examples/typescript/tix-example.ts#events -->

```ts
const { events } = await call<{ events: { seq: number; type: string }[] }>("GET", "/events/poll", undefined, {
  since_seq: cursor, types: "task.*,comment.added", limit: 10,
});
for (const e of events) console.log(`event ${e.seq} ${e.type}`);
```

<!-- example: examples/go/main.go#events -->

```go
since := url.Values{"since_seq": {fmt.Sprint(start.NextSeq)}, "types": {"task.*,comment.added"}, "limit": {"10"}}
var batch struct {
    Events []struct {
        Seq  int64
        Type string
    }
}
if err := call("GET", "/events/poll", since, nil, &batch); err != nil {
    return err
}
for _, e := range batch.Events {
    fmt.Printf("event %d %s\n", e.Seq, e.Type)
}
```

## Related

- [api.md](api.md) for every route, the error codes and the event vocabulary
- [agents.md](agents.md) for claiming tasks and lease tokens, which these examples do not use
- [scripting.md](scripting.md) for doing the same from the `tix` command line
