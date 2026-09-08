export type Operation = 'api' | 'render' | 'clipboard'

declare global {
  interface Window {
    bookingsTelemetry?: { captureError(error: unknown, operation: Operation): void }
  }
}

export function captureError(error: unknown, operation: Operation) {
  window.bookingsTelemetry?.captureError(error, operation)
}
