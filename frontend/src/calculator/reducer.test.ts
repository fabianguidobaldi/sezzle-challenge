import { describe, expect, it } from 'vitest'
import { clearLabel, expression, initialState, isBusy, reducer } from './reducer'
import type { Action, Digit, KeyAction, PendingRequest, State } from './types'

// Key tokens, as written on the keypad; a run of digits types each digit.
const KEYS: Record<string, KeyAction> = {
  '.': { type: 'DECIMAL' },
  '±': { type: 'TOGGLE_SIGN' },
  '⌫': { type: 'BACKSPACE' },
  C: { type: 'CLEAR' },
  '+': { type: 'OPERATOR', op: 'add' },
  '−': { type: 'OPERATOR', op: 'subtract' },
  '×': { type: 'OPERATOR', op: 'multiply' },
  '÷': { type: 'OPERATOR', op: 'divide' },
  '^': { type: 'OPERATOR', op: 'power' },
  '√': { type: 'SQRT' },
  '%': { type: 'PERCENT' },
  '=': { type: 'EQUALS' },
}

function parseKeys(keys: string): KeyAction[] {
  return keys
    .split(' ')
    .filter(Boolean)
    .flatMap((token) => {
      if (token in KEYS) return [KEYS[token]]
      if (/^[0-9.]+$/.test(token)) {
        return [...token].map((c): KeyAction => (c === '.' ? KEYS['.'] : { type: 'DIGIT', digit: c as Digit }))
      }
      throw new Error(`unknown key ${token}`)
    })
}

/** A simulated response: a result string (exact), or an explicit outcome. */
type Response = string | { result: string; exact: boolean } | { error: string }

const describeRequest = (r: PendingRequest) => [r.operation, ...r.operands].join(' ')

/**
 * Presses `keys` from `start`. Whenever a new request appears, it's answered from
 * `responses` (keyed like 'add 2 3') if listed there; otherwise it stays in flight.
 */
function press(keys: string, responses: Record<string, Response> = {}, start: State = initialState) {
  let state = start
  const requests: string[] = []
  const dispatch = (action: Action) => {
    state = reducer(state, action)
    const req = state.request
    if (!req || requests.length >= req.id - start.lastRequestId) return
    const key = describeRequest(req)
    requests.push(key)
    const response = responses[key]
    if (response === undefined) return
    if (typeof response === 'string') dispatch({ type: 'CALC_SUCCESS', id: req.id, result: response, exact: true })
    else if ('error' in response) dispatch({ type: 'CALC_FAILURE', id: req.id, message: response.error })
    else dispatch({ type: 'CALC_SUCCESS', id: req.id, result: response.result, exact: response.exact })
  }
  parseKeys(keys).forEach(dispatch)
  return { state, requests }
}

describe('initialState', () => {
  it('shows 0, exact, with nothing pending', () => {
    expect(initialState).toEqual({
      display: '0',
      displayExact: true,
      accumulator: null,
      accumulatorExact: true,
      pendingOp: null,
      phase: 'input',
      request: null,
      lastRequestId: 0,
      error: null,
    })
  })
})

describe('entry', () => {
  it.each([
    ['5', '5'],
    ['0', '0'],
    ['0 0 7', '7'],
    ['1 2 3', '123'],
    ['.', '0.'],
    ['. 5', '0.5'],
    ['1 . 5', '1.5'],
    ['1 . . 5', '1.5'],
    ['1 . 5 .', '1.5'],
    ['±', '0'],
    ['5 ±', '-5'],
    ['5 ± ±', '5'],
    ['1 . ±', '-1.'],
    ['1 2 ⌫', '1'],
    ['5 ⌫', '0'],
    ['5 ± ⌫', '0'],
    ['5 . ⌫', '5'],
    ['⌫', '0'],
    ['0 . 5 ⌫', '0.'],
    ['1 . 5 ± ⌫', '-1.'],
    ['5 C', '0'],
  ])('%s → %s', (keys, display) => {
    const { state } = press(keys)
    expect(state.display).toBe(display)
    expect(state.phase).toBe('input')
  })

  it('ignores BACKSPACE outside input', () => {
    const operator = press('5 +').state
    expect(reducer(operator, { type: 'BACKSPACE' })).toBe(operator)
    const result = press('2 √', { 'sqrt 2': '1.4' }).state
    expect(reducer(result, { type: 'BACKSPACE' })).toBe(result)
  })

  it('returns the same state when BACKSPACE has nothing to remove', () => {
    expect(reducer(initialState, { type: 'BACKSPACE' })).toBe(initialState)
  })

  it('ignores DECIMAL when the entry already has one', () => {
    const state = press('1 .').state
    expect(reducer(state, { type: 'DECIMAL' })).toBe(state)
  })

  it('ignores TOGGLE_SIGN on "0"', () => {
    expect(reducer(initialState, { type: 'TOGGLE_SIGN' })).toBe(initialState)
  })
})

