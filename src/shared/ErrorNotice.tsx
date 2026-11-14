import { describeError } from './apiErrorText'
import { Icon } from './Icon'
import './ErrorNotice.css'

/**
 * Inline error box for forms and dialogs: a friendly headline for known
 * ApiError codes (forbidden_role, conflict, …) with the server's message
 * underneath. Renders nothing while `error` is null or undefined.
 */
export function ErrorNotice({ error }: { error: unknown }) {
  if (error === null || error === undefined) return null
  const { message, detail } = describeError(error)
  return (
    <div className="kb-error-notice" role="alert">
      <Icon name="error" size={14} />
      <div>
        <div>{message}</div>
        {detail && <div className="kb-error-notice__detail">{detail}</div>}
      </div>
    </div>
  )
}
