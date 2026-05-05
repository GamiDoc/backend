# Sessions

Session routes support the anonymous flow.

## `POST /api/v1/sessions/create`

| Field | Value |
| --- | --- |
| Auth | No |
| Success | `201` session object |
| Errors | `500 INTERNAL_SERVER_ERROR` |
| Notes | Creates an anonymous session with a fresh wizard |

Response:

```json
{
  "sessionId": "1f9f2b8d-1f0b-4c3c-9e2c-3dbd8f8b2d77",
  "wizardStatus": {
    "currentStep": 1,
    "isComplete": false,
    "steps": {}
  },
  "createdAt": "2026-05-05T12:00:00Z",
  "expiresAt": "2026-05-07T12:00:00Z"
}
```

## `GET /api/v1/sessions/{sessionId}`

| Field | Value |
| --- | --- |
| Auth | No |
| Success | `200` session object |
| Errors | `400 INVALID_SESSION_ID`, `404 SESSION_NOT_FOUND`, `500 INTERNAL_SERVER_ERROR` |
| Notes | Session may expire depending on stored TTL |

## `PUT /api/v1/sessions/{sessionId}/wizard/step/{stepNumber}`

| Field | Value |
| --- | --- |
| Auth | No |
| Success | `200` `{ sessionId, stepNumber, stepData, wizardStatus, createdAt, expiresAt }` |
| Errors | `400 INVALID_SESSION_ID`, `400 INVALID_STEP_NUMBER`, `400 INVALID_INPUT`, `400 INVALID_STEP_DATA`, `400 STEP_PREREQUISITE_NOT_MET`, `404 SESSION_NOT_FOUND`, `500 INTERNAL_SERVER_ERROR` |
| Notes | The server enforces step order and schema before saving |

Body example for step 1:

Request:

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
  "sessionId": "1f9f2b8d-1f0b-4c3c-9e2c-3dbd8f8b2d77",
  "stepNumber": 1,
  "stepData": {
    "evaluationGoals": ["Usability & Playability"],
    "projectType": "Concept test",
    "participants": "Limited set of participants",
    "developmentStage": "Concept idea"
  },
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
  },
  "createdAt": "2026-05-05T12:00:00Z",
  "expiresAt": "2026-05-07T12:00:00Z"
}
```

## `POST /api/v1/sessions/{sessionId}/wizard/recommendations`

| Field | Value |
| --- | --- |
| Auth | No |
| Success | `200` `{ forStep, recommendations }` |
| Errors | `400 INVALID_SESSION_ID`, `400 INVALID_INPUT`, `400 INVALID_RECOMMENDATION_STEP`, `404 SESSION_NOT_FOUND`, `500 INTERNAL_SERVER_ERROR` |
| Notes | The current ruleset supports `forStep` values 2 through 4 |

## `POST /api/v1/sessions/{sessionId}/convert`

| Field | Value |
| --- | --- |
| Auth | Yes |
| Success | `201` project object |
| Errors | `401 UNAUTHORIZED`, `400 INVALID_INPUT`, `400 INVALID_PROJECT_NAME`, `404 SESSION_NOT_FOUND`, `500 INTERNAL_SERVER_ERROR` |
| Notes | Creates a project from the session wizard state; the source session stays in place and the existing PDF reference is not copied |

Request:

```json
{
  "name": "Converted project",
  "description": "Created from an anonymous session"
}
```

Response:

```json
{
  "projectId": "f0d47d7a-5a4f-4f31-bd0d-63f1e5d40d4d",
  "name": "Converted project",
  "description": "Created from an anonymous session",
  "wizardStatus": {
    "currentStep": 4,
    "isComplete": true,
    "steps": {
      "1": {
        "evaluationGoals": ["Usability & Playability"],
        "projectType": "Concept test",
        "participants": "Limited set of participants",
        "developmentStage": "Concept idea"
      },
      "2": {
        "selectedMethods": ["think-aloud"]
      },
      "3": {
        "selectedInstruments": ["SUS"]
      },
      "4": {
        "nextSteps": ["Draft report"]
      }
    }
  },
  "pdfUrl": null,
  "createdAt": "2026-05-05T12:10:00Z",
  "updatedAt": "2026-05-05T12:10:00Z"
}
```

## `POST /api/v1/sessions/{sessionId}/generate-pdf`

| Field | Value |
| --- | --- |
| Auth | No |
| Success | `200` `{ pdfUrl, email }` |
| Errors | `400 INVALID_INPUT`, `400 WIZARD_INCOMPLETE`, `400 INVALID_NOTIFY_EMAIL`, `404 SESSION_NOT_FOUND`, `500 PDF_GENERATION_FAILED` |
| Notes | `notifyEmail` is optional and the PDF URL points to public storage |
