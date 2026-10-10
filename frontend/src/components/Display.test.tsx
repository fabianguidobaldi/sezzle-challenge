import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Display, type DisplayProps } from './Display'

const renderDisplay = (props: Partial<DisplayProps> = {}) =>
  render(<Display value="0" exact expression="" busy={false} {...props} />)

describe('Display', () => {
  it('shows the value in a polite live output', () => {
    renderDisplay({ value: '123.45' })
    const output = screen.getByRole('status')
    expect(output.tagName).toBe('OUTPUT')
    expect(output).toHaveAttribute('aria-live', 'polite')
    expect(output).toHaveTextContent(/^123\.45$/)
  })

  it('shows no ≈ marker when exact', () => {
    renderDisplay({ value: '0.3' })
    expect(screen.queryByText('≈')).not.toBeInTheDocument()
    expect(screen.queryByText(/approximately/)).not.toBeInTheDocument()
  })

  it('shows the ≈ marker with an accessible name when inexact', () => {
    renderDisplay({ value: '0.3333333333333333', exact: false })
    expect(screen.getByText('≈')).toHaveAttribute('aria-hidden', 'true')
    expect(screen.getByRole('status')).toHaveTextContent('≈approximately 0.3333333333333333')
  })

  it('shows the pending expression', () => {
    renderDisplay({ expression: '12 ×' })
    const expr = screen.getByTestId('display-expression')
    expect(expr).toHaveTextContent('12 ×')
    expect(expr).not.toHaveClass('display__expression--error')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('shows an error message as an alert', () => {
    renderDisplay({ expression: "Can't divide by zero", error: true })
    const alert = screen.getByRole('alert')
    expect(alert).toHaveTextContent("Can't divide by zero")
    expect(alert).toHaveClass('display__expression--error')
  })

  it.each([true, false])('sets aria-busy=%s', (busy) => {
    renderDisplay({ busy })
    expect(screen.getByRole('region', { name: 'Display' })).toHaveAttribute('aria-busy', String(busy))
  })

  it.each([
    ['0', 'display__value--xl'],
    ['1234567', 'display__value--xl'],
    ['12345678', 'display__value--lg'],
    ['1234567890', 'display__value--lg'],
    ['12345678901', 'display__value--md'],
    ['12345678901234', 'display__value--md'],
    ['123456789012345', 'display__value--sm'],
    ['-0.1234567890123456', 'display__value--sm'],
  ])('steps the font size down for %s', (value, cls) => {
    renderDisplay({ value })
    expect(screen.getByRole('status')).toHaveClass(cls)
    expect(screen.getByRole('status')).toHaveTextContent(value)
  })
})
