import { useEffect, useState } from 'react'
import { ArrowLeft, CalendarCheck, Check, ExternalLink, Fish, Plus, Save } from 'lucide-react'
import { api } from './api'
import { ThemeProvider } from './ThemeProvider'
import { themeByID } from './themes'
import type { CalendarConnection, CalendarInfo, MeetingType, Session } from './types'

export default function Admin() {
  const [session, setSession] = useState<Session | null>(null)
  const [connections, setConnections] = useState<CalendarConnection[]>([])
  const [meetings, setMeetings] = useState<MeetingType[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    Promise.all([api.adminSession(), api.connections(), api.meetingTypes()])
      .then(([nextSession, nextConnections, nextMeetings]) => {
        setSession(nextSession)
        setConnections(nextConnections)
        setMeetings(nextMeetings)
      })
      .catch((reason: Error) => setError(reason.message))
  }, [])

  return <ThemeProvider theme={themeByID('nerdswhofish')}>
    <main className="admin-shell">
      <header className="admin-header">
        <a className="wordmark" href="/"><Fish size={24} /><span>NerdsWho<span className="wordmark-fish">Fish</span></span></a>
        <a className="back-link" href="/"><ArrowLeft size={16} /> Public page</a>
      </header>
      <div className="admin-title"><p className="kicker">Booking controls</p><h1>Keep the calendars tidy.</h1><p>{session ? `Signed in as ${session.email}` : 'Connect an allowed Google account to manage this deployment.'}</p></div>
      {error && <div className="error" role="alert">{error}</div>}
      {!session ? <a className="primary-button admin-signin" href="/api/admin/google/start">Continue with Google <ExternalLink size={17} /></a> : <div className="admin-grid">
        <section className="admin-section">
          <div className="admin-section-heading"><div><p className="kicker">Google Calendar</p><h2>Busy calendars</h2></div><a className="icon-link" href="/api/admin/google/start"><Plus size={17} /> Add account</a></div>
          {connections.length === 0 ? <div className="admin-empty"><CalendarCheck /><strong>No Google accounts yet</strong><span>Add one before publishing meeting types.</span></div> : connections.map((connection) => <ConnectionEditor key={connection.id} connection={connection} onSaved={(next) => setConnections((values) => values.map((value) => value.id === next.id ? next : value))} onError={setError} />)}
        </section>
        <section className="admin-section">
          <div className="admin-section-heading"><div><p className="kicker">Meeting catalog</p><h2>Types and lengths</h2></div></div>
          <div className="admin-meetings">{meetings.map((meeting) => <MeetingEditor key={meeting.id} meeting={meeting} connections={connections} onSaved={(next) => setMeetings((values) => values.map((value) => value.id === next.id ? next : value))} onError={setError} />)}</div>
        </section>
      </div>}
    </main>
  </ThemeProvider>
}

function ConnectionEditor({ connection, onSaved, onError }: { connection: CalendarConnection; onSaved: (value: CalendarConnection) => void; onError: (message: string) => void }) {
  const [calendars, setCalendars] = useState<CalendarInfo[]>([])
  const [selected, setSelected] = useState(connection.busyCalendarIds)
  const [saved, setSaved] = useState(false)
  useEffect(() => { api.calendars(connection.id).then(setCalendars).catch((reason: Error) => onError(reason.message)) }, [connection.id, onError])
  const toggle = (id: string) => setSelected((values) => values.includes(id) ? values.filter((value) => value !== id) : [...values, id])
  const save = async () => {
    try {
      onSaved(await api.saveConnection(connection.id, selected))
      setSaved(true)
    } catch (reason) {
      onError(reason instanceof Error ? reason.message : 'Could not save calendars')
    }
  }
  return <article className="connection-editor">
    <div><strong>{connection.email}</strong><span>{selected.length} calendar{selected.length === 1 ? '' : 's'} checked for conflicts</span></div>
    <div className="calendar-checks">{calendars.map((calendar) => <label key={calendar.id}><input type="checkbox" checked={selected.includes(calendar.id)} onChange={() => toggle(calendar.id)} /><span><Check size={13} />{calendar.name}{calendar.primary ? ' · primary' : ''}</span></label>)}</div>
    <button className="small-button" disabled={selected.length === 0} onClick={save}>{saved ? <Check size={16} /> : <Save size={16} />}{saved ? 'Saved' : 'Save calendars'}</button>
  </article>
}

function MeetingEditor({ meeting, connections, onSaved, onError }: { meeting: MeetingType; connections: CalendarConnection[]; onSaved: (value: MeetingType) => void; onError: (message: string) => void }) {
  const [draft, setDraft] = useState(meeting)
  const [open, setOpen] = useState(false)
  const save = async () => {
    try {
      const saved = await api.saveMeetingType(draft)
      onSaved(saved)
      setOpen(false)
    } catch (reason) {
      onError(reason instanceof Error ? reason.message : 'Could not save meeting type')
    }
  }
  return <article className="meeting-editor">
    <button className="meeting-editor-summary" onClick={() => setOpen((value) => !value)} aria-expanded={open}>
      <span><strong>{draft.name}</strong><small>{draft.description}</small></span><b>{draft.durationMinutes} min</b>
    </button>
    {open && <div className="meeting-fields">
      <label>Name<input value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></label>
      <label>Duration<input type="number" min={5} max={480} value={draft.durationMinutes} onChange={(event) => setDraft({ ...draft, durationMinutes: Number(event.target.value) })} /></label>
      <label>Buffer before<input type="number" min={0} value={draft.bufferBeforeMinutes} onChange={(event) => setDraft({ ...draft, bufferBeforeMinutes: Number(event.target.value) })} /></label>
      <label>Buffer after<input type="number" min={0} value={draft.bufferAfterMinutes} onChange={(event) => setDraft({ ...draft, bufferAfterMinutes: Number(event.target.value) })} /></label>
      <label>Minimum notice<input type="number" min={0} value={draft.minimumNoticeMinutes} onChange={(event) => setDraft({ ...draft, minimumNoticeMinutes: Number(event.target.value) })} /></label>
      <label>Booking window<input type="number" min={1} max={365} value={draft.bookingWindowDays} onChange={(event) => setDraft({ ...draft, bookingWindowDays: Number(event.target.value) })} /></label>
      <label className="wide-field">Destination account<select value={draft.destinationConnectionId ?? ''} onChange={(event) => setDraft({ ...draft, destinationConnectionId: event.target.value })}><option value="">Development mock</option>{connections.map((connection) => <option key={connection.id} value={connection.id}>{connection.email}</option>)}</select></label>
      <button className="small-button wide-field" onClick={save}><Save size={16} /> Save meeting type</button>
    </div>}
  </article>
}

