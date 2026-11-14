import { createRoot } from 'react-dom/client'
import '@vscode/codicons/dist/codicon.css'
import '@/core/theme/theme.css'
import '@/i18n'
import '@/core/locales'
// Starts itself: follows the Settings theme and, for "system", the OS.
import '@/core/theme/themeController'
import { startApp } from '@/core/boot/startApp'
import { registry } from '@/core/plugin/registry'
import { adminPlugins, allPlugins } from '@/plugins'

allPlugins.forEach((plugin) => registry.register(plugin))
void startApp(createRoot(document.getElementById('root')!), { adminPlugins })
