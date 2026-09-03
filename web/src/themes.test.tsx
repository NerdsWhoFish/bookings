import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { themeByID } from './themes'
import { ThemeProvider } from './ThemeProvider'

describe('themes', () => {
  it('falls back to the bundled Nerds Who Fish theme', () => {
    const theme = themeByID('missing')
    expect(theme.id).toBe('nerdswhofish')
    expect(theme.colors).toMatchObject({
      canvas: '#07090f',
      ink: '#fcfbfc',
      primary: '#20d066',
    })
    const { container } = render(<ThemeProvider theme={theme}><span>booking</span></ThemeProvider>)
    expect(container.firstElementChild).toHaveClass('theme-nerdswhofish')
  })
})
