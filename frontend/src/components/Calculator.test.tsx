// End-to-end flows through the real reducer, hooks and components, with only
// the API mocked (see docs/specs/frontend.md §7).

import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { mockCalculations, type MockReply } from '../test/server'
import { Calculator } from './Calculator'

// Keypad symbols to the buttons' accessible names.
const KEY_NAMES: Record<string, string> = {
  '+': 'add',
  '−': 'subtract',
  '×': 'multiply',
  '÷': 'divide',
  '^': 'power',
  '√': 'square root',
  '%': 'percent',
  '±': 'toggle sign',
  '=': 'equals',
  '.': 'decimal point',
}

/** Clicks keys given as space-separated tokens; numbers are typed digit by digit. */
async function click(keys: string) {
  const user = userEvent.setup()
  for (const token of keys.split(' ')) {
    const names = KEY_NAMES[token] ? [KEY_NAMES[token]] : [...token].map((c) => KEY_NAMES[c] ?? c)
    for (const name of names) await user.click(screen.getByRole('button', { name }))
  }
}

const value = () => screen.getByRole('status')
const expressionLine = () => screen.getByTestId('display-expression')

describe('Calculator', () => {
  it('starts at 0', () => {
    render(<Calculator />)
    expect(value()).toHaveTextContent(/^0$/)
    expect(screen.getByRole('button', { name: 'all clear' })).toBeInTheDocument()
  })

  it('shows the pending expression and highlights the operator', async () => {
    render(<Calculator />)
    await click('12 ×')
    expect(expressionLine()).toHaveTextContent('12 ×')
    expect(screen.getByRole('button', { name: 'multiply' })).toHaveAttribute('aria-pressed', 'true')
  })

  it('switches the clear key between C and AC', async () => {
    render(<Calculator />)
    await click('5')
    await userEvent.setup().click(screen.getByRole('button', { name: 'clear' }))
    expect(value()).toHaveTextContent(/^0$/)
    expect(screen.getByRole('button', { name: 'all clear' })).toBeInTheDocument()
  })

  it.each<{ keys: string; table: Record<string, MockReply>; want: RegExp }>([
    {
      keys: '2 + 3 × 4 =',
      table: { 'add 2 3': { result: '5' }, 'multiply 5 4': { result: '20' } },
      want: /^20$/,
    },
    { keys: '0.1 + 0.2 =', table: { 'add 0.1 0.2': { result: '0.3' } }, want: /^0\.3$/ },
    {
      keys: '200 + 10 % =',
      table: { 'percent_of 200 10': { result: '20' }, 'add 200 20': { result: '220' } },
      want: /^220$/,
    },
    {
      keys: '200 × 10 % =',
      table: { 'percent 10': { result: '0.1' }, 'multiply 200 0.1': { result: '20' } },
      want: /^20$/,
    },
    { keys: '16 √', table: { 'sqrt 16': { result: '4' } }, want: /^4$/ },
  ])('$keys gives $want', async ({ keys, table, want }) => {
    mockCalculations(table)
    render(<Calculator />)
    await click(keys)
    await waitFor(() => expect(value()).toHaveTextContent(want))
  })

  it('marks inexact results with ≈', async () => {
    mockCalculations({ 'divide 1 3': { result: '0.3333333333333333', exact: false } })
    render(<Calculator />)
    await click('1 ÷ 3 =')
    await waitFor(() => expect(value()).toHaveTextContent('≈approximately 0.3333333333333333'))
  })

  it('shows errors and recovers on the next digit', async () => {
    mockCalculations({ 'divide 1 0': { status: 422, code: 'DIVISION_BY_ZERO' } })
    render(<Calculator />)
    await click('1 ÷ 0 =')

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent("Can't divide by zero"))
    expect(value()).toHaveTextContent('Error')

    await click('7')
    expect(value()).toHaveTextContent(/^7$/)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('keeps keys typed faster than the API answers', async () => {
    mockCalculations({ 'percent_of 200 10': { result: '20' }, 'add 200 20': { result: '220' } })
    render(<Calculator />)

    // No delay between keys: '=' arrives while percent_of is still in flight.
    await userEvent.setup({ delay: null }).keyboard('200+10%=')

    await waitFor(() => expect(value()).toHaveTextContent(/^220$/))
  })

  it('works from the physical keyboard', async () => {
    mockCalculations({ 'add 2 3': { result: '5' }, 'multiply 5 4': { result: '20' } })
    render(<Calculator />)
    const user = userEvent.setup()

    await user.keyboard('2+3*4{Enter}')
    await waitFor(() => expect(value()).toHaveTextContent(/^20$/))

    await user.keyboard('{Escape}')
    expect(value()).toHaveTextContent(/^0$/)
  })
})