describe('input limits', () => {
  const sixteen = '1234567890123456'

  it.each([
    ['16 digits', sixteen, sixteen],
    ['a 17th digit is ignored', sixteen + '7', sixteen],
    ['16 digits with a decimal point', '12345678.12345678 9', '12345678.12345678'],
    ['16 decimal places', '0.' + sixteen, '0.' + sixteen],
    ['a 17th decimal place is ignored', '0.' + sixteen + '7', '0.' + sixteen],
    ['leading zeros after the point count as places', '0.0000000000000001 2', '0.0000000000000001'],
    ['a leading zero is not a digit', '0.123456789012345 6', '0.1234567890123456'],
    ['zeros after the point lead too', '0.000000000000001 2', '0.0000000000000012'],
    ['a negative entry has the same limits', sixteen + ' ± 7', '-' + sixteen],
    ['trailing zeros count', '1000000000000000 0', '1000000000000000'],
  ])('%s', (_, keys, display) => {
    expect(press(keys).state.display).toBe(display)
  })

  it('returns the same state for an ignored digit', () => {
    const state = press(sixteen).state
    expect(reducer(state, { type: 'DIGIT', digit: '1' })).toBe(state)
  })
})

describe('TOGGLE_SIGN', () => {
  it('starts a "-0" entry after an operator', () => {
    const { state } = press('5 + ±')
    expect(state).toMatchObject({ display: '-0', phase: 'input', accumulator: '5', pendingOp: 'add' })
  })

  it.each([
    ['5 + ± 3', '-3'],
    ['5 + ± .', '-0.'],
    ['5 + ± . 5', '-0.5'],
    ['5 + ± 0', '-0'],
    ['5 + ± ±', '0'],
    ['5 + ± ⌫', '0'],
  ])('%s → %s', (keys, display) => {
    expect(press(keys).state.display).toBe(display)
  })

  it('negates a result and keeps it a result', () => {
    const { state } = press('2 √ ±', { 'sqrt 2': { result: '1.414213562373095', exact: false } })
    expect(state).toMatchObject({ display: '-1.414213562373095', displayExact: false, phase: 'result' })
  })

  it('sends a "-0" entry as is', () => {
    expect(press('5 + ± =').requests).toEqual(['add 5 -0'])
  })
})

describe('operators', () => {
  it('stores the accumulator', () => {
    const { state } = press('1 2 ×')
    expect(state).toMatchObject({ display: '12', accumulator: '12', pendingOp: 'multiply', phase: 'operator' })
    expect(state.request).toBeNull()
  })

  it('drops a trailing "." from the stored operand', () => {
    expect(press('5 . +').state).toMatchObject({ display: '5', accumulator: '5' })
    expect(press('5 + 3 . =').requests).toEqual(['add 5 3'])
  })

  it('replaces the pending operator', () => {
    expect(press('2 + × 3 =').requests).toEqual(['multiply 2 3'])
  })

  it('evaluates immediately: 2 + 3 × 4 = → 20', () => {
    const { state, requests } = press('2 + 3 × 4 =', { 'add 2 3': '5', 'multiply 5 4': '20' })
    expect(requests).toEqual(['add 2 3', 'multiply 5 4'])
    expect(state).toEqual({ ...initialState, display: '20', phase: 'result', lastRequestId: 2 })
  })

  it('creates a request for a chained operator', () => {
    const { state } = press('2 + 3 ×')
    expect(state.request).toEqual({
      id: 1,
      operation: 'add',
      operands: ['2', '3'],
      operandsExact: true,
      next: 'multiply',
      unary: false,
    })
    expect(state).toMatchObject({ display: '3', phase: 'input', lastRequestId: 1 })
  })

  it('shows a chained result as the new accumulator', () => {
    const { state } = press('2 + 3 ×', { 'add 2 3': '5' })
    expect(state).toMatchObject({ display: '5', accumulator: '5', pendingOp: 'multiply', phase: 'operator' })
    expect(state.request).toBeNull()
  })

  it('uses a result as the next left operand', () => {
    expect(press('2 + 3 = × 4 =', { 'add 2 3': '5' }).requests).toEqual(['add 2 3', 'multiply 5 4'])
  })

  it('starts a new calculation when typing after a result', () => {
    const { state, requests } = press('2 + 3 = 7 − 1 =', { 'add 2 3': '5' })
    expect(requests).toEqual(['add 2 3', 'subtract 7 1'])
    expect(state.display).toBe('1')
  })

  it.each([
    ['2 ^ 1 0 =', 'power 2 10'],
    ['8 ÷ 2 =', 'divide 8 2'],
    ['8 − 2 =', 'subtract 8 2'],
  ])('%s sends %s', (keys, request) => {
    expect(press(keys).requests).toEqual([request])
  })
})

