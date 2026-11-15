import { definePluginData } from '@/data/pluginData'
import { NS } from '../i18n'
import { createLiveSecurity } from './live'
import { createMockSecurity } from './mock'

/** The security plugin's data capability, mock or live by the boot data mode. */
export const securityData = definePluginData(NS, { mock: createMockSecurity, live: createLiveSecurity })
