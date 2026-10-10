# Frontend spec

A keypad calculator in React + TypeScript that uses **immediate execution** (like a basic physical calculator). The client **never does arithmetic**: every calculation goes to `POST /api/v1/calculate` (see [backend spec](./backend.md)), and numbers are kept as strings from start to finish.

## 1. Stack

- Vite, React and TypeScript (`strict`); plain CSS with custom properties, no UI library.
- Vitest, React Testing Library, `@testing-library/user-event`, MSW (network mocking), `@vitest/coverage-v8`.
- oxlint (the current Vite template's linter) and `tsc -b` for type checking.
- The app always calls relative URLs (`/api/...`). In development the Vite proxy forwards `/api` to `http://localhost:8080`; in Docker nginx does the same (see [deployment spec](./deployment.md)). No CORS or env configuration is needed.

## 2. Layout

```
frontend/
├── Dockerfile                  # node build → nginx (see deployment spec)
├── nginx.conf                  # serves dist/, proxies /api → backend
├── index.html
├── vite.config.ts              # proxy + vitest config
├── package.json                # dev, build, test, test:coverage, lint, typecheck
└── src/
    ├── main.tsx, App.tsx, index.css
    ├── api/
    │   ├── types.ts            # Operation, CalculateRequest/Response, ApiErrorBody
    │   └── client.ts           # calculate(), ApiError, NetworkError
    ├── calculator/
    │   ├── types.ts            # State, Action, BinaryOp, PendingRequest
    │   ├── reducer.ts          # pure state machine + initialState
    │   ├── keys.ts             # keypad definition + keyboard map
    │   └── messages.ts         # error code → user-facing message
    ├── hooks/
    │   ├── useCalculator.ts    # reducer + API side effect
    │   └── useKeyboard.ts      # physical keyboard → actions
    ├── components/
    │   ├── Calculator.tsx      # composes Display + Keypad, owns the hooks
    │   ├── Display.tsx         # presentational
    │   └── Keypad.tsx          # presentational
    └── test/
        ├── setup.ts            # jest-dom, MSW server lifecycle
        └── server.ts           # MSW handlers
```

Tests live next to the files they test (`*.test.ts(x)`).

## 3. API client (`api/client.ts`)

```ts
type Operation = 'add' | 'subtract' | 'multiply' | 'divide' | 'power' | 'sqrt' | 'percent' | 'percent_of';

interface CalculateResponse { operation: Operation; operands: string[]; result: string; exact: boolean }

class ApiError extends Error { constructor(readonly status: number, readonly code: string, message: string) }
class NetworkError extends Error {}

function calculate(operation: Operation, operands: string[], signal?: AbortSignal): Promise<CalculateResponse>
```

- On a 2xx response, it returns the parsed body.
- On a non-2xx response with the standard error envelope, it throws `ApiError(status, code, message)`.
- On a non-2xx response without the envelope (e.g. a proxy returned HTML), it throws `ApiError(status, 'UNKNOWN', 'Unexpected server response')`.
- If `fetch` rejects, it throws `NetworkError`. An `AbortError` is re-thrown unchanged so callers can ignore it.

## 4. Calculator state machine (`calculator/reducer.ts`)

A pure `reducer(state, action) → state`. Side effects are described in the state as a `request` (the hook performs them), so the reducer can be tested exhaustively without any network.

### 4.1 State

```ts
type BinaryOp = 'add' | 'subtract' | 'multiply' | 'divide' | 'power';

type Phase =
  | 'input'     // the user is typing into the display
  | 'operator'  // a binary operator was just pressed; display shows the accumulator
  | 'result'    // display shows a computed value; typing replaces it, but it's a valid operand
  | 'error';    // last calculation failed

interface State {
  display: string;              // current entry or last result (a decimal string)
  displayExact: boolean;
  accumulator: string | null;   // left operand
  accumulatorExact: boolean;
  pendingOp: BinaryOp | null;
  phase: Phase;
  request: PendingRequest | null; // an in-flight calculation
  queued: KeyAction[];            // keys pressed during the request, replayed after it
  lastRequestId: number;          // ids increase with every request
  error: string | null;           // user-facing message
}

interface PendingRequest {
  id: number;                   // increments on every request; stale responses are ignored
  operation: Operation;
  operands: string[];
  operandsExact: boolean;       // false if any operand was itself a rounded result
  next: BinaryOp | null;        // operator to chain after a binary result (null for '=')
  unary: boolean;               // √ and % replace the display and keep accumulator/pendingOp
}
```

### 4.2 Actions

User actions: `DIGIT(d)`, `DECIMAL`, `TOGGLE_SIGN`, `BACKSPACE`, `CLEAR`, `OPERATOR(op)`, `SQRT`, `PERCENT`, `EQUALS`.
API actions: `CALC_SUCCESS(id, result, exact)`, `CALC_FAILURE(id, message)`.

### 4.3 Transitions

**While a request is in flight**, user actions are **queued** (in `state.queued`) and replayed in order when the response arrives, so keys typed faster than the API answers aren't lost (`200+10%=` typed quickly still gives 220). Once a replayed key starts a new request, the remaining keys queue up behind it. A failed request drops the queue. `CLEAR` (AC) is never queued: it resets everything, including the queue, and the in-flight response is then dropped because its id no longer matches.

| Action          | Behaviour |
| --------------- | --------- |
| `DIGIT d`       | `input`: append `d`, replacing a lone `"0"`. Ignored if the entry would leave the backend's number format: more than 16 digits in total, or more than 16 after the decimal point. Any other phase: `display = d`, phase becomes `input` (from `error`, everything is reset first). |
| `DECIMAL`       | `input`: append `"."` if there isn't one. Any other phase: `display = "0."`, phase becomes `input`. |
| `TOGGLE_SIGN`   | `input` or `result`: add or remove a leading `"-"` (no effect on `"0"`). This is string manipulation, not arithmetic. `operator`: `display = "-0"`, phase becomes `input`. |
| `BACKSPACE`     | `input` only: remove the last character; `""` or `"-"` becomes `"0"`. |
| `CLEAR`         | If phase is `input` and display isn't `"0"`, it acts as **C**: `display = "0"`, keeping the accumulator and operator. Otherwise it acts as **AC**: back to `initialState`. The key's label follows the same rule. |
| `OPERATOR op`   | `operator`: replace `pendingOp` (`2 + × 3` means `2 × 3`). `input`/`result` with a pending operator: create request `{pendingOp, [accumulator, display], next: op}`. Otherwise: `accumulator = display`, `pendingOp = op`, phase becomes `operator`. |
| `EQUALS`        | `input`/`result` with a pending operator: create request `{pendingOp, [accumulator, display], next: null}`. Otherwise: no-op. Neither `2 + =` nor repeated `=` is supported (documented). |
| `SQRT`          | Not `error`: create request `{sqrt, [display], unary: true}`. |
| `PERCENT`       | Not `error`. With a pending `add` or `subtract`: create request `{percent_of, [accumulator, display], unary: true}`, so the entry becomes "display % of the accumulator". Otherwise (a pending `×`, `÷`, `xʸ`, or nothing pending): create request `{percent, [display], unary: true}`, so the entry becomes display ÷ 100. |
| `CALC_SUCCESS`  | Ignored if the id is stale. **Unary**: `display = result`, phase becomes `result`, and the accumulator and operator are kept (`9 + 16 √ =` gives `13`; `200 + 10 % =` gives `220`). **Binary with `next`**: `accumulator = display = result`, `pendingOp = next`, phase becomes `operator`. **Binary with `=`**: `display = result`, accumulator and operator cleared, phase becomes `result`. Exactness carries through the chain: `displayExact = response.exact && every operand was exact`, so `1 ÷ 3 × 3` stays "≈". |
| `CALC_FAILURE`  | Ignored if the id is stale. Display shows `"Error"`, phase becomes `error`, `error = message`, accumulator and operator cleared. From `error`, only `DIGIT`, `DECIMAL` and `CLEAR` do anything. |

Example of immediate execution: `2 + 3 × 4 =`
1. `+` stores 2.
2. `×` sends `add(2, 3)`; the result 5 is shown and becomes the accumulator.
3. `=` sends `multiply(5, 4)`, giving **20**.

Percentage follows the usual pocket-calculator convention:

| Keys            | Requests                                   | Result |
| --------------- | ------------------------------------------ | ------ |
| `200 + 10 % =`  | `percent_of(200, 10)` → 20, `add(200, 20)` | 220    |
| `200 − 10 % =`  | `percent_of(200, 10)` → 20, `subtract(200, 20)` | 180 |
| `200 × 10 % =`  | `percent(10)` → 0.1, `multiply(200, 0.1)`  | 20     |
| `50 %`          | `percent(50)`                              | 0.5    |

## 5. Hooks

`useCalculator()` → `{ state, dispatch, clearLabel }`
- Wraps `useReducer`.
- A `useEffect` keyed on `state.request?.id` calls `calculate()` with an `AbortController`, then dispatches `CALC_SUCCESS` or `CALC_FAILURE`. Cleanup aborts the request, which covers unmounting and AC.
- Turns errors into messages using `messages.ts`:

| Source                          | Message                     |
| ------------------------------- | --------------------------- |
| `DIVISION_BY_ZERO`              | Can't divide by zero        |
| `UNDEFINED_RESULT`              | Result is undefined         |
| `RESULT_OUT_OF_RANGE`           | Result is out of range      |
| `INVALID_NUMBER`                | Invalid number              |
| `NetworkError`                  | Can't reach the server      |
| anything else                   | Something went wrong        |

`useKeyboard(dispatch)` adds a `keydown` listener on `window`:

| Key(s)                 | Action               |
| ---------------------- | -------------------- |
| `0`–`9`                | `DIGIT`              |
| `.` `,`                | `DECIMAL`            |
| `+` `-` `*` `/` `^`    | `OPERATOR`           |
| `r`                    | `SQRT`               |
| `%`                    | `PERCENT`            |
| `Enter` `=`            | `EQUALS`             |
| `Backspace`            | `BACKSPACE`          |
| `Escape` `Delete`      | `CLEAR`              |
| `F9`                   | `TOGGLE_SIGN`        |

Handled keys call `preventDefault()`. Keys pressed with Ctrl, Meta or Alt are ignored, so browser shortcuts keep working.

## 6. UI

### 6.1 Keypad (4 × 6)

```
┌──────┬──────┬──────┬──────┐
│ AC/C │  ⌫   │  %   │  ÷   │
├──────┼──────┼──────┼──────┤
│  √   │  xʸ  │  ±   │  ×   │
├──────┼──────┼──────┼──────┤
│  7   │  8   │  9   │  −   │
├──────┼──────┼──────┼──────┤
│  4   │  5   │  6   │  +   │
├──────┼──────┼──────┼──────┤
│  1   │  2   │  3   │      │
├──────┴──────┼──────┤  =   │
│      0      │  .   │      │
└─────────────┴──────┴──────┘
```

- The keypad is defined as data in `keys.ts` (label, aria-label, action, style variant, grid span) and rendered by `Keypad`. Adding `%` freed space for an on-screen backspace `⌫`, so backspace now works on mobile too.
- The active `pendingOp` key is highlighted (`aria-pressed`) while in the `operator` phase.

### 6.2 Display

- **Secondary line**: the pending expression (e.g. `12 ×`), or the error message in the error state.
- **Main line**: the display value, exactly as typed or as returned by the backend. There's no client-side rounding or reformatting. A `≈` marker appears when `displayExact` is false.
- **Long values**: values are plain decimals of at most 19 characters (e.g. `-0.1234567890123456`); there is no exponent notation. The font steps down in size by length so every value fits on a phone screen. Values are never truncated.
- **While a request is in flight**: `aria-busy="true"` and a subtle pending indicator. Keys aren't disabled, to avoid flicker; the reducer queues input instead.
- The main line is an `<output aria-live="polite">`, so screen readers announce results.

### 6.3 Responsive and theming

- A centred card with `max-width: 380px`. Below 480px it fills the viewport (`100dvh`).
- Keys use CSS grid with square buttons, at least 56px for touch targets, and a visible `:focus-visible` outline.
- Light and dark themes follow `prefers-color-scheme`, using CSS custom properties.

## 7. Testing

| Layer           | What                                                                                                     |
| --------------- | -------------------------------------------------------------------------------------------------------- |
| `reducer`       | Table-driven: a key sequence plus simulated responses gives the expected state. Covers every row in §4.3, chaining, operator replacement, unary √ and % within a pending operation, choosing `percent` vs `percent_of`, stale ids, C vs AC, exactness carried through a chain, the error state, and input limits. |
| `client`        | MSW: success, error envelope → `ApiError`, non-JSON error, network failure → `NetworkError`, abort.        |
| `useCalculator` | `renderHook` + MSW: a request is sent with the right body, success and failure are dispatched, AC aborts the request. |
| Components      | Clicking keys and typing on the keyboard gives the right display; the `≈` marker; error message; AC/C label; `aria-pressed`. |
| End to end (RTL)| `2 + 3 × 4 =` → `20`; `0.1 + 0.2 =` → `0.3`; `200 + 10 % =` → `220`; `1 ÷ 0 =` → "Can't divide by zero"; `1 ÷ 3 =` → `≈ 0.333…`. |

Run with `npm run test:coverage` (v8 provider, with text and HTML reports). Target: at least 90% statement coverage across `src/` (excluding `main.tsx`).
