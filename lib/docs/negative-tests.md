# Negative tests: `expect:`

A new request comes with a hook that fails it unless the response is a 2xx. A request that is meant to be refused
("a wrong password gives 401") says so with `expect:` on its version, and passes on that status:

```yaml
versions:
  "1":
    method: POST
    url: "{{env.base_url}}/v1/login"
    body:
      json: '{ "email": "a@example.com", "password": "nope" }'
    expect: 401
```

`expect:` takes a status (`401`), a range (`400-499`), a class (`4xx`) or a list (`200, 404`). The request then

- passes on a matching status and fails on any other, with `"auth/wrong" expected 401, got 200`;
- shows the check as `expect: 401` beside its `assert:` lines, and as an "expects 401" mark on its card in a flow;
- runs without the seeded "fail unless 2xx" node. The rest of that hook (the log line) stays. A hook you wrote or
  edited yourself is never touched: if it also checks the status, it still runs.

`flowi validate` reports an `expect:` that is not a status (`FLW-EXPECT-001`), and one on a message, gRPC or
database request, which has no HTTP status. In the app, the Assert tab of an HTTP request has an *Expected status* field.

Not covered yet: a flow step cannot expect a different status than its request does (use another version of the
request), and a hook flow that checks the status is not recognised, so remove that check by hand.
