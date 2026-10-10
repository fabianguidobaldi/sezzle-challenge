// User-facing error messages (see docs/specs/frontend.md §5).

import { ApiError, NetworkError } from '../api/client'

const API_MESSAGES = new Map([
  ['DIVISION_BY_ZERO', "Can't divide by zero"],
  ['UNDEFINED_RESULT', 'Result is undefined'],
  ['RESULT_OUT_OF_RANGE', 'Result is out of range'],
  ['INVALID_NUMBER', 'Invalid number'],
])

const FALLBACK = 'Something went wrong'

/** The text to show for a failed calculation. */
export function messageFor(error: unknown): string {
  if (error instanceof NetworkError) return "Can't reach the server"
  if (error instanceof ApiError) return API_MESSAGES.get(error.code) ?? FALLBACK
  return FALLBACK
}
