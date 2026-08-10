import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '@vscode/codicons/dist/codicon.css'
import '@/core/theme/theme.css'
import { registry } from '@/core/plugin/registry'
import { allPlugins } from '@/plugins'
import { DataContext } from '@/data/DataContext'
import { createMockProviders } from '@/data/mock'
import { Shell } from '@/core/shell/Shell'

allPlugins.forEach((plugin) => registry.register(plugin))

// Swap createMockProviders() for the real API client bundle when the
// backend lands — the UI is written against the provider interfaces only.
const providers = createMockProviders()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <DataContext.Provider value={providers}>
      <Shell />
    </DataContext.Provider>
  </StrictMode>,
)
