# PDF Delivery

## `POST /api/v1/projects/{projectId}/generate-pdf`

| Field | Value |
| --- | --- |
| Auth | Yes |
| Success | `200` `{ pdfUrl, email }` |
| Errors | `401 UNAUTHORIZED`, `400 INVALID_INPUT`, `400 WIZARD_INCOMPLETE`, `400 INVALID_NOTIFY_EMAIL`, `403 FORBIDDEN`, `404 PROJECT_NOT_FOUND`, `500 PDF_GENERATION_FAILED` |
| Notes | `notifyEmail` is optional and the returned URL points to public storage |

Response shape:

```json
{
  "pdfUrl": "https://api.example.com/files/pdfs/projects/4ec4aa78-4ce0-4a77-aad1-5f74b66b1f5b/1714904700000000000.pdf",
  "email": {
    "requested": true,
    "to": "qa@example.com",
    "provider": "noop",
    "sent": false,
    "messageId": ""
  }
}
```

## `POST /api/v1/sessions/{sessionId}/generate-pdf`

| Field | Value |
| --- | --- |
| Auth | No |
| Success | `200` `{ pdfUrl, email }` |
| Errors | `400 INVALID_INPUT`, `400 WIZARD_INCOMPLETE`, `400 INVALID_NOTIFY_EMAIL`, `404 SESSION_NOT_FOUND`, `500 PDF_GENERATION_FAILED` |
| Notes | Same response shape as the project PDF route |

## `GET /files/pdfs/*`

| Field | Value |
| --- | --- |
| Auth | No |
| Success | `200 application/pdf` |
| Errors | `404 PDF_NOT_FOUND` |
| Notes | Serves stored bytes from public object storage |

Example headers:

```text
Content-Type: application/pdf
Content-Disposition: attachment; filename="evaluation-plan.pdf"
```

## Email Behavior

`notifyEmail` is optional.

- If it is omitted, `email` is `null`
- If it is present and valid, the service attempts delivery for that request
- Repeat the same generation request to try delivery again
- On the current test deployment, the mail provider is noop, so the response reports a requested delivery without a sent message