describe('EQUALS', () => {
  it('creates a request with next: null', () => {
    expect(press('2 + 3 =').state.request).toEqual({
      id: 1,
      operation: 'add',
      operands: ['2', '3'],
      operandsExact: true,
      next: null,
      unary: false,
    })
  })

  it.each([
    ['without a pending operator', '5'],
    ['right after an operator (2 + =)', '2 +'],
    ['repeated', '2 + 3 ='],
  ])('is a no-op %s', (_, keys) => {
    const state = press(keys, { 'add 2 3': '5' }).state
    expect(reducer(state, { type: 'EQUALS' })).toBe(state)
  })
})

describe('SQRT and PERCENT', () => {
  it('sends sqrt as a unary request', () => {
    expect(press('1 6 √').state.request).toEqual({
      id: 1,
      operation: 'sqrt',
      operands: ['16'],
      operandsExact: true,
      next: null,
      unary: true,
    })
  })

  it('replaces the display and keeps the pending operation: 9 + 16 √ = → 13', () => {
    const responses = { 'sqrt 16': '4', 'add 9 4': '13' }
    const afterSqrt = press('9 + 1 6 √', responses).state
    expect(afterSqrt).toMatchObject({ display: '4', accumulator: '9', pendingOp: 'add', phase: 'result' })
    const { state, requests } = press('9 + 1 6 √ =', responses)
    expect(requests).toEqual(['sqrt 16', 'add 9 4'])
    expect(state.display).toBe('13')
  })

  it.each([
    ['200 + 10 % =', { 'percent_of 200 10': '20', 'add 200 20': '220' }, '220'],
    ['200 − 10 % =', { 'percent_of 200 10': '20', 'subtract 200 20': '180' }, '180'],
    ['200 × 10 % =', { 'percent 10': '0.1', 'multiply 200 0.1': '20' }, '20'],
    ['200 ÷ 10 % =', { 'percent 10': '0.1', 'divide 200 0.1': '2000' }, '2000'],
    ['2 ^ 10 % =', { 'percent 10': '0.1', 'power 2 0.1': '1.071773462536293' }, '1.071773462536293'],
    ['50 %', { 'percent 50': '0.5' }, '0.5'],
  ])('%s', (keys, responses, display) => {
    const { state, requests } = press(keys, responses)
    expect(requests).toEqual(Object.keys(responses))
    expect(state.display).toBe(display)
  })

  it('marks % requests as unary', () => {
    expect(press('200 + 10 %').state.request).toMatchObject({ operation: 'percent_of', unary: true, next: null })
    expect(press('50 %').state.request).toMatchObject({ operation: 'percent', unary: true, next: null })
  })

  it('applies to the accumulator right after an operator', () => {
    expect(press('9 + √').requests).toEqual(['sqrt 9'])
    expect(press('200 + %').requests).toEqual(['percent_of 200 200'])
  })

  it('drops a trailing "." from the operand', () => {
    expect(press('4 . √').requests).toEqual(['sqrt 4'])
    expect(press('200 + 10 . %').requests).toEqual(['percent_of 200 10'])
  })
})

