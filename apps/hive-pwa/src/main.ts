import App from './App.svelte'
import './styles.css'

if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js').catch(() => {
      // Phase-2 scaffold: keep registration errors non-fatal for now.
    })
  })
}

const target = document.getElementById('root')
if (!target) {
  throw new Error('hive-pwa root element not found')
}

const app = new App({
  target,
})

export default app
