import { act, cleanup, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import Admin from './Admin'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  delete window.bookingsTelemetry
})

describe('admin startup', () => {
  it('checks the session before requesting protected data and shows sign-in for a 401', async () => {
    let resolveSession!: (response: Response) => void
    const pendingSession = new Promise<Response>((resolve) => { resolveSession = resolve })
    const fetch = vi.fn().mockImplementation((input: string) => input === '/api/admin/session'
      ? pendingSession
      : Promise.resolve(Response.json({ title: 'Sign in required' }, { status: 401 })))
    vi.stubGlobal('fetch', fetch)
    const captureError = vi.fn()
    window.bookingsTelemetry = { captureError }

    render(<Admin />)
    expect(fetch.mock.calls.map(([input]) => input)).toEqual(['/api/admin/session'])
    expect(screen.getByRole('status')).toHaveTextContent('Loading booking controls')
    expect(screen.queryByRole('link', { name: /Continue with Google/ })).not.toBeInTheDocument()

    await act(async () => resolveSession(Response.json({ title: 'Sign in required' }, { status: 401 })))

    expect(await screen.findByRole('link', { name: /Continue with Google/ })).toHaveAttribute('href', '/api/admin/google/start')
    expect(fetch).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
    expect(screen.queryByText('No Google accounts yet')).not.toBeInTheDocument()
    expect(captureError).not.toHaveBeenCalled()
  })

  it('loads protected data after an authenticated session', async () => {
    let resolveSession!: (response: Response) => void
    const pendingSession = new Promise<Response>((resolve) => { resolveSession = resolve })
    const fetch = vi.fn().mockImplementation((input: string) => input === '/api/admin/session'
      ? pendingSession
      : Promise.resolve(Response.json([])))
    vi.stubGlobal('fetch', fetch)

    render(<Admin />)
    expect(fetch).toHaveBeenCalledTimes(1)
    await act(async () => resolveSession(Response.json({ email: 'admin@example.com', expiresAt: 2000000000 })))

    expect(await screen.findByText('Signed in as admin@example.com')).toBeInTheDocument()
    expect(fetch.mock.calls.map(([input]) => input)).toEqual([
      '/api/admin/session', '/api/admin/connections', '/api/admin/meeting-types', '/api/admin/calendar-invitations',
    ])
    expect(screen.getByText('No Google accounts yet')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /Continue with Google/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('reports a failed session lookup without treating it as signed out', async () => {
    const captureError = vi.fn()
    window.bookingsTelemetry = { captureError }
    const fetch = vi.fn().mockImplementation(() => Promise.resolve(Response.json({ title: 'Service unavailable' }, { status: 503 })))
    vi.stubGlobal('fetch', fetch)

    render(<Admin />)

    expect(await screen.findByRole('alert')).toHaveTextContent('Service unavailable')
    expect(fetch).toHaveBeenCalledTimes(1)
    expect(captureError).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('link', { name: /Continue with Google/ })).not.toBeInTheDocument()
    expect(screen.queryByText('No Google accounts yet')).not.toBeInTheDocument()
  })

  it('reports protected-data failures after sign-in without showing an empty dashboard', async () => {
    const captureError = vi.fn()
    window.bookingsTelemetry = { captureError }
    vi.stubGlobal('fetch', vi.fn().mockImplementation((input: string) => Promise.resolve(
      input === '/api/admin/session'
        ? Response.json({ email: 'admin@example.com', expiresAt: 2000000000 })
        : input === '/api/admin/connections'
          ? Response.json({ title: 'Could not load connections' }, { status: 503 })
          : Response.json([]),
    )))

    render(<Admin />)

    expect(await screen.findByRole('alert')).toHaveTextContent('Could not load connections')
    await waitFor(() => expect(captureError).toHaveBeenCalledTimes(1))
    expect(screen.queryByRole('link', { name: /Continue with Google/ })).not.toBeInTheDocument()
    expect(screen.queryByText('No Google accounts yet')).not.toBeInTheDocument()
  })
})
