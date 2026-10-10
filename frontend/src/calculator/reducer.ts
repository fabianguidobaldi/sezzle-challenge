// The calculator's state machine (see docs/specs/frontend.md §4).
// Pure: a calculation is described as `state.request`, and the useCalculator
// hook performs it and dispatches CALC_SUCCESS or CALC_FAILURE.

import type { Operation } from '../api/types'
import type { Action, BinaryOp, Digit, KeyAction, PendingRequest, ResultAction, State } from './types'

/** The backend's number format (backend spec §2.3). */
const MAX_DIGITS = 16
const MAX_DECIMAL_PLACES = 16

export const initialState: State = {
  display: '0',
  displayExact: true,
  accumulator: null,
  accumulatorExact: true,
  pendingOp: null,
  phase: 'input',
  request: null,
  queued: [],
  lastRequestId: 0,
  error: null,
}

export function reducer(state: State, action: Action): State {
  switch (action.type) {
    case 'CALC_SUCCESS':
    case 'CALC_FAILURE':
      return applyResult(state, action)
    default:
      return applyKey(state, action)
  }
}

/** 'C' clears the current entry; 'AC' resets everything. */
export function clearLabel(state: State): 'AC' | 'C' {
  // While busy, CLEAR always resets, so the label says so too.
  return state.phase === 'input' && state.display !== '0' && !isBusy(state) ? 'C' : 'AC'
}

const SYMBOLS: Record<BinaryOp, string> = {
  add: '+',
  subtract: '−',
  multiply: '×',
  divide: '÷',
  power: '^',
}

/** The secondary display line, e.g. "12 ×", or the error message. */
export function expression(state: State): string {
  if (state.phase === 'error') return state.error ?? ''
  if (state.pendingOp && state.accumulator !== null) {
    return `${state.accumulator} ${SYMBOLS[state.pendingOp]}`
  }
  return ''
}

/** True while a calculation is in flight. */
export function isBusy(state: State): boolean {
  return state.request !== null
}

function applyKey(state: State, action: KeyAction): State {
  if (action.type === 'CLEAR') return clear(state)
  // Keys typed faster than the API answers aren't lost: they run once it does.
  if (isBusy(state)) return { ...state, queued: [...state.queued, action] }
  if (state.phase === 'error') {
    if (action.type !== 'DIGIT' && action.type !== 'DECIMAL') return state
    return applyKey(fresh(state), action)
  }

  switch (action.type) {
    case 'DIGIT':
      return digit(state, action.digit)
    case 'DECIMAL':
      if (state.phase !== 'input') return entry(state, '0.')
      return state.display.includes('.') ? state : { ...state, display: state.display + '.' }
    case 'TOGGLE_SIGN':
      return toggleSign(state)
    case 'BACKSPACE':
      return backspace(state)
    case 'OPERATOR':
      return operator(state, action.op)
    case 'EQUALS':
      return hasOperand(state) ? binaryRequest(state, null) : state
    case 'SQRT':
      return request(state, 'sqrt', [operand(state.display)], state.displayExact, null, true)
    case 'PERCENT':
      return percent(state)
  }
}

function applyResult(state: State, action: ResultAction): State {
  const req = state.request
  if (!req || req.id !== action.id) return state

  if (action.type === 'CALC_FAILURE') {
    return {
      ...state,
      display: 'Error',
      displayExact: true,
      accumulator: null,
      accumulatorExact: true,
      pendingOp: null,
      phase: 'error',
      request: null,
      queued: [], // what was typed after a failed step no longer makes sense
      error: action.message,
    }
  }

  return replay(succeed(state, req, action), state.queued)
}

function succeed(state: State, req: PendingRequest, action: { result: string; exact: boolean }): State {
  const exact = action.exact && req.operandsExact
  const base = { ...state, display: action.result, displayExact: exact, request: null, queued: [] }
  if (req.unary) return { ...base, phase: 'result' }
  if (req.next) {
    return { ...base, accumulator: action.result, accumulatorExact: exact, pendingOp: req.next, phase: 'operator' }
  }
  return { ...base, accumulator: null, accumulatorExact: true, pendingOp: null, phase: 'result' }
}

