import { Component, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { Icon } from '@/shared/Icon'
import { Unavailable } from '@/shared/Unavailable'
import '@/shared/tabPage.css'

/** Placeholder while a lazily loaded view's chunk downloads. */
export function ViewLoading() {
  const { t } = useTranslation(PLATFORM_NS)
  return (
    <div className="page-loading" role="status">
      <Icon name="loading" size={16} className="codicon-modifier-spin" />
      &nbsp;{t('lazyView.loading')}
    </div>
  )
}

interface BoundaryProps {
  onRetry: () => void
  children: ReactNode
}

/** Shows why a view could not load or render, with a Retry button, instead of breaking the workbench. */
export class LoadBoundary extends Component<BoundaryProps, { error: unknown; failed: boolean }> {
  state = { error: null as unknown, failed: false }

  static getDerivedStateFromError(error: unknown) {
    return { error, failed: true }
  }

  render() {
    if (!this.state.failed) return this.props.children
    return <LoadFailed error={this.state.error} onRetry={this.props.onRetry} />
  }
}

function LoadFailed({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const { t } = useTranslation(PLATFORM_NS)
  return (
    <Unavailable
      icon="warning"
      title={t('lazyView.loadFailed.title')}
      hint={t('lazyView.loadFailed.hint')}
      error={error}
      onRetry={onRetry}
    />
  )
}
