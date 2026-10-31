# Node reference

Generated from the node registry (`go test ./internal/nodes/ -run Reference -update`). Do not edit.

Every node also takes the common fields `id`, `name`, `description`, `needs` (steps this one waits for, taken when they succeed), `after_failure_of` (steps whose failure starts this one; a step in both lists runs either way), `after_outcome` (`step:outcome` entries: the step starts this one when its request ends with that outcome, one of the `outcomes` its version names; a step with no arrows follows the one before it), `when`, `concurrent`, `on_error` (`fail`, `continue`, `goto:<id>`), `timeout` and `attach` (the id of a request or subflow node right next to it that the canvas shows it with; a view only, the run order is the list order) and `attach_at` (`above` or `below`: draw it in a stack above or below that request, not beside it).

## Call

### `request`

Run a saved request, with its version resolution and API hooks.

Fields: `request`, `version`, `overlay`, `open`, `over`, `close`, `hooks`, `with`, `headers`.

### `fetch`

An ad-hoc HTTP call (no saved request, no hooks), optionally transformed.

| Parameter | Kind | Required | Notes |
|---|---|---|---|
| `url` | template | yes |  |
| `method` | string |  | Default GET. |
| `headers` | map of templates |  |  |
| `query` | map of templates |  |  |
| `body` | any |  | A string is sent as text; a mapping or list as JSON. |
| `transform` | template |  | Rendered with response in scope; becomes `nodes.<id>.value`. |

### `webhook`

Send a payload to a URL; wait for the answer or fire and forget.

| Parameter | Kind | Required | Notes |
|---|---|---|---|
| `url` | template | yes |  |
| `method` | string |  | Default POST. |
| `headers` | map of templates |  |  |
| `query` | map of templates |  |  |
| `payload` | any |  | A string is sent as text; a mapping or list as JSON. |
| `wait` | boolean |  | Default true. With false the run goes on; a failure is only a warning. |

### `subflow`

Run another flow with its declared inputs; its outputs become this node's.

Fields: `flow`, `with`.

## Data

### `set`

Write variables. `values` is a list run in order, each `{name, from, value}` where `from` is `expr` (the default; `value` is a template), `literal` (`value` as written), `generate` (`value` uses generators such as `{{$uuid-v4}}`; optional `seed`) or `response` (post-hooks; `in` is `body`, `header`, `cookie` or `status`, with `path`, `header`, `cookie` or `regex`). Or `target` and `value` for one variable or, in pre-hooks, a request field.

Fields: `values`.

| Parameter | Kind | Required | Notes |
|---|---|---|---|
| `target` | string |  | Without `values`: `vars.<name>`; in pre-hooks also `request.method`, `request.url`, `request.headers.<Name>`, `request.query.<name>`, `request.body[.<path>]`, `cookies.<name>`. |
| `value` | any |  | Without `values`: what `target` is set to. |

## Check

### `assert`

Check a condition, or that an earlier request's response matches its kept schema; on_fail: error stops, warn continues.

| Parameter | Kind | Required | Notes |
|---|---|---|---|
| `check` | expression |  | True or false, written without braces. Give check, schema, problem or any of them. |
| `schema` | string |  | The id of an earlier request step: passes when what it received matches the response schema kept for its request. |
| `problem` | string |  | The id of an earlier request step: passes when what it received is a problem details document (RFC 9457): application/problem+json, a JSON object whose type, title, status, detail and instance are used as the RFC says. |
| `on_fail` | one of `error`, `warn` |  |  |
| `message` | template |  |  |

### `fuzz`

Send a request with bad bodies made from its good one, and fail when the server breaks.

Fields: `request`, `version`, `overlay`, `hooks`, `with`, `headers`.

| Parameter | Kind | Required | Notes |
|---|---|---|---|
| `count` | integer 1–200 |  | How many bad bodies to send (default 20). They are spread over the fields, in a fixed order. |
| `expect` | one of `no_server_error`, `rejected` |  | What passes: no_server_error (default) fails on a 5xx or no answer; rejected also fails when the server accepts a bad body (a 2xx or 3xx). |

