// The on-screen keypad, rendered from the KEYPAD definition.

import { KEYPAD, type KeyDef } from '../calculator/keys'
import type { BinaryOp, KeyAction } from '../calculator/types'

export interface KeypadProps {
  clearLabel: 'AC' | 'C'
  activeOp: BinaryOp | null // highlighted while waiting for the right operand
  onAction: (action: KeyAction) => void
}

function keyClass(key: KeyDef): string {
  const classes = ['key', `key--${key.variant}`]
  if (key.span?.cols === 2) classes.push('key--wide')
  if (key.span?.rows === 2) classes.push('key--tall')
  return classes.join(' ')
}

export function Keypad({ clearLabel, activeOp, onAction }: KeypadProps) {
  return (
    <div className="keypad" role="group" aria-label="Keypad">
      {KEYPAD.map((key) => {
        const isClear = key.action.type === 'CLEAR'
        const op = key.action.type === 'OPERATOR' ? key.action.op : null
        return (
          <button
            key={key.id}
            type="button"
            className={keyClass(key)}
            aria-label={isClear ? (clearLabel === 'AC' ? 'all clear' : 'clear') : key.ariaLabel}
            aria-pressed={op ? activeOp === op : undefined}
            onClick={() => onAction(key.action)}
          >
            {isClear ? clearLabel : key.label}
          </button>
        )
      })}
    </div>
  )
}
