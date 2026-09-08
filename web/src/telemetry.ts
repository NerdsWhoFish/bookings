import { initializeTelemetry } from '@nerdswhofish/browser-telemetry'
import type { Operation } from './telemetry-client'

declare const __APP_VERSION__: string

const meta = (name: string) => document.querySelector<HTMLMetaElement>(`meta[name="${name}"]`)?.content ?? ''
const telemetry = initializeTelemetry({
  url: meta('faro-collector-url'),
  app: { name: meta('faro-app-name') || 'bookings', version: __APP_VERSION__, environment: 'production' },
  routes: ['/', '/admin', '/connect', '/api/public/config', '/api/public/meeting-types', '/api/public/bookings', '/api/admin/session', '/api/admin/connections', '/api/admin/meeting-types', '/api/admin/calendar-invitations'],
  assets: JSON.parse(meta('faro-assets') || '[]') as string[],
  operations: ['api', 'render', 'clipboard'] satisfies Operation[],
})
window.bookingsTelemetry = telemetry
