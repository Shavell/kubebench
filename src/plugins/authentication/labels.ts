import type { ExternalAuthSource, MappingRole, SignInFailure } from '@/data/types'

/** i18n keys (authentication namespace) of names the page shows. */
export const sourceKey = (source: ExternalAuthSource | '') => (source ? `sources.${source}` : 'sources.any')
export const roleKey = (role: MappingRole) => `roles.${role}`
export const failureKey = (failure: SignInFailure) => `failures.${failure}`
/** Label of a provider setting; unknown keys (a newer server) show the key itself. */
export const settingKey = (key: string) => `settings.keys.${key}`

export const MAPPING_ROLES: readonly MappingRole[] = ['admin', 'operator', 'viewer']
export const SOURCES: readonly (ExternalAuthSource | '')[] = ['', 'oidc', 'ldap']
