import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '@vscode/codicons/dist/codicon.css'
import '@/core/theme/theme.css'
import { registry } from '@/core/plugin/registry'
import { allPlugins, usersPlugin } from '@/plugins'
import { DataContext } from '@/data/DataContext'
import type { DataProviders } from '@/data/providers'
import { createMockProviders } from '@/data/mock'
import { createApiProviders } from '@/data/api'
import { setDataMode } from '@/data/mode'
import { useContextStore } from '@/core/store/contextStore'
import { Shell } from '@/core/shell/Shell'
import { LoginScreen } from '@/core/shell/LoginScreen'

interface Bootstrap {
  providers: DataProviders | null // null → show login
}

/**
 * Boot sequence:
 * 1. Probe /api/info. Reachable + authenticated → live providers.
 * 2. 401 → the backend needs a login; render LoginScreen.
 * 3. Proxy unreachable → mock mode, no auth.
 */
async function bootstrap(): Promise<Bootstrap> {
  let res: Response
  try {
    res = await fetch('/api/info', { signal: AbortSignal.timeout(2000) })
  } catch {
    setDataMode({ mode: 'mock', context: 'kind-dev-cluster (mock)' })
    return { providers: createMockProviders() }
  }

  if (res.status === 401) {
    return { providers: null }
  }
  if (!res.ok) {
    setDataMode({ mode: 'mock', context: 'kind-dev-cluster (mock)' })
    return { providers: createMockProviders() }
  }

  const info = (await res.json()) as {
    context: string
    server: string
    user?: string
    role?: 'admin' | 'user'
    capabilities?: { crds: boolean; helm: boolean; exec: boolean }
  }
  setDataMode({
    mode: 'live',
    context: info.context || '(no cluster)',
    server: info.server,
    user: info.user,
    role: info.role,
  })
  if (info.context) {
    useContextStore.setState({ contexts: [info.context], activeContext: info.context })
  }
  // The users module is admin-only; register it before render.
  if (info.role === 'admin') registry.register(usersPlugin)
  return {
    providers: createApiProviders(info.capabilities ?? { crds: false, helm: false, exec: false }),
  }
}

allPlugins.forEach((plugin) => registry.register(plugin))

const { providers } = await bootstrap()
const root = createRoot(document.getElementById('root')!)

if (!providers) {
  root.render(
    <StrictMode>
      <LoginScreen />
    </StrictMode>,
  )
} else {
  // Mock mode has no auth, so expose the users module there too for demo.
  if (!registry.get('users')) registry.register(usersPlugin)
  root.render(
    <StrictMode>
      <DataContext.Provider value={providers}>
        <Shell />
      </DataContext.Provider>
    </StrictMode>,
  )
}
