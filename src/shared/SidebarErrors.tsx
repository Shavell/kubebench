import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { Icon } from './Icon'
import { sidebarMessages, type SidebarError } from './sidebarMessages'
import './SidebarErrors.css'

/**
 * Why a sidebar's live data could not be loaded, shown below its tree:
 * without it a failed query would look like an empty list (design §0).
 * Pass each query's `error`, or the query itself so that refusals of the
 * cluster's RBAC collapse into one notice; each distinct message is shown
 * once.
 */
export function SidebarErrors({ errors }: { errors: (string | SidebarError | null | undefined)[] }) {
  const { t } = useTranslation(PLATFORM_NS)
  const { messages, forbidden } = sidebarMessages(errors)
  if (messages.length === 0 && !forbidden) return null
  return (
    <div className="kb-sidebar-errors" role="status">
      {messages.map((message) => (
        <p key={message} className="kb-sidebar-errors__item">
          <Icon name="warning" size={13} className="kb-sidebar-errors__icon" />
          <span>{message}</span>
        </p>
      ))}
      {forbidden && (
        <p className="kb-sidebar-errors__item kb-sidebar-errors__item--quiet">
          <Icon name="lock" size={13} className="kb-sidebar-errors__icon" />
          <span>{t('ui.sidebarForbidden')}</span>
        </p>
      )}
    </div>
  )
}
