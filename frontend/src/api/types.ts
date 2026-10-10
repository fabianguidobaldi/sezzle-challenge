// Wire types for the calculator API (see docs/specs/backend.md §3).
// Numbers are always decimal strings, never JavaScript numbers.

export type Operation =
  | 'add'
  | 'subtract'
  | 'multiply'
  | 'divide'
  | 'power'
  | 'sqrt'
  | 'percent'
  | 'percent_of'

export interface CalculateRequest {
  operation: Operation
  operands: string[]
}

export interface CalculateResponse {
  operation: Operation
  operands: string[]
  result: string
  exact: boolean
}

export interface ApiErrorBody {
  error: { code: string; message: string }
}
