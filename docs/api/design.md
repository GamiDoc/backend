# Design System

The Design System component guides a user through the six sections of the GamiDoc design flow. Routes exist in two mirrored scopes:

- `/api/v1/sessions/{sessionId}/design/...` for the anonymous flow, no auth, state stored in Redis with the session TTL
- `/api/v1/projects/{projectId}/design/...` for authenticated users, Bearer auth plus ownership check, state stored in Postgres

Unless stated otherwise, every route in both scopes shares the guard errors below in addition to its own:

| Scope | Guard errors |
| --- | --- |
| Sessions | `400 INVALID_SESSION_ID`, `404 SESSION_NOT_FOUND`, `500 INTERNAL_SERVER_ERROR` |
| Projects | `401 UNAUTHORIZED`, `400 INVALID_PROJECT_ID`, `404 PROJECT_NOT_FOUND`, `403 FORBIDDEN`, `500 INTERNAL_SERVER_ERROR` |

## Flow Model

- Sections are, in order: 1 Context, 2 Experience Timeline, 3 Personalization & Dynamics, 4 Gameful Core, 5 Feedback & Evaluation, 6 Technical Requirements.
- On the first pass, sections unlock in order (1 to 6); a visited section can be re-saved at any time. Sections can also be skipped.
- After a full first pass, navigation is free and the dashboard unlocks (it needs at least one filled section).
- The AI provider is selected with `AI_PROVIDER` (`noop` default, or `openai-compatible` with `AI_BASE_URL`, `AI_API_KEY`, `AI_MODEL`). `AI_RESPONSE_FORMAT` says how JSON answers are requested: `json_schema` (strict structured outputs, default), `json_object` or `none` (schema only in the prompt) for providers without structured outputs. `AI_TIMEOUT` applies to each call.

## `GET .../design`

| Field | Value |
| --- | --- |
| Success | `200` design status object |
| Notes | Returns the full design state: spark, cursor, first-pass flag, sections, session-generated reports, `prefill` (AI prefill progress, see below). A `prefill` stored as `running` whose job no longer runs on this instance is returned as `failed` (`error: "interrupted"`) |

## `PUT .../design/spark`

| Field | Value |
| --- | --- |
| Body | `{ "spark": "..." }` |
| Success | `200` `{ "designStatus", "prefillStarted", "prefillApplied", "prefillFailed" }` |
| Errors | `400 INVALID_INPUT` |
| Notes | A non-empty spark, with a real AI provider and at least one empty section, starts the AI prefill **in the background** (`prefillStarted: true`) and returns at once. Poll `GET .../design` while `designStatus.prefill.state` is `running`. `prefillApplied` / `prefillFailed` are always `false` (kept for older clients). A new spark cancels the running job |

### AI prefill

The section schema (fields, options, conditions) comes from the frontend: `npm run export-schema` in `frontend_new/` writes `internal/design/schema/sections.json`, embedded in the binary. Run it after every change to `designSections.js`.

