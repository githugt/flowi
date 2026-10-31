# 0023: An arrow into a step says on which outcomes it is taken

- **Status:** Accepted
- **Date:** 2026-10-30
- **Builds on:** 0021 (flow cards and data wires)

## Context

The only arrow a step could have was "after the one before it, or after what `needs` names", and it was taken
when that step succeeded. Running a step because another failed meant `on_error: goto:<later step>`, which
skips every step in between and also leaves the target in the list, so it ran on the success path too. There
was no way to say "this step runs only when that one fails", or "whichever way it ends".

## Decision

- **`after_failure_of`** (`model.Node.AfterFailureOf`) lists the steps of the same list whose failure starts
  this step. `needs` stays the list of steps whose success does. A step named in both lists runs either way.
  With neither list the step follows the step before it, on success, as before.
- **`Node.Arrows(previous)`** is the one place that turns a step's fields into arrows (`From`, `OnSuccess`,
  `OnFailure`). The scheduler, the cycle check and the partial-run closure read it, so none of them
  re-implements the "implicit previous step" rule.
- **Every arrow into a step has to be taken for it to run** (the rule `needs` always had). The scheduler skips
  a step when an arrow's outcome does not match, and that skip passes on to the steps that wait for it on
  success. A failure that some step waits on is handled: it is reported as a warning (`FLW-RUN-015`), does not
  stop the list or fail the run, and the failed step still shows as failed. A failure nothing waits on stops
  the run as before.
- **The canvas draws one edge per outcome** (`Fail` for a failure arrow, with `FromStep` naming the step; `Goto`
  marks the old `on_error` route, which still works). An arrow taken either way is two edges. Clicking an arrow
  shows two switches, "On success" and "On failure"; switching both off removes it. The arrow from the step
  before cannot be removed (the step would follow it again by itself): it is switched to failure instead.
- **Dragging from a Fail port onto a step** makes that step run on failure and drops its implied arrow from the
  step before it. Keeping it would mean the step needs the success of a step that was skipped when this one
  failed, so it could never run.
- **Arazzo:** `x-flowi-after-failure-of` carries the failure arrows, as `x-flowi-depends-on` carries `needs`.

## Named outcomes (added the same day)

Success and failure were all an arrow could tell apart, so "on a 404, go to A; on a 500 or 503, go to B" was
not expressible. A request now says in what ways it can end, and an arrow names the ones it follows.

- **`outcomes:` on a request version** is an ordered list of rows `{name, when, disabled}`. `success` and
  `failure` are built in (the request and its hooks passed, or it failed or could not be sent) and take no
  `when`; any other name has a `when`, an expression over the response (`response.status in [200, 201]`),
  evaluated after the hooks have run. With no list, or an empty one, a request has success and failure. A written list is the
  whole list: success or failure can be removed (the list never becomes empty: the editor refuses to remove
  the last row), and a disabled row stays in the list but never matches.
- **A finished request has every outcome whose rule holds**, not the first (`engine.matchOutcomes`). With
  "first match wins", the natural list `success, failure, ok, not-found` would make the named rows
  unreachable, since failure matches before not-found. The order is therefore the order of the ports and
  switches, not a priority. A rule that cannot be evaluated does not match; a cancelled request has none.
  A step that is not a request has success or failure by its status.
- **`after_outcome: [step:outcome]`** names an arrow for an outcome. `needs` is `step:success` and
  `after_failure_of` is `step:failure`; `Node.Arrows` merges the three, and `Arrow.Takes(outcome)` is the
  one test the scheduler uses. An arrow is taken when any outcome the step ended with is one it names.
- **A failed step is handled when some arrow takes one of its outcomes**, any outcome, so a request that
  failed on a 404 is handled by an arrow for `not-found`; if no arrow takes it, the run stops as before.
