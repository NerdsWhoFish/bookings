import { afterEach, describe, expect, it, vi } from 'vitest'
import { api } from './api'

describe('list API responses', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    delete window.bookingsTelemetry
  })

  it('reports handled network and invalid response errors before rethrowing', async () => {
    const captureError = vi.fn()
    window.bookingsTelemetry = { captureError }
    const failure = new TypeError('private response detail')
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(failure))
    await expect(api.meetingTypes()).rejects.toBe(failure)
    expect(captureError).toHaveBeenCalledExactlyOnceWith(failure, 'api')

    captureError.mockClear()
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('invalid JSON', { status: 200 })))
    await expect(api.meetingTypes()).rejects.toThrow()
    expect(captureError).toHaveBeenCalledTimes(1)
  })

  it('normalizes a null availability response to an empty list', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('null', { status: 200 })))

    await expect(api.availability('quick-cast')).resolves.toEqual([])
  })

  it('treats only a session-probe 401 as signed out', async () => {
    const captureError = vi.fn()
    window.bookingsTelemetry = { captureError }
    vi.stubGlobal('fetch', vi.fn().mockImplementation(() => Promise.resolve(
      Response.json({ title: 'Sign in required' }, { status: 401 }),
    )))

    await expect(api.adminSession()).resolves.toBeNull()
    expect(captureError).not.toHaveBeenCalled()
    await expect(api.connections()).rejects.toThrow('Sign in required')
    expect(captureError).toHaveBeenCalledTimes(1)
  })

  it.each([403, 500, 503])('reports session HTTP %s failures', async (status) => {
    const captureError = vi.fn()
    window.bookingsTelemetry = { captureError }
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(Response.json({ title: 'Lookup failed' }, { status })))

    await expect(api.adminSession()).rejects.toThrow('Lookup failed')
    expect(captureError).toHaveBeenCalledTimes(1)
  })

  it('reports session transport and malformed JSON failures', async () => {
    const captureError = vi.fn()
    window.bookingsTelemetry = { captureError }
    const failure = new TypeError('Network unavailable')
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(failure))
    await expect(api.adminSession()).rejects.toBe(failure)
    expect(captureError).toHaveBeenCalledExactlyOnceWith(failure, 'api')

    captureError.mockClear()
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('invalid JSON', { status: 200 })))
    await expect(api.adminSession()).rejects.toThrow()
    expect(captureError).toHaveBeenCalledTimes(1)
  })

  it.each([null, {}, [], { email: 'admin@example.com' }])('rejects an invalid successful session response: %j', async (body) => {
    const captureError = vi.fn()
    window.bookingsTelemetry = { captureError }
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(Response.json(body)))

    await expect(api.adminSession()).rejects.toThrow('Invalid session response')
    expect(captureError).toHaveBeenCalledTimes(1)
  })

  it('rejects a malformed list response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{}', { status: 200 })))

    await expect(api.availability('quick-cast')).rejects.toThrow('Expected a list response')
  })
})
