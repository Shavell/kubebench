import { definePluginData } from '@/data/pluginData'
import { NS } from '../i18n'
import { createLiveHelm } from './live'
import { createMockHelm } from './mock'

export type { HelmData } from './provider'
export type * from './types'

/** The Helm plugin's data (design §6.5): demo data in mock mode, `/api/plugins/helm` live. */
export const helmData = definePluginData(NS, { mock: createMockHelm, live: createLiveHelm })
