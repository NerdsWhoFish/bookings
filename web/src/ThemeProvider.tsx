import type { CSSProperties, ReactNode } from 'react'
import type { ThemeManifest } from './themes'

export function ThemeProvider({ theme, children }: { theme: ThemeManifest; children: ReactNode }) {
  const style = {
    '--canvas': theme.colors.canvas,
    '--panel': theme.colors.panel,
    '--panel-raised': theme.colors.panelRaised,
    '--ink': theme.colors.ink,
    '--muted': theme.colors.muted,
    '--line': theme.colors.line,
    '--primary': theme.colors.primary,
    '--primary-ink': theme.colors.primaryInk,
    '--accent': theme.colors.accent,
    '--danger': theme.colors.danger,
    '--panel-radius': theme.shape.panel,
    '--control-radius': theme.shape.control,
  } as CSSProperties
  return <div className={`theme theme-${theme.id}`} style={style}>{children}</div>
}
