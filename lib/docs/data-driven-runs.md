# Data-driven runs

Run the same flow over many cases: one run per row of a file, or one pass per item of a list inside a flow.

## One run per row: `flowi run --data`

```bash
flowi run signup --data users.csv
```

```text
✓ Ada
✓ Bob
✗ Eve
  Eve: <the first error of Eve's run, with its code>
```

`users.csv` has a header line, and each column is an input of the flow:

```csv
name,email,plan
Ada,ada@example.com,pro
Bob,bob@example.com,free
Eve,eve@example.com,pro
```

- A JSON file works too: an array of objects, `[{"email": "ada@example.com", "plan": "pro"}]`. A number or boolean is
  passed as its text, `null` as empty, and an object or array as JSON.
- A row is named by its `label`, `name` or `id` column; a CSV without one uses its first column, and a JSON row
  without one is "row 3". A repeated name gets `#2`, `#3`.
- A column beats an `--input` of the same name. At most 100 rows run from one command.
- Each row is a separate run. The command exits 1 when any row failed, 4 for a system failure, and prints why each
  failing row failed. `--report junit=report.xml` writes one report per row, named after it.
- It runs one saved flow: it does not combine with `--tag`, `--all`, `--matrix`, an Arazzo file or a bundle.

## One pass per item inside a flow: `batch`

A `batch` step runs its body for each item of a list (`over`) or N times (`times`), with `concurrency` iterations at
once. Give it a `label` and each pass is named by its item instead of its number:

```yaml
- id: users
  type: batch
  over: "{{inputs.users}}"
  label: "{{item.email}}"
  concurrency: 4
  body:
    - id: create
      type: request
      request: { path: users/create }
```

A failure then reads `users[ada@example.com]/create` in the run tree, the JUnit report and the failure list, instead of
`users[17]/create`. The label may read `item` and `index`; slashes and brackets in it become `_`, it is cut at 40
characters, and a repeat gets `#2`.
