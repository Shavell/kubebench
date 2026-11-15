import { Icon } from './Icon'
import './SidebarErrors.css'

/**
 * Why a sidebar's live data could not be loaded, shown below its tree:
 * without it a failed query would look like an empty list (design §0).
 * Pass the `error` of each query; each distinct message is shown once.
 */
export function SidebarErrors({ errors }: { errors: (string | null | undefined)[] }) {
  const messages = [...new Set(errors.filter((e): e is string => Boolean(e)))]
  if (messages.length === 0) return null
  return (
    <div className="kb-sidebar-errors" role="status">
      {messages.map((message) => (
        <p key={message} className="kb-sidebar-errors__item">
          <Icon name="warning" size={13} className="kb-sidebar-errors__icon" />
          <span>{message}</span>
        </p>
      ))}
    </div>
  )
}
