import { useEffect, useState } from 'react'
import { ArrowLeft, CalendarCheck, Check, Copy, ExternalLink, Fish, Link, Plus, Save, Trash2 } from 'lucide-react'
import { api } from './api'
import { normalizeBusyCalendarIDs } from './calendarSelection'
import { ThemeProvider } from './ThemeProvider'
import { themeByID } from './themes'
import { formatMinutes } from './timeUnits'
import type { CalendarConnection, CalendarInfo, CalendarInvitation, MeetingType, Session } from './types'

export default function Admin() {
  const [session, setSession] = useState<Session | null>(null)
  const [connections, setConnections] = useState<CalendarConnection[]>([])
  const [meetings, setMeetings] = useState<MeetingType[]>([])
  const [invitations, setInvitations] = useState<CalendarInvitation[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    Promise.all([api.adminSession(), api.connections(), api.adminMeetingTypes(), api.calendarInvitations()])
      .then(([nextSession, nextConnections, nextMeetings, nextInvitations]) => {
        setSession(nextSession)
        setConnections(nextConnections)
        setMeetings(nextMeetings)
        setInvitations(nextInvitations)
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
        <div className="admin-stack">
          <section className="admin-section">
            <div className="admin-section-heading"><div><p className="kicker">Google Calendar</p><h2>Busy calendars</h2></div><a className="icon-link" href="/api/admin/google/start"><Plus size={17} /> Add my account</a></div>
            {connections.length === 0 ? <div className="admin-empty"><CalendarCheck /><strong>No Google accounts yet</strong><span>Add one before publishing meeting types.</span></div> : connections.map((connection) => <ConnectionEditor key={connection.id} connection={connection} onSaved={(next) => setConnections((values) => values.map((value) => value.id === next.id ? next : value))} onError={setError} />)}
          </section>
          <CalendarInvitationEditor invitations={invitations} onChange={setInvitations} onError={setError} />
        </div>
        <section className="admin-section">
          <div className="admin-section-heading"><div><p className="kicker">Meeting catalog</p><h2>Types and lengths</h2></div><button className="icon-button" onClick={() => setMeetings((values) => [...values, newMeetingType()])}><Plus size={17} /> New type</button></div>
          <div className="admin-meetings">{meetings.length === 0 && <div className="admin-empty"><strong>No meeting types yet</strong><span>Create one to open the booking page.</span></div>}{meetings.map((meeting) => <MeetingEditor key={meeting.id} meeting={meeting} connections={connections} onSaved={(next) => setMeetings((values) => values.some((value) => value.id === next.id) ? values.map((value) => value.id === next.id ? next : value) : [...values, next])} onDeleted={(id) => setMeetings((values) => values.filter((value) => value.id !== id))} onError={setError} />)}</div>
        </section>
      </div>}
    </main>
  </ThemeProvider>
}

function CalendarInvitationEditor({ invitations, onChange, onError }: { invitations: CalendarInvitation[]; onChange: (value: CalendarInvitation[]) => void; onError: (message: string) => void }) {
  const [email, setEmail] = useState('')
  const [url, setURL] = useState('')
  const [copied, setCopied] = useState(false)
  const create = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    try {
      const result = await api.createCalendarInvitation(email)
      onChange([result.invitation, ...invitations])
      setURL(result.url)
      setEmail('')
      setCopied(false)
    } catch (reason) {
      onError(reason instanceof Error ? reason.message : 'Could not create calendar invitation')
    }
  }
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(url)
      setCopied(true)
    } catch {
      onError('Could not copy the link. Select it and copy it manually.')
    }
  }
  const revoke = async (id: string) => {
    try {
      await api.revokeCalendarInvitation(id)
      onChange(invitations.filter((invitation) => invitation.id !== id))
    } catch (reason) {
      onError(reason instanceof Error ? reason.message : 'Could not revoke calendar invitation')
    }
  }
  return <section className="admin-section">
    <div className="admin-section-heading"><div><p className="kicker">Invite a calendar</p><h2>Connection links</h2></div></div>
    <p className="admin-section-copy">Create a one-time link for someone else. Their Google account must match the email below, and the link expires after seven days.</p>
    <form className="invitation-form" onSubmit={create}>
      <label>Google account email<input type="email" autoComplete="email" value={email} onChange={(event) => setEmail(event.target.value)} required /></label>
      <button className="small-button" type="submit"><Link size={16} /> Create invite link</button>
    </form>
    {url && <div className="invitation-link"><label>Send this private link<input readOnly value={url} onFocus={(event) => event.currentTarget.select()} /></label><button className="small-button" type="button" onClick={copy}>{copied ? <Check size={16} /> : <Copy size={16} />}{copied ? 'Copied' : 'Copy link'}</button></div>}
    {invitations.length > 0 && <div className="invitation-list">{invitations.map((invitation) => {
      const status = invitation.usedAt ? 'Connected' : new Date(invitation.expiresAt) <= new Date() ? 'Expired' : 'Waiting'
      return <div key={invitation.id}><span><strong>{invitation.email}</strong><small>{status} · expires {new Date(invitation.expiresAt).toLocaleDateString()}</small></span>{!invitation.usedAt && <button className="danger-link" type="button" onClick={() => revoke(invitation.id)}>Revoke</button>}</div>
    })}</div>}
  </section>
}

