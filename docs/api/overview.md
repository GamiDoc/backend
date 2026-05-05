# API Overview

## Base URL

Use `{BASE_URL}` in local and shared examples. When you need a concrete example, use `https://api.example.com`.

```text
{BASE_URL}/health
{BASE_URL}/api/v1/ping
```

!!! note
    This backend does not define a canonical production domain in the docs. Keep examples generic.

## Core Routes

### `GET /health`

| Field | Value |
| --- | --- |
| Auth | No |
| Success | `200` `{ "status": "ok" }` |
| Notes | Liveness check only |

### `GET /ready`

| Field | Value |
| --- | --- |
| Auth | No |
| Success | `200` `{ "status": "ok", "postgres": "ok", "redis": "ok" }` |
| Errors | `503` with the same shape and failed dependencies marked `error` |
| Notes | Uses a short dependency timeout |

### `GET /api/v1/ping`

| Field | Value |
| --- | --- |
| Auth | No |
| Success | `200` `{ "message": "pong" }` |
| Notes | Simple API probe |

### `GET /api/v1/panic`

| Field | Value |
| --- | --- |
| Auth | No |
| Success | None |
| Errors | `500` via recovery middleware |
| Notes | Intentional panic route for recovery testing |

### `GET /api/v1/error`

| Field | Value |
| --- | --- |
| Auth | No |
| Success | `400` error envelope |
| Errors | `BAD_REQUEST` with `details.path` |
| Notes | Sample structured error response |

## CORS And OPTIONS

The router adds CORS headers only when the request `Origin` is allowed.

- `Access-Control-Allow-Origin` echoes the request origin
- `Access-Control-Allow-Headers` allows `Authorization, Content-Type`
- `Access-Control-Allow-Methods` allows `GET, POST, PUT, PATCH, DELETE, OPTIONS`
- Any `OPTIONS` request returns `204 No Content`

If the origin is not allowed, the response still completes, but it does not add CORS headers.

## Error Envelope

All JSON errors use the same envelope.

```json
{
  "error": {
    "code": "BAD_REQUEST",
    "message": "Bad request",
    "details": {
      "path": "/api/v1/error"
    }
  }
}
```

## Health And Probe Responses

```json
{
  "status": "ok"
}
```

```json
{
  "message": "pong"
}
```

## Readiness Response

```json
{
  "status": "ok",
  "postgres": "ok",
  "redis": "ok"
}
```

When a dependency check fails, the handler returns `503` and marks the failing service as `error`.

## Global Notes

- `GET /` returns `404` in the backend only deployment.
- `GET /files/pdfs/*` serves generated PDF bytes from public object storage.
- The API uses bearer tokens for protected routes.
