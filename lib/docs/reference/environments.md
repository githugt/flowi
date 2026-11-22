# Environments

An environment is a named set of variables for one target (local, staging, production). A request or a flow reads
them as `{{env.name}}`. This page describes the file; the schema is `schema/environment.schema.json`.

Environments belong to the workspace: they live in its `environments/` folder, one file for each, and are the same in
every collection. A collection can also carry environments of its own in `<collection>/environments/`, which go to a
colleague through git.

```yaml
id: 01J9ZE3M4N5P6Q7R8S9T0VWXYZ
name: payments-prod        # optional: the file name without .yaml is the default
colour: red                # red, orange, yellow, lime, green, aqua, blue, purple or gray
groups: [payments]         # optional: one or more groups, see "Groups"
extends: payments-staging  # optional: the environment this one builds on
protected: true            # ask before a request that changes data is sent
vars:
  timeout: 30              # a number
  mode: fast               # text
  api_key:
    secret: true           # the value is in the vault, never in the file
```

## Variable types

A scalar keeps its type: `30` is a number, `true` a boolean, `"30"` text; an object or a list is held as JSON. A
variable used on its own in a request (`{"timeout": {{env.timeout}}}`) sends its own type, so `30` as text and `30` as
a number are different values even though they print the same. Compare environments treats them as different.

## Groups

`groups:` lists the sets of environments this one belongs to, such as one set for each set of services. An environment
can be in several groups (a database shared by two sets of services is in both), and is then compared in each. An
environment with no `groups:` is in the default group, alone. To be in the default group as well as in named ones, list
`default` too (`groups: [checkout, default]`); it is the one spelling of that name a list takes, and `default` alone is
the same as no line. The names are kept sorted, without repeats.

A group can exist with no environment in it, so there are two places a group comes from, and the groups of a workspace
are both together:

- **`environment_groups:` in `workspace.yaml`** lists the groups the person made. It is what lets a group be made first
  and filled afterwards, and it stays when its last environment leaves. It is personal, like the rest of
  `workspace.yaml`.
- **`groups:` in an environment file** names the groups of that environment. A group named only there (an environment
  shared through git, for instance) exists too, without being listed.

Rules:

- Renaming a group swaps the name in the `groups:` line of each of its environments and in the list; deleting one takes
  the name out of those lines and the list (an environment left in no group is in the default group, one in other
  groups stays in them, and none is deleted). Both are refused with
  `FLW-REQ-014` when the name is taken, unknown or the default group's.
- The `groups:` line goes where the environment file goes: saved with the workspace, and shared with a collection (and
  so through git) when the environment is shared. A group that is only in the list of `workspace.yaml` stays on the
  person's machine.
- It does not touch the vault. Secrets are found by the environment's `id`, which a move does not change.
- A group name is text for people: at most 40 characters, no control characters, no space at either end, and not
  `default` in any case (that is the default group's name, which only the `groups:` list may use, in lower case). An environment with another name, an empty one or a repeat is `FLW-REQ-012`.
- The apps compare the environments of one group at a time (Compare environments) and sort them on Manage
  environments. Running a request or a flow with an environment does not depend on its group.
- An environment that `extends:` one that is missing from one of its groups still gets its values, but the comparison
  of that group does not list the base as a column, so `flowi validate` warns (`FLW-REQ-013`) and the apps note it.
