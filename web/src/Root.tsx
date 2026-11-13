import App from './App'
import Admin from './Admin'
import Connect from './Connect'

const Root = window.location.pathname.startsWith('/admin') ? Admin : window.location.pathname.startsWith('/connect') ? Connect : App

export default Root
