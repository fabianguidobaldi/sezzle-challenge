// Connects the pure reducer to the API: whenever the reducer describes a
// request, send it and dispatch the outcome (see docs/specs/frontend.md §5).

import { useEffect, useReducer } from 'react'
import { calculate } from '../api/client'
import { messageFor } from '../calculator/messages'
import { initialState, reducer } from '../calculator/reducer'
import type { KeyAction, State } from '../calculator/types'

export interface Calculator {
  state: State
  press: (action: KeyAction) => void
}

export function useCalculator(): Calculator {
  const [state, dispatch] = useReducer(reducer, initialState)
  const { request } = state

  useEffect(() => {
    if (!request) return
    const controller = new AbortController()
    calculate(request.operation, request.operands, controller.signal).then(
      (res) => dispatch({ type: 'CALC_SUCCESS', id: request.id, result: res.result, exact: res.exact }),
      (err: unknown) => {
        // Aborted by AC or unmount: the reducer has already moved on.
        if (controller.signal.aborted) return
        dispatch({ type: 'CALC_FAILURE', id: request.id, message: messageFor(err) })
      },
    )
    return () => controller.abort()
  }, [request])

  return { state, press: dispatch }
}
