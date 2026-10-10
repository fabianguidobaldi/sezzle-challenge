import { describe, expect, it } from 'vitest'
import { ApiError, NetworkError } from '../api/client'
import { messageFor } from './messages'

describe('messageFor', () => {
  it.each([
    ['DIVISION_BY_ZERO', new ApiError(422, 'DIVISION_BY_ZERO', 'division by zero'), "Can't divide by zero"],
    ['UNDEFINED_RESULT', new ApiError(422, 'UNDEFINED_RESULT', 'undefined'), 'Result is undefined'],
    ['RESULT_OUT_OF_RANGE', new ApiError(422, 'RESULT_OUT_OF_RANGE', 'out of range'), 'Result is out of range'],
    ['INVALID_NUMBER', new ApiError(400, 'INVALID_NUMBER', 'operand 0'), 'Invalid number'],
    ['a NetworkError', new NetworkError('offline'), "Can't reach the server"],
    ['another API code', new ApiError(500, 'INTERNAL_ERROR', 'internal error'), 'Something went wrong'],
    ['an UNKNOWN response', new ApiError(502, 'UNKNOWN', 'Unexpected server response'), 'Something went wrong'],
    ['an inherited property name as code', new ApiError(400, 'toString', 'x'), 'Something went wrong'],
    ['a plain Error', new Error('boom'), 'Something went wrong'],
    ['a string', 'DIVISION_BY_ZERO', 'Something went wrong'],
    ['undefined', undefined, 'Something went wrong'],
  ])('maps %s', (_, error, expected) => {
    expect(messageFor(error)).toBe(expected)
  })
})
