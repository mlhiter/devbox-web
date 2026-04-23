---
name: devbox-api-helper
description: Help external clients call, validate, and troubleshoot the Devbox REST API and codex gateway over HTTP with JWT authentication. Use when users need ready-to-run curl examples for devbox lifecycle, info, exec, file upload/download, SSH info retrieval, gateway discovery/access, codex-gateway-ready devbox creation, codex-gateway session API usage, or API and gateway error diagnosis.
---

# Devbox Api Helper

## Overview

Use this skill to provide client-facing API usage guidance and executable curl commands for Devbox operations, including codex gateway discovery and access.

## Load Reference

1. Read `references/endpoint-cheatsheet.md` first.
2. Treat this skill as client-side usage guidance.
3. If the user provides a newer API contract, follow the user-provided contract.

## Session Bootstrap

1. When this skill is activated, first ask the user to provide `DEVBOX_SERVER_HOST` and `DEVBOX_SERVER_TOKEN`.
2. If either value is missing, ask for it before generating API curl commands.
3. If the user wants to call codex-gateway APIs and does not already have a gateway base URL, ask for `DEVBOX_NAME` so you can derive `data.gateway.url` from `GET /api/v1/devbox/{name}`.
4. If the user wants to create a Devbox that will run `codex-gateway`, ask for `CODEX_GATEWAY_OPENAI_API_KEY` and `CODEX_GATEWAY_OPENAI_BASE_URL` before generating the create request.
5. For codex-gateway creation, also ask whether they want to set optional `CODEX_GATEWAY_MODEL` and `CODEX_GATEWAY_JWT_SECRET`.
6. If the caller wants to use codex-gateway session APIs on a protected gateway, ask for the bearer token they should send to codex-gateway. Only reuse `data.gateway.token` when the running codex-gateway is configured to validate the same HS256 secret.
7. If the user already has `DEVBOX_GATEWAY_URL` and an auth token decision, allow direct gateway examples. Otherwise, derive gateway access from `GET /api/v1/devbox/{name}`.
8. After the user provides values, show examples using those values (or clearly marked placeholders if redaction is needed).

## Apply Core Constraints

1. Use API prefix `/api/v1/devbox`.
2. Do not add `namespace` query/body fields for business APIs.
3. Treat namespace as JWT claim only (`namespace` in token payload).
4. Keep `Authorization: Bearer <JWT>` on all business endpoints.
5. Treat `GET /api/v1/devbox/{name}/files/download` as binary response, not JSON.
6. Treat codex gateway access as a discovered route: prefer `data.gateway.url` and `data.gateway.token` from `GET /api/v1/devbox/{name}` instead of hand-building `/codex/{uniqueID}`.
7. If `data.gateway` is absent, explain that the gateway route is not configured or not available for that Devbox.
8. For codex-gateway provisioning, pass gateway runtime configuration through the create request `env` object using the exact variable names from codex-gateway: `CODEX_GATEWAY_OPENAI_API_KEY`, `CODEX_GATEWAY_OPENAI_BASE_URL`, optional `CODEX_GATEWAY_MODEL`, and optional `CODEX_GATEWAY_JWT_SECRET`.
9. Do not invent alternative env names for codex-gateway LLM settings.
10. For codex-gateway session usage, treat the base URL as the discovered `data.gateway.url` and use the session workflow routes `/api/sessions`, `/api/sessions/{id}/events`, `/api/sessions/{id}/turn`, `/api/sessions/{id}/state`, `/api/sessions/{id}/thread/new`, and `DELETE /api/sessions/{id}`.
11. Do not assume `data.gateway.token` always authenticates codex-gateway. If `CODEX_GATEWAY_JWT_SECRET` is custom and not aligned with the Devbox JWT secret, require a separate codex-gateway bearer token or omit auth entirely when gateway auth is disabled.
12. For browser-style SSE examples, use `?access_token=<JWT>` when the client cannot set `Authorization` headers on `EventSource`.

## Execute Common Workflows

1. Initialize shell vars (`DEVBOX_SERVER_HOST`, `DEVBOX_SERVER_TOKEN`, `DEVBOX_NAME`) before giving curl examples.
2. For lifecycle: use create, info, pause, resume, destroy endpoints.
3. For command execution: call `POST /exec` with non-empty `command` array and valid timeout.
4. For file transfer: call upload/download with required `path`.
5. For SSH access: call info endpoint, read `data.ssh.*`, decode `privateKeyBase64`, then run ssh command.
6. For codex gateway access: call info endpoint first, read `data.gateway.url`, `data.gateway.token`, `data.gateway.uniqueID`, and `data.gateway.port`, then decide whether to use `data.gateway.token` as the gateway bearer token based on the running codex-gateway auth configuration.
7. When showing gateway examples, use the discovered gateway base URL and append app-relative paths. Do not reconstruct the public path manually unless the user explicitly asks for the path contract.
8. For codex-gateway-ready devbox creation: collect gateway env values first, then place them under the create payload `env` object.
9. Treat `CODEX_GATEWAY_OPENAI_API_KEY` and `CODEX_GATEWAY_OPENAI_BASE_URL` as the minimum gateway deployment inputs.
10. Treat `CODEX_GATEWAY_MODEL` as optional model override and `CODEX_GATEWAY_JWT_SECRET` as optional gateway auth, but recommend the JWT secret when the gateway will be exposed to external callers.
11. For codex-gateway API usage: create a session first, subscribe to that session's SSE stream, send turns with `POST /turn`, inspect `GET /state` as fallback, use `POST /thread/new` to reset context within the same session, and `DELETE /api/sessions/{id}` when finished.
12. When the user wants codex-gateway integration examples, prefer a full end-to-end sequence instead of isolated single-route snippets.
13. Explain that a single session can only have one active turn at a time, and that process output is mainly observed through SSE.

## Troubleshoot Predictably

1. Map failures by HTTP code:
`400` invalid params or malformed JSON, `401` token invalid, `404` resource, gateway route, or session missing, `409` state conflict such as active turn already running, `410` legacy single-session gateway endpoint removed, `500` server error, `502` gateway upstream unavailable, `503` maximum concurrent sessions reached, `504` timeout.
2. If gateway/TLS/HTTP2 issues appear with curl, retry using `--http1.1` and optionally `-k` in non-production tests.
3. For `exec`/file failures, verify Devbox pod is running before retrying.
4. For gateway failures, refresh Devbox info first in case `uniqueID` changed, then retry with the latest `data.gateway.url` and `data.gateway.token`.
5. For codex-gateway startup failures, verify the create payload included `CODEX_GATEWAY_OPENAI_API_KEY` and `CODEX_GATEWAY_OPENAI_BASE_URL` under `env`.
6. For codex-gateway auth failures, verify whether gateway auth is disabled, aligned with the Devbox JWT secret, or expecting a separately minted HS256 bearer token.
7. For `turn` conflicts, wait for the current turn to complete or start a fresh session instead of sending concurrent prompts into the same session.

## Output Style

1. Give executable curl commands with concrete placeholders.
2. Keep examples aligned with current API doc fields.
3. When returning info examples, include:
`creationTimestamp`, `deletionTimestamp`, `state`, `ssh`, and `gateway` fields when present.
4. Do not expose local machine paths in output.
5. When the user is creating a codex-gateway devbox, show the exact `env` block for gateway startup instead of a generic placeholder-only `env`.
6. When the user is using codex-gateway directly, show the complete session lifecycle with concrete endpoint paths and clearly label which token is for Devbox server auth versus codex-gateway auth.
