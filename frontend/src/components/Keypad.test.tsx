import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { BinaryOp, KeyAction } from '../calculator/types'
import { Keypad, type KeypadProps } from './Keypad'

const renderKeypad = (props: Partial<KeypadProps> = {}) => {
  const onAction = vi.fn<(action: KeyAction) => void>()
  render(<Keypad clearLabel="AC" activeOp={null} onAction={onAction} {...props} />)
  return onAction
}

// [label, aria-label, action] in reading order (spec §6.1).
const EXPECTED: [string, string, KeyAction][] = [
  ['AC', 'all clear', { type: 'CLEAR' }],
  ['⌫', 'backspace', { type: 'BACKSPACE' }],
  ['%', 'percent', { type: 'PERCENT' }],
  ['÷', 'divide', { type: 'OPERATOR', op: 'divide' }],
  ['√', 'square root', { type: 'SQRT' }],
  ['xʸ', 'power', { type: 'OPERATOR', op: 'power' }],
  ['±', 'toggle sign', { type: 'TOGGLE_SIGN' }],
  ['×', 'multiply', { type: 'OPERATOR', op: 'multiply' }],
  ['7', '7', { type: 'DIGIT', digit: '7' }],
  ['8', '8', { type: 'DIGIT', digit: '8' }],
  ['9', '9', { type: 'DIGIT', digit: '9' }],
  ['−', 'subtract', { type: 'OPERATOR', op: 'subtract' }],
  ['4', '4', { type: 'DIGIT', digit: '4' }],
  ['5', '5', { type: 'DIGIT', digit: '5' }],
  ['6', '6', { type: 'DIGIT', digit: '6' }],
  ['+', 'add', { type: 'OPERATOR', op: 'add' }],
  ['1', '1', { type: 'DIGIT', digit: '1' }],
  ['2', '2', { type: 'DIGIT', digit: '2' }],
  ['3', '3', { type: 'DIGIT', digit: '3' }],
  ['=', 'equals', { type: 'EQUALS' }],
  ['0', '0', { type: 'DIGIT', digit: '0' }],
  ['.', 'decimal point', { type: 'DECIMAL' }],
]

const OPS: [BinaryOp, string][] = [
  ['divide', 'divide'],
  ['power', 'power'],
  ['multiply', 'multiply'],
  ['subtract', 'subtract'],
  ['add', 'add'],
]

describe('Keypad', () => {
  it('renders the keys in layout order with labels and aria-labels', () => {
    renderKeypad()
    const buttons = screen.getAllByRole('button')
    expect(buttons.map((b) => b.textContent)).toEqual(EXPECTED.map(([label]) => label))
    expect(buttons.map((b) => b.getAttribute('aria-label'))).toEqual(EXPECTED.map(([, aria]) => aria))
    for (const b of buttons) expect(b).toHaveAttribute('type', 'button')
  })

  it('spans the 0 key across two columns and = across two rows', () => {
    renderKeypad()
    expect(screen.getByRole('button', { name: '0' })).toHaveClass('key--wide')
    expect(screen.getByRole('button', { name: 'equals' })).toHaveClass('key--tall', 'key--equals')
    expect(screen.getByRole('button', { name: '7' })).not.toHaveClass('key--wide', 'key--tall')
  })

  it('gives each key its variant class', () => {
    renderKeypad()
    expect(screen.getByRole('button', { name: '5' })).toHaveClass('key--digit')
    expect(screen.getByRole('button', { name: 'decimal point' })).toHaveClass('key--digit')
    expect(screen.getByRole('button', { name: 'square root' })).toHaveClass('key--function')
    expect(screen.getByRole('button', { name: 'add' })).toHaveClass('key--operator')
  })

  it.each(EXPECTED)('clicking %s dispatches its action', async (_label, aria, action) => {
    const user = userEvent.setup()
    const onAction = renderKeypad()
    await user.click(screen.getByRole('button', { name: aria }))
    expect(onAction).toHaveBeenCalledTimes(1)
    expect(onAction).toHaveBeenCalledWith(action)
  })

  it('shows the C label with a "clear" aria-label', async () => {
    const user = userEvent.setup()
    const onAction = renderKeypad({ clearLabel: 'C' })
    const key = screen.getByRole('button', { name: 'clear' })
    expect(key).toHaveTextContent(/^C$/)
    await user.click(key)
    expect(onAction).toHaveBeenCalledWith({ type: 'CLEAR' })
  })

  it('marks no operator as pressed when activeOp is null', () => {
    renderKeypad()
    for (const [, name] of OPS) {
      expect(screen.getByRole('button', { name })).toHaveAttribute('aria-pressed', 'false')
    }
  })

  it.each(OPS)('marks only %s as pressed when active', (op, name) => {
    renderKeypad({ activeOp: op })
    expect(screen.getByRole('button', { name, pressed: true })).toBeInTheDocument()
    expect(screen.getAllByRole('button', { pressed: true })).toHaveLength(1)
  })

  it('leaves aria-pressed off non-operator keys', () => {
    renderKeypad({ activeOp: 'add' })
    expect(screen.getByRole('button', { name: '1' })).not.toHaveAttribute('aria-pressed')
    expect(screen.getByRole('button', { name: 'equals' })).not.toHaveAttribute('aria-pressed')
  })
})
