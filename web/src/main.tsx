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

const Root = window.location.pathname.startsWith('/admin') ? Admin : App
createRoot(document.getElementById('root')!).render(<StrictMode><Root /></StrictMode>)
