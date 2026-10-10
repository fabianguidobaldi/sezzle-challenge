import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import App from './App'
import { CALCULATE_URL, mockCalculations } from './test/server'

describe('App', () => {
  it('renders', () => {
    render(<App />)
    expect(screen.getByRole('heading', { name: 'Calculator' })).toBeInTheDocument()
  })
})

// Checks the test pipeline itself: relative /api URLs are intercepted by MSW.
describe('test setup', () => {
  it('mocks calculations by operation and operands', async () => {
    mockCalculations({ 'add 0.1 0.2': { result: '0.3' } })
    const res = await fetch(CALCULATE_URL, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ operation: 'add', operands: ['0.1', '0.2'] }),
    })
    expect(await res.json()).toEqual({ operation: 'add', operands: ['0.1', '0.2'], result: '0.3', exact: true })
  })
})