- **The card has one port per enabled outcome** (Done and Fail keep their ids; a named one is
  `outcome-<name>`). Any port can be dragged onto a step, several at once with shift-click, and the + beside
  a port adds a step that runs on that outcome (`AddStepOnOutcome`: the step goes right after the source and
  the step that followed it keeps following it, so the main line does not change). Clicking an arrow lists
  every outcome of the card it leaves as a switch.

## Colour, description, per-step lists, blueprints, placement (same day)

- **An outcome has an optional `color` (one of the step colours) and `description`.** The colour shows on the
  card's port, its label and the arrows and switches of that outcome; the description is the port's tooltip.
- **A request step can carry its own `outcomes:`** (`RequestCall.Outcomes`), which replace the request
  version's for that step: the same request can end in different ways in different flows. The step panel edits
  them; "Use the request's" drops them and "Save to the request" writes them into the version the step runs
  (and drops the step's own, since both then say the same).
- **Blueprints carry `outcomes:`** (`Blueprint.Outcomes`). Seeding a request copies the first listed blueprint's
  outcomes into its opening version unless that already writes its own. The built-in defaults per kind (HTTP,
  gRPC, message, dataset) list success and failure with that kind's own description, so a new request starts
  with them written and they can be changed in a blueprint. There is no blueprint editor screen: a stored
  blueprint is edited as a file.
- **Rules have their own help** (`outcome.rule`, `.message`, `.dataset`): what the rule can read, `in` and
  `not in`, and examples.
- **Placement.** A step that only runs on an outcome (no success arrow) hangs below the main line in its own
  column right after its source, each further one stacked under it; the steps after it are pushed right, and
  units placed by hand to its right are moved over (`FlowDoc.MakeRoom`). The arrow between the main-line steps
  no longer runs through it. A step added with the + on a port goes after the ones already hanging from that
  step, so the order of creation is the order top to bottom.

## Removing an arrow, drop-points, colours (same day, after use)

- **Any arrow can be removed.** The arrow from the step before could not (the step would follow it again by
  itself). A step now has `unlinked: true` when its last arrow is taken away: no arrow into it, it does not follow
  the step before, and any arrow put in later links it again. It still waits its turn in list order, and a failure
  nothing handles still stops the run. The picked arrow has a remove button next to its switches.
- **Every arrow is solid.** Dotted had meant "an explicit needs, not the list order", which stopped meaning anything
  once any arrow could be removed or added: dragging from the green port makes a success arrow like any other. Failure
  and outcome arrows are solid too, in their outcome's colour.
- **Arrows land on a ring (a drop-point) on the card's edge,** in the arrow's colour, not on an arrowhead.
- **Colours:** `red` and `sky` (aqua blue) join the step colours. An outcome starts green (success), red (failure)
  or aqua blue (named), shown as a swatch picked in the same style as a step's colour.
- **The request card has an Outcomes row** (always shown, like Hooks) that opens the editor under the card.

- **A step that hangs from an outcome is never reordered by a drop.** Its place on screen comes from the layout (below
  its source), not from its place in the list, so a drop leaves it where it is put; before, a drop over a gap inserted it
  into the list and the layout then moved it back under its source. A drop only counts as being in the gap of an arrow
  within about a card height of it (`sideReach`).
- **The ring at the end of an arrow has a marker per colour.** `context-stroke` is not drawn by every web view (the macOS
  one drew a ring with no outline, so arrows seemed to end in nothing).

## Not done

- Several arrows into a step are all required (AND). There is no "any of these" join, so a step after both a
  success path and a failure path of the same step cannot be written; a step after only one of two branches
  that were each skipped is skipped too.
- An outcome rule sees the response only. A transport error (no response) has `failure` and nothing else,
  and a rule about the kind of error (timeout, refused) cannot be written yet.
- A rule is not checked against the request kind (a dataset request has no `response.status`): it just never
  matches. The validator checks only that it parses.
- An `after_outcome` naming an outcome the request does not have is not reported: the arrow is never taken.
- Outcomes live on the request version, shared by every flow that uses it; a step cannot add its own.
- `on_error: goto` is unchanged and not converted.
