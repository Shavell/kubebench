import { useTranslation } from 'react-i18next'
import { useProviderQuery } from '@/data/DataContext'
import { Button } from '@/shared/Button'
import { Field } from '@/shared/form'
import { AUTH_NS } from './namespace'
import { openAuthenticationTab } from './tabs'

/** Settings → Authentication: the sign-in methods at a glance and the way to the page. */
export function AuthSettingsSection() {
  const { t } = useTranslation(AUTH_NS)
  const settings = useProviderQuery((p) => p.identity.settings(), [])
  const mappings = useProviderQuery((p) => p.identity.listMappings(), [])

  const methods = [t('methods.local'), ...(settings.data?.providers ?? []).map((p) => p.name)]
  return (
    <Field
      layout="inline"
      label={t('settings.methods')}
      help={
        settings.error ??
        t('settings.summary', { methods: methods.join(', '), count: mappings.data?.length ?? 0 })
      }
    >
      <Button variant="secondary" icon="shield" onClick={openAuthenticationTab}>
        {t('settings.open')}
      </Button>
    </Field>
  )
}
