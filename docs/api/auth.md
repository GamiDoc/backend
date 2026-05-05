# Authentication

## Model

The backend uses bearer tokens.

Send the token in the `Authorization` header.

```text
Authorization: Bearer <token>
```

## `POST /api/v1/auth/register`

| Field | Value |
| --- | --- |
| Auth | No |
| Success | `201` auth result `{ token, user }` |
| Errors | `400 INVALID_INPUT`, `400 INVALID_EMAIL`, `400 INVALID_PASSWORD`, `400 EMAIL_ALREADY_EXISTS`, `500 INTERNAL_SERVER_ERROR` |
| Notes | Password must be at least 8 characters |

Request:

```json
{
  "email": "demo@example.com",
  "password": "password123"
}
```

Response:

```json
{
  "token": "eyJhbGciOi...",
  "user": {
    "id": "7d7efb4d-0f1c-4f69-9b74-5e2a0d5d5a50",
    "email": "demo@example.com",
    "created_at": "2026-05-05T12:00:00Z"
  }
}
```

Validation errors use the shared envelope, for example:

```json
{
  "error": {
    "code": "INVALID_EMAIL",
    "message": "Invalid email",
    "details": {
      "field": "email"
    }
  }
}
```

## `POST /api/v1/auth/login`

| Field | Value |
| --- | --- |
| Auth | No |
| Success | `200` auth result `{ token, user }` |
| Errors | `400 INVALID_INPUT`, `401 INVALID_CREDENTIALS`, `500 INTERNAL_SERVER_ERROR` |
| Notes | Missing users and bad passwords both map to `INVALID_CREDENTIALS` |

Request:

```json
{
  "email": "demo@example.com",
  "password": "password123"
}
```

Response:

```json
{
  "token": "eyJhbGciOi...",
  "user": {
    "id": "7d7efb4d-0f1c-4f69-9b74-5e2a0d5d5a50",
    "email": "demo@example.com",
    "created_at": "2026-05-05T12:00:00Z"
  }
}
```

## `GET /api/v1/auth/me`

| Field | Value |
| --- | --- |
| Auth | Yes |
| Success | `200` user object `{ id, email, created_at }` |
| Errors | `401 UNAUTHORIZED`, `500 INTERNAL_SERVER_ERROR` |
| Notes | Returns the current token subject |

Response:

```json
{
  "id": "7d7efb4d-0f1c-4f69-9b74-5e2a0d5d5a50",
  "email": "demo@example.com",
  "created_at": "2026-05-05T12:00:00Z"
}
```

Missing or invalid bearer tokens return `401` with `UNAUTHORIZED`.
