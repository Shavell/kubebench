import { useTranslation } from 'react-i18next'
import { NS } from '../i18n'
import type { ParameterHint, ProvisionerHint } from '../model/provisioners'

/**
 * What a known provisioner or CSI driver expects. With `onUseParameter`
 * each suggested parameter can be added to the form with its example value.
 */
export function ProvisionerHintBox({
  hint,
  onUseParameter,
}: {
  hint: ProvisionerHint
  onUseParameter?: (param: ParameterHint) => void
}) {
  const { t } = useTranslation(NS)
  return (
    <div className="storage-hint" role="note">
      <div>
        <strong>{hint.title}</strong> — {t(hint.note)}
      </div>
      {onUseParameter && hint.parameters.length > 0 && (
        <ul className="storage-hint__params">
          {hint.parameters.map((p) => (
            <li key={p.key}>
              <button type="button" onClick={() => onUseParameter(p)} title={t('wizard.class.useParameter')}>
                <code>
                  {p.key}: {p.example}
                </code>
              </button>{' '}
              — {t(p.note)}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
