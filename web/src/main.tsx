import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '@fontsource/bricolage-grotesque/latin-600.css'
import '@fontsource/bricolage-grotesque/latin-700.css'
import '@fontsource/ibm-plex-sans/latin-400.css'
import '@fontsource/ibm-plex-sans/latin-500.css'
import '@fontsource/ibm-plex-sans/latin-600.css'
import './styles.css'
import App from './App'
import Admin from './Admin'
import Connect from './Connect'
import { captureError } from './telemetry-client'

const Root = window.location.pathname.startsWith('/admin') ? Admin : window.location.pathname.startsWith('/connect') ? Connect : App
createRoot(document.getElementById('root')!, {
  onCaughtError: (error) => captureError(error, 'render'),
  onUncaughtError: (error) => captureError(error, 'render'),
  onRecoverableError: (error) => captureError(error, 'render'),
}).render(<StrictMode><Root /></StrictMode>)
