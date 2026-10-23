# Node reference

Generated from the node registry (`go test ./internal/nodes/ -run Reference -update`). Do not edit.

Every node also takes the common fields `id`, `name`, `description`, `needs`, `when`, `concurrent`, `on_error` (`fail`, `continue`, `goto:<id>`), `timeout` and `attach` (the id of a request or subflow node right next to it that the canvas shows it with; a view only, the run order is the list order) and `attach_at` (`above` or `below`: draw it in a stack above or below that request, not beside it).

## Call

### `request`

Run a saved request, with its version resolution and API hooks.

Fields: `request`, `version`, `hooks`, `with`, `headers`.

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

### `extract`

JSONPath / header / cookie / regex from a response (post-hooks), or templates (map form), into variables.

Other keys are `<variable>: template`; each result is stored in `vars.<variable>` and output as `nodes.<id>.<variable>`.

| Parameter | Kind | Required | Notes |
|---|---|---|---|
| `name` | string |  | The variable the value is saved in. |
| `from` | one of `body`, `header`, `cookie`, `status` |  | Where in the response to read. |
| `path` | string |  | JSONPath (RFC 9535) into the body. |
| `header` | string |  |  |
| `cookie` | string |  |  |
| `regex` | string |  | First capture group, or the whole match. |

### `set`

Insert or override a variable or (in pre-hooks) a request field.

| Parameter | Kind | Required | Notes |
|---|---|---|---|
| `target` | string | yes | `vars.<name>`; in pre-hooks also `request.method`, `request.url`, `request.headers.<Name>`, `request.query.<name>`, `request.body[.<path>]`, `cookies.<name>`. |
| `value` | any | yes |  |

### `generate`

Generate values ($first-name, $uuid-v4, …) into variables; seedable.

Other keys are `<variable>: template`; each result is stored in `vars.<variable>` and output as `nodes.<id>.<variable>`.

| Parameter | Kind | Required | Notes |
|---|---|---|---|
| `seed` | integer ≥ 1 |  | Seed for this node only, so its values do not change with the run seed. |

### `transform`

Reshape data with an expression.

| Parameter | Kind | Required | Notes |
|---|---|---|---|
| `value` | template | yes | The result, e.g. "{{map(nodes.list.body.items, .id)}}". |
| `into` | string |  | Also store the result in `vars.<into>`. |

### `calc`

Arithmetic on variables (+ - * / %, min, max) into variables.

Other keys are `<variable>: template`; each result is stored in `vars.<variable>` and output as `nodes.<id>.<variable>`.

## Check

### `assert`

Check a condition; on_fail: error stops, warn continues.

| Parameter | Kind | Required | Notes |
|---|---|---|---|
| `check` | expression | yes |  |
| `on_fail` | one of `error`, `warn` |  |  |
| `message` | template |  |  |

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
| `subject` | template |  | For style email: the subject. Default "Flow-I digest". |

### `save`

Save a value (masked) under the workspace's `saved/<flow-id>/`: overwrite, latest by status, or append.

| Parameter | Kind | Required | Notes |
|---|---|---|---|
| `key` | file name | yes | What the value is saved as: a plain file name without extension. |
| `value` | any | yes | What to save, e.g. "{{nodes.login}}". |
| `mode` | one of `overwrite`, `latest-by-status`, `append` |  | overwrite (default): `<key>.json`; latest-by-status: `<key>/<status>.json` from `value.status`; append: a line in `<key>.ndjson`. |
