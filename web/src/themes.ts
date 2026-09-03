export type ThemeManifest = {
  version: 1
  id: string
  brand: string
  eyebrow: string
  headline: string
  intro: string
  colors: {
    canvas: string
    panel: string
    panelRaised: string
    ink: string
    muted: string
    line: string
    primary: string
    primaryInk: string
    accent: string
    danger: string
  }
  shape: { panel: string; control: string }
}

const nerdsWhoFish: ThemeManifest = {
  version: 1,
  id: 'nerdswhofish',
  brand: 'Nerds Who Fish',
  eyebrow: 'Make a little room in the boat',
  headline: 'Let’s find a good time to talk.',
  intro: 'Pick the kind of conversation you need. We’ll check every connected calendar before showing a time.',
  colors: {
    canvas: '#07090f',
    panel: '#12171d',
    panelRaised: '#182029',
    ink: '#fcfbfc',
    muted: '#9faaa6',
    line: '#2a3539',
    primary: '#20d066',
    primaryInk: '#07090f',
    accent: '#20d066',
    danger: '#ff756d',
  },
  shape: { panel: '22px', control: '12px' },
}

const themes: Record<string, ThemeManifest> = { nerdswhofish: nerdsWhoFish }

export function themeByID(id: string): ThemeManifest {
  return themes[id] ?? nerdsWhoFish
}
