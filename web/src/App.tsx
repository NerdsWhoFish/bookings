import { useEffect, useMemo, useState } from 'react'
import { ArrowLeft, CalendarDays, Check, Clock3, Fish, MapPin, MoveRight, Radio, ShieldCheck } from 'lucide-react'
import { getWebInstrumentations, initializeFaro } from '@grafana/faro-web-sdk'
import { TracingInstrumentation } from '@grafana/faro-web-tracing'
import { api } from './api'
import { themeByID } from './themes'
import { ThemeProvider } from './ThemeProvider'
import { Turnstile } from './Turnstile'
import type { Confirmation, MeetingType, PublicConfig, Slot } from './types'

type Step = 'type' | 'time' | 'details' | 'done'

export default function App() {
  const [config, setConfig] = useState<PublicConfig>({ theme: 'nerdswhofish', turnstileSiteKey: '', faroURL: '', faroAppName: 'bookings' })
  const [meetings, setMeetings] = useState<MeetingType[]>([])
  const [meeting, setMeeting] = useState<MeetingType | null>(null)
  const [slots, setSlots] = useState<Slot[]>([])
  const [slot, setSlot] = useState<Slot | null>(null)
  const [step, setStep] = useState<Step>('type')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [confirmation, setConfirmation] = useState<Confirmation | null>(null)

  useEffect(() => {
    Promise.all([api.config(), api.meetingTypes()])
      .then(([nextConfig, nextMeetings]) => {
        setConfig(nextConfig)
        setMeetings(nextMeetings)
        if (nextConfig.faroURL) {
          initializeFaro({
            url: nextConfig.faroURL,
            instrumentations: [
              ...getWebInstrumentations({ captureConsole: true }),
              new TracingInstrumentation(),
            ],
            app: { name: nextConfig.faroAppName, version: 'dev', environment: 'production' },
          })
        }
      })
      .catch((reason: Error) => setError(reason.message))
      .finally(() => setLoading(false))
  }, [])

  const chooseMeeting = async (selected: MeetingType) => {
    setMeeting(selected)
    setStep('time')
    setLoading(true)
    setError('')
    try {
      setSlots(await api.availability(selected.slug))
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Could not load availability')
    } finally {
      setLoading(false)
    }
  }

  const restart = () => {
    setStep('type')
    setMeeting(null)
    setSlot(null)
    setConfirmation(null)
    setError('')
  }

  const theme = themeByID(config.theme)
  return (
    <ThemeProvider theme={theme}>
      <main className="shell">
        <aside className="brand-panel">
          <div className="sonar" aria-hidden="true"><span /><span /><span /></div>
          <a className="wordmark" href="/" onClick={(event) => { event.preventDefault(); restart() }}>
            <Fish size={24} strokeWidth={2.4} />
            <span>NerdsWho<span className="wordmark-fish">Fish</span></span>
          </a>
          <div className="brand-copy">
            <p className="eyebrow"><Radio size={14} /> {theme.eyebrow}</p>
            <h1>{theme.headline}</h1>
            <p>{theme.intro}</p>
          </div>
          <div className="trust"><ShieldCheck size={18} /><span>Availability is checked live. Nothing here signs you up for reminders.</span></div>
        </aside>

        <section className="booking-panel" aria-live="polite">
          <Progress step={step} />
          {error && <div className="error" role="alert">{error}</div>}
          {step === 'type' && <MeetingPicker meetings={meetings} loading={loading} onChoose={chooseMeeting} />}
          {step === 'time' && meeting && <TimePicker meeting={meeting} slots={slots} loading={loading} onBack={restart} onChoose={(selected) => { setSlot(selected); setStep('details') }} />}
          {step === 'details' && meeting && slot && <GuestDetails config={config} meeting={meeting} slot={slot} onBack={() => setStep('time')} onComplete={(result) => { setConfirmation(result); setStep('done') }} onError={setError} />}
          {step === 'done' && meeting && confirmation && <Success meeting={meeting} confirmation={confirmation} onRestart={restart} />}
        </section>
      </main>
      <footer><span>Built for conversations, not funnels.</span><a href="/admin">Admin</a></footer>
    </ThemeProvider>
  )
}

function Progress({ step }: { step: Step }) {
  const index = { type: 1, time: 2, details: 3, done: 4 }[step]
  return <div className="progress" role="progressbar" aria-label="Booking progress" aria-valuenow={index} aria-valuemin={1} aria-valuemax={4}><span style={{ width: `${index * 25}%` }} /></div>
}

function MeetingPicker({ meetings, loading, onChoose }: { meetings: MeetingType[]; loading: boolean; onChoose: (meeting: MeetingType) => void }) {
  return <div className="step-content">
    <div className="section-heading"><span className="step-number">01</span><div><p className="kicker">Choose your depth</p><h2>What do you need?</h2></div></div>
    {loading ? <Skeleton /> : <div className="meeting-list">
      {meetings.map((meeting) => <button className="meeting-card" key={meeting.id} onClick={() => onChoose(meeting)}>
        <span className="meeting-duration"><Clock3 size={17} /> {meeting.durationMinutes} min</span>
        <strong>{meeting.name}</strong>
        <span>{meeting.description}</span>
        <MoveRight className="meeting-arrow" aria-hidden="true" />
      </button>)}
    </div>}
  </div>
}

