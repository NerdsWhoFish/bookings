import { useEffect, useRef } from 'react'

declare global {
  interface Window {
    turnstile?: {
      render: (element: HTMLElement, options: Record<string, unknown>) => string
      remove: (id: string) => void
    }
  }
}

export function Turnstile({ siteKey, onToken }: { siteKey: string; onToken: (token: string) => void }) {
  const target = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!siteKey || !target.current) return
    let widget = ''
    const render = () => {
      if (window.turnstile && target.current && !widget) {
        widget = window.turnstile.render(target.current, {
          sitekey: siteKey,
          theme: 'dark',
          callback: onToken,
          'expired-callback': () => onToken(''),
        })
      }
    }
    const existing = document.querySelector<HTMLScriptElement>('script[data-bookings-turnstile]')
    if (existing) {
      existing.addEventListener('load', render)
      render()
    } else {
      const script = document.createElement('script')
      script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit'
      script.async = true
      script.defer = true
      script.dataset.bookingsTurnstile = 'true'
      script.addEventListener('load', render)
      document.head.appendChild(script)
    }
    return () => {
      if (widget) window.turnstile?.remove(widget)
    }
  }, [onToken, siteKey])

  if (!siteKey) return null
  return <div className="turnstile" ref={target} aria-label="Spam protection" />
}