/** Applies queued keys in order; once one starts a new request, the rest queue up again. */
function replay(state: State, keys: KeyAction[]): State {
  return keys.reduce(applyKey, state)
}

/** C clears the entry; AC (and any CLEAR while busy) resets everything. */
function clear(state: State): State {
  if (!isBusy(state) && state.phase === 'input' && state.display !== '0') {
    return { ...state, display: '0', displayExact: true }
  }
  return fresh(state)
}

/** initialState, keeping the id counter so request ids never repeat. */
function fresh(state: State): State {
  return { ...initialState, lastRequestId: state.lastRequestId }
}

/** Starts a new entry, replacing whatever the display showed. */
function entry(state: State, display: string): State {
  return { ...state, display, displayExact: true, phase: 'input' }
}

function digit(state: State, d: Digit): State {
  if (state.phase !== 'input') return entry(state, d)
  const { display } = state
  let next: string
  if (display === '0') next = d
  else if (display === '-0') next = '-' + d
  else next = display + d
  return withinLimits(next) ? { ...state, display: next } : state
}

/** Whether an entry fits the backend's number format; leading zeros don't count. */
function withinLimits(entry: string): boolean {
  const [int, frac = ''] = entry.replace('-', '').split('.')
  const digits = (int + frac).replace(/^0+/, '').length
  return digits <= MAX_DIGITS && frac.length <= MAX_DECIMAL_PLACES
}

function toggleSign(state: State): State {
  const { display, phase } = state
  if (phase === 'operator') return entry(state, '-0')
  if (display.startsWith('-')) return { ...state, display: display.slice(1) }
  if (display === '0') return state
  return { ...state, display: '-' + display }
}

function backspace(state: State): State {
  if (state.phase !== 'input') return state
  const next = state.display.slice(0, -1)
  const display = next === '' || next === '-' || next === '-0' ? '0' : next
  return display === state.display ? state : { ...state, display }
}

function operator(state: State, op: BinaryOp): State {
  if (state.phase === 'operator') return { ...state, pendingOp: op }
  if (hasOperand(state)) return binaryRequest(state, op)
  const value = operand(state.display)
  return {
    ...state,
    display: value,
    accumulator: value,
    accumulatorExact: state.displayExact,
    pendingOp: op,
    phase: 'operator',
  }
}

function percent(state: State): State {
  const { pendingOp, display } = state
  if (pendingOp === 'add' || pendingOp === 'subtract') {
    // A pending operator always has an accumulator.
    const operands = [state.accumulator as string, operand(display)]
    const exact = state.accumulatorExact && state.displayExact
    return request(state, 'percent_of', operands, exact, null, true)
  }
  return request(state, 'percent', [operand(display)], state.displayExact, null, true)
}

/** A pending operator and a right operand on the display: ready to compute. */
function hasOperand(state: State): boolean {
  return state.pendingOp !== null && (state.phase === 'input' || state.phase === 'result')
}

function binaryRequest(state: State, next: BinaryOp | null): State {
  const operands = [state.accumulator as string, operand(state.display)]
  const exact = state.accumulatorExact && state.displayExact
  return request(state, state.pendingOp as BinaryOp, operands, exact, next, false)
}

function request(
  state: State,
  operation: Operation,
  operands: string[],
  operandsExact: boolean,
  next: BinaryOp | null,
  unary: boolean,
): State {
  const id = state.lastRequestId + 1
  return { ...state, lastRequestId: id, request: { id, operation, operands, operandsExact, next, unary } }
}

/** The display as an operand: drops a dangling '.', which is all the backend needs. */
function operand(display: string): string {
  return display.endsWith('.') ? display.slice(0, -1) : display
}
