import type { TFunction } from 'i18next'
// Relative import with extension: unit-tested with node --test.
import type { CloudCredentialProvider, CloudCredentials } from '../../data/types.ts'

/**
 * The cloud credential form's state (design §7.12). `server` signs in with
 * the KubeBench server's own cloud identity; `keep` (Edit only) leaves the
 * stored per-cluster credentials as they are.
 */
export type CredentialMode = 'keep' | 'server' | 'aws-keys' | 'aws-profile' | 'gcp' | 'azure'

export interface CredentialDraft {
  mode: CredentialMode
  accessKeyId: string
  secretAccessKey: string
  sessionToken: string
  profile: string
  serviceAccountJson: string
  tenantId: string
  clientId: string
  clientSecret: string
}

export function emptyDraft(mode: CredentialMode): CredentialDraft {
  return {
    mode,
    accessKeyId: '',
    secretAccessKey: '',
    sessionToken: '',
    profile: '',
    serviceAccountJson: '',
    tenantId: '',
    clientId: '',
    clientSecret: '',
  }
}

/** The cloud each mode signs in to; keep and server name none. */
export const MODE_PROVIDER: Record<CredentialMode, CloudCredentialProvider | null> = {
  keep: null,
  server: null,
  'aws-keys': 'aws',
  'aws-profile': 'aws',
  gcp: 'gcp',
  azure: 'azure',
}

const GUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
const PROFILE = /^[A-Za-z0-9_.+=,@-]{1,128}$/

export type CredentialField = Exclude<keyof CredentialDraft, 'mode'>

/** What is missing or malformed, per field; the server checks again. */
export function credentialProblems(t: TFunction, d: CredentialDraft): Partial<Record<CredentialField, string>> {
  const out: Partial<Record<CredentialField, string>> = {}
  const required = (field: CredentialField) => {
    if (!d[field].trim()) out[field] = t('credentials.errors.required')
  }
  switch (d.mode) {
    case 'aws-keys':
      required('accessKeyId')
      required('secretAccessKey')
      break
    case 'aws-profile':
      if (!PROFILE.test(d.profile.trim())) out.profile = t('credentials.errors.profile')
      break
    case 'gcp':
      if (!serviceAccountKey(d.serviceAccountJson)) out.serviceAccountJson = t('credentials.errors.serviceAccount')
      break
    case 'azure':
      if (!GUID.test(d.tenantId.trim())) out.tenantId = t('credentials.errors.guid')
      if (!GUID.test(d.clientId.trim())) out.clientId = t('credentials.errors.guid')
      required('clientSecret')
      break
  }
  return out
}

/** Whether text is a Google service-account key file. */
function serviceAccountKey(text: string): boolean {
  try {
    const key = JSON.parse(text) as { type?: unknown; client_email?: unknown; private_key?: unknown }
    return key.type === 'service_account' && typeof key.client_email === 'string' && typeof key.private_key === 'string'
  } catch {
    return false
  }
}

/**
 * The request value: credentials to store, null to remove the stored ones
 * (mode server), undefined to leave them (mode keep).
 */
export function toCredentials(d: CredentialDraft): CloudCredentials | null | undefined {
  switch (d.mode) {
    case 'keep':
      return undefined
    case 'server':
      return null
    case 'aws-keys':
      return {
        provider: 'aws',
        accessKeyId: d.accessKeyId.trim(),
        secretAccessKey: d.secretAccessKey.trim(),
        ...(d.sessionToken.trim() ? { sessionToken: d.sessionToken.trim() } : {}),
      }
    case 'aws-profile':
      return { provider: 'aws', profile: d.profile.trim() }
    case 'gcp':
      return { provider: 'gcp', serviceAccountJson: d.serviceAccountJson.trim() }
    case 'azure':
      return {
        provider: 'azure',
        tenantId: d.tenantId.trim(),
        clientId: d.clientId.trim(),
        clientSecret: d.clientSecret.trim(),
      }
  }
}

/** The modes for a kubeconfig that signs in to provider (all clouds when unknown). */
export function modesFor(provider: CloudCredentialProvider | undefined, base: readonly CredentialMode[]): CredentialMode[] {
  const clouds: CredentialMode[] = ['aws-keys', 'aws-profile', 'gcp', 'azure']
  return [...base, ...clouds.filter((mode) => !provider || MODE_PROVIDER[mode] === provider)]
}
