import { definePluginData } from '@/data/pluginData'
import { NS } from '../i18n'
import { createLiveServiceBrowser } from './live'
import { createMockServiceBrowser } from './mock'

/** The service browser's data capability, mock or live by the boot data mode. */
export const serviceBrowserData = definePluginData(NS, {
  mock: createMockServiceBrowser,
  live: () => createLiveServiceBrowser(),
})
