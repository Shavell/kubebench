import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { Icon } from '../Icon'
import './editor.css'

/** Placeholder while the editor chunk loads. */
export function EditorLoading() {
  const { t } = useTranslation(PLATFORM_NS)
  return (
    <div className="kb-editor-state" role="status">
      <Icon name="loading" size={16} className="codicon-modifier-spin" />
      <span>{t('editor.loading')}</span>
    </div>
  )
}
