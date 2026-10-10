import { act, renderHook, waitFor } from '@testing-library/react'
import { delay, http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import type { KeyAction } from '../calculator/types'
import { CALCULATE_URL, mockCalculations, server } from '../test/server'
import { useCalculator } from './useCalculator'

const two: KeyAction = { type: 'DIGIT', digit: '2' }
const three: KeyAction = { type: 'DIGIT', digit: '3' }
const plus: KeyAction = { type: 'OPERATOR', op: 'add' }
const equals: KeyAction = { type: 'EQUALS' }

function pressAll(press: (action: KeyAction) => void, actions: KeyAction[]) {
  act(() => actions.forEach(press))
}

describe('useCalculator', () => {
  it('sends the request the reducer describes and shows the result', async () => {
    mockCalculations({ 'add 2 3': { result: '5' } })
    const { result } = renderHook(() => useCalculator())

    pressAll(result.current.press, [two, plus, three, equals])
    expect(result.current.state.request).not.toBeNull()

    await waitFor(() => expect(result.current.state.display).toBe('5'))
    expect(result.current.state).toMatchObject({ phase: 'result', displayExact: true, request: null })
  })

  it('carries the exact flag from the response', async () => {
    mockCalculations({ 'add 2 3': { result: '5', exact: false } })
    const { result } = renderHook(() => useCalculator())

    pressAll(result.current.press, [two, plus, three, equals])

    await waitFor(() => expect(result.current.state.display).toBe('5'))
    expect(result.current.state.displayExact).toBe(false)
  })

  it.each([
    ['an API error', { status: 422, code: 'DIVISION_BY_ZERO' }, "Can't divide by zero"],
    ['an unknown API error', { status: 500, code: 'INTERNAL_ERROR' }, 'Something went wrong'],
  ])('turns %s into an error message', async (_, reply, message) => {
    mockCalculations({ 'add 2 3': reply })
    const { result } = renderHook(() => useCalculator())

    pressAll(result.current.press, [two, plus, three, equals])

    await waitFor(() => expect(result.current.state.phase).toBe('error'))
    expect(result.current.state.error).toBe(message)
  })

  it('reports a network failure', async () => {
    server.use(http.post(CALCULATE_URL, () => HttpResponse.error()))
    const { result } = renderHook(() => useCalculator())

    pressAll(result.current.press, [two, plus, three, equals])

    await waitFor(() => expect(result.current.state.error).toBe("Can't reach the server"))
  })

  it('aborts the in-flight request on AC and ignores it', async () => {
    let aborted = false
    server.use(
      http.post(CALCULATE_URL, async ({ request }) => {
        request.signal.addEventListener('abort', () => {
          aborted = true
        })
        await delay('infinite')
        return HttpResponse.json({})
      }),
    )
    const { result } = renderHook(() => useCalculator())

    pressAll(result.current.press, [two, plus, three, equals])
    await waitFor(() => expect(result.current.state.request).not.toBeNull())
    pressAll(result.current.press, [{ type: 'CLEAR' }])

    await waitFor(() => expect(aborted).toBe(true))
    expect(result.current.state).toMatchObject({ display: '0', phase: 'input', request: null, error: null })
  })

  it('aborts the in-flight request on unmount', async () => {
    let aborted = false
    server.use(
      http.post(CALCULATE_URL, async ({ request }) => {
        request.signal.addEventListener('abort', () => {
          aborted = true
        })
        await delay('infinite')
        return HttpResponse.json({})
      }),
    )
    const { result, unmount } = renderHook(() => useCalculator())

    pressAll(result.current.press, [two, plus, three, equals])
    await waitFor(() => expect(result.current.state.request).not.toBeNull())
    unmount()

    await waitFor(() => expect(aborted).toBe(true))
  })
})
