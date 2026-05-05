# Projects

All project routes require a bearer token.

## `POST /api/v1/projects`

| Field | Value |
| --- | --- |
| Auth | Yes |
| Success | `201` project object |
| Errors | `401 UNAUTHORIZED`, `400 INVALID_INPUT`, `400 INVALID_PROJECT_NAME`, `500 INTERNAL_SERVER_ERROR` |
| Notes | Creates an initial wizard state |

Request:

```json
{
  "name": "Mobile playtest plan",
  "description": "Evaluation plan for the May release"
}
```

Response:

```json
{
  "projectId": "4ec4aa78-4ce0-4a77-aad1-5f74b66b1f5b",
  "name": "Mobile playtest plan",
  "description": "Evaluation plan for the May release",
  "wizardStatus": {
    "currentStep": 1,
    "isComplete": false,
    "steps": {}
  },
  "pdfUrl": null,
  "createdAt": "2026-05-05T12:00:00Z",
  "updatedAt": "2026-05-05T12:00:00Z"
}
```

## `GET /api/v1/projects`

| Field | Value |
| --- | --- |
| Auth | Yes |
| Success | `200` `{ "projects": [...], "total": n }` |
| Errors | `401 UNAUTHORIZED`, `500 INTERNAL_SERVER_ERROR` |
| Notes | No pagination |

Response:

```json
{
  "projects": [
    {
      "projectId": "4ec4aa78-4ce0-4a77-aad1-5f74b66b1f5b",
      "name": "Mobile playtest plan",
      "description": "Evaluation plan for the May release",
      "wizardStatus": {
        "currentStep": 1,
        "isComplete": false,
        "steps": {}
      },
      "pdfUrl": null,
      "createdAt": "2026-05-05T12:00:00Z",
      "updatedAt": "2026-05-05T12:00:00Z"
    }
  ],
  "total": 1
}
```

## `GET /api/v1/projects/{projectId}`

| Field | Value |
| --- | --- |
| Auth | Yes |
| Success | `200` project object |
| Errors | `401 UNAUTHORIZED`, `400 INVALID_PROJECT_ID`, `403 FORBIDDEN`, `404 PROJECT_NOT_FOUND`, `500 INTERNAL_SERVER_ERROR` |
| Notes | Returns only projects owned by the caller |

## `PATCH /api/v1/projects/{projectId}`

| Field | Value |
| --- | --- |
| Auth | Yes |
| Success | `200` updated project object |
| Errors | `401 UNAUTHORIZED`, `400 INVALID_INPUT`, `400 INVALID_PROJECT_NAME`, `400 INVALID_PROJECT_ID`, `403 FORBIDDEN`, `404 PROJECT_NOT_FOUND`, `500 INTERNAL_SERVER_ERROR` |
| Notes | Updates name and description only |

## `DELETE /api/v1/projects/{projectId}`

| Field | Value |
| --- | --- |
| Auth | Yes |
| Success | `204 No Content` |
| Errors | `401 UNAUTHORIZED`, `400 INVALID_PROJECT_ID`, `403 FORBIDDEN`, `404 PROJECT_NOT_FOUND`, `500 INTERNAL_SERVER_ERROR` |
| Notes | Hard delete of the project record |

## `PUT /api/v1/projects/{projectId}/wizard/step/{stepNumber}`

| Field | Value |
| --- | --- |
| Auth | Yes |
| Success | `200` `{ projectId, stepNumber, stepData, updatedAt, wizardStatus }` |
| Errors | `401 UNAUTHORIZED`, `400 INVALID_INPUT`, `400 INVALID_PROJECT_ID`, `400 INVALID_STEP_NUMBER`, `400 INVALID_STEP_DATA`, `400 STEP_PREREQUISITE_NOT_MET`, `403 FORBIDDEN`, `404 PROJECT_NOT_FOUND`, `500 INTERNAL_SERVER_ERROR` |
| Notes | The server enforces step order and step schema before saving |

Request example for step 1:

```json
{
  "stepData": {
    "evaluationGoals": ["Usability & Playability"],
    "projectType": "Concept test",
    "participants": "Limited set of participants",
    "developmentStage": "Concept idea"
  }
}
```

Response:

```json
{
  "projectId": "4ec4aa78-4ce0-4a77-aad1-5f74b66b1f5b",
  "stepNumber": 1,
  "stepData": {
    "evaluationGoals": ["Usability & Playability"],
    "projectType": "Concept test",
    "participants": "Limited set of participants",
    "developmentStage": "Concept idea"
  },
  "updatedAt": "2026-05-05T12:05:00Z",
  "wizardStatus": {
    "currentStep": 2,
    "isComplete": false,
    "steps": {
      "1": {
        "evaluationGoals": ["Usability & Playability"],
        "projectType": "Concept test",
        "participants": "Limited set of participants",
        "developmentStage": "Concept idea"
      }
    }
  }
}
```

## `POST /api/v1/projects/{projectId}/wizard/recommendations`

| Field | Value |
| --- | --- |
| Auth | Yes |
| Success | `200` `{ forStep, recommendations }` |
| Errors | `401 UNAUTHORIZED`, `400 INVALID_INPUT`, `400 INVALID_PROJECT_ID`, `400 INVALID_RECOMMENDATION_STEP`, `403 FORBIDDEN`, `404 PROJECT_NOT_FOUND`, `500 INTERNAL_SERVER_ERROR` |
| Notes | Step 2 and 3 are the supported recommendation targets in the current ruleset |

## `POST /api/v1/projects/{projectId}/generate-pdf`

| Field | Value |
| --- | --- |
| Auth | Yes |
| Success | `200` `{ pdfUrl, email }` |
| Errors | `401 UNAUTHORIZED`, `400 INVALID_INPUT`, `400 WIZARD_INCOMPLETE`, `400 INVALID_NOTIFY_EMAIL`, `403 FORBIDDEN`, `404 PROJECT_NOT_FOUND`, `500 PDF_GENERATION_FAILED` |
| Notes | `notifyEmail` is optional and the PDF URL points to public storage |

Request:

```json
{
  "notifyEmail": "qa@example.com"
}
```

Response:

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

The project must have a complete wizard before PDF generation succeeds.
