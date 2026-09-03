import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { BrandLogo } from './BrandLogo'

describe('BrandLogo', () => {
  it('renders the approved Nerds Who Fish asset', () => {
    render(<BrandLogo href="/" />)

    expect(screen.getByRole('link')).toHaveAttribute('href', '/')
    expect(screen.getByRole('img', { name: 'Nerds Who Fish, Software Solutions' }))
      .toHaveAttribute('src', '/assets/brand/nwf-logo.svg')
  })
})
