# Wording in the window

How buttons, labels, placeholders and messages are written, so the same thing reads the same everywhere. When a
screen needs a word this file does not cover, add it here in the same change.

## Buttons

- A button names what it does, with a verb: **Save**, **Send**, **Import**. Not "OK", "Yes" or "Submit".
- A button that opens a dialog or needs more input ends with an ellipsis: **Delete…**, **Import an API…**.
- Use the **Button** component. `primary` is the main action of a page, `go` is the Send-coloured "start something",
  `danger` is for what removes things.

## Verbs that remove things

| Verb | Means | Example |
| --- | --- | --- |
| **Delete** | Removes a file or an item for good, or to the Trash. | Delete the request. |
| **Remove** | Takes an item out of a list; the thing itself stays. | Remove a header row, Remove from the collection list. |
| **Unmount** | Stops showing a collection; its files stay. | Unmount Payments. |
| **Discard** | Throws away unsaved changes or a draft. | Discard changes. |
| **Clear** | Empties a view or a log; nothing on disk changes. | Clear the traffic log. |

Do not use "Forget", "Throw away", "Drop" or "Erase" for these.

A confirmation says what goes, what stays, and whether **Undo** brings it back:
"Delete the step login? 3 steps inside them go too. Undo brings it back."

## Names for things

- A **step** is a box on the canvas. The file format calls it a node; messages for people say "step".
- A **branch** is one of the paths of a Route or a Group. Do not say "lane" or "strip" in text.
- **Drafts** is the place for requests and flows that are not in a collection folder yet.
- A **request** has **versions**; the footer says "Request ver. 2", the application says "Flowi v1.4".
- Explain a term the first time it matters, from `lib/glossary.ts`: Arazzo, drift, seed, digest.

## Labels and placeholders

- A label is a noun phrase in sentence case: "Listen on", "Latency". No colon at the end.
- A placeholder shows an example value, never an instruction: `250ms`, not "Enter the latency".
- Help that is longer than a line goes in a Hint (the "?"), not in the placeholder.

## Messages

- Say what happened, then what to do: "Could not save: the file is read-only. Choose another folder."
- Use the window's words (step, name), not the file's (node, id).
- Do not blame the person ("you entered…"); say what is wrong with the value.
- Count with `plural()`: "1 step", "2 steps", "0 steps".
