import { definePluginData } from '@/data/pluginData'
import { NS } from '../i18n'
import { liveArgoCd } from './live'
import { mockArgoCd } from './mock'

/** The Argo CD data capability for the boot data mode (live never falls back to mock). */
export const argoCd = definePluginData(NS, { mock: mockArgoCd, live: liveArgoCd })
