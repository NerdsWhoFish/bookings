import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { themeByID } from './themes'
import { ThemeProvider } from './ThemeProvider'

describe('themes', () => {
  it('falls back to the bundled Nerds Who Fish theme', () => {
    const theme = themeByID('missing')
    expect(theme.id).toBe('nerdswhofish')
    const { container } = render(<ThemeProvider theme={theme}><span>booking</span></ThemeProvider>)
    expect(container.firstElementChild).toHaveClass('theme-nerdswhofish')
  })
})
