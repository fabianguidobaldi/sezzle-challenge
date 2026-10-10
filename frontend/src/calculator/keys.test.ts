import { describe, expect, it } from 'vitest'
import { KEYPAD, keyboardAction } from './keys'
import type { KeyAction } from './types'

const press = (key: string, mods: { ctrlKey?: boolean; metaKey?: boolean; altKey?: boolean } = {}) =>
  keyboardAction({ key, ctrlKey: false, metaKey: false, altKey: false, ...mods })

describe('keyboardAction', () => {
  it.each('0123456789'.split(''))('maps digit %s', (d) => {
    expect(press(d)).toEqual({ type: 'DIGIT', digit: d })
  })

  const table: [string, KeyAction][] = [
    ['.', { type: 'DECIMAL' }],
    [',', { type: 'DECIMAL' }],
    ['+', { type: 'OPERATOR', op: 'add' }],
    ['-', { type: 'OPERATOR', op: 'subtract' }],
    ['*', { type: 'OPERATOR', op: 'multiply' }],
    ['/', { type: 'OPERATOR', op: 'divide' }],
    ['^', { type: 'OPERATOR', op: 'power' }],
    ['r', { type: 'SQRT' }],
    ['%', { type: 'PERCENT' }],
    ['Enter', { type: 'EQUALS' }],
    ['=', { type: 'EQUALS' }],
    ['Backspace', { type: 'BACKSPACE' }],
    ['Escape', { type: 'CLEAR' }],
    ['Delete', { type: 'CLEAR' }],
    ['F9', { type: 'TOGGLE_SIGN' }],
  ]

  it.each(table)('maps %s', (key, action) => {
    expect(press(key)).toEqual(action)
  })

  it.each(['ctrlKey', 'metaKey', 'altKey'] as const)('ignores keys pressed with %s', (mod) => {
    expect(press('1', { [mod]: true })).toBeNull()
    expect(press('Enter', { [mod]: true })).toBeNull()
    expect(press('+', { [mod]: true })).toBeNull()
  })

  it.each(['a', 'R', 'x', 'Tab', 'ArrowUp', 'F1', ' ', '12', 'toString', ''])('ignores %j', (key) => {
    expect(press(key)).toBeNull()
  })

  it('returns a fresh object each time', () => {
    const a = press('Enter')
    expect(a).not.toBe(press('Enter'))
  })
})

describe('KEYPAD', () => {
  it('has unique ids', () => {
    const ids = KEYPAD.map((k) => k.id)
    expect(new Set(ids).size).toBe(ids.length)
  })

  it('fills a 4 × 6 grid exactly', () => {
    const cells = KEYPAD.reduce((sum, k) => sum + (k.span?.cols ?? 1) * (k.span?.rows ?? 1), 0)
    expect(cells).toBe(24)
  })
})