### `condition`

Branch: check with then/else, or cases (first true when) with else as default.

Fields: `then`, `else`, `cases`.

| Parameter | Kind | Required | Notes |
|---|---|---|---|
| `check` | expression |  | True or false, written without braces, e.g. nodes.login.status == 200. then runs when it is true, else when it is not; leave it empty when you use cases. |

## Control

### `delay`

Wait a fixed or random time (2s, 1s..3s).

| Parameter | Kind | Required | Notes |
|---|---|---|---|
| `duration` | template | yes | A duration ("2s") or a range ("1s..3s") picked with the run's seed; may be a template. |

### `batch`

Run the body for each item of a list, or N times, with bounded concurrency.

Fields: `body`.

| Parameter | Kind | Required | Notes |
|---|---|---|---|
| `over` | template |  | A list to iterate (item, index in scope). Exactly one of over and times. |
| `times` | integer 1–100000 |  | Iterate N times (item = index). |
| `concurrency` | integer ≥ 1 |  | Iterations in flight (default 1); capped by the run-wide limit. |

### `run_till`

Repeat the body until a condition holds or a limit is reached.

Fields: `body`.

| Parameter | Kind | Required | Notes |
|---|---|---|---|
| `until` | expression | yes | Checked after each iteration; sees that iteration's nodes. |
| `max` | integer 1–10000 |  | Most iterations (default 10). |
| `interval` | duration |  | Wait between iterations. |
| `max_duration` | duration |  | Stop once this much time has passed. |

### `parallel`

Run branches at once; wait for all, any, or the first success.

Fields: `branches`.

| Parameter | Kind | Required | Notes |
|---|---|---|---|
| `wait` | one of `all`, `any`, `first-success` |  | Default all. |
| `concurrency` | integer ≥ 1 |  | Branches in flight (default: all); capped by the run-wide limit. |

### `stop`

End the run here: nothing after it starts. Useful in a branch of a condition.

| Parameter | Kind | Required | Notes |
|---|---|---|---|
| `status` | one of `passed`, `failed` |  | How the run ends: passed (default) is a normal end that skips the rest, failed ends it as a failure. |
| `message` | template |  | Why it stops; written to the run log, or into the failure. |

## Output

### `log`

Write a message or value to the run log.

| Parameter | Kind | Required | Notes |
|---|---|---|---|
| `value` | template | yes |  |

### `note`

An annotation in the run summary; no side effect.

| Parameter | Kind | Required | Notes |
|---|---|---|---|
| `text` | template | yes |  |

### `digest`

A summary of the run so far, or of a finished node, sent to stdout, a file or a webhook.

| Parameter | Kind | Required | Notes |
|---|---|---|---|
| `of` | string |  | "run" (default) or the id of an earlier node. |
| `sink` | one of `stdout`, `file`, `webhook` |  | Default stdout. |
| `format` | one of `markdown`, `text`, `json` |  | Default markdown. |
| `file` | file name |  | For sink file: a plain file name, written under the workspace's `digests/<flow-id>/`. |
| `url` | template |  | For sink webhook: receives {"text": …}, or the report itself for format json. |
| `style` | one of `text`, `slack`, `discord`, `teams`, `email` |  | For sink webhook: the payload shape: text and slack send {"text"}, discord {"content"}, teams a MessageCard, email {to, subject, text}. Default text. Ignored for format json. |
| `to` | template |  | For style email: the recipient. |
| `subject` | template |  | For style email: the subject. Default "Flowi digest". |

### `save`

Save a value (masked) under the workspace's `saved/<flow-id>/`: overwrite, latest by status, or append.

| Parameter | Kind | Required | Notes |
|---|---|---|---|
| `key` | file name | yes | What the value is saved as: a plain file name without extension. |
| `value` | any | yes | What to save, e.g. "{{nodes.login}}". |
| `mode` | one of `overwrite`, `latest-by-status`, `append` |  | overwrite (default): `<key>.json`; latest-by-status: `<key>/<status>.json` from `value.status`; append: a line in `<key>.ndjson`. |
