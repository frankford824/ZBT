import { AppProviders } from './app/providers'
import { AppRouter } from './routes/router'

export default function App() {
  return (
    <AppProviders>
      <AppRouter />
    </AppProviders>
  )
}
