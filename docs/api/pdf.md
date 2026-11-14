# PDF Delivery

## `POST /api/v1/projects/{projectId}/generate-pdf`

| Field | Value |
| --- | --- |
| Auth | Yes |
| Success | `200` `{ pdfUrl, email }` |
| Errors | `401 UNAUTHORIZED`, `400 INVALID_INPUT`, `400 WIZARD_INCOMPLETE`, `400 INVALID_NOTIFY_EMAIL`, `400 INVALID_PDF_TEMPLATE`, `403 FORBIDDEN`, `404 PROJECT_NOT_FOUND`, `503 PDF_RENDERER_UNAVAILABLE`, `500 PDF_GENERATION_FAILED` |
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

## `PATCH /api/v1/projects/{projectId}/pdf`

Body `{ "name": "My plan" }`: renames the evaluation plan PDF (the download name). Response `200 { "fileName": "My-plan.pdf" }`; `400 INVALID_NAME`, `404 PDF_NOT_FOUND`.

## `DELETE /api/v1/projects/{projectId}/pdf`

Deletes the evaluation plan PDF (file and `pdfUrl`). Response `204`; `404 PDF_NOT_FOUND` when there is none.

## `POST /api/v1/sessions/{sessionId}/generate-pdf`

| Field | Value |
| --- | --- |
| Auth | No |
| Success | `200` `{ pdfUrl, email }` |
| Errors | `400 INVALID_INPUT`, `400 WIZARD_INCOMPLETE`, `400 INVALID_NOTIFY_EMAIL`, `400 INVALID_PDF_TEMPLATE`, `404 SESSION_NOT_FOUND`, `503 PDF_RENDERER_UNAVAILABLE`, `500 PDF_GENERATION_FAILED` |
| Notes | Same response shape as the project PDF route |

## Custom HTML/CSS Templates

PDF generation routes accept an optional `template` object. If omitted, the backend uses the built-in FPDF layout.

```json
{
  "notifyEmail": "qa@example.com",
  "template": {
    "html": "<main><h1>{{ .Title }}</h1><p>{{ join .EvaluationGoals \", \" }}</p></main>",
    "css": "body { font-family: Inter, sans-serif; } h1 { color: #2a578d; }"
  }
}
```

Template data is the generated evaluation plan:

- `.Title`
- `.Date`
- `.EvaluationGoals`
- `.ProjectType`
- `.Participants`
- `.DevelopmentStage`
- `.Accessibility`
- `.TimeConstraint`
- `.ExtraConstraints`
- `.ResearchEnabled`
- `.ResearchObjective`
- `.ResearchQuestions`
- `.Hypotheses`
- `.SelectedMethods`
- `.SelectedInstruments`
- `.NextSteps`
- `.Notes`

Template helpers:

- `{{ join .EvaluationGoals ", " }}`
- `{{ formatDate .Date "January 2, 2006" }}`
- `{{ default .Notes "No notes provided." }}`

Operational notes:

- Custom templates require `PDF_HTML_RENDERER_URL` to point to a Gotenberg-compatible `/forms/chromium/convert/html` endpoint.
- The Docker Compose production stack includes `gamidoc-backend-prod-gotenberg`; `.env.prod.example` points `PDF_HTML_RENDERER_URL` at that service.
- HTML templates are capped at 128 KB and CSS at 64 KB.
- JavaScript and inline event-handler attributes are rejected.
- The backend injects the `css` field into the document `<head>` or wraps body fragments in a complete HTML document.

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
