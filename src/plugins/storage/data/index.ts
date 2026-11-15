import { definePluginData } from '@/data/pluginData'
import { NS } from '../i18n'
import { createLiveStorage } from './live'
import { createMockStorage } from './mock'

/** The storage plugin's data capability, mock or live by the boot data mode. */
export const storageData = definePluginData(NS, { mock: createMockStorage, live: createLiveStorage })