1. Analysis: one call turns the spark into `prefill.analysis` (`summary`, `domain`, `targetUsers`, `encouragedBehavior`, `goals`, `constraints`, `assumptions`, `gaps`, `language`).
2. Sections 1→6, one call each, in order. Each call gets the spark, the analysis and the content of the previous sections, and answers with a JSON Schema generated from the section schema (only valid options). Sections that already have content are kept (`skippedSections`) and passed as context.
3. Answers are sanitized before being stored: unknown keys and options are dropped (unknown options go to the field's "Other" text when it has one), fields hidden by `showIf` are dropped, lists are flattened as `"<list> <n> · <field>"`, mechanic links must name existing mechanics. Each section is stored as soon as it is ready; sections are never marked complete.

`prefill` object: `jobId`, `state` (`running` \| `done` \| `failed`), `currentSection` (0 = analysing), `completedSections`, `skippedSections`, `failedSections`, `error`, `startedAt`, `finishedAt`, `analysis`, `notes`. `notes` maps the section number to `[{ field, reason, confidence }]` (`confidence`: `high` stated in the spark, `medium` implied, `low` assumed by the AI), shown next to the fields in the interface.

## `PUT .../design/title`

Body: `{ "title": "GreenStep" }` (max 120 characters; `""` = back to the default).
Response: the design status (`title` included).

The title is the name printed on the documents and used in their file names:
`<Title>_design-report.pdf`, `<Title>_game-design-document.pdf` (letters and digits only, the
rest becomes `-`). Without a title the project name is used, else "Your project". The
evaluation plan is named `<project name>_evaluation-plan.pdf`.

## `PUT .../design/section/{sectionNumber}`

| Field | Value |
| --- | --- |
| Body | `{ "content": {...}, "complete": true\|false, "skip": true\|false }` |
| Success | `200` `{ "sectionNumber", "section", "designStatus" }` |
| Errors | `400 INVALID_INPUT`, `400 INVALID_SECTION_NUMBER`, `400 INVALID_SECTION_DATA`, `400 SECTION_LOCKED` |
| Notes | `content` is free-form JSON, field order is preserved in reports. `complete` is optional; omitting it keeps the current completion flag. `skip: true` marks the section visited without content |

## `GET .../design/dashboard`

| Field | Value |
| --- | --- |
| Success | `200` `{ "overallPercent", "firstPassDone", "sections": [{ "sectionNumber", "name", "description", "status", "percent" }] }` |
| Errors | `403 DASHBOARD_LOCKED` |
| Notes | Section status is `not_started` (no content, even if visited or skipped), `in_progress` (has content, not marked complete), or `complete`; the percentages are 0, 50, 100 |

## `POST .../design/generate-pdf`

| Field | Value |
| --- | --- |
| Body | `{ "format": "report" \| "gdd" }` (default `report`) |
| Success | `202` `{ "designStatus", "reportJob": { "jobId", "format", "state": "running", "startedAt" } }` |
| Errors | `400 INVALID_INPUT`, `400 INVALID_REPORT_FORMAT`, `403 DASHBOARD_LOCKED`, `500 INTERNAL_SERVER_ERROR` |
| Notes | Starts the generation of one final document **in the background** (one AI request, 2–4 minutes). `format`: `report`, `gdd`, or `evaluation` (projects only: the reasoned UX evaluation report, from the complete evaluation wizard and the design; `409 EVALUATION_INCOMPLETE` otherwise). Poll `GET .../design` while `reportJob.state` is `running`; when `done`, `reportJob.report` holds `{ reportId, version, url, createdAt }` (`version` = the format). A new request replaces the previous job. Project reports are recorded in `design_reports`; session reports are kept in the session design state |

### Final documents

Two formats, chosen by the user:

- `report` — **Design report**, a reasoned report with sources: the design at a glance (who, what, motivation, how we will know, main risk), core and long loop diagram, then one section per design section with the choices made, *why*, *in the design* and *watch out*, feedback cards, how the choices fit together, link to the evaluation, glossary and references.
- `gdd` — **Game Design Document**: design on one page (high concept, pillars, loops), players and Hexad types, core gameplay and rules, one card per game system (purpose, rules, values, feedback, risks, GamiDoc fields), progression and rewards, player journey, adaptation rules, feedback, technical requirements, risks, success metrics, rationale, glossary and references.

The AI answers with JSON constrained by a schema generated from the Go structs (`internal/design/document.go`). References come from a curated catalogue (`document_generate.go`): the AI cites them only by id, the document renumbers them; the glossary is built from the descriptions of the options chosen. A report without explanations is asked again once and otherwise completed with the entered data; without AI (or if it fails) the document lists the entered data only.

Documents are HTML (`internal/design/doctemplates`) printed by Chromium: Gotenberg when `PDF_HTML_RENDERER_URL` is set, otherwise a local Chrome/Edge (`PDF_CHROME_PATH`, empty = auto-detect, `none` = disabled); without either, a plain PDF. Uploaded design images are embedded in the document.

## `POST .../design/images`

| Field | Value |
| --- | --- |
| Body | `multipart/form-data` with the image in field `file` (PNG or JPEG, max 5 MB). The global body limit does not apply to this route |
| Success | `201` `{ "url" }` |
| Errors | `400 INVALID_INPUT` (no `file` field), `413 IMAGE_TOO_LARGE`, `415 UNSUPPORTED_IMAGE` (type sniffed from the content, not the file name), `500 INTERNAL_SERVER_ERROR` |
| Notes | Stored under `design/{sessions|projects}/{id}/images/` in the object store and served from the same public base URL as the PDFs, with `image/png` or `image/jpeg`. The client saves the returned `url` as a section value (e.g. `"Loop diagram"`). Removing it from a section does not delete the file |

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
| Body | `{ "sectionNumber": 1..6, "message": "..." }` |
| Success | `200` `{ "sectionNumber", "reply", "faq" }` |
| Errors | `400 INVALID_INPUT`, `400 INVALID_SECTION_NUMBER`, `502 AI_PROVIDER_ERROR` |

## `GET /api/v1/projects/{projectId}/design/reports`

| Field | Value |
| --- | --- |
| Success | `200` `{ "reports": [...], "total" }` |
| Notes | Project scope only; newest first. Each report has `fileName` (the PDF file name, empty for files stored before documents had names) and `missing` when the file is gone |

## `PATCH /api/v1/projects/{projectId}/design/reports/{reportId}`

| Field | Value |
| --- | --- |
| Body | `{ "name": "My report" }` |
| Success | `200` the report with its new `url` and `fileName` (`My-report.pdf`) |
| Errors | `400 INVALID_NAME`, `404 REPORT_NOT_FOUND`, `409 PDF_NOT_FOUND` (file no longer stored) |
| Notes | Renames the PDF file (it becomes the download name): letters, digits, `_` and `-` are kept, the rest becomes `-` |

## `DELETE /api/v1/projects/{projectId}/design/reports/{reportId}`

| Field | Value |
| --- | --- |
| Success | `204` |
| Errors | `404 REPORT_NOT_FOUND` |
| Notes | Deletes the report record and its PDF file |

## `POST /api/v1/projects/{projectId}/design/evaluation/ai-fill`

| Field | Value |
| --- | --- |
| Success | `202` `{ "wizardStatus" }` with `aiFill.state = "running"` |
| Errors | `409 DESIGN_NOT_READY` (no first pass of the design), `409 AI_FILL_RUNNING`, `503 AI_UNAVAILABLE` |
| Notes | The AI reads the design and fills in the four steps of the UX evaluation wizard, picking only options of the evaluation catalog (`internal/evaluation/catalog`, exported from the frontend with `npm run export-evaluation-catalog`). It runs in the background: poll `GET /projects/{projectId}` and read `wizardStatus.aiFill` (`jobId`, `state` running \| done \| failed, `startedAt`, `finishedAt`, `summary`, `notes`). `notes` maps the step ("1"–"3") to `[{ field, value, reason }]`, shown next to the choices. The steps are saved only when the job succeeds. |

## `POST /api/v1/projects/{projectId}/design/import-session`

| Field | Value |
| --- | --- |
| Body | `{ "sessionId": "..." }` |
| Success | `200` imported design status object |
| Errors | `400 INVALID_INPUT`, `404 SESSION_NOT_FOUND`, `409 SESSION_DESIGN_EMPTY`, `409 DESIGN_NOT_EMPTY` |
| Notes | Project scope only. Refuses to import an empty session state and refuses to overwrite existing project design content. Session-generated reports are migrated into `design_reports` under fresh ids, keeping their timestamps. The design state lives independently of the session record, so importing still works after the session was converted or deleted, until the state expires with the Redis TTL; `404 SESSION_NOT_FOUND` is returned only when neither the session nor any design state exists |

## Activity Events

Design routes are tracked by the activity middleware: `design_section_saved`, `design_pdf_generated`, `design_imported`; everything else falls back to `api_request`.
