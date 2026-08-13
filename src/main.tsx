import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '@vscode/codicons/dist/codicon.css'
import '@/core/theme/theme.css'
import { registry } from '@/core/plugin/registry'
import { allPlugins } from '@/plugins'
import { DataContext } from '@/data/DataContext'
import type { DataProviders } from '@/data/providers'
import { createMockProviders } from '@/data/mock'
import { createApiProviders } from '@/data/api'
import { setDataMode } from '@/data/mode'
import { useContextStore } from '@/core/store/contextStore'
import { Shell } from '@/core/shell/Shell'

allPlugins.forEach((plugin) => registry.register(plugin))

/** Use the live proxy when it responds, otherwise fall back to mocks. */
async function createProviders(): Promise<DataProviders> {
  try {
    const res = await fetch('/api/info', { signal: AbortSignal.timeout(1500) })
    if (!res.ok) throw new Error(String(res.status))
    const info = (await res.json()) as {
      context: string
      server: string
      capabilities?: { crds: boolean; helm: boolean; exec: boolean }
    }
    setDataMode({ mode: 'live', context: info.context, server: info.server })
    useContextStore.setState({ contexts: [info.context], activeContext: info.context })
    return createApiProviders(info.capabilities ?? { crds: false, helm: false, exec: false })
  } catch {
    setDataMode({ mode: 'mock', context: 'kind-dev-cluster (mock)' })
    return createMockProviders()
  }
}

const providers = await createProviders()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <DataContext.Provider value={providers}>
      <Shell />
    </DataContext.Provider>
  </StrictMode>,
)