function TimePicker({ meeting, slots, loading, onBack, onChoose }: { meeting: MeetingType; slots: Slot[]; loading: boolean; onBack: () => void; onChoose: (slot: Slot) => void }) {
  const days = useMemo(() => groupByDay(slots, meeting.timeZone), [meeting.timeZone, slots])
  const [day, setDay] = useState('')
  useEffect(() => { if (!day && days.length) setDay(days[0][0]) }, [day, days])
  const visible = days.find(([key]) => key === day)?.[1] ?? []
  return <div className="step-content">
    <Back onClick={onBack} />
    <div className="section-heading"><span className="step-number">02</span><div><p className="kicker">{meeting.name} · {meeting.durationMinutes} min</p><h2>Pick a clear patch.</h2></div></div>
    {loading ? <Skeleton /> : days.length === 0 ? <div className="empty"><CalendarDays /><h3>No open water yet.</h3><p>Try again later or pick another meeting type.</p></div> : <>
      <div className="day-strip" role="tablist" aria-label="Available days">
        {days.map(([key, values]) => <button role="tab" aria-selected={key === day} key={key} onClick={() => setDay(key)}>
          <span>{format(values[0].start, meeting.timeZone, { weekday: 'short' })}</span>
          <strong>{format(values[0].start, meeting.timeZone, { day: 'numeric' })}</strong>
          <small>{format(values[0].start, meeting.timeZone, { month: 'short' })}</small>
        </button>)}
      </div>
      <div className="time-grid">
        {visible.map((available) => <button key={available.start} onClick={() => onChoose(available)}>{format(available.start, meeting.timeZone, { hour: 'numeric', minute: '2-digit' })}</button>)}
      </div>
      <p className="timezone">Times shown in {meeting.timeZone.replaceAll('_', ' ')}</p>
    </>}
  </div>
}

function GuestDetails({ config, meeting, slot, onBack, onComplete, onError }: { config: PublicConfig; meeting: MeetingType; slot: Slot; onBack: () => void; onComplete: (confirmation: Confirmation) => void; onError: (message: string) => void }) {
  const [submitting, setSubmitting] = useState(false)
  const [token, setToken] = useState('')
  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const data = new FormData(event.currentTarget)
    setSubmitting(true)
    onError('')
    try {
      onComplete(await api.book({
        meetingTypeSlug: meeting.slug,
        start: slot.start,
        guestName: String(data.get('name')),
        guestEmail: String(data.get('email')),
        guestNotes: String(data.get('notes')),
        turnstileToken: token,
      }))
    } catch (reason) {
      onError(reason instanceof Error ? reason.message : 'Could not finish the booking')
    } finally {
      setSubmitting(false)
    }
  }
  return <div className="step-content">
    <Back onClick={onBack} />
    <div className="section-heading"><span className="step-number">03</span><div><p className="kicker">One last thing</p><h2>Who’s joining?</h2></div></div>
    <div className="selection-summary"><CalendarDays size={19} /><div><strong>{format(slot.start, meeting.timeZone, { weekday: 'long', month: 'long', day: 'numeric' })}</strong><span>{format(slot.start, meeting.timeZone, { hour: 'numeric', minute: '2-digit' })} · {meeting.durationMinutes} minutes · {meeting.location}</span></div></div>
    <form onSubmit={submit}>
      <label>Name<input name="name" autoComplete="name" maxLength={120} required /></label>
      <label>Email<input name="email" type="email" autoComplete="email" required /></label>
      <label>Anything useful before we talk?<textarea name="notes" rows={4} maxLength={2000} placeholder="Context, links, or the problem you want to untangle." /></label>
      <Turnstile siteKey={config.turnstileSiteKey} onToken={setToken} />
      <button className="primary-button" disabled={submitting || (!!config.turnstileSiteKey && !token)}>{submitting ? 'Checking calendars…' : 'Book this time'}<MoveRight size={18} /></button>
    </form>
  </div>
}

function Success({ meeting, confirmation, onRestart }: { meeting: MeetingType; confirmation: Confirmation; onRestart: () => void }) {
  return <div className="step-content success">
    <div className="success-mark"><Check size={32} /></div>
    <p className="kicker">You’re on the calendar</p>
    <h2>Nice. We’ll see you there.</h2>
    <p>An invite is headed to <strong>{confirmation.booking.guestEmail}</strong>. There are no reminder campaigns hiding behind it.</p>
    <div className="ticket">
      <span>{meeting.name}</span>
      <strong>{format(confirmation.booking.start, meeting.timeZone, { weekday: 'long', month: 'long', day: 'numeric' })}</strong>
      <span><Clock3 size={16} /> {format(confirmation.booking.start, meeting.timeZone, { hour: 'numeric', minute: '2-digit' })}</span>
      <span><MapPin size={16} /> {meeting.location}</span>
    </div>
    <button className="quiet-button" onClick={onRestart}>Book another conversation</button>
  </div>
}

function Back({ onClick }: { onClick: () => void }) {
  return <button className="back" onClick={onClick}><ArrowLeft size={17} /> Back</button>
}

function Skeleton() {
  return <div className="skeleton" aria-label="Loading"><span /><span /><span /></div>
}

function groupByDay(slots: Slot[], timeZone: string): [string, Slot[]][] {
  const grouped = new Map<string, Slot[]>()
  for (const slot of slots) {
    const key = format(slot.start, timeZone, { year: 'numeric', month: '2-digit', day: '2-digit' })
    grouped.set(key, [...(grouped.get(key) ?? []), slot])
  }
  return Array.from(grouped.entries()).slice(0, 8)
}

function format(value: string, timeZone: string, options: Intl.DateTimeFormatOptions) {
  return new Intl.DateTimeFormat(undefined, { timeZone, ...options }).format(new Date(value))
}
