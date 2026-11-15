import { i18n } from '@/i18n'

/** i18n namespace of this plugin: its plugin id, under which the registry registers `locales`. */
export const NS = 'security'

/** `t` bound to this namespace for code outside React render (commands, tab titles, toasts). */
export const translate = i18n.getFixedT(null, NS)
