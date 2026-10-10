import { delay, http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { CALCULATE_URL, mockCalculations, server } from '../test/server'
import { ApiError, calculate, NetworkError } from './client'

/** Answers every calculate request with the given response. */
function reply(response: () => Response | Promise<Response>): void {
  server.use(http.post(CALCULATE_URL, response))
}

describe('calculate', () => {
  it('returns the parsed body on success', async () => {
    mockCalculations({ 'divide 1 3': { result: '0.3333333333333333', exact: false } })
    await expect(calculate('divide', ['1', '3'])).resolves.toEqual({
      operation: 'divide',
      operands: ['1', '3'],
      result: '0.3333333333333333',
      exact: false,
    })
  })

  it('POSTs the operation and operands as JSON', async () => {
    let sent: Request | undefined
    let body: unknown
    server.use(
      http.post(CALCULATE_URL, async ({ request }) => {
        sent = request
        body = await request.json()
        return HttpResponse.json({ operation: 'sqrt', operands: ['2'], result: '1.414', exact: false })
      }),
    )
    await calculate('sqrt', ['2'])
    expect(sent?.method).toBe('POST')
    expect(sent?.headers.get('Content-Type')).toBe('application/json')
    expect(body).toEqual({ operation: 'sqrt', operands: ['2'] })
  })

  it.each([
    [422, 'DIVISION_BY_ZERO', 'division by zero'],
    [400, 'INVALID_NUMBER', 'operand 1 is not a valid number'],
  ])('throws ApiError for a %i error envelope', async (status, code, message) => {
    reply(() => HttpResponse.json({ error: { code, message } }, { status }))
    const err = await calculate('divide', ['1', '0']).catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ name: 'ApiError', status, code, message })
  })

  it('throws an UNKNOWN ApiError for a non-JSON error response', async () => {
    reply(() => HttpResponse.html('<h1>502 Bad Gateway</h1>', { status: 502 }))
    await expect(calculate('add', ['1', '2'])).rejects.toMatchObject({
      name: 'ApiError',
      status: 502,
      code: 'UNKNOWN',
      message: 'Unexpected server response',
    })
  })

  it.each([
    ['an object without the envelope', { message: 'nope' }],
    ['an envelope with a non-string code', { error: { code: 42, message: 'x' } }],
    ['an envelope without a message', { error: { code: 'X' } }],
    ['a non-object error', { error: 'boom' }],
    ['an array', [1, 2]],
    ['null', null],
  ])('throws an UNKNOWN ApiError for JSON that is %s', async (_, json) => {
    reply(() => HttpResponse.json(json, { status: 500 }))
    await expect(calculate('add', ['1', '2'])).rejects.toMatchObject({ status: 500, code: 'UNKNOWN' })
  })

  it.each([
    ['not JSON', () => HttpResponse.text('ok')],
    ['empty', () => new HttpResponse(null, { status: 200 })],
    ['missing the result', () => HttpResponse.json({ operation: 'add', operands: ['1', '2'], exact: true })],
    ['a numeric result', () => HttpResponse.json({ operation: 'add', operands: ['1', '2'], result: 3, exact: true })],
    ['non-string operands', () => HttpResponse.json({ operation: 'add', operands: [1, 2], result: '3', exact: true })],
  ])('throws an UNKNOWN ApiError for a 2xx body that is %s', async (_, response) => {
    reply(response)
    await expect(calculate('add', ['1', '2'])).rejects.toMatchObject({
      name: 'ApiError',
      status: 200,
      code: 'UNKNOWN',
    })
  })

  it('throws NetworkError when fetch rejects', async () => {
    reply(() => HttpResponse.error())
    const err = await calculate('add', ['1', '2']).catch((e: unknown) => e)
    expect(err).toBeInstanceOf(NetworkError)
    expect(err).toMatchObject({ name: 'NetworkError' })
    expect((err as NetworkError).cause).toBeInstanceOf(TypeError)
  })

  it('re-throws the AbortError for an already-aborted signal', async () => {
    reply(() => HttpResponse.json({ operation: 'add', operands: ['1', '2'], result: '3', exact: true }))
    const controller = new AbortController()
    controller.abort()
    const err = await calculate('add', ['1', '2'], controller.signal).catch((e: unknown) => e)
    expect(err).toMatchObject({ name: 'AbortError' })
    expect(err).not.toBeInstanceOf(NetworkError)
  })

  it('re-throws the AbortError when aborted while pending', async () => {
    let received!: () => void
    const requested = new Promise<void>((resolve) => (received = resolve))
    server.use(
      http.post(CALCULATE_URL, async () => {
        received()
        await delay('infinite')
        return HttpResponse.json({})
      }),
    )
    const controller = new AbortController()
    const pending = calculate('add', ['1', '2'], controller.signal).catch((e: unknown) => e)
    await requested
    controller.abort()
    const err = await pending
    expect(err).toMatchObject({ name: 'AbortError' })
    expect(err).not.toBeInstanceOf(NetworkError)
  })
})
