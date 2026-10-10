// Keypad layout and keyboard map (see docs/specs/frontend.md §5 and §6.1).

import type { BinaryOp, Digit, KeyAction } from './types'

export type KeyVariant = 'digit' | 'operator' | 'function' | 'equals'

/** One on-screen key. The keypad renders these in reading order on a 4-column grid. */
export interface KeyDef {
  id: string
  label: string // the clear key's label is replaced at render time (AC/C)
  ariaLabel: string
  action: KeyAction
  variant: KeyVariant
  span?: { cols?: number; rows?: number }
}

const digit = (d: Digit, span?: KeyDef['span']): KeyDef => ({
  id: `digit-${d}`,
  label: d,
  ariaLabel: d,
  action: { type: 'DIGIT', digit: d },
  variant: 'digit',
  span,
})

const operator = (op: BinaryOp, label: string, ariaLabel: string): KeyDef => ({
  id: op,
  label,
  ariaLabel,
  action: { type: 'OPERATOR', op },
  variant: 'operator',
})

export const KEYPAD: KeyDef[] = [
  { id: 'clear', label: 'AC', ariaLabel: 'clear', action: { type: 'CLEAR' }, variant: 'function' },
  { id: 'backspace', label: '⌫', ariaLabel: 'backspace', action: { type: 'BACKSPACE' }, variant: 'function' },
  { id: 'percent', label: '%', ariaLabel: 'percent', action: { type: 'PERCENT' }, variant: 'function' },
  operator('divide', '÷', 'divide'),

  { id: 'sqrt', label: '√', ariaLabel: 'square root', action: { type: 'SQRT' }, variant: 'function' },
  operator('power', 'xʸ', 'power'),
  { id: 'toggle-sign', label: '±', ariaLabel: 'toggle sign', action: { type: 'TOGGLE_SIGN' }, variant: 'function' },
  operator('multiply', '×', 'multiply'),

  digit('7'),
  digit('8'),
  digit('9'),
  operator('subtract', '−', 'subtract'),

  digit('4'),
  digit('5'),
  digit('6'),
  operator('add', '+', 'add'),

  digit('1'),
  digit('2'),
  digit('3'),
  { id: 'equals', label: '=', ariaLabel: 'equals', action: { type: 'EQUALS' }, variant: 'equals', span: { rows: 2 } },

  digit('0', { cols: 2 }),
  { id: 'decimal', label: '.', ariaLabel: 'decimal point', action: { type: 'DECIMAL' }, variant: 'digit' },
]

const OPERATOR_KEYS: Record<string, BinaryOp> = {
  '+': 'add',
  '-': 'subtract',
  '*': 'multiply',
  '/': 'divide',
  '^': 'power',
}

const SIMPLE_KEYS: Record<string, KeyAction> = {
  '.': { type: 'DECIMAL' },
  ',': { type: 'DECIMAL' },
  r: { type: 'SQRT' },
  '%': { type: 'PERCENT' },
  Enter: { type: 'EQUALS' },
  '=': { type: 'EQUALS' },
  Backspace: { type: 'BACKSPACE' },
  Escape: { type: 'CLEAR' },
  Delete: { type: 'CLEAR' },
  F9: { type: 'TOGGLE_SIGN' },
}

/**
 * Maps a keyboard event to a keypad action, or null if the key isn't handled.
 * Ctrl/Meta/Alt combinations are ignored so browser shortcuts keep working;
 * Shift is allowed because +, *, % and ^ need it on many layouts.
 */
export function keyboardAction(
  e: Pick<KeyboardEvent, 'key' | 'ctrlKey' | 'metaKey' | 'altKey'>,
): KeyAction | null {
  if (e.ctrlKey || e.metaKey || e.altKey) return null
  const { key } = e
  if (/^[0-9]$/.test(key)) return { type: 'DIGIT', digit: key as Digit }
  if (Object.hasOwn(OPERATOR_KEYS, key)) return { type: 'OPERATOR', op: OPERATOR_KEYS[key] }
  if (Object.hasOwn(SIMPLE_KEYS, key)) return { ...SIMPLE_KEYS[key] }
  return null
}
