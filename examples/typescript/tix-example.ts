// SPDX-License-Identifier: AGPL-3.0-or-later
// One pass over the tix HTTP API with Node's built-in fetch. See docs/examples.md.

// region setup
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
// endregion

async function main(): Promise<void> {
  // region whoami
  const me = await call<{ handle: string; kind: string }>("GET", "/whoami");
  console.log(`signed in as ${me.handle} (${me.kind})`);
  // endregion

  // region cursor
  const { next_seq: cursor } = await call<{ next_seq: number }>("GET", "/events/poll");
  // endregion

  // region create-project
  const key = process.env.TIX_PROJECT || `ex${Math.floor(Date.now() / 1000)}`;
  const project = await call<{ key: string }>("POST", "/projects", { key, name: "API examples" });
  console.log(`created project ${project.key}`);
  // endregion

  // region create-task
  let task = await call<Task>("POST", "/tasks", { project_ref: key, title: "Try the API" });
  const { ref, version: createdVersion } = task;
  console.log(`created ${ref} at version ${createdVersion}`);
  // endregion

  // region update-task
  task = await call<Task>("PATCH", `/tasks/${ref}`, {
    title: "Try the tix API",
    priority: 1,
    due_at: "2030-01-15T17:00:00Z",
    version: task.version,
  });
  console.log(`updated ${ref} to version ${task.version}`);
  // endregion

  // region conflict
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
  // endregion

  // region transition
  task = await call<Task>("POST", `/tasks/${ref}/transition`, { to: "doing", version: task.version });
  console.log(`${ref} is now ${task.status}`);
  // endregion

  // region comment
  const comment = await call<{ id: string }>("POST", `/tasks/${ref}/comments`, { body: "Started from the API example." });
  console.log(`commented ${comment.id}`);
  // endregion

  // region list
  const page = await call<{ items: Task[] }>("GET", "/tasks", undefined, {
    project: key, status: "doing", priority: 1, sort: "due_at",
  });
  for (const t of page.items) console.log(`listed ${t.ref} ${t.status} p${t.priority} ${t.title}`);
  // endregion

  // region events
  const { events } = await call<{ events: { seq: number; type: string }[] }>("GET", "/events/poll", undefined, {
    since_seq: cursor, types: "task.*,comment.added", limit: 10,
  });
  for (const e of events) console.log(`event ${e.seq} ${e.type}`);
  // endregion
}

main().catch((err) => {
  console.error(err instanceof Error ? err.message : err);
  process.exit(1);
});
