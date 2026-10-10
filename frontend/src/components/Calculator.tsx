// The whole calculator: owns the state and wires it to the display, the
// keypad and the physical keyboard.

import { clearLabel, expression, isBusy } from '../calculator/reducer'
import { useCalculator } from '../hooks/useCalculator'
import { useKeyboard } from '../hooks/useKeyboard'
import { Display } from './Display'
import { Footer } from './Footer'
import { Keypad } from './Keypad'

export function Calculator() {
  const { state, press } = useCalculator()
  useKeyboard(press)

  return (
    <main className="calculator">
      <h1 className="visually-hidden">Calculator</h1>
      <Display
        value={state.display}
        exact={state.displayExact}
        expression={expression(state)}
        busy={isBusy(state)}
        error={state.phase === 'error'}
      />
      <Keypad
        clearLabel={clearLabel(state)}
        activeOp={state.phase === 'operator' ? state.pendingOp : null}
        onAction={press}
      />
      <Footer />
    </main>
  )
}
