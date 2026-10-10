// Client for the calculator API (see docs/specs/frontend.md §3).

import type { ApiErrorBody, CalculateResponse, Operation } from './types'

const CALCULATE_PATH = '/api/v1/calculate'

/** The server answered, but not with a result: an error envelope or an unexpected body. */
export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

/** The request never got an answer (offline, DNS, connection reset, ...). */
export class NetworkError extends Error {
  constructor(message: string, options?: { cause?: unknown }) {
    super(message, options)
    this.name = 'NetworkError'
  }
}

/**
 * Runs one operation on the backend. Rejects with ApiError or NetworkError;
 * an aborted request rejects with the original AbortError so callers can
 * tell it apart and ignore it.
 */
export async function calculate(
  operation: Operation,
  operands: string[],
  signal?: AbortSignal,
): Promise<CalculateResponse> {
  let res: Response
  let text: string
  try {
    res = await fetch(CALCULATE_PATH, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ operation, operands }),
      signal,
    })
    text = await res.text()
  } catch (err) {
    if (signal?.aborted || isAbortError(err)) throw err
    throw new NetworkError("Can't reach the server", { cause: err })
  }

  const body = parseJSON(text)
  if (res.ok) {
    if (isCalculateResponse(body)) return body
    throw unexpected(res.status)
  }
  if (isErrorBody(body)) throw new ApiError(res.status, body.error.code, body.error.message)
  throw unexpected(res.status)
}

function unexpected(status: number): ApiError {
  return new ApiError(status, 'UNKNOWN', 'Unexpected server response')
}

function isAbortError(err: unknown): boolean {
  // Checked by name, not instanceof: jsdom and Node each have their own DOMException.
  return typeof err === 'object' && err !== null && (err as { name?: unknown }).name === 'AbortError'
}

function parseJSON(text: string): unknown {
  try {
    return JSON.parse(text)
  } catch {
    return undefined
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function isCalculateResponse(value: unknown): value is CalculateResponse {
  return (
    isRecord(value) &&
    typeof value.operation === 'string' &&
    Array.isArray(value.operands) &&
    value.operands.every((o) => typeof o === 'string') &&
    typeof value.result === 'string' &&
    typeof value.exact === 'boolean'
  )
}

function isErrorBody(value: unknown): value is ApiErrorBody {
  return (
    isRecord(value) &&
    isRecord(value.error) &&
    typeof value.error.code === 'string' &&
    typeof value.error.message === 'string'
  )
}
