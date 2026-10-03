import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import 'antd/dist/reset.css'
import '@fontsource/noto-sans-sc/400.css'
import '@fontsource/noto-sans-sc/500.css'
import '@fontsource/noto-sans-sc/700.css'
import '@fontsource/jetbrains-mono/400.css'
import '@fontsource/jetbrains-mono/600.css'
import './index.css'
import App from './App.tsx'
import { AppErrorBoundary } from './shared/components/AppErrorBoundary'
import { shouldReloadAfterPreloadError } from './shared/preloadRecovery'

window.addEventListener('vite:preloadError', (event) => {
  let reload = false
  try {
    reload = shouldReloadAfterPreloadError(window.sessionStorage)
  } catch {
    // Browsers may disallow storage access; the visible fallback still works.
  }
  if (reload) {
    event.preventDefault()
    window.location.reload()
  }
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <AppErrorBoundary><App /></AppErrorBoundary>
  </StrictMode>,
)
