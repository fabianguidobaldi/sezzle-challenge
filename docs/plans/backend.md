# Backend implementation plan

The [backend spec](../specs/backend.md) is built in independent stages. Each stage builds, passes its own tests and lands as one commit. Once stage 1 has fixed the domain's public types and errors, the domain track (2) and the HTTP track (3 → 4) can proceed in parallel.

```
            ┌──► 2. Operations + Calculate ───────────┐
1. Domain ──┤                                         ├──► 5. Server binary ──► 6. Container + tooling
   contract └──► 3. HTTP contract ──► 4. Handlers ────┘
```

## Stage 1: Domain contract and decimal foundation

- **Files:**
  - `go.mod` (module path plus `apd/v3`)
  - `calculator/calculator.go` (types only: `Operation` and its constants, `Result`, `Precision`, the context)
  - `calculator/errors.go`
  - `calculator/decimal.go`
- **Scope:**
  - `parseOperand` (§2.4): 64-character limit, rejects NaN and Infinity, at most 16 significant digits after trailing zeros are removed, exponent range.
  - `formatResult` (§2.5): removes trailing zeros, turns `-0` into `0`, chooses plain or scientific notation.
  - Sentinel errors and `OperandError` (§2.6).
- **Tests:** table-driven tests for every §2.4 and §2.5 rule, and a round-trip test (a formatted result always parses back as a valid operand).
- **Done when:** the types and error values match the spec exactly, so stages 2 and 3 can build on them without changing them.

## Stage 2: Operations and `Calculate`

*Depends on 1. Runs in parallel with 3 and 4.*

- **Files:** `operations.go`, plus the registry and `New`/`Calculate` in `calculator.go`.
- **Scope:**
  - The 8 operation functions.
  - Checks that run before calling apd:
    - `0 ^ -n` → division by zero (apd would return Infinity silently)
    - `√` of a negative number → undefined
    - |exponent| > 9999 → out of range
  - Registry and arity check.
  - Mapping apd conditions to domain errors:
    - overflow and underflow → out of range
    - invalid operation → undefined
    - division by zero, including `0/0` → division by zero
  - The `Exact` flag.
- **Tests:**
  - every row of the §2.7 table, plus normal cases for each operation
  - unknown operation and wrong arity
  - a concurrency smoke test with `-race`

## Stage 3: HTTP contract (decoding and encoding)

*Depends on 1.*

- **Files:** `api/request.go`, `api/response.go`.
- **Scope:**
  - `operand` JSON type: accepts a string, or a number whose original text is kept (no float conversion).
  - `decodeCalculateRequest`:
    - Content-Type check (415)
    - 4 KiB body limit (413)
    - rejects unknown fields, trailing data and nulls
    - requires both fields
  - `writeJSON` and `writeError`.
  - One table mapping errors to status and code (§3.2).
- **Tests:** decoding cases, and a check that every domain error maps to the expected status and code.

## Stage 4: Handlers, router and middleware

*Depends on 3. Uses a stub calculator, so it doesn't need stage 2.*

- **Files:** `api/calculate.go`, `api/router.go`, `api/middleware.go`.
- **Scope:**
  - The `Calculator` interface, defined on the consumer side.
  - Routes:
    - `POST /api/v1/calculate`
    - other methods → JSON 405 with an `Allow` header
    - `GET /healthz`
    - any other path → JSON 404
  - Recover middleware (500, details only logged) and `slog` request logging.
- **Tests:** `httptest` against `NewRouter`: every status, the response shapes, and panic → 500.

## Stage 5: Server binary and integration

*Depends on 2 and 4.*

- **Files:** `cmd/server/main.go`, plus an integration test.
- **Scope:** `PORT`, server timeouts, JSON `slog`, graceful shutdown on SIGINT/SIGTERM, and the `-healthcheck` flag.
- **Tests:**
  - integration test with the real calculator behind `NewRouter`, replaying the key §2.7 cases over HTTP
  - healthcheck against an `httptest` server
- **Manual check:** `go run ./cmd/server` plus `curl`. Keep the commands for the README.

## Stage 6: Container and tooling

*Depends on 5.*

- **Files:** `backend/Dockerfile`, `backend/.dockerignore`, root `Makefile` (backend targets).
- **Scope:**
  - Make targets:
    - `make test` (race detector)
    - `make cover` (`coverage.out` plus an HTML report)
    - `make lint` (`gofmt` check plus `go vet`)
  - Multi-stage build into a distroless image.
- **Tests:** `docker build`; `docker run`, with the healthcheck reporting healthy; `curl` against the container.

## Every stage is done when

- `gofmt` and `go vet` are clean.
- `go test -race ./...` passes.
- Coverage of the package that stage changes is at least 90%.
- It's committed as one commit, with a message naming the stage.
