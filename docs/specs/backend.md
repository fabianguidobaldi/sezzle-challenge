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
6. Round the result to the number format and format it (§2.5).

Adding an operation means adding one function and one registry entry.

### 2.3 Number format and precision

Operands and results share one number format: **plain decimals** (no exponent notation) with

- at most `Precision` = **16 significant digits**,
- at most **16 integer digits**, so every number is below 10¹⁶ in magnitude,
- at most `MaxDecimalPlaces` = **16 decimal places**.

This is a 16-digit pocket calculator's range. Because results always satisfy the same rules as operands, any result can be sent back as the next operand.

Computation uses a single `apd.Context` with a **working precision of 34 digits** (`2 × Precision + 2`). That holds any sum or product of two operands exactly, so those results are rounded only once, to the number format (§2.5). Its exponent range (±9999) only matters for `power`.

- **A result is exact whenever it fits the format**: `0.1 + 0.2 = 0.3`, `1/4`, `√16`, `2^-3`, `99999999 × 99999999 = 9999999800000001`.
- **Otherwise it's correctly rounded** (half-even) to 16 significant digits and 16 decimal places: `1/3 → 0.3333333333333333`, `1/3000 → 0.0003333333333333`, `√2 → 1.414213562373095`. Results too small for the format round to `0` (`0.0000000001² → 0`), as on a physical calculator.
- **Results of 10¹⁶ or more are `ErrOutOfRange`** (`2^64`, `9999999999999999 + 1`).
- `Exact = !condition.Inexact()`, combining the conditions of the operation and of the final rounding. This is conservative: apd computes fractional powers with exp/ln, so `4 ^ 0.5` reports `exact: false` even though the result is 2. `exact: false` means "may have been rounded".

### 2.4 Parsing operands (`decimal.go`)

`parseOperand(index int, s string) (*apd.Decimal, error)` accepts a plain decimal: optional sign, digits with an optional `.`. Examples: `"42"`, `"-0.5"`, `".5"`, `"1.000"`.

It rejects:
- empty strings, or strings longer than **64 characters**
- exponent notation (`"1E+5"`, `"1e-3"`)
- anything `apd` can't parse (`"abc"`, `" 1"`, `"1_000"`)
- non-finite values: `NaN` and `Infinity` parse successfully in apd and are rejected explicitly
- values outside the number format (§2.3), checked after trailing zeros are removed (so `"1.000"` is fine): more than 16 significant digits, decimal places or integer digits. Inputs are rejected, never silently rounded.

Every rejection returns `&OperandError{Index: i, Value: s, Reason: "..."}`, which wraps `ErrInvalidNumber`.

### 2.5 Rounding and formatting results (`decimal.go`)

`roundResult(ctx, d) (apd.Condition, error)` rounds the computed value in place to the number format with a single `Quantize`. The target exponent is `max(adjustedExponent − 15, −16)`: 16 significant digits, but no more than 16 decimal places. It returns `ErrOutOfRange` if the rounded value has more than 16 integer digits; this includes a rounding carry such as `9999999999999999.5 → 10¹⁶`.

`formatResult(d) string` turns negative zero into `"0"`, removes trailing zeros (`3.00 → 3`, `0.2500 → 0.25`) and writes plain notation (`d.Text('f')`). The longest possible value is 19 characters (`-0.1234567890123456`).

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
| `2^64`, `9999999999999999 + 1` | `ErrOutOfRange`                      | 10¹⁶ or more                                  |
| `9999999999999999 + 0.5`    | `ErrOutOfRange`                           | rounding carries into a 17th digit            |
| `0.0000000001²`             | `0`, inexact                              | too small for 16 decimal places               |
| `0.5 ^ 9999999999999999`    | `0`, inexact                              | apd overflows/underflows; the math decides (§2.8) |
| `2 ^ 9999999999999999`      | `ErrOutOfRange`                           | likewise                                      |
| `1E+5` as an operand        | `ErrInvalidNumber`                        | no exponent notation                          |
| `2.50 - 2.50`               | `0`                                       | reduced                                       |
| `-0 × 5`                    | `0`                                       | negative zero normalized                      |
| `1 / 3`                     | `0.3333333333333333` (16 digits), inexact |                                               |
| `percent(50)`               | `0.5`, exact                              |                                               |
| `percent_of(200, 10)`       | `20`, exact                               | "10% of 200"                                  |
| `percent_of(-80, 12.5)`     | `-10`, exact                              | negative values and fractional rates are fine |
| `percent(0.0000000000000001)` | `0`, inexact                            | too small for 16 decimal places               |

### 2.8 `power` beyond the working range

apd's cost for `x^y` grows with log(y), so no exponent limit is needed. When the result leaves the working exponent range, apd reports overflow or underflow, but not always the right one: `0.5^-9999999999999999` is reported as underflow. So the math decides. If |x| < 1 with y > 0, or |x| > 1 with y < 0, the true result tends to 0 and rounds to `0` (inexact) like any other tiny result; otherwise it's `ErrOutOfRange`.

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
- `operands` (array, required) has 2 items, or 1 for `sqrt` and `percent`. Each item should be a **decimal string** in the number format (§2.3). JSON numbers are also accepted for convenience: the custom `operand` type's `UnmarshalJSON` keeps the number's original text, so it's never converted to a float. A number written in exponent form (`1e5`) is rejected by the calculator like any other operand. `power` operands are `[base, exponent]`; `percent_of` operands are `[base, rate]`.
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
