# Backend spec

A Go REST microservice that performs exact decimal arithmetic. HTTP uses only the standard library; [`cockroachdb/apd/v3`](https://github.com/cockroachdb/apd) is the one third-party dependency, used for decimal math.

## 1. Package layout

```
backend/
├── go.mod
├── Dockerfile                  # go build → distroless (see deployment spec)
├── cmd/server/main.go          # config, wiring, HTTP server, graceful shutdown, -healthcheck
└── internal/
    ├── calculator/             # domain: pure, no HTTP, no I/O
    │   ├── calculator.go       # Calculator, Calculate(), operation registry
    │   ├── operations.go       # one function per operation
    │   ├── decimal.go          # parsing (with limits) and formatting
    │   ├── errors.go           # sentinel errors + OperandError
    │   └── *_test.go
    └── api/                    # transport: HTTP <-> domain
        ├── router.go           # NewRouter(): routes + middleware chain
        ├── calculate.go        # POST /api/v1/calculate handler
        ├── request.go          # request decoding + operand JSON type
        ├── response.go         # writeJSON / writeError, error → status mapping
        ├── middleware.go       # recover, request logging
        └── *_test.go
```

Module path: `github.com/fabianguidobaldi/sezzle-challenge/backend`.

Dependencies point one way: `cmd/server → api → calculator`. The `calculator` package doesn't import `api`, and `api` never imports `apd`, because the domain API uses strings only.

## 2. Domain layer (`internal/calculator`)

### 2.1 Public API

```go
type Operation string

const (
    Add       Operation = "add"
    Subtract  Operation = "subtract"
    Multiply  Operation = "multiply"
    Divide    Operation = "divide"
    Power     Operation = "power"
    Sqrt      Operation = "sqrt"
    Percent   Operation = "percent"    // x / 100
    PercentOf Operation = "percent_of" // base × rate / 100
)

type Result struct {
    Value string // canonical decimal string (see §2.5)
    Exact bool   // false if the value may have been rounded
}

type Calculator struct { ctx apd.Context }

func New() *Calculator

// Calculate validates the operation and operands, computes the result and formats it.
// It returns one of the errors listed in §2.6.
func (c *Calculator) Calculate(op Operation, operands []string) (Result, error)
```

`Calculator` is stateless after construction and safe for concurrent use. Each call creates its own `apd.Decimal` values; the context is read-only.

### 2.2 Operation registry

```go
type operation struct {
    arity int
    apply func(c *Calculator, args []*apd.Decimal) (*apd.Decimal, apd.Condition, error)
}

var registry = map[Operation]operation{
    Add:       {2, add},
    Subtract:  {2, subtract},
    Multiply:  {2, multiply},
    Divide:    {2, divide},
    Power:     {2, power},
    Sqrt:      {1, sqrt},
    Percent:   {1, percent},
    PercentOf: {2, percentOf},
}
```

`Calculate` runs these steps:
1. Look up the operation, or return `ErrUnknownOperation`.
2. Check the arity, or return `ErrOperandCount`.
3. Parse each operand (§2.4), or return `*OperandError`.
4. Call `apply`.
5. Map the apd condition or error to a domain error (§2.6).
6. Format the result (§2.5).

Adding an operation means adding one function and one registry entry.

### 2.3 Precision model

One `apd.Context`, shared by every operation:

```go
const Precision = 16 // significant digits; the single knob for operands, results and display

ctx := apd.Context{Precision: Precision, Rounding: apd.RoundHalfEven, MaxExponent: 9999, MinExponent: -9999,
    Traps: apd.Overflow | apd.Underflow | apd.DivisionByZero | apd.DivisionUndefined | apd.InvalidOperation}
```

- **Exact whenever the result fits in 16 significant digits.** That covers every add, subtract, multiply, percent and percent_of whose result has 16 digits or fewer (dividing by 100 only shifts the decimal point), and any divide, sqrt or power that happens to terminate (`1/4`, `√16`, `2^-3`).
- **Otherwise correctly rounded** (half-even) to 16 significant digits: `1/3`, `√2`, `2^64`, and products whose result has more than 16 digits.
- `Exact = !condition.Inexact()`. This is conservative: apd computes fractional powers with exp/ln, so `4 ^ 0.5` reports `exact: false` even though the result is 2. `exact: false` means "may have been rounded".

### 2.4 Parsing operands (`decimal.go`)

`parseOperand(s string) (*apd.Decimal, error)` accepts the decimal syntax of the General Decimal Arithmetic spec as implemented by `apd.NewFromString`: optional sign, digits with an optional `.`, optional exponent. Examples: `"42"`, `"-0.5"`, `".5"`, `"1.5E+40"`.

It rejects:
- empty strings, or strings longer than **64 characters**
- anything `apd` can't parse (`"abc"`, `"1e"`, `" 1"`, `"1_000"`)
- non-finite values: `NaN` and `Infinity` parse successfully in apd and are rejected explicitly
- more than **`Precision` (16) significant digits**, counted after removing trailing zeros, so `"1.000"` and `"1E+20"` are fine. Inputs are rejected, never silently rounded.
- an adjusted exponent outside `[-9999, 9999]`

Every rejection returns `&OperandError{Index: i, Value: s, Reason: "..."}`, which wraps `ErrInvalidNumber`.

### 2.5 Formatting results (`decimal.go`)

`formatResult(d *apd.Decimal) string`:
1. `Reduce`: remove trailing zeros (`3.00 → 3`, `0.2500 → 0.25`).
2. Turn negative zero into `"0"`.
3. If the adjusted exponent is in `[-7, Precision)`, use plain notation (`d.Text('f')`), e.g. `"1000"`, `"0.125"`, `"1234567890123456"`. Otherwise use scientific notation (`d.String()`), e.g. `"1.844674407370955E+19"`, `"1.5E-8"`. Values with more than 16 integer digits are never written out in full, because padding with zeros would suggest precision the value doesn't have.

A formatted result never has more than 16 significant digits and always satisfies §2.4, so the frontend can send any result back as the next operand.

### 2.6 Errors (`errors.go`)

```go
var (
    ErrUnknownOperation = errors.New("unknown operation")
    ErrOperandCount     = errors.New("wrong number of operands")
    ErrInvalidNumber    = errors.New("invalid number")
    ErrDivisionByZero   = errors.New("division by zero")
    ErrUndefined        = errors.New("result is undefined")
    ErrOutOfRange       = errors.New("result out of range")
)

type OperandError struct { Index int; Value, Reason string } // Unwrap() → ErrInvalidNumber
```

Errors are wrapped with `fmt.Errorf("%w: ...")` so their messages are specific, and they're matched with `errors.Is` or `errors.As`.

### 2.7 Edge cases

| Input                       | Outcome                                   | Note                                          |
| --------------------------- | ----------------------------------------- | --------------------------------------------- |
| `0.1 + 0.2`                 | `0.3`, exact                              | the reason for using decimals                 |
| `x / 0`, `0 / 0`            | `ErrDivisionByZero`                       | apd reports these as division by zero / undefined |
| `0 ^ -n`                    | `ErrDivisionByZero`                       | **checked before calling apd**, which returns `Infinity` with no error |
| `0 ^ 0`                     | `ErrUndefined`                            | invalid operation under the GDA spec          |
| `√(-x)`                     | `ErrUndefined` ("square root of a negative number") | checked before calling apd          |
| `(-8) ^ 0.5`                | `ErrUndefined`                            | invalid operation                             |
| `1E+9000 × 1E+9000`         | `ErrOutOfRange`                           | overflow                                      |
| `1E-9000 × 1E-9000`         | `ErrOutOfRange`                           | underflow (not silently 0)                    |
| `x ^ y` with \|y\| > 9999   | `ErrOutOfRange`                           | checked first; bounds CPU use                 |
| `2^64`                      | `1.844674407370955E+19`, inexact          |                                               |
| `2.50 - 2.50`               | `0`                                       | reduced                                       |
| `-0 × 5`                    | `0`                                       | negative zero normalized                      |
| `1 / 3`                     | `0.3333333333333333` (16 digits), inexact |                                               |
| `percent(50)`               | `0.5`, exact                              |                                               |
| `percent_of(200, 10)`       | `20`, exact                               | "10% of 200"                                  |
| `percent_of(-80, 12.5)`     | `-10`, exact                              | negative values and fractional rates are fine |
| `percent(1E-9999)`          | `ErrOutOfRange`                           | underflow                                     |

## 3. HTTP API (`internal/api`)

### 3.1 Endpoints

| Method | Path                | Description                         |
| ------ | ------------------- | ----------------------------------- |
| POST   | `/api/v1/calculate` | Perform one calculation             |
| GET    | `/healthz`          | Liveness: `200 {"status":"ok"}`     |

The backend is a pure JSON API and serves no static files. Every response is JSON, including 404 for unknown paths and 405. The calculate route is registered without a method so the handler can return a JSON 405 with an `Allow: POST` header; the standard mux would return plain text.

### 3.2 `POST /api/v1/calculate`

Request (`Content-Type: application/json`, maximum 4 KiB):

```json
{ "operation": "divide", "operands": ["1", "3"] }
```

- `operation` (string, required) is one of `add`, `subtract`, `multiply`, `divide`, `power`, `sqrt`, `percent`, `percent_of`.
- `operands` (array, required) has 2 items, or 1 for `sqrt` and `percent`. Each item should be a **decimal string**. JSON numbers are also accepted for convenience: the custom `operand` type's `UnmarshalJSON` keeps the number's original text, so it's never converted to a float. `power` operands are `[base, exponent]`; `percent_of` operands are `[base, rate]`.
- Unknown fields, trailing data after the JSON object, and `null` values are rejected.

Success, `200 OK`:

```json
{ "operation": "divide", "operands": ["1", "3"], "result": "0.3333333333333333", "exact": false }
```

Errors use the same shape everywhere:

```json
{ "error": { "code": "DIVISION_BY_ZERO", "message": "division by zero" } }
```

| Status | Code                     | When                                                          |
| ------ | ------------------------ | ------------------------------------------------------------- |
| 400    | `INVALID_REQUEST`        | malformed JSON, wrong types, unknown fields, missing fields   |
| 400    | `UNKNOWN_OPERATION`      | `operation` isn't in the registry                             |
| 400    | `INVALID_OPERAND_COUNT`  | wrong arity                                                   |
| 400    | `INVALID_NUMBER`         | operand fails §2.4; the message names the operand index       |
| 404    | `NOT_FOUND`              | unknown path                                                  |
| 405    | `METHOD_NOT_ALLOWED`     | anything other than POST on `/api/v1/calculate`               |
| 413    | `REQUEST_TOO_LARGE`      | body > 4 KiB                                                  |
| 415    | `UNSUPPORTED_MEDIA_TYPE` | `Content-Type` isn't `application/json`                       |
| 422    | `DIVISION_BY_ZERO`       | `ErrDivisionByZero`                                           |
| 422    | `UNDEFINED_RESULT`       | `ErrUndefined`                                                |
| 422    | `RESULT_OUT_OF_RANGE`    | `ErrOutOfRange`                                               |
| 500    | `INTERNAL_ERROR`         | anything unexpected, including recovered panics; the message is generic and details are only logged |

The 400 codes mean "your request is malformed". The 422 codes mean "the request is well formed, but the math has no answer". The frontend relies on this split.

### 3.3 Handler design

```go
// Defined on the consumer side, so tests can use a stub (e.g. to cover the 500 path).
type Calculator interface {
    Calculate(op calculator.Operation, operands []string) (calculator.Result, error)
}

func NewRouter(calc Calculator, logger *slog.Logger) http.Handler
```

- Error mapping lives in one place: a table from sentinel error to `(status, code)`, checked with `errors.Is`.
- Middleware, from outermost to innermost:
  1. **recover**: logs the panic and returns a 500 JSON response.
  2. **request logging**: `slog` with method, path, status and duration.
- No CORS. The browser never calls the backend directly: in development the Vite proxy forwards `/api`, and in Docker nginx does. Both are same-origin from the browser's point of view.

## 4. Server (`cmd/server`)

| Env var | Default | Purpose     |
| ------- | ------- | ----------- |
| `PORT`  | `8080`  | listen port |

- `http.Server` timeouts: ReadHeader 5s, Read 10s, Write 10s, Idle 60s.
- Logging: `slog` JSON handler to stdout.
- Graceful shutdown on SIGINT/SIGTERM via `signal.NotifyContext` and `Shutdown`, with a 10s deadline.
- `server -healthcheck`: sends `GET /healthz` to `localhost:$PORT` and exits with 0 or 1. The distroless image has no shell or curl, so the Docker healthcheck runs this binary.

## 5. Testing

- **`calculator`**: table-driven tests for every operation, every row of §2.7, the parsing rules in §2.4 and the formatting rules in §2.5, plus a round-trip test that sends a formatted result back as an operand.
- **`api`**: `httptest` tests against `NewRouter`, covering every status and code in §3.2, the response shapes, JSON numbers accepted as operands, the body-size limit, 405 with `Allow`, and the panic → 500 path (using a stub `Calculator`).
- **Command**: `go test -race -coverprofile=coverage.out ./...` followed by `go tool cover -html`.
- **Target**: at least 90% statement coverage in `calculator` and `api`.
