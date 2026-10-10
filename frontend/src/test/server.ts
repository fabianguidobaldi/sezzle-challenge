// MSW server shared by all tests. No handlers are registered by default and
// unhandled requests fail the test, so every test states the API calls it
// expects (usually with mockCalculations).

import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'
import type { CalculateRequest } from '../api/types'

export const CALCULATE_URL = '/api/v1/calculate'

export const server = setupServer()

/** A canned API reply: a result, or an error response. */
export type MockReply =
  | { result: string; exact?: boolean }
  | { status: number; code: string; message?: string }

/**
 * Answers POST /api/v1/calculate from a table keyed by the operation and its
 * operands, e.g. `{ 'add 0.1 0.2': { result: '0.3' } }`. Requests that aren't
 * in the table get a 500 naming the missing key. The handler never does
 * arithmetic, so tests don't depend on JavaScript numbers.
 */
export function mockCalculations(table: Record<string, MockReply>): void {
  server.use(
    http.post(CALCULATE_URL, async ({ request }) => {
      const body = (await request.json()) as CalculateRequest
      const key = [body.operation, ...body.operands].join(' ')
      const reply = table[key]
      if (!reply) {
        return HttpResponse.json(
          { error: { code: 'TEST_UNMOCKED', message: `no mock for "${key}"` } },
          { status: 500 },
        )
      }
      if ('status' in reply) {
        return HttpResponse.json(
          { error: { code: reply.code, message: reply.message ?? reply.code } },
          { status: reply.status },
        )
      }
      return HttpResponse.json({
        operation: body.operation,
        operands: body.operands,
        result: reply.result,
        exact: reply.exact ?? true,
      })
    }),
  )
}
