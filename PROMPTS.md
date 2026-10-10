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
