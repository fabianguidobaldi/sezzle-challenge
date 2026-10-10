import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Footer } from './Footer'

describe('Footer', () => {
  it('credits the makers, with the icons out of the accessible text', () => {
    render(<Footer />)
    const footer = screen.getByRole('contentinfo')
    expect(footer).toHaveTextContent('Made with ❤️ and Claude')
    expect(screen.getByRole('img', { name: 'love' })).toBeInTheDocument()
    expect(footer.querySelector('svg')).toHaveAttribute('aria-hidden', 'true')
  })
})
