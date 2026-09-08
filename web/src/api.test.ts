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

  it('rejects a malformed list response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{}', { status: 200 })))

    await expect(api.availability('quick-cast')).rejects.toThrow('Expected a list response')
  })
})
