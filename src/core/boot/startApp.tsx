import { StrictMode, type ReactNode } from 'react'
import type { Root } from 'react-dom/client'
import { loadExternalPlugins } from '@/core/plugin/externalPlugins'
import { installGlobalSdk } from '@/core/plugin/globalSdk'
import { usePluginStatusStore } from '@/core/plugin/pluginStatusStore'
import { registry } from '@/core/plugin/registry'
import type { PluginManifest } from '@/core/plugin/types'
import { BootError } from '@/core/shell/BootError'
import { BootSplash } from '@/core/shell/BootSplash'
import { ChangePasswordScreen } from '@/core/shell/ChangePasswordScreen'
import { LoginScreen } from '@/core/shell/LoginScreen'
import { Shell } from '@/core/shell/Shell'
import { createWorkbenchPlugin } from '@/core/shell/workbenchPlugin'
import { DataContext } from '@/data/DataContext'
import { dataMode } from '@/data/mode'
import type { DataProviders } from '@/data/providers'
import { mockPolicy } from './mockPolicy'
import { openSession } from './openSession'

export interface StartOptions {
  /** Plugins only global admins get (users, audit); registered once the session is known. */
  adminPlugins: PluginManifest[]
}

/** Contributions that depend on the session: the workbench's own commands and the admin plugins. */
function registerSessionPlugins(providers: DataProviders, options: StartOptions): void {
  const plugins = [createWorkbenchPlugin(providers)]
  if (dataMode.permissions?.isAdmin) plugins.push(...options.adminPlugins)
  for (const plugin of plugins) {
    if (!registry.get(plugin.id)) registry.register(plugin)
  }
}

/**
 * Boot sequence (design §6.3, §6.8): ask the server about the session,
 * then show the sign-in screen, the forced password change, the boot
 * error screen — or load the plugin catalog and enabled external plugins
 * and render the workbench. `attempt` counts retries from the error screen.
 */
export async function startApp(root: Root, options: StartOptions, attempt = 0): Promise<void> {
  const render = (node: ReactNode) => root.render(<StrictMode>{node}</StrictMode>)
  const fail = (error: unknown) =>
    render(<BootError key={attempt} error={error} onRetry={() => startApp(root, options, attempt + 1)} />)

  if (attempt === 0) render(<BootSplash />)
  try {
    const session = await openSession(mockPolicy())
    switch (session.kind) {
      case 'signIn':
        render(<LoginScreen />)
        return
      case 'failed':
        fail(session.error)
        return
      case 'changePassword':
        render(
          <DataContext.Provider value={session.providers}>
            <ChangePasswordScreen />
          </DataContext.Provider>,
        )
        return
    }

    const { providers } = session
    registerSessionPlugins(providers, options)
    installGlobalSdk(providers)
    const pluginStatus = usePluginStatusStore.getState()
    await pluginStatus.load(providers.plugins, { assumeAvailable: dataMode.mode === 'mock' })
    await loadExternalPlugins(Object.values(usePluginStatusStore.getState().serverPlugins))
    render(
      <DataContext.Provider value={providers}>
        <Shell />
      </DataContext.Provider>,
    )
  } catch (error) {
    // A bug in boot itself must not leave the splash up forever.
    fail(error)
  }
}
