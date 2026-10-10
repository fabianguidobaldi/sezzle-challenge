# Sezzle Calculator

A full-stack calculator: a React + TypeScript keypad frontend, and a Go REST microservice that does all the arithmetic using exact decimals, so `0.1 + 0.2 = 0.3`.

- [Quick start](#quick-start)
- [Development setup](#development-setup)
- [Tests and coverage](#tests-and-coverage)
- [API](#api)
- [Repository layout](#repository-layout)
- [Assumptions](#assumptions)
- [Design decisions](#design-decisions)
- [AI prompts](#ai-prompts)

## Quick start

With Docker installed:

```sh
make up        # or: docker compose up --build
```

Open **http://localhost:8080**. nginx serves the app and proxies `/api` to the Go backend, which isn't exposed on its own. Stop the stack with `make down` (or Ctrl+C, then `make down`).

To try it on a phone on the same network, open `http://<your-computer's-IP>:8080`.

## Development setup

**Prerequisites:** Go 1.26+, Node 24 (see `frontend/.nvmrc`) and GNU Make. Docker is only needed for `make up` and `make build`.

```sh
make frontend-install   # npm ci; Go fetches its modules on first run
```

Run each part in its own terminal:

```sh
make backend-dev    # API on http://localhost:8080
make frontend-dev   # app on http://localhost:5173, proxies /api to :8080
```

The frontend always calls relative `/api/...` URLs. The Vite dev server forwards them to the backend in development and nginx does the same in Docker, so neither needs CORS or an API URL setting.

Backend configuration: `PORT` (default `8080`).

### Make targets

| Target | What it does |
|---|---|
| `make backend-dev` | Run the API on :8080 |
| `make backend-test` | Backend tests (race detector), writes `backend/coverage/index.html` |
| `make backend-lint` | `gofmt` check and `go vet` |
| `make frontend-install` | Install frontend dependencies (`npm ci`) |
| `make frontend-dev` | Vite dev server |
| `make frontend-test` | Frontend tests, writes `frontend/coverage/index.html` |
| `make frontend-lint` | oxlint and TypeScript type check |
| `make check` | Lint and test both projects |
| `make build` | Build both Docker images |
| `make up` / `make down` | Start / stop the full stack with Docker Compose |

These are also the commands a CI/CD pipeline would run: `make frontend-install`, `make check`, then `make build`.

## Tests and coverage

```sh
make check
```

| Project | Tests | Statement coverage | Report |
|---|---|---|---|
| Backend (Go) | domain, HTTP layer, and an end-to-end test over HTTP with the real calculator | 96% | [`backend/coverage/index.html`](backend/coverage/index.html) |
| Frontend (Vitest + Testing Library + MSW) | 283 tests: API client, state machine, components, hooks, and end-to-end flows by click and keyboard | 100% | [`frontend/coverage/index.html`](frontend/coverage/index.html) |

The reports are committed. Open them in a browser to see covered and uncovered lines. The backend's uncovered statements are `main()` and error branches that can't be reached (e.g. a `Quantize` that can't fail).

## API

### `POST /api/v1/calculate`

```json
{ "operation": "divide", "operands": ["1", "3"] }
```

| Operation | Operands | Result |
|---|---|---|
| `add`, `subtract`, `multiply`, `divide` | `[a, b]` | a + b, a − b, a × b, a ÷ b |
| `power` | `[base, exponent]` | base^exponent |
| `sqrt` | `[x]` | √x |
| `percent` | `[x]` | x / 100 |
| `percent_of` | `[base, rate]` | base × rate / 100 ("rate% of base") |

- **Operands** are decimal strings in plain notation (`"-12.5"`), with at most 16 significant digits, 16 integer digits and 16 decimal places. JSON numbers are accepted too and their exact text is kept, but strings are recommended, because a JSON number becomes a float in most clients.
- **Response:** `200` with the `result` as a decimal string, and `exact: false` when the result may have been rounded (shown as `≈` in the UI).

```sh
$ curl -s localhost:8080/api/v1/calculate -H 'Content-Type: application/json' \
    -d '{"operation":"add","operands":["0.1","0.2"]}'
{"operation":"add","operands":["0.1","0.2"],"result":"0.3","exact":true}

$ curl -s localhost:8080/api/v1/calculate -H 'Content-Type: application/json' \
    -d '{"operation":"divide","operands":["1","3"]}'
{"operation":"divide","operands":["1","3"],"result":"0.3333333333333333","exact":false}

$ curl -s localhost:8080/api/v1/calculate -H 'Content-Type: application/json' \
    -d '{"operation":"percent_of","operands":["200","10"]}'
{"operation":"percent_of","operands":["200","10"],"result":"20","exact":true}

$ curl -s localhost:8080/api/v1/calculate -H 'Content-Type: application/json' \
    -d '{"operation":"multiply","operands":[1.5,4]}'
{"operation":"multiply","operands":["1.5","4"],"result":"6","exact":true}
```

### Errors

Every error has the same shape: `{"error": {"code": "...", "message": "..."}}`. A `400` means "fix your request"; a `422` means the request is valid but the maths has no answer.

| Status | Code | When |
|---|---|---|
| 400 | `INVALID_REQUEST` | malformed JSON, unknown or missing fields, wrong types |
| 400 | `UNKNOWN_OPERATION` | `operation` isn't one of the above |
| 400 | `INVALID_OPERAND_COUNT` | wrong number of operands |
| 400 | `INVALID_NUMBER` | an operand isn't a plain decimal within the format |
| 404 | `NOT_FOUND` | unknown path |
| 405 | `METHOD_NOT_ALLOWED` | not a `POST` (the `Allow` header says which method is) |
| 413 | `REQUEST_TOO_LARGE` | body over 4 KiB |
| 415 | `UNSUPPORTED_MEDIA_TYPE` | `Content-Type` isn't `application/json` |
| 422 | `DIVISION_BY_ZERO` | `x / 0`, `0 ^ -1` |
| 422 | `UNDEFINED_RESULT` | `√-4`, `0 ^ 0`, `(-8) ^ 0.5` |
| 422 | `RESULT_OUT_OF_RANGE` | result of 10¹⁶ or more, e.g. `2 ^ 64` |
| 500 | `INTERNAL_ERROR` | unexpected; details are logged, never returned |

```sh
$ curl -s localhost:8080/api/v1/calculate -H 'Content-Type: application/json' \
    -d '{"operation":"divide","operands":["1","0"]}'
{"error":{"code":"DIVISION_BY_ZERO","message":"division by zero"}}

$ curl -s localhost:8080/api/v1/calculate -H 'Content-Type: application/json' \
    -d '{"operation":"power","operands":["2","64"]}'
{"error":{"code":"RESULT_OUT_OF_RANGE","message":"result out of range: results must be less than 10^16 in magnitude"}}

$ curl -s localhost:8080/api/v1/calculate -H 'Content-Type: application/json' \
    -d '{"operation":"add","operands":["1","1e5"]}'
{"error":{"code":"INVALID_NUMBER","message":"operands[1]: must be a plain decimal number, without an exponent"}}

$ curl -s localhost:8080/api/v1/calculate -H 'Content-Type: application/json' \
    -d '{"operation":"sqrt","operands":["1","2"]}'
{"error":{"code":"INVALID_OPERAND_COUNT","message":"wrong number of operands: sqrt expects 1 operand, got 2"}}
```

### `GET /healthz`

```sh
$ curl -s localhost:8080/healthz
{"status":"ok"}
```

Used by the Docker healthcheck (`/server -healthcheck`, since the distroless image has no curl).

## Repository layout

```
backend/                Go module (stdlib HTTP + cockroachdb/apd)
  cmd/server/           entry point: config, graceful shutdown, -healthcheck
  internal/calculator/  domain: number format, operations, errors (no HTTP)
  internal/api/         HTTP: decoding, routing, error mapping, middleware
frontend/               Vite + React + TypeScript
  src/api/              typed API client
  src/calculator/       keypad state machine (pure reducer), keys, messages
  src/hooks/            useCalculator (API calls), useKeyboard
  src/components/       Calculator, Display, Keypad, Footer
  nginx.conf            static files + /api proxy
docs/
  specs/                backend, frontend and deployment specs
  plans/                staged implementation plans
  PROMPTS.md            the AI prompts used to build this
compose.yaml, Makefile
```

## Assumptions

- **Immediate execution:** each operator applies to the running result, like a basic physical calculator: `2 + 3 × 4 =` is `20`, not `14`.
- **Percentage** follows the pocket-calculator convention: after `+` or `−` it's a percentage of the running value (`200 + 10 % =` → `220`); otherwise it divides by 100 (`200 × 10 % =` → `20`).
- **Numbers** are plain decimals within a 16-digit pocket calculator's range: at most 16 significant digits, 16 integer digits and 16 decimal places.
  - Inputs outside the range are rejected, never silently rounded.
  - Results are exact when they fit. Otherwise they're rounded half-even, and too-small results become `0`.
  - Results of 10¹⁶ or more are errors.
- **`exact: false` means "may have been rounded".** It's conservative: `4 ^ 0.5` is computed through logarithms and reported as inexact even though it's `2`.
- **Special cases:**
  - `0 / 0` and `0 ^ -n` are division by zero.
  - `0 ^ 0` is undefined, as in the General Decimal Arithmetic spec.
  - `√` of a negative number is undefined.
- **The service is stateless:** no users, authentication, history or persistence.
- **Clients:** modern evergreen browsers. On phones, portrait is the main layout.

## Design decisions

Each decision lists what was chosen, the alternative that was discarded, and why.

### 1. Keypad-style UI

- **Chosen:** a calculator keypad (digits, operators, `=`, `AC/C`, `⌫`, `±`, `%`, `√`, `xʸ`), with physical keyboard support.
- **Discarded:** a form with two number inputs, an operation selector and a submit button.
- **Why:** a form maps 1:1 to the API and is trivial to validate, but it doesn't feel like a calculator. A keypad is the interface people already know. Its main cost is the key-sequence state machine, which is kept as a pure, fully unit-tested reducer.

### 2. Immediate execution

- **Chosen:** each operator evaluates the pending pair right away, like a basic physical calculator: `2 + 3 × 4 =` → `20`.
- **Discarded:** operator precedence, as on a phone calculator (`2 + 3 × 4 =` → `14`).
- **Why:** precedence needs either an expression builder on the client that sends calls in the right order, or an `/evaluate` endpoint with an expression parser on the backend. Both add a lot of scope for little benefit. Immediate execution maps each step to exactly one binary API call.

### 3. One endpoint for all operations

- **Chosen:** `POST /api/v1/calculate` with `{"operation": "...", "operands": [...]}`.
- **Discarded:** one endpoint per operation (`/add`, `/divide`, …).
- **Why:** a calculation is a function call, not a resource, so separate routes would spread the same handler across several paths. With one endpoint, the operation is data: the keypad sends whatever key was pressed, there's a single validation path and schema, and adding an operation means adding one registry entry.

### 4. Exact decimal arithmetic

- **Chosen:** [`cockroachdb/apd`](https://github.com/cockroachdb/apd) (General Decimal Arithmetic), with numbers sent as **strings** in JSON.
- **Discarded:**
  - `float64`, where `0.1 + 0.2 = 0.30000000000000004`.
  - [`shopspring/decimal`](https://github.com/shopspring/decimal), which has no square root and doesn't report rounding or overflow conditions.
- **Why:** correctness is the priority. Operands and results are strings because a JSON number becomes an IEEE float in JavaScript and would lose precision before the backend ever saw it.

### 5. Number format: plain decimals, 16 digits

- **Chosen:** one number format shared by operands, results and the display: plain decimals with at most 16 significant digits, 16 integer digits and 16 decimal places. This is the range of a 16-digit pocket calculator.
  - A result is exact whenever it fits.
  - Otherwise it's correctly rounded (half-even) and returned with `"exact": false`, and the UI shows `≈`. Results too small for 16 decimal places round to `0`.
  - Results of 10¹⁶ or more are a `RESULT_OUT_OF_RANGE` error (e.g. `2^64`).
  - Operands outside the format are rejected, never silently rounded.
- **Discarded:**
  - 34 digits (IEEE decimal128) for rounded operations plus 100 digits for exact ones.
  - Exponent notation (`1.5E+40`) with an exponent range of ±9999.
- **Why:** correctness comes from the decimal representation and honest rounding, not from the number of digits. `1 ÷ 3 × 3` can't come back as exactly `1` at any finite precision. Thirty-four digits don't fit on a phone display, and two precision levels added complexity for no visible benefit.

  Exponent notation and a huge range brought edge cases nobody using a keypad needs: results too small to reuse, intermediate overflow in `percent_of`, and values the keypad can't type that the display, `±` and backspace would all have to handle. With plain decimals, the only number format is the one the keypad can produce, and the longest value is 19 characters.

### 6. Percentage as two explicit operations

- **Chosen:** the backend exposes `percent(x) = x / 100` and `percent_of(base, rate) = base × rate / 100`. The frontend picks one from context, following the usual calculator convention:
  - `200 + 10 % =` → `220`
  - `200 × 10 % =` → `20`
  - `50 %` → `0.5`
- **Discarded:**
  - a single `percentage` operation with 1 or 2 operands
  - computing percentages on the client
- **Why:** each operation keeps a fixed arity and a clear meaning. The context-dependent behaviour is a property of the keypad, so it belongs in the frontend, while the arithmetic stays on the backend.

### 7. The client never does arithmetic

- **Chosen:** the frontend keeps every number as a string and sends every calculation to the API. The keypad logic is a pure `useReducer` state machine that describes the next API call as data, and a small hook performs it.
- **Discarded:** doing simple steps (e.g. `÷ 100`) in JavaScript to save a request.
- **Why:** one source of truth for the maths and no float errors in the browser. The reducer can be tested exhaustively without a network.

### 8. nginx + Docker Compose instead of Go serving the SPA

- **Chosen:** two containers. nginx serves the built SPA and proxies `/api` to the Go backend, which isn't exposed and stays a pure JSON API. `docker compose up` starts everything.
- **Discarded:**
  - A single image where Go also serves the static files. This is the most literal reading of "a Dockerfile".
  - Embedding the SPA in the Go binary.
  - nginx and Go in one container, which would need a process supervisor.
- **Why:** serving files from Go would mix static-file and SPA-fallback code into the calculation microservice. It would also tie frontend releases to backend releases and miss what nginx provides for free (caching headers, compression). The chosen setup mirrors production (static host or CDN plus an API behind a proxy) and still runs with one command.

### 9. Same-origin everywhere, no CORS

- **Chosen:** the browser only ever calls relative `/api/...` URLs. The Vite dev server proxies them in development and nginx proxies them in Docker.
- **Discarded:** enabling CORS on the backend and configuring an API base URL in the frontend.
- **Why:** less configuration and nothing that differs between environments. The backend has no CORS code at all.

### 10. Go standard library for HTTP

- **Chosen:** `net/http` with Go 1.22+ routing patterns, plus `log/slog` for logging.
- **Discarded:** a router or web framework (chi, gin, echo).
- **Why:** the API has two routes, so a framework would add a dependency without adding value. `apd` is the only third-party dependency.

### 11. Error contract

- **Chosen:** every error returns `{"error": {"code", "message"}}`, with a machine-readable code.
  - `400`: the request is malformed (bad JSON, unknown operation, wrong operand count, invalid number).
  - `422`: the request is well formed but the maths has no answer (division by zero, undefined result, out of range).
- **Discarded:** returning `400` for everything, or plain-text errors.
- **Why:** clients can tell "fix your request" apart from "this calculation has no result", and the frontend maps codes to friendly messages without parsing text.

### 12. Keys typed during a calculation are queued, not dropped

- **Chosen:** while a request is in flight, key presses are queued in the reducer's state and replayed in order when the response arrives. A failure or AC discards them.
- **Discarded:**
  - Ignoring input while busy (the original spec).
  - Disabling the keypad, which makes it flicker on every calculation.
- **Why:** found by testing in a real browser. Typing `200+10%=` quickly lost the `=`, because it arrived before the `%` result, and the display ended on `20` instead of `220`. Unit tests with an instant fake API couldn't catch it. A new end-to-end test types with no delay between keys.

### Out of scope

- A CI/CD pipeline. The Make targets are ready for one (see [Make targets](#make-targets)).
- Repeated `=` (re-applying the last operation) and `2 + =` (applying the operation to the displayed value).
- Calculation history.

## AI prompts

This project was built with Claude Code. The prompts used, and what came out of each, are in [`docs/PROMPTS.md`](docs/PROMPTS.md). The specs and staged plans those prompts produced are in [`docs/specs/`](docs/specs/) and [`docs/plans/`](docs/plans/).