function ConnectionEditor({ connection, onSaved, onError }: { connection: CalendarConnection; onSaved: (value: CalendarConnection) => void; onError: (message: string) => void }) {
  const [calendars, setCalendars] = useState<CalendarInfo[]>([])
  const [selected, setSelected] = useState(connection.busyCalendarIds)
  const [saved, setSaved] = useState(false)
  useEffect(() => {
    api.calendars(connection.id).then((next) => {
      setCalendars(next)
      setSelected((values) => normalizeBusyCalendarIDs(values, next))
    }).catch((reason: Error) => onError(reason.message))
  }, [connection.id, onError])
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

function MeetingEditor({ meeting, connections, onSaved, onDeleted, onError }: { meeting: MeetingType; connections: CalendarConnection[]; onSaved: (value: MeetingType) => void; onDeleted: (id: string) => void; onError: (message: string) => void }) {
  const [draft, setDraft] = useState({ ...meeting, attendeeEmails: meeting.attendeeEmails ?? [] })
  const [open, setOpen] = useState(meeting.name === 'New meeting type')
  const [calendars, setCalendars] = useState<CalendarInfo[]>([])
  const [persisted, setPersisted] = useState(meeting.name !== 'New meeting type')
  const [confirmDelete, setConfirmDelete] = useState(false)
  useEffect(() => {
    if (!draft.destinationConnectionId) {
      setCalendars([])
      return
    }
    api.calendars(draft.destinationConnectionId).then(setCalendars).catch((reason: Error) => onError(reason.message))
  }, [draft.destinationConnectionId, onError])
  const save = async () => {
    try {
      const saved = await api.saveMeetingType(draft)
      onSaved(saved)
      setDraft(saved)
      setPersisted(true)
      setOpen(false)
    } catch (reason) {
      onError(reason instanceof Error ? reason.message : 'Could not save meeting type')
    }
  }
  const remove = async () => {
    try {
      if (persisted) await api.deleteMeetingType(draft.id)
      onDeleted(draft.id)
    } catch (reason) {
      onError(reason instanceof Error ? reason.message : 'Could not delete meeting type')
    }
  }
  const toggleAttendee = (email: string) => setDraft((value) => ({
    ...value,
    attendeeEmails: value.attendeeEmails.includes(email) ? value.attendeeEmails.filter((item) => item !== email) : [...value.attendeeEmails, email],
  }))
  return <article className="meeting-editor">
    <button className="meeting-editor-summary" onClick={() => setOpen((value) => !value)} aria-expanded={open}>
      <span><span className="meeting-name-line"><strong>{draft.name}</strong>{draft.hidden && <em>Hidden</em>}</span><small>{draft.description}</small></span><b>{draft.durationMinutes} min</b>
    </button>
    {open && <div className="meeting-fields">
      <label>Name<input value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} /></label>
      <label>Slug<input value={draft.slug} onChange={(event) => setDraft({ ...draft, slug: event.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '-') })} /></label>
      <label className="wide-field">Description<textarea rows={2} value={draft.description} onChange={(event) => setDraft({ ...draft, description: event.target.value })} /></label>
      <label>Duration (minutes)<input type="number" min={5} max={480} value={draft.durationMinutes} onChange={(event) => setDraft({ ...draft, durationMinutes: Number(event.target.value) })} /></label>
      <label>Slot interval (minutes)<input type="number" min={5} max={120} value={draft.slotIntervalMinutes} onChange={(event) => setDraft({ ...draft, slotIntervalMinutes: Number(event.target.value) })} /></label>
      <label>Buffer before (minutes)<input type="number" min={0} value={draft.bufferBeforeMinutes} onChange={(event) => setDraft({ ...draft, bufferBeforeMinutes: Number(event.target.value) })} /></label>
      <label>Buffer after (minutes)<input type="number" min={0} value={draft.bufferAfterMinutes} onChange={(event) => setDraft({ ...draft, bufferAfterMinutes: Number(event.target.value) })} /></label>
      <label>Minimum notice (minutes)<input type="number" min={0} value={draft.minimumNoticeMinutes} onChange={(event) => setDraft({ ...draft, minimumNoticeMinutes: Number(event.target.value) })} /><span className="field-help">{formatMinutes(draft.minimumNoticeMinutes)}</span></label>
      <label>Booking window (days)<input type="number" min={1} max={365} value={draft.bookingWindowDays} onChange={(event) => setDraft({ ...draft, bookingWindowDays: Number(event.target.value) })} /></label>
      <label>Time zone<input value={draft.timeZone} onChange={(event) => setDraft({ ...draft, timeZone: event.target.value })} /></label>
      <label>Location<input value={draft.location} onChange={(event) => setDraft({ ...draft, location: event.target.value })} /></label>
      <label>Destination account<select value={draft.destinationConnectionId ?? ''} onChange={(event) => setDraft({ ...draft, destinationConnectionId: event.target.value, destinationCalendarId: '' })}><option value="">Development mock</option>{connections.map((connection) => <option key={connection.id} value={connection.id}>{connection.email}</option>)}</select></label>
      <label>Destination calendar<select value={draft.destinationCalendarId ?? ''} disabled={!draft.destinationConnectionId} onChange={(event) => setDraft({ ...draft, destinationCalendarId: event.target.value })}><option value="">Choose a calendar</option>{calendars.map((calendar) => <option key={calendar.id} value={calendar.id}>{calendar.name}{calendar.primary ? ' · primary' : ''}</option>)}</select></label>
      <fieldset className="attendee-editor wide-field"><legend>Connected account attendees</legend><p>Selected people are automatically added to every calendar event for this meeting type.</p>{connections.length === 0 ? <span>No connected accounts yet.</span> : <div className="calendar-checks">{connections.map((connection) => <label key={connection.id}><input type="checkbox" checked={draft.attendeeEmails.includes(connection.email)} onChange={() => toggleAttendee(connection.email)} /><span><Check size={13} />{connection.email}</span></label>)}</div>}</fieldset>
      <ScheduleEditor availability={draft.availability} onChange={(availability) => setDraft({ ...draft, availability })} />
      <label className="active-toggle"><input type="checkbox" checked={draft.active} onChange={(event) => setDraft({ ...draft, active: event.target.checked })} />Available to book</label>
      <label className="active-toggle"><input type="checkbox" checked={draft.hidden} onChange={(event) => setDraft({ ...draft, hidden: event.target.checked })} />Hidden from the booking page (direct link still works)</label>
      <div className="meeting-actions wide-field">
        <a className="icon-link" href={`/meet/${encodeURIComponent(draft.slug)}`} target="_blank" rel="noreferrer"><ExternalLink size={16} /> Direct booking link</a>
        {confirmDelete ? <span className="delete-confirm"><span>Delete this meeting type?</span><button className="danger-button" type="button" onClick={remove}>Yes, delete</button><button className="icon-button" type="button" onClick={() => setConfirmDelete(false)}>Keep it</button></span> : <button className="danger-link" type="button" onClick={() => setConfirmDelete(true)}><Trash2 size={15} /> Delete</button>}
        <button className="small-button" type="button" onClick={save}><Save size={16} /> Save meeting type</button>
      </div>
    </div>}
  </article>
}

