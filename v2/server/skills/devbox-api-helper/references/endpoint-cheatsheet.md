# Devbox API Endpoint Cheatsheet

## Table of Contents

- [Source](#source)
- [Environment](#environment)
- [Auth Model](#auth-model)
- [Codex Gateway Create Inputs](#codex-gateway-create-inputs)
- [Codex Gateway Auth](#codex-gateway-auth)
- [Codex Gateway Session Workflow](#codex-gateway-session-workflow)
- [Endpoint List](#endpoint-list)
- [Quick Curl Templates](#quick-curl-templates)
- [Create Codex Gateway Devbox](#create-codex-gateway-devbox)
- [Gateway Discovery From Info API](#gateway-discovery-from-info-api)
- [Use Codex Gateway Session API](#use-codex-gateway-session-api)
- [Gateway Access Examples](#gateway-access-examples)
- [SSH From Info API](#ssh-from-info-api)
- [Common Error Mapping](#common-error-mapping)
- [Curl Compatibility Flags](#curl-compatibility-flags)

## Source

- This cheatsheet is intended for external API clients.
- Use this file as a quick reference for endpoint behavior and curl usage.

## Environment

```bash
DEVBOX_SERVER_HOST='https://your-devbox-api.example.com'
DEVBOX_SERVER_TOKEN='your-jwt-token'
DEVBOX_NAME='demo-devbox'
DEVBOX_GATEWAY_URL=''
DEVBOX_GATEWAY_TOKEN=''
DEVBOX_GATEWAY_AUTH_TOKEN=''
CODEX_SESSION_ID=''
CODEX_GATEWAY_OPENAI_API_KEY=''
CODEX_GATEWAY_OPENAI_BASE_URL=''
CODEX_GATEWAY_MODEL=''
CODEX_GATEWAY_JWT_SECRET=''
```

## Auth Model

- JWT HS256
- Required claim: `namespace`
- Namespace is derived from token only
- Gateway access token is returned by `GET /api/v1/devbox/{name}` in `data.gateway.token`
- Codex-gateway bearer auth, when enabled, is a separate gateway concern and should not be assumed to match the Devbox server JWT

## Codex Gateway Create Inputs

Required for a Devbox that should start codex-gateway against an OpenAI-compatible upstream:

- `CODEX_GATEWAY_OPENAI_API_KEY`
- `CODEX_GATEWAY_OPENAI_BASE_URL`

Optional but commonly useful:

- `CODEX_GATEWAY_MODEL`
- `CODEX_GATEWAY_JWT_SECRET`

Guidance:

- Ask for the two required values before producing a codex-gateway create request.
- Ask whether the caller also wants a fixed default model.
- Ask whether the gateway should require bearer-token auth; if yes, include `CODEX_GATEWAY_JWT_SECRET`.

## Codex Gateway Auth

How to authenticate codex-gateway requests:

- If `CODEX_GATEWAY_JWT_SECRET` is not configured in the running gateway, omit gateway auth headers entirely.
- If the running gateway is configured to validate the same HS256 secret as the Devbox gateway token, you may reuse `data.gateway.token` as `DEVBOX_GATEWAY_AUTH_TOKEN`.
- If the running gateway uses its own `CODEX_GATEWAY_JWT_SECRET`, do not assume `data.gateway.token` will work. Ask for a separately minted HS256 JWT for codex-gateway.
- The current codex-gateway auth middleware only requires a valid HS256 JWT with `exp`; it does not require a `namespace` claim.

Minimal JWT payload example for codex-gateway auth:

```json
{
  "exp": 1779289401
}
```

## Codex Gateway Session Workflow

Recommended client flow for codex-gateway:

1. Discover the gateway base URL.
2. Create a session with `POST /api/sessions`.
3. Save `sessionId`.
4. Subscribe to `GET /api/sessions/{id}/events`.
5. Send prompts with `POST /api/sessions/{id}/turn`.
6. Use `GET /api/sessions/{id}/state` as a snapshot and fallback.
7. Use `POST /api/sessions/{id}/thread/new` when you want fresh context in the same session.
8. Delete the session with `DELETE /api/sessions/{id}` when done.

Operational notes:

- One session maps to one `codex app-server` child process.
- One session can only have one active turn at a time.
- Process output is mainly observed through the SSE stream.
- Legacy single-session routes such as `/api/turn` are gone; create a session first.

## Endpoint List

- `GET /healthz`
- `POST /api/v1/devbox`
- `GET /api/v1/devbox`
- `GET /api/v1/devbox/{name}`
- `POST /api/v1/devbox/{name}/pause/refresh`
- `POST /api/v1/devbox/{name}/pause`
- `POST /api/v1/devbox/{name}/resume`
- `DELETE /api/v1/devbox/{name}`
- `POST /api/v1/devbox/{name}/exec`
- `POST /api/v1/devbox/{name}/files/upload`
- `GET /api/v1/devbox/{name}/files/download`
- `/{gateway.pathPrefix}/{uniqueID}[/*rest]` runtime gateway proxy path discovered from `data.gateway.url`

## Quick Curl Templates

Create (generic):

```bash
curl -sS -X POST "${DEVBOX_SERVER_HOST}/api/v1/devbox" \
  -H "Authorization: Bearer ${DEVBOX_SERVER_TOKEN}" \
  -H "Content-Type: application/json" \
  --data "{\"name\":\"${DEVBOX_NAME}\",\"image\":\"registry.example.com/devbox/runtime:custom-v2\",\"upstreamID\":\"session-123\",\"kubeAccess\":{\"enabled\":true,\"roleTemplate\":\"edit\"},\"env\":{\"FOO\":\"bar\",\"NODE_ENV\":\"production\"},\"pauseAt\":\"2026-03-03T09:00:00Z\",\"archiveAfterPauseTime\":\"24h\"}"
```

List all in namespace:

```bash
curl -sS -X GET "${DEVBOX_SERVER_HOST}/api/v1/devbox" \
  -H "Authorization: Bearer ${DEVBOX_SERVER_TOKEN}"
```

List by upstreamID:

```bash
curl -sS -X GET "${DEVBOX_SERVER_HOST}/api/v1/devbox?upstreamID=session-123" \
  -H "Authorization: Bearer ${DEVBOX_SERVER_TOKEN}"
```

Info:

```bash
curl -sS -X GET "${DEVBOX_SERVER_HOST}/api/v1/devbox/${DEVBOX_NAME}" \
  -H "Authorization: Bearer ${DEVBOX_SERVER_TOKEN}"
```

Info fields to expect when gateway is configured:

- `data.gateway.url`
- `data.gateway.token`
- `data.gateway.uniqueID`
- `data.gateway.port`

Refresh pauseAt:

```bash
curl -sS -X POST "${DEVBOX_SERVER_HOST}/api/v1/devbox/${DEVBOX_NAME}/pause/refresh" \
  -H "Authorization: Bearer ${DEVBOX_SERVER_TOKEN}" \
  -H "Content-Type: application/json" \
  --data '{"pauseAt":"2026-03-03T12:00:00Z"}'
```

Pause:

```bash
curl -sS -X POST "${DEVBOX_SERVER_HOST}/api/v1/devbox/${DEVBOX_NAME}/pause" \
  -H "Authorization: Bearer ${DEVBOX_SERVER_TOKEN}"
```

Resume:

```bash
curl -sS -X POST "${DEVBOX_SERVER_HOST}/api/v1/devbox/${DEVBOX_NAME}/resume" \
  -H "Authorization: Bearer ${DEVBOX_SERVER_TOKEN}"
```

Destroy:

```bash
curl -sS -X DELETE "${DEVBOX_SERVER_HOST}/api/v1/devbox/${DEVBOX_NAME}" \
  -H "Authorization: Bearer ${DEVBOX_SERVER_TOKEN}"
```

Exec:

```bash
curl -sS -X POST "${DEVBOX_SERVER_HOST}/api/v1/devbox/${DEVBOX_NAME}/exec" \
  -H "Authorization: Bearer ${DEVBOX_SERVER_TOKEN}" \
  -H "Content-Type: application/json" \
  --data '{"command":["sh","-lc","whoami"],"timeoutSeconds":30}'
```

Upload:

```bash
curl -sS -X POST "${DEVBOX_SERVER_HOST}/api/v1/devbox/${DEVBOX_NAME}/files/upload?path=/home/devbox/workspace/a.txt&mode=0644" \
  -H "Authorization: Bearer ${DEVBOX_SERVER_TOKEN}" \
  -H "Content-Type: application/octet-stream" \
  --data-binary @/tmp/a.txt
```

Download:

```bash
curl -sS -X GET "${DEVBOX_SERVER_HOST}/api/v1/devbox/${DEVBOX_NAME}/files/download?path=/home/devbox/workspace/a.txt" \
  -H "Authorization: Bearer ${DEVBOX_SERVER_TOKEN}" \
  -o /tmp/a.download.txt
```

## Create Codex Gateway Devbox

When the user is creating a Devbox that should run codex-gateway, prefer a create payload like this:

```bash
curl -sS -X POST "${DEVBOX_SERVER_HOST}/api/v1/devbox" \
  -H "Authorization: Bearer ${DEVBOX_SERVER_TOKEN}" \
  -H "Content-Type: application/json" \
  --data "{\"name\":\"${DEVBOX_NAME}\",\"image\":\"registry.example.com/devbox/runtime:custom-v2\",\"upstreamID\":\"session-123\",\"env\":{\"CODEX_GATEWAY_OPENAI_API_KEY\":\"${CODEX_GATEWAY_OPENAI_API_KEY}\",\"CODEX_GATEWAY_OPENAI_BASE_URL\":\"${CODEX_GATEWAY_OPENAI_BASE_URL}\",\"CODEX_GATEWAY_MODEL\":\"${CODEX_GATEWAY_MODEL}\",\"CODEX_GATEWAY_JWT_SECRET\":\"${CODEX_GATEWAY_JWT_SECRET}\"},\"pauseAt\":\"2026-03-03T09:00:00Z\",\"archiveAfterPauseTime\":\"24h\"}"
```

If the optional values are not needed, omit them from `env` instead of sending empty strings.

Minimum `env` block for codex-gateway:

```json
{
  "CODEX_GATEWAY_OPENAI_API_KEY": "<required>",
  "CODEX_GATEWAY_OPENAI_BASE_URL": "https://your-openai-compatible-endpoint.example.com"
}
```

Recommended `env` block when the caller also wants a default model and gateway auth:

```json
{
  "CODEX_GATEWAY_OPENAI_API_KEY": "<required>",
  "CODEX_GATEWAY_OPENAI_BASE_URL": "https://your-openai-compatible-endpoint.example.com",
  "CODEX_GATEWAY_MODEL": "gpt-5-codex",
  "CODEX_GATEWAY_JWT_SECRET": "<shared-hs256-secret>"
}
```

Notes:

- `CODEX_GATEWAY_OPENAI_API_KEY` and `CODEX_GATEWAY_OPENAI_BASE_URL` are the deployment minimum for the current codex-gateway implementation.
- `CODEX_GATEWAY_MODEL` is optional and becomes the default `CODEX_MODEL` for new bridges.
- `CODEX_GATEWAY_JWT_SECRET` is optional, but recommended if the gateway will be reachable by external clients.

## Gateway Discovery From Info API

```bash
INFO_JSON="$(curl -sS -X GET "${DEVBOX_SERVER_HOST}/api/v1/devbox/${DEVBOX_NAME}" \
  -H "Authorization: Bearer ${DEVBOX_SERVER_TOKEN}")"
DEVBOX_GATEWAY_URL="$(echo "$INFO_JSON" | jq -r '.data.gateway.url // empty')"
DEVBOX_GATEWAY_TOKEN="$(echo "$INFO_JSON" | jq -r '.data.gateway.token // empty')"
DEVBOX_GATEWAY_UNIQUE_ID="$(echo "$INFO_JSON" | jq -r '.data.gateway.uniqueID // empty')"
DEVBOX_GATEWAY_PORT="$(echo "$INFO_JSON" | jq -r '.data.gateway.port // empty')"

if [ -z "$DEVBOX_GATEWAY_URL" ] || [ -z "$DEVBOX_GATEWAY_TOKEN" ]; then
  echo "gateway is not configured for ${DEVBOX_NAME}" >&2
  exit 1
fi
```

If the running codex-gateway is configured to trust the same HS256 secret as the Devbox gateway token, you can reuse it like this:

```bash
DEVBOX_GATEWAY_AUTH_TOKEN="${DEVBOX_GATEWAY_TOKEN}"
```

If codex-gateway auth is disabled, leave `DEVBOX_GATEWAY_AUTH_TOKEN` empty and omit the auth header on codex-gateway requests.

If codex-gateway auth is enabled with its own `CODEX_GATEWAY_JWT_SECRET`, set `DEVBOX_GATEWAY_AUTH_TOKEN` to a separately minted JWT instead of reusing `DEVBOX_GATEWAY_TOKEN`.

## Use Codex Gateway Session API

Create a session:

```bash
CREATE_JSON="$(curl -sS -X POST "${DEVBOX_GATEWAY_URL}/api/sessions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${DEVBOX_GATEWAY_AUTH_TOKEN}" \
  --data '{"model":"gpt-5.4"}')"
CODEX_SESSION_ID="$(echo "$CREATE_JSON" | jq -r '.sessionId')"
```

If gateway auth is disabled, omit the `Authorization` header entirely.

Subscribe to the SSE stream with curl:

```bash
curl -sS -N "${DEVBOX_GATEWAY_URL}/api/sessions/${CODEX_SESSION_ID}/events" \
  -H "Accept: text/event-stream" \
  -H "Authorization: Bearer ${DEVBOX_GATEWAY_AUTH_TOKEN}" \
  --http1.1
```

If a browser `EventSource` client cannot set headers, pass the token as a query parameter:

```text
${DEVBOX_GATEWAY_URL}/api/sessions/${CODEX_SESSION_ID}/events?access_token=<JWT>
```

Send one prompt:

```bash
curl -sS -X POST "${DEVBOX_GATEWAY_URL}/api/sessions/${CODEX_SESSION_ID}/turn" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${DEVBOX_GATEWAY_AUTH_TOKEN}" \
  --data '{"prompt":"Please explain the repository structure."}'
```

Read the current snapshot:

```bash
curl -sS -X GET "${DEVBOX_GATEWAY_URL}/api/sessions/${CODEX_SESSION_ID}/state" \
  -H "Authorization: Bearer ${DEVBOX_GATEWAY_AUTH_TOKEN}"
```

Start a fresh thread inside the same session:

```bash
curl -sS -X POST "${DEVBOX_GATEWAY_URL}/api/sessions/${CODEX_SESSION_ID}/thread/new" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${DEVBOX_GATEWAY_AUTH_TOKEN}" \
  --data '{"model":"gpt-5.4"}'
```

Delete the session when finished:

```bash
curl -sS -X DELETE "${DEVBOX_GATEWAY_URL}/api/sessions/${CODEX_SESSION_ID}" \
  -H "Authorization: Bearer ${DEVBOX_GATEWAY_AUTH_TOKEN}"
```

Notes:

- If gateway auth is disabled, remove the `Authorization` header from the examples above.
- If a session already has an active turn, a new `POST /turn` can fail with `409`.
- Use the SSE stream for progress, and `GET /state` when you need a full snapshot including transcript and thread status.

## Gateway Access Examples

Probe the gateway root without downloading the full response body:

```bash
curl -sS -D - -o /dev/null "${DEVBOX_GATEWAY_URL}" \
  -H "Authorization: Bearer ${DEVBOX_GATEWAY_TOKEN}" \
  --http1.1
```

Call an app-relative path behind the gateway:

```bash
curl -sS "${DEVBOX_GATEWAY_URL}/api/status" \
  -H "Authorization: Bearer ${DEVBOX_GATEWAY_TOKEN}" \
  --http1.1
```

Stream an SSE-style response if the app exposes one:

```bash
curl -sS -N "${DEVBOX_GATEWAY_URL}/api/status?watch=1" \
  -H "Authorization: Bearer ${DEVBOX_GATEWAY_TOKEN}" \
  --http1.1
```

Notes:

- Reuse `DEVBOX_GATEWAY_URL` as the base URL and append app-relative paths.
- Do not rebuild `/codex/${DEVBOX_GATEWAY_UNIQUE_ID}` manually unless the info API is unavailable and you explicitly need the route contract.
- Keep the gateway token separate from the Devbox server token.
- For codex-gateway session APIs, prefer the dedicated `/api/sessions/...` workflow above instead of ad-hoc probing.

## SSH From Info API

```bash
INFO_JSON="$(curl -sS -X GET "${DEVBOX_SERVER_HOST}/api/v1/devbox/${DEVBOX_NAME}" -H "Authorization: Bearer ${DEVBOX_SERVER_TOKEN}")"
printf '%s' "$(echo "$INFO_JSON" | jq -r '.data.ssh.privateKeyBase64')" \
  | openssl base64 -d -A > /tmp/devbox.key
chmod 600 /tmp/devbox.key
SSH_USER="$(echo "$INFO_JSON" | jq -r '.data.ssh.user')"
SSH_HOST="$(echo "$INFO_JSON" | jq -r '.data.ssh.host')"
SSH_PORT="$(echo "$INFO_JSON" | jq -r '.data.ssh.port')"
ssh -i /tmp/devbox.key "${SSH_USER}@${SSH_HOST}" -p "${SSH_PORT}"
```

## Common Error Mapping

- `400`: invalid input
- `401`: invalid/expired token
- `404`: devbox, file, gateway route, or codex-gateway session not found
- `409`: pod not running, state conflict, or a codex-gateway turn is already in progress
- `410`: legacy single-session codex-gateway endpoint removed
- `500`: server or k8s API error
- `502`: gateway upstream unavailable
- `503`: maximum concurrent codex-gateway sessions reached
- `504`: exec/transfer timeout

## Curl Compatibility Flags

- Use `--http1.1` if HTTP2 framing issues appear
- Use `-k` only in non-production testing
