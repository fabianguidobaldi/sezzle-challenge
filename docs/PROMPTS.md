# Prompts

These is a brief summary of prompts that I've used with Claude Code (Claude Opus 5.5) while building this project. It aims to reflect my way of working without going on the discussion details, and a short summary of the outcome generated.

---

## 1. Planning

### Input

> We'll build a calculator app using Go and React. Here are the requirements: _(challenge requirements pasted)_. First, generate a plan (including a proposed folder structure) that includes all relevant design decisions.

### Outcome

Claude proposed a repo structure: `backend/` (Go), `frontend/` (React + TS), a root `Dockerfile`, a `Makefile` and a `README.md`.

Also proposed a 6-step plan:

1. Backend domain + tests
2. Backend HTTP layer + tests
3. Frontend scaffold + API client + tests
4. Keypad UI + tests
5. Dockerfile + Makefile
6. README

Agreed decisions:

- A single endpoint, `POST /api/v1/calculate`, with the operation in the request body. Use Go's http stdlib.
- Exact decimals using `cockroachdb/apd`, with numbers sent as strings. Results that can't be exact (e.g. `1/3`) are rounded to 34 digits and flagged as `exact: false`.
- A keypad-style UI with immediate execution (`2 + 3 × 4 = 20`).

## 2. Specs

### Input

> Based on that plan, generate the specs for both the backend (APIs and domain layer design) and the frontend (UI and API calls).

### Outcome

Claude wrote `docs/specs/backend.md`, `docs/specs/frontend.md` and `docs/specs/deployment.md`.

Agreed decisions:

- Percentage as two backend operations, `percent(x)` and `percent_of(base, rate)`. The frontend picks one from context (`200 + 10 % =` → `220`, `200 × 10 % =` → `20`).
- A single precision of 16 significant digits instead of 34/100. Operands longer than that are rejected, and rounded results are flagged as `exact: false`.
- nginx serves the SPA and proxies `/api` to the Go backend, run with Docker Compose. The backend stays a pure JSON API, with no CORS.

## 3. Backend implementation

### Input

> Based on the backend spec, create a plan to split the implementation into independent stages.

> Using that plan, implement the backend. Use subagents to parallelize implementations when possible.

### Outcome

Claude saved a 6-stage plan in `docs/plans/backend.md` and implemented it. Claude wrote stage 1 (the shared domain types and errors) first, then two subagents built stage 2 (operations) and stages 3–4 (HTTP layer) in parallel, while Claude wrote stages 5–6 (server binary, Dockerfile, Makefile). Each stage is one commit. Coverage is 96%.

It generated also an Insomnia collection for manual API testing.

Agreed decisions:

- Plain decimals only, with no exponent notation: at most 16 significant digits, 16 integer digits and 16 decimal places. Tiny results round to `0`, and results of 10¹⁶ or more are out of range.

## 4. Frontend implementation

### Input

> Generate a plan to implement the frontend in independent stages.

### Outcome

Claude saved a 6-stage plan in `docs/plans/frontend.md` and implemented it. Claude wrote stage 1 (scaffold, test setup and shared types) first, then three subagents built the API client, the calculator reducer and the presentational UI in parallel, while Claude wrote the hooks and wiring, and the nginx and Docker Compose setup. Each stage is one commit. Coverage is 100%.

Claude also checked the app in a real browser (desktop and phone widths, and the Docker stack with no CSP errors) and found that keys typed during an in-flight request were lost. Those keys are now queued and replayed when the response arrives.
