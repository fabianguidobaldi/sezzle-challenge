// Physical keyboard support: maps keydown events to keypad actions
// (see docs/specs/frontend.md §5).

import { useEffect } from 'react'
import { keyboardAction } from '../calculator/keys'
import type { KeyAction } from '../calculator/types'

export function useKeyboard(onAction: (action: KeyAction) => void): void {
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      const action = keyboardAction(event)
      if (!action) return
      // Also stops Enter from clicking a focused keypad button.
      event.preventDefault()
      onAction(action)
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [onAction])
}
