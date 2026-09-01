import { afterEach, describe, expect, it, vi } from 'vitest'
import { api } from './api'

describe('list API responses', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('normalizes a null availability response to an empty list', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('null', { status: 200 })))

    await expect(api.availability('quick-cast')).resolves.toEqual([])
  })

  it('rejects a malformed list response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{}', { status: 200 })))

    await expect(api.availability('quick-cast')).rejects.toThrow('Expected a list response')
  })
})
