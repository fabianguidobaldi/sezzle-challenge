// State machine types for the keypad (see docs/specs/frontend.md §4).

import type { Operation } from '../api/types'

export type BinaryOp = Extract<Operation, 'add' | 'subtract' | 'multiply' | 'divide' | 'power'>

export type Digit = '0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9'

export type Phase =
  | 'input' // the user is typing into the display
  | 'operator' // a binary operator was just pressed; the display shows the accumulator
  | 'result' // the display shows a computed value; typing replaces it, but it's a valid operand
  | 'error' // the last calculation failed

/** A calculation the reducer wants performed; the useCalculator hook sends it. */
export interface PendingRequest {
  id: number // stale responses (whose id doesn't match) are ignored
  operation: Operation
  operands: string[]
  operandsExact: boolean // false if any operand was itself a rounded result
  next: BinaryOp | null // operator to chain after a binary result (null for '=')
  unary: boolean // √ and % replace the display and keep accumulator/pendingOp
}

export interface State {
  display: string // current entry or last result, as a decimal string
  displayExact: boolean
  accumulator: string | null // left operand
  accumulatorExact: boolean
  pendingOp: BinaryOp | null
  phase: Phase
  request: PendingRequest | null // the in-flight calculation
  queued: KeyAction[] // keys pressed while a request was in flight, replayed after it
  lastRequestId: number // ids increase with every request
  error: string | null // user-facing message
}

/** Actions a user can trigger, from the keypad or the keyboard. */
export type KeyAction =
  | { type: 'DIGIT'; digit: Digit }
  | { type: 'DECIMAL' }
  | { type: 'TOGGLE_SIGN' }
  | { type: 'BACKSPACE' }
  | { type: 'CLEAR' }
  | { type: 'OPERATOR'; op: BinaryOp }
  | { type: 'SQRT' }
  | { type: 'PERCENT' }
  | { type: 'EQUALS' }

/** Outcomes of a PendingRequest, dispatched by the useCalculator hook. */
export type ResultAction =
  | { type: 'CALC_SUCCESS'; id: number; result: string; exact: boolean }
  | { type: 'CALC_FAILURE'; id: number; message: string }

export type Action = KeyAction | ResultAction
