# Development status and tags

A request, a flow or a folder can say where the work stands (its **status**) and carry free-form **tags** to be found
by. Both are written in the item's own file, so they go through git with it. The schemas are
`schema/request.schema.json`, `flow.schema.json`, `folder.schema.json` and `manifest.schema.json`.

```yaml
# requests/auth/login.req.yaml, flows/nightly.flow.yaml, or a _folder.yaml
tags:
  - smoke
  - auth
status: review      # one status, by name
```

## Status

A status has a name, a colour and an icon. Every collection has five, in the order work moves through them:

| Name             | Colour | Icon     |
| ---------------- | ------ | -------- |
| `planning`       | gray   | planning |
| `in-development` | blue   | code     |
| `review`         | purple | eye      |
| `testing`        | amber  | flask    |
| `production`     | green  | shield   |

A collection adds its own, or changes the colour and icon of a default, under `statuses:` in its `flowi.yaml`:

```yaml
statuses:
  - name: needs-design      # lower-case letters, digits, '-' and '_'; at most 40 characters
    color: pink             # gray, blue, sky, teal, green, amber, red, pink or purple (default gray)
    icon: bookmark          # planning, code, eye, flask, shield, flag, star, bug, bolt, lock, archive,
                            # bookmark, pause, clock or alert (default flag)
  - name: testing           # a default's name: recolours it, and it keeps its place in the order
    color: red
```

`production` used to be drawn with a rocket. A collection that still names `icon: rocket` keeps working, but the
window no longer offers it and `production` now has a shield.

The colours are names for theme colours, not hex values: each is drawn from a token that is checked for contrast on
every theme, so no status can be unreadable on one. A name given twice keeps its first definition.

### Which status an item has

An item has one status, and it is found in this order:

1. the item's own `status:`;
2. otherwise the `status:` of the **nearest folder above it** that has one (a folder's status is the default for
   everything under it, like its tags, but nearest wins instead of adding up, because an item has one status);
3. otherwise none.

A request in `requests/auth/legacy/` with no `status:` of its own gets the status of `requests/auth/legacy/_folder.yaml`
if that has one, else that of `requests/auth/_folder.yaml`. A folder's own status is resolved the same way from the
folders above it. The apps show an inherited status quietly and say which folder it comes from.

### Checks

`flowi validate` and the apps report:

- `FLW-STATUS-001`: a request, flow or folder names a status the collection does not have. The message lists the ones it
  has and says where to add another.
- `FLW-STATUS-002`: a status in `flowi.yaml` that cannot be used (a bad name, a colour or icon that is not on the lists
  above, a name defined twice).

## Tags

Tags are words to search by. A request's, a flow's and a folder's own tags add up with those of the folders above (see
`flowi run --tag`); a tag appears once. When the apps write tags they keep each as one word: spaces and commas become
`-`, empty tags and repeats are dropped, and the order you gave is kept. The apps suggest the tags already in use in
the collection, most used first, so one is not typed twice in two spellings.

## In the apps

- **Where it is edited.** A request's status and tags are at the top of its **Docs** section; a flow's are in the
  **Flow** panel of the inspector (what shows when no step is selected); a folder's are in **Tags and status…** on its
  right-click menu. Statuses are added from the status picker (**New status…**).
- **Where it shows.** A small coloured icon, with the status name as its tooltip and label for screen readers, sits
  beside the item in the sidebar and in the editor.
- **Finding things.** The sidebar's "Jump to…" field also matches tags and statuses: type a word, or narrow it with
  `status:review` and `tag:smoke` (any number, all must match). The filter button beside it lists them to pick from.
  The command palette finds items the same way.
