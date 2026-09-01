import { useEffect, useState } from 'react'
import { CalendarCheck, ExternalLink, Fish, ShieldCheck } from 'lucide-react'
import { api } from './api'
import { ThemeProvider } from './ThemeProvider'
import { themeByID } from './themes'

export default function Connect() {
  const [themeID, setThemeID] = useState('nerdswhofish')
  const [token] = useState(() => window.location.hash.slice(1))
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const connected = new URLSearchParams(window.location.search).get('status') === 'connected'

  useEffect(() => {
    if (window.location.hash) window.history.replaceState(null, '', window.location.pathname + window.location.search)
    api.config().then((config) => setThemeID(config.theme)).catch(() => undefined)
  }, [])

  const connect = async () => {
    setLoading(true)
    setError('')
    try {
      const result = await api.startCalendarInvitation(token)
      window.location.assign(result.url)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Could not start the Google connection')
      setLoading(false)
    }
  }

  return <ThemeProvider theme={themeByID(themeID)}>
    <main className="connect-shell">
      <a className="wordmark" href="/"><Fish size={24} /><span>NerdsWho<span className="wordmark-fish">Fish</span></span></a>
      <section className="connect-card">
        <p className="kicker">Calendar invitation</p>
        {connected ? <>
          <div className="success-mark"><CalendarCheck size={32} /></div>
          <h1>Calendar connected.</h1>
          <p>Your availability can now be included in booking checks. You can close this page.</p>
        </> : <>
          <h1>Share your availability.</h1>
          <p>Connect the Google account named in your invitation. The booking administrator can choose which calendars count as busy and add your account to meeting invites.</p>
          <div className="connect-trust"><ShieldCheck size={19} /><span>This grants calendar access only. It does not give you access to the booking admin.</span></div>
          {error && <div className="error" role="alert">{error}</div>}
          <button className="primary-button" disabled={loading || !token} onClick={connect}>{loading ? 'Opening Google…' : 'Connect Google Calendar'}<ExternalLink size={17} /></button>
          {!token && !error && <p className="connect-expired">This link is incomplete. Ask the booking administrator for a new invitation.</p>}
        </>}
      </section>
    </main>
  </ThemeProvider>
}