describe('in-flight requests', () => {
  const busy = press('2 + 3 =').state

  it.each(
    parseKeys('5 . ± ⌫ + √ % =').map((action) => [action.type, action] as const),
  )('ignores %s', (_, action) => {
    expect(reducer(busy, action)).toBe(busy)
  })

  it('resets on CLEAR but keeps the id counter', () => {
    expect(reducer(busy, { type: 'CLEAR' })).toEqual({ ...initialState, lastRequestId: 1 })
  })

  it('drops the response after CLEAR, and later ids do not repeat', () => {
    const cleared = reducer(busy, { type: 'CLEAR' })
    const late = { type: 'CALC_SUCCESS', id: 1, result: '5', exact: true } as const
    expect(reducer(cleared, late)).toBe(cleared)
    expect(press('4 √', {}, cleared).state.request?.id).toBe(2)
  })

  it('ignores responses with a stale id', () => {
    const stale: Action[] = [
      { type: 'CALC_SUCCESS', id: 0, result: '5', exact: true },
      { type: 'CALC_SUCCESS', id: 2, result: '5', exact: true },
      { type: 'CALC_FAILURE', id: 0, message: 'nope' },
    ]
    for (const action of stale) expect(reducer(busy, action)).toBe(busy)
  })

  it('ignores responses when nothing is in flight', () => {
    const idle = press('5').state
    expect(reducer(idle, { type: 'CALC_SUCCESS', id: 1, result: '5', exact: true })).toBe(idle)
    expect(reducer(idle, { type: 'CALC_FAILURE', id: 1, message: 'nope' })).toBe(idle)
  })

  it('numbers requests consecutively', () => {
    const { state } = press('2 + 3 × 4 = √', { 'add 2 3': '5', 'multiply 5 4': '20' })
    expect(state.request?.id).toBe(3)
    expect(state.lastRequestId).toBe(3)
  })
})

describe('CLEAR', () => {
  it('acts as C on an entry, keeping the pending operation', () => {
    const { state, requests } = press('2 + 3 C 4 =')
    expect(requests).toEqual(['add 2 4'])
    expect(state.accumulator).toBe('2')
  })

  it('C resets exactness of the entry', () => {
    const start = { ...initialState, display: '5', displayExact: false }
    expect(reducer(start, { type: 'CLEAR' })).toMatchObject({ display: '0', displayExact: true })
  })

  it.each([
    ['on "0"', '2 + 3 C C'],
    ['after an operator', '2 +'],
    ['on a result', '2 + 3 ='],
    ['in the error phase', '1 ÷ 0 ='],
  ])('acts as AC %s', (_, keys) => {
    const responses: Record<string, Response> = { 'add 2 3': '5', 'divide 1 0': { error: "Can't divide by zero" } }
    const state = press(keys, responses).state
    expect(reducer(state, { type: 'CLEAR' })).toEqual({ ...initialState, lastRequestId: state.lastRequestId })
  })
})

describe('errors', () => {
  const responses: Record<string, Response> = { 'divide 1 0': { error: "Can't divide by zero" } }
  const failed = press('5 + 1 ÷ 0 =', { ...responses, 'add 5 1': '6', 'divide 6 0': { error: "Can't divide by zero" } })
    .state

  it('shows Error and clears the pending operation', () => {
    expect(failed).toEqual({
      ...initialState,
      display: 'Error',
      phase: 'error',
      error: "Can't divide by zero",
      lastRequestId: 2,
    })
  })

  it.each(parseKeys('± ⌫ + √ % =').map((action) => [action.type, action] as const))('ignores %s', (_, action) => {
    expect(reducer(failed, action)).toBe(failed)
  })

  it.each([
    ['7', '7'],
    ['.', '0.'],
    ['. 5', '0.5'],
  ])('starts fresh on %s', (keys, display) => {
    const { state } = press(keys, {}, failed)
    expect(state).toEqual({ ...initialState, display, lastRequestId: 2 })
  })

  it('recovers for a new calculation', () => {
    const { state, requests } = press('1 ÷ 0 = 2 + 3 =', { ...responses, 'add 2 3': '5' })
    expect(requests).toEqual(['divide 1 0', 'add 2 3'])
    expect(state).toMatchObject({ display: '5', phase: 'result', error: null })
  })

  it('fails a unary request the same way', () => {
    const { state } = press('9 + 1 ± √', { 'sqrt -1': { error: 'Undefined' } })
    expect(state).toMatchObject({ display: 'Error', phase: 'error', accumulator: null, pendingOp: null })
  })
})

