import { definePluginData } from '@/data/pluginData'
import { NS } from '../i18n'
import { createLiveAccess } from './live'
import { createMockAccess } from './mock'

/** The access plugin's data capability, mock or live by the boot data mode. */
export const accessData = definePluginData(NS, { mock: createMockAccess, live: createLiveAccess })
