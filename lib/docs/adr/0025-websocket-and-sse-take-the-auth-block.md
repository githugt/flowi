# 0025: WebSocket and SSE requests take the `auth` block

- **Status:** Accepted
- **Date:** 2026-11-26
- **Builds on:** 0019 (managed messaging), 0020 (gRPC requests)

## Context

An HTTP or gRPC request has an `auth` block (a scheme and its parameters, or `use: <name>` for one of the collection's
named authentications, `auths/<name>.auth.yaml`). A message request had none: validation refused it
(FLW-AUTH-001, "a message request sets its credentials in its binding"), and the editor showed a Credentials tab only
for protocols that define credential bindings (`username`, `password`, `token`, cloud keys). WebSocket and SSE define
none, so the only way to sign in was an `Authorization` header typed by hand, with none of the named
authentications, OAuth 2.0 token handling, secret masking of derived values or Auth tab that an HTTP request has.

Both protocols open with an ordinary HTTP request (the WebSocket handshake, the SSE `GET`), and both transports only
copy `Operation.Headers`, `Operation.Server` and `Operation.TLS` into it.

## Decision

- **Reuse the block; add no credentials model.** A `ws` or `sse` version may have the same `auth` block as an HTTP
  one, in the same file position, so the file format and `schema/` are unchanged (`common.schema.json#/$defs/auth`
  already applied to every version). `{use: name}` works the same way, and `auth.default` is inherited too, but only
  when the protocol can use the scheme (a default written for HTTP, say Digest, is skipped instead of failing every
  WebSocket request of the collection).
- **Applied by the engine, before the transport runs.** `engine/async_auth.go` builds the same signer as for HTTP
  (`authx.New`), runs it on a throw-away `httpx.Request` for the target address, and copies what the scheme added into
  the operation: a header or cookie into `Operation.Headers`, a query parameter into `Operation.Server` (a WebSocket
  channel is folded into the address first, since it is a path that must come before the query), a client
  certificate into `Operation.TLS`. The transports are untouched. What is added goes through the same
  `recordApplied` as HTTP, so it is masked in the recorded request, the run log and every output, and it is in the
  operation before the secret host check (FLW-SEC-005) reads it.
- **Only the schemes that can work.** `model.AsyncAuthTypes`: `apikey`, `oauth2`, `jwt` and `mtls`. Digest and NTLM
  answer a 401 (a handshake has no second try), and SigV4, Hawk, HTTP signatures, OAuth 1.0 and DPoP sign the method,
  URL and body of a plain HTTP request. A scheme outside the list is FLW-AUTH-001 in validation and at send time, with
  the list in the message. Other message protocols keep their credential bindings and still refuse an auth block.
- **Lints.** FLW-LINT-005 (a credential over plain `http`/`ws` to a remote host) reads the server of a WebSocket or SSE
  request, and counts an auth block (or a named authentication) as a credential, for HTTP too. FLW-LINT-004 (a
  credential typed in directly) reads the secret parameters of an auth block, in a request and in a named
  authentication.
- **Editor.** WebSocket and SSE show the Auth tab (same dropdown and `AuthFields`) instead of Credentials, listing
  only the schemes and named authentications they can use. None, Bearer, Basic and Other still write the
  `Authorization` connection header, as for HTTP. Moving a request to a protocol that cannot use its auth block
  drops the block, as it drops binding options the new protocol does not know.

## Consequences

- Not supported yet: schemes that need the response (Digest, NTLM) or the whole request (SigV4, Hawk, HTTP
  signatures), an auth block on a GraphQL subscription (validation still refuses it: put the credential in a
  header), and an auth block that appears in the AsyncAPI or Arazzo description (an HTTP one does not either: the
  description never carries credentials).
- A preview does not apply the auth block, as for HTTP.
