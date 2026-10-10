# Sezzle Calculator

A full-stack calculator with a React + TypeScript keypad frontend and a Go REST microservice that does all the arithmetic, using exact decimals.

> Setup instructions, how to run each part, API examples and coverage reports will be added as the implementation lands. The detailed specs are in [`docs/specs/`](docs/specs/):
> [backend](docs/specs/backend.md) · [frontend](docs/specs/frontend.md) · [deployment](docs/specs/deployment.md).

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

### Out of scope (for now)

- CI pipeline
- Repeated `=` (re-applying the last operation) and `2 + =` (applying the operation to the displayed value)
- Calculation history
