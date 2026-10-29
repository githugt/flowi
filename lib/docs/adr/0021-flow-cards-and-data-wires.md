# 0021: Flow steps are cards with summary rows; values pass on wires

- **Status:** Accepted
- **Date:** 2026-10-27

## Context

The canvas of ADR 0012 drew every step as a box with a title, strips of small
actions and blocks with attached actions. It did not say what a step did, hid
what a request would send, and had about twenty step types that did a few jobs.
The owner is the only user of the file formats, so they may change outright.

## Decision

- **A card says what its step does.** A pure package (`flowi/internal/card`) turns a node
  into a badge, a sub-line, at most four summary rows and its ports. The window
  draws what it returns and never reads a step's fields. Text is 13 px or larger;
  a row is an icon and a short phrase, with the name on hover.
- **Rows open in place.** A click opens that part's editor under the card, one at a
  time. The side inspector stays and shares its fields with the row editors.
- **A port per outcome.** A request has Done and Fail; a Route has a port for each
  case and an else; a status test is shown short ("== 404", "5xx").
- **One `set` step.** It holds a list of values, each typed, an expression, read
  from a response or generated. `extract`, `generate`, `transform` and `calc`
  are gone from the format; a file that uses one fails with a message that names
  `set`. `fetch`, `webhook`, `batch`, `run_till`, `parallel`, `log`, `save` and
  `digest` stay in files; the step popup offers ten kinds and converts between
  the modes, naming what does not carry over.
- **Data wires.** Dragging from a card's data port onto a later card writes a
  template into a field of it. A step that runs later, in another lane or
  inside another step is refused with the reason. The wire is the template, so
  there is nothing new to save.
- **Skip is a run option, not a file field.** `RunInput.Skip` leaves steps out;
  the editor lists each step that reads what a skipped step would have made, and
  can hold the skipped steps' last values. The list belongs to the open tab.
- **Colour is cosmetic.** A step may carry one of six colour names; a run never
  reads it.
- **Start and End are views.** Start shows the flow's inputs in file order with
  their types; End shows its outputs and, after a run, their values and a short
  report. They sit beside the first and last card and are not steps.
- **An output dock after a run.** Tabs under the selected or pinned card show
  Input, Output, Checks or Messages for the kind of step, with secrets masked.

## Consequences

- Strips and attached blocks are no longer drawn; `attach` and `attach_at` stay
  in files and are candidates for removal.
- A row panel and the dock are drawn under a card by the window, so they can
  cover a lane below it.
- The card rules are tested in Go, so a new request kind needs a card rule.
- Mode conversion is a view over several node types, so a person who reads the
  YAML still meets the older names.
