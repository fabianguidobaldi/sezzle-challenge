# Frontend implementation plan

The [frontend spec](../specs/frontend.md) and the frontend part of the [deployment spec](../specs/deployment.md) are built in independent stages. Each stage builds, passes its own checks and lands as one commit. Once stage 1 has fixed the shared types and the test setup, stages 2, 3 and 4 touch disjoint files and can run in parallel.

```
               ┌──► 2. API client + messages ──┐
1. Scaffold +  ├──► 3. Calculator reducer ─────┼──► 5. Hooks + wiring ──► 6. Container + compose
   contracts   └──► 4. Presentational UI ──────┘
```

## Stage 1: Scaffold and contracts

- **Files:**
  - `frontend/`: a Vite React + TypeScript (`strict`) project, with the template demo content removed.
  - `vite.config.ts`: `/api` proxy to `http://localhost:8080`, plus the Vitest config (jsdom, setup file, v8 coverage). Coverage is reported as text in the terminal and HTML in `frontend/coverage/index.html`, and nothing else.
  - `package.json` scripts: `dev`, `build`, `test`, `test:coverage`, `lint`, `typecheck`.
  - `src/api/types.ts`: `Operation`, `CalculateResponse`, `ApiErrorBody`.
  - `src/calculator/types.ts`: `BinaryOp`, `Phase`, `State`, `Action`, `PendingRequest` (spec §4.1 and §4.2).
  - `src/test/setup.ts` and `src/test/server.ts`: jest-dom matchers, plus the MSW server lifecycle with a default `/api/v1/calculate` handler that individual tests can override.
  - A placeholder `App.tsx`.
  - Root `Makefile`: `frontend-dev`, `frontend-test`, `frontend-cover`, `frontend-lint`.
- **Tests:** a smoke test that renders `App`, so the test pipeline and coverage reporting run end to end.
- **Done when:** the types match the spec exactly, and `npm run build`, `lint`, `typecheck` and `test` all pass. Stages 2–4 build on these types without changing them.

## Stage 2: API client and error messages

*Depends on 1. Runs in parallel with 3 and 4.*

- **Files:** `src/api/client.ts`, `src/calculator/messages.ts`.
- **Scope:**
  - `calculate(operation, operands, signal)` (spec §3):
    - a 2xx response → the parsed body
    - an error envelope → `ApiError(status, code, message)`
    - an error response without the envelope → `ApiError(status, 'UNKNOWN', …)`
    - `fetch` rejects → `NetworkError`
    - `AbortError` is re-thrown unchanged
  - `messageFor(error)`: maps an `ApiError` code or `NetworkError` to the user-facing text (spec §5 table).
- **Tests:** MSW for every branch above, including abort; table-driven tests for `messageFor`.

## Stage 3: Calculator reducer

*Depends on 1. The largest stage; runs in parallel with 2 and 4.*

- **Files:** `src/calculator/reducer.ts`.
- **Scope:**
  - A pure `reducer` and `initialState` implementing every transition in spec §4.3, including:
    - immediate execution
    - operator replacement
    - unary `√` and `%`, choosing `percent` or `percent_of` from the pending operator
    - stale request ids
    - C vs AC
    - exactness carried through a chain
    - the error state
  - Input limits matching the backend's number format: at most 16 digits in total, and at most 16 after the decimal point.
  - Selectors so components stay dumb:
    - `clearLabel(state)`: `"AC"` or `"C"`
    - `expression(state)`: the secondary line, e.g. `"12 ×"`
    - `isBusy(state)`
- **Tests:**
  - Table-driven: a key sequence plus simulated `CALC_SUCCESS` / `CALC_FAILURE` gives the expected state and request.
  - The spec's worked examples: `2 + 3 × 4 =` → 20, and the four percentage rows.
  - No network involved.

## Stage 4: Presentational UI

*Depends on 1 (types only). Runs in parallel with 2 and 3.*

- **Files:**
  - `src/calculator/keys.ts`: the keypad definition (spec §6.1: label, aria-label, action, variant, grid span) and the keyboard map (spec §5).
  - `src/components/Display.tsx` and `src/components/Keypad.tsx`.
  - `src/index.css`.
