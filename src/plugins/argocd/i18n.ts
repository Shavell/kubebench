import { i18n } from '@/i18n'

/** i18n namespace of this plugin: its plugin id, also the backend plugin id. */
export const NS = 'argocd'

/** `t` bound to this namespace for code outside React render (commands, tab titles). */
export const translate = i18n.getFixedT(null, NS)
