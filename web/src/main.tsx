import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '@fontsource/bricolage-grotesque/latin-600.css'
import '@fontsource/bricolage-grotesque/latin-700.css'
import '@fontsource/ibm-plex-sans/latin-400.css'
import '@fontsource/ibm-plex-sans/latin-500.css'
import '@fontsource/ibm-plex-sans/latin-600.css'
import './styles.css'
import Root from './Root'
import { captureError } from './telemetry-client'

createRoot(document.getElementById('root')!, {
  onCaughtError: (error) => captureError(error, 'render'),
  onUncaughtError: (error) => captureError(error, 'render'),
  onRecoverableError: (error) => captureError(error, 'render'),
}).render(<StrictMode><Root /></StrictMode>)