- **Scope:**
  - **Components:** purely presentational and driven by props (`value`, `exact`, `expression`, `error`, `busy`, `clearLabel`, `activeOp`, `onAction`).
  - **Display:** `≈` marker, `<output aria-live="polite">`, `aria-busy`, font steps for long values.
  - **Keypad:** a CSS grid with the wide `0` and tall `=`, and `aria-pressed` on the active operator.
  - **Styling:**
    - responsive card: max 380px wide, full viewport below 480px
    - touch targets of at least 56px
    - `:focus-visible` outline
    - light and dark themes via `prefers-color-scheme`
- **Tests:**
  - RTL: every key renders with its label and aria-label, and clicking dispatches the right action.
  - The display shows the value, the `≈` marker and the error text.
  - The keyboard map covers every key in the spec table.

## Stage 5: Hooks and wiring

*Depends on 2, 3 and 4.*

- **Files:**
  - `src/hooks/useCalculator.ts`
  - `src/hooks/useKeyboard.ts`
  - `src/components/Calculator.tsx`
  - the real `App.tsx`
- **Scope:**
  - `useCalculator`: a `useReducer` plus an effect keyed on `request.id` that calls `calculate` with an `AbortController`, dispatches success or failure (using `messageFor`), and aborts on cleanup.
  - `useKeyboard`: a `window` `keydown` listener using the keyboard map. It calls `preventDefault` on handled keys and ignores Ctrl, Meta and Alt combinations.
  - `Calculator`: composes everything.
- **Tests:**
  - `renderHook` + MSW: the request body, success, failure, and AC aborting an in-flight request.
  - Keyboard: handled keys, ignored modifier keys.
  - End-to-end RTL flows from spec §7:
    - `2 + 3 × 4 =` → `20`
    - `0.1 + 0.2 =` → `0.3`
    - `200 + 10 % =` → `220`
    - `1 ÷ 0 =` → "Can't divide by zero"
    - `1 ÷ 3 =` → `≈ 0.333…`
    - the same flows driven from the keyboard
- **Manual check:** `make backend-run` plus `npm run dev`, then use the calculator in a browser at desktop and phone widths.

## Stage 6: Container and Compose

*Depends on 5.*

- **Files:** `frontend/Dockerfile`, `frontend/nginx.conf`, `frontend/.dockerignore`, root `compose.yaml`, and `Makefile` targets (`up`, `down`).
- **Scope:** as in the deployment spec:
  - The build: Node, then an unprivileged nginx image.
  - nginx:
    - proxies `/api` and `/healthz` to `backend:8080`
    - SPA fallback to `index.html`
    - immutable caching for `/assets/`, and `no-cache` for `index.html`
    - gzip and security headers, including the CSP
  - Compose:
    - the backend isn't published
    - the frontend waits for the backend to be healthy
    - `restart: unless-stopped`
- **Tests:**
  - `docker compose up --build`, then `curl` through nginx: the app's HTML, `/healthz`, a calculation, and a deep link falling back to `index.html`.
  - The app works in a browser at `http://localhost:8080` with no CSP errors in the console.

## Every stage is done when

- `npm run lint` and `npm run typecheck` are clean.
- `npm test` passes.
- Coverage of the files the stage changes is at least 90%.
- It's committed as one commit, with a message naming the stage.

## Updates after implementation

The plan above is kept as it was written. These changes came after it:

- **Queued keys (fix after stage 5).** Browser testing showed that keys typed while a request was in flight were dropped (`200+10%=` typed quickly gave `20`, not `220`). The reducer now queues them and replays them when the response arrives (spec §4.3).
- **Footer.** A "Made with ❤️ and Claude" credit line, plus tighter spacing on short phones so it stays on screen (spec §6.3 and §6.4).
- **Make targets.** `frontend-cover` was merged into `frontend-test`, which now always writes the coverage report, and `backend-run` was renamed `backend-dev`. `check` (lint and test both projects) and `build` (Docker images) were added, and Node is pinned in `frontend/.nvmrc`. Coverage reports are committed.
- **Linter.** The current Vite template ships oxlint instead of ESLint, so stage 1 uses oxlint.