function ScheduleEditor({ availability, onChange }: { availability: MeetingType['availability']; onChange: (value: MeetingType['availability']) => void }) {
  const weekdays = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']
  const update = (weekday: number, field: 'enabled' | 'start' | 'end', value: string | boolean) => {
    if (field === 'enabled') {
      onChange(value ? [...availability, { weekday, start: '09:00', end: '17:00' }].sort((a, b) => a.weekday - b.weekday) : availability.filter((hours) => hours.weekday !== weekday))
      return
    }
    onChange(availability.map((hours) => hours.weekday === weekday ? { ...hours, [field]: value } : hours))
  }
  return <fieldset className="schedule-editor wide-field"><legend>Weekly availability</legend>{weekdays.map((name, weekday) => {
    const hours = availability.find((value) => value.weekday === weekday)
    return <div key={name}><label><input type="checkbox" checked={!!hours} onChange={(event) => update(weekday, 'enabled', event.target.checked)} />{name}</label><input aria-label={`${name} start`} type="time" disabled={!hours} value={hours?.start ?? '09:00'} onChange={(event) => update(weekday, 'start', event.target.value)} /><span>to</span><input aria-label={`${name} end`} type="time" disabled={!hours} value={hours?.end ?? '17:00'} onChange={(event) => update(weekday, 'end', event.target.value)} /></div>
  })}</fieldset>
}

function newMeetingType(): MeetingType {
  const id = crypto.randomUUID()
  return {
    id,
    slug: `meeting-${id.slice(0, 8)}`,
    name: 'New meeting type',
    description: '',
    durationMinutes: 30,
    bufferBeforeMinutes: 0,
    bufferAfterMinutes: 10,
    minimumNoticeMinutes: 120,
    bookingWindowDays: 30,
    slotIntervalMinutes: 15,
    timeZone: 'America/New_York',
    location: 'Google Meet',
    destinationConnectionId: '',
    destinationCalendarId: '',
    attendeeEmails: [],
    availability: [1, 2, 3, 4, 5].map((weekday) => ({ weekday, start: '09:00', end: '17:00' })),
    active: true,
    hidden: false,
  }
}