describe('exactness', () => {
  const third = { result: '0.3333333333333333', exact: false }

  it('marks a rounded result as inexact', () => {
    expect(press('1 ÷ 3 =', { 'divide 1 3': third }).state.displayExact).toBe(false)
  })

  it('carries through a chain: 1 ÷ 3 × 3 = stays inexact', () => {
    const { state, requests } = press('1 ÷ 3 × 3 =', {
      'divide 1 3': third,
      'multiply 0.3333333333333333 3': '1',
    })
    expect(requests).toEqual(['divide 1 3', 'multiply 0.3333333333333333 3'])
    expect(state).toMatchObject({ display: '1', displayExact: false })
  })

  it('marks the accumulator of a chained result', () => {
    const { state } = press('1 ÷ 3 ×', { 'divide 1 3': third })
    expect(state).toMatchObject({ accumulatorExact: false, displayExact: false })
    expect(state.request).toBeNull()
  })

  it('passes the operands exactness into requests', () => {
    const responses = { 'divide 1 3': third, 'sqrt 2': { result: '1.414213562373095', exact: false } }
    expect(press('1 ÷ 3 × 3 =', responses).state.request?.operandsExact).toBe(false)
    expect(press('1 ÷ 3 = √', responses).state.request?.operandsExact).toBe(false)
    expect(press('1 ÷ 3 = + 1 =', responses).state.request?.operandsExact).toBe(false)
    expect(press('1 ÷ 3 = + 1 %', responses).state.request?.operandsExact).toBe(false)
    expect(press('2 √ %', responses).state.request?.operandsExact).toBe(false)
    expect(press('5 + 2 √ =', responses).state.request?.operandsExact).toBe(false)
  })

  it('carries the operator-phase accumulator exactness from the display', () => {
    const { state } = press('2 √ +', { 'sqrt 2': { result: '1.414213562373095', exact: false } })
    expect(state).toMatchObject({ accumulator: '1.414213562373095', accumulatorExact: false })
  })

  it('a freshly typed entry is exact', () => {
    const responses = { 'divide 1 3': third }
    expect(press('1 ÷ 3 = 5', responses).state.displayExact).toBe(true)
    expect(press('1 ÷ 3 = .', responses).state.displayExact).toBe(true)
    expect(press('1 ÷ 3 × ±', responses).state.displayExact).toBe(true)
    expect(press('1 ÷ 3 × 2', responses).state).toMatchObject({ displayExact: true, accumulatorExact: false })
  })

  it('an inexact answer to exact operands is inexact', () => {
    const { state } = press('4 ^ 0.5 =', { 'power 4 0.5': { result: '2', exact: false } })
    expect(state.displayExact).toBe(false)
  })

  it('resets after a binary result with =', () => {
    const { state } = press('1 ÷ 3 × 3 =', { 'divide 1 3': third, 'multiply 0.3333333333333333 3': '1' })
    expect(state.accumulatorExact).toBe(true)
  })
})

describe('selectors', () => {
  it.each([
    ['', 'AC'],
    ['0', 'AC'],
    ['5', 'C'],
    ['.', 'C'],
    ['2 +', 'AC'],
    ['2 + 3', 'C'],
    ['2 + 3 C', 'AC'],
    ['2 + 3 =', 'AC'],
    ['2 + 3 ×', 'AC'], // busy: CLEAR resets everything
  ])('clearLabel after "%s" is %s', (keys, label) => {
    expect(clearLabel(press(keys, { 'add 2 3': '5' }).state)).toBe(label)
  })

  it('clearLabel is AC on a result', () => {
    expect(clearLabel(press('2 + 3 =', { 'add 2 3': '5' }).state)).toBe('AC')
  })

  it('clearLabel is AC in the error phase', () => {
    expect(clearLabel(press('1 ÷ 0 =', { 'divide 1 0': { error: 'x' } }).state)).toBe('AC')
  })

  it.each([
    ['', ''],
    ['12', ''],
    ['12 +', '12 +'],
    ['12 −', '12 −'],
    ['12 ×', '12 ×'],
    ['12 ÷', '12 ÷'],
    ['12 ^', '12 ^'],
    ['12 × 3', '12 ×'],
    ['12 × 3 =', ''],
  ])('expression after "%s" is "%s"', (keys, text) => {
    expect(expression(press(keys, { 'multiply 12 3': '36' }).state)).toBe(text)
  })

  it('expression shows the error message', () => {
    expect(expression(press('1 ÷ 0 =', { 'divide 1 0': { error: "Can't divide by zero" } }).state)).toBe(
      "Can't divide by zero",
    )
  })

  it('expression is empty for an error state without a message', () => {
    expect(expression({ ...initialState, phase: 'error' })).toBe('')
  })

  it('isBusy while a request is in flight', () => {
    expect(isBusy(initialState)).toBe(false)
    expect(isBusy(press('2 + 3 =').state)).toBe(true)
    expect(isBusy(press('2 + 3 =', { 'add 2 3': '5' }).state)).toBe(false)
  })
})
