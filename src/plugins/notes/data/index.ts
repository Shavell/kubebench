import { definePluginData } from '@/data/pluginData'
import { NS } from '../i18n'
import { createLiveNotes } from './live'
import { createMockNotes } from './mock'

/** The notes plugin's data capability, mock or live by the boot data mode. */
export const notesData = definePluginData(NS, { mock: createMockNotes, live: createLiveNotes })
