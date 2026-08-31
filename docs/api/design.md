# Design System

The Design System component guides a user through the seven sections of the GamiDoc design flow. Routes exist in two mirrored scopes:

- `/api/v1/sessions/{sessionId}/design/...` for the anonymous flow, no auth, state stored in Redis with the session TTL
- `/api/v1/projects/{projectId}/design/...` for authenticated users, Bearer auth plus ownership check, state stored in Postgres

Unless stated otherwise, every route in both scopes shares the guard errors below in addition to its own:

| Scope | Guard errors |
| --- | --- |
| Sessions | `400 INVALID_SESSION_ID`, `404 SESSION_NOT_FOUND`, `500 INTERNAL_SERVER_ERROR` |
| Projects | `401 UNAUTHORIZED`, `400 INVALID_PROJECT_ID`, `404 PROJECT_NOT_FOUND`, `403 FORBIDDEN`, `500 INTERNAL_SERVER_ERROR` |

## Flow Model

- Section 1 (Context) is always first. After it, the user picks path `A` (1,2,3,4,5,6,7) or `B` (1,4,5,2,3,6,7).
- On the first pass, sections unlock in path order; a visited section can be re-saved at any time. Sections can also be skipped.
- After a full first pass, navigation is free and the dashboard unlocks (it needs at least one filled section).
- The AI provider is selected with `AI_PROVIDER` (`noop` default, or `openai-compatible` with `AI_BASE_URL`, `AI_API_KEY`, `AI_MODEL`).

## `GET .../design`

| Field | Value |
| --- | --- |
| Success | `200` design status object |
| Notes | Returns the full design state: spark, path, cursor, first-pass flag, sections, session-generated reports |

## `PUT .../design/spark`

| Field | Value |
| --- | --- |
| Body | `{ "spark": "..." }` |
| Success | `200` `{ "designStatus", "prefillApplied", "prefillFailed" }` |
| Errors | `400 INVALID_INPUT` |
| Notes | A non-empty spark asks the AI provider to prefill empty sections; user-entered content is never overwritten. `prefillFailed` is `true` when the provider errored, the spark itself is still saved |

## `GET .../design/branch`

| Field | Value |
| --- | --- |
| Success | `200` `{ "recommendedPath": "A"\|"B", "basis": "spark"\|"default" }` |
| Errors | `502 AI_PROVIDER_ERROR` |
| Notes | Without a spark the default recommendation is path `A` |

## `POST .../design/path`

| Field | Value |
| --- | --- |
| Body | `{ "path": "A"\|"B" }` |
| Success | `200` design status object |
| Errors | `400 INVALID_INPUT`, `400 INVALID_PATH`, `400 SECTION_LOCKED`, `409 PATH_ALREADY_CHOSEN` |
| Notes | Allowed only right after the Context section on the first pass |

## `PUT .../design/section/{sectionNumber}`

| Field | Value |
| --- | --- |
| Body | `{ "content": {...}, "complete": true\|false, "skip": true\|false }` |
| Success | `200` `{ "sectionNumber", "section", "designStatus" }` |
| Errors | `400 INVALID_INPUT`, `400 INVALID_SECTION_NUMBER`, `400 INVALID_SECTION_DATA`, `400 SECTION_LOCKED`, `400 PATH_NOT_CHOSEN` |
| Notes | `content` is free-form JSON, field order is preserved in reports. `complete` is optional; omitting it keeps the current completion flag. `skip: true` marks the section visited without content |

## `GET .../design/dashboard`

| Field | Value |
| --- | --- |
| Success | `200` `{ "overallPercent", "firstPassDone", "path", "sections": [{ "sectionNumber", "name", "description", "status", "percent" }] }` |
| Errors | `403 DASHBOARD_LOCKED` |
| Notes | Section status is `not_started` (no content, even if visited or skipped), `in_progress` (has content, not marked complete), or `complete`; the percentages are 0, 50, 100 |

## `POST .../design/generate-pdf`

| Field | Value |
| --- | --- |
| Body | optional; when present it must be valid JSON |
| Success | `200` `{ "standard": { "reportId", "version", "url", "createdAt" }, "enhanced": {...} }` |
| Errors | `400 INVALID_INPUT`, `403 DASHBOARD_LOCKED`, `502 AI_PROVIDER_ERROR` (AI failures), `500 INTERNAL_SERVER_ERROR` (build, storage, or database failures) |
| Notes | One trigger produces both report versions: `standard` mirrors the form content in order, `enhanced` is AI-consolidated prose with cross-section deduplication. Project reports are recorded in `design_reports`; session reports are kept in the session design state |

## `GET .../design/faq/{sectionNumber}`

| Field | Value |
| --- | --- |
| Success | `200` `{ "sectionNumber", "faq": [{ "question", "answer" }] }` |
| Errors | `400 INVALID_SECTION_NUMBER` |

## `POST .../design/ai/rewrite`

| Field | Value |
| --- | --- |
| Body | `{ "text": "..." }` |
| Success | `200` `{ "text", "previousText" }` |
| Errors | `400 INVALID_INPUT`, `400 EMPTY_TEXT`, `502 AI_PROVIDER_ERROR` |

## `POST .../design/ai/chat`

| Field | Value |
| --- | --- |
| Body | `{ "sectionNumber": 1..7, "message": "..." }` |
| Success | `200` `{ "sectionNumber", "reply", "faq" }` |
| Errors | `400 INVALID_INPUT`, `400 INVALID_SECTION_NUMBER`, `502 AI_PROVIDER_ERROR` |

## `GET /api/v1/projects/{projectId}/design/reports`

| Field | Value |
| --- | --- |
| Success | `200` `{ "reports": [...], "total" }` |
| Notes | Project scope only; newest first |

## `POST /api/v1/projects/{projectId}/design/import-session`

| Field | Value |
| --- | --- |
| Body | `{ "sessionId": "..." }` |
| Success | `200` imported design status object |
| Errors | `400 INVALID_INPUT`, `404 SESSION_NOT_FOUND`, `409 SESSION_DESIGN_EMPTY`, `409 DESIGN_NOT_EMPTY` |
| Notes | Project scope only. Refuses to import an empty session state and refuses to overwrite existing project design content. Session-generated reports are migrated into `design_reports` under fresh ids, keeping their timestamps. The design state lives independently of the session record, so importing still works after the session was converted or deleted, until the state expires with the Redis TTL; `404 SESSION_NOT_FOUND` is returned only when neither the session nor any design state exists |

## Activity Events

Design routes are tracked by the activity middleware: `design_section_saved`, `design_path_chosen`, `design_pdf_generated`, `design_imported`; everything else falls back to `api_request`.
