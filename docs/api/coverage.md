# Requirement Coverage And Gaps

## Coverage Snapshot

| Phase | Status | Notes |
| --- | --- | --- |
| Phase 1, anonymous MVP | Passed | Anonymous session flow passed API testing, including wizard saves, recommendations, PDF generation, and PDF download |
| Phase 2, auth and project work | Partial | Register and login routes exist, protected routes enforce bearer auth, but valid registration returned `500` in the deployed test |
| Phase 3, session conversion | Partial | Conversion route exists and creates a project from a session, but the current implementation keeps the source session and does not copy an existing PDF reference |
| Phase 4, refinement and optimization | Partial | Core routes exist, but logging, monitoring, performance, and PDF format requirements were not fully verified |

## Observed Test Results

- `GET /health` returned `200`
- `GET /ready` returned `200` with Postgres and Redis marked `ok`
- `GET /api/v1/ping` returned `200`
- `GET /` returned `404`
- Invalid registration data returned `400`
- Valid registration returned `500` in the deployed test environment
- Login for a missing user returned `401`
- Protected routes without a token returned `401`
- Browser UI testing was blocked because Playwright Chrome was missing locally

## Known Gaps

- No canonical frontend is mounted at the root URL, so the deployment behaves like an API only backend
- Project listing has no pagination
- The step 4 recommendation route exists, but the current recommendation rules file does not define step 4 rules
- PDF download uses a public storage URL instead of an authenticated `/download-pdf` route
- Production object storage and email are not enabled on the test deployment
- PDF/A and PDF 1.7 output were not verified
- Logging, monitoring, and performance requirements were not fully verified

## Current Contract Notes

- `GET /files/pdfs/*` serves public PDF bytes
- The error envelope is always `{ "error": { "code", "message", "details" } }`
- `OPTIONS` requests return `204 No Content`; CORS allow headers are added only for allowed origins
