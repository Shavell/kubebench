import { Component, lazy, Suspense, useState, type ComponentType, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { Unavailable } from '../Unavailable'
import { EditorLoading } from './EditorLoading'

interface LazyEditorProps<P extends object> {
  /** Imports the Monaco-backed component (a separate chunk). */
  load: () => Promise<{ default: ComponentType<P> }>
  props: P
}

/**
 * Loads an editor chunk on first use with a placeholder meanwhile. A failed
 * load (server unreachable, stale deployment) shows the reason and a Retry
 * button instead of breaking the page.
 */
export function LazyEditor<P extends object>({ load, props }: LazyEditorProps<P>) {
  // A rejected lazy() keeps rejecting, so a retry needs a fresh one.
  const [Editor, setEditor] = useState(() => lazy(load))
  const [attempt, setAttempt] = useState(0)

  const retry = () => {
    setEditor(() => lazy(load))
    setAttempt((n) => n + 1)
  }

  return (
    <EditorBoundary key={attempt} onRetry={retry}>
      <Suspense fallback={<EditorLoading />}>
        <Editor {...props} />
      </Suspense>
    </EditorBoundary>
  )
}

interface BoundaryProps {
  onRetry: () => void
  children: ReactNode
}

class EditorBoundary extends Component<BoundaryProps, { error: unknown; failed: boolean }> {
  state = { error: null as unknown, failed: false }

  static getDerivedStateFromError(error: unknown) {
    return { error, failed: true }
  }

  render() {
    if (!this.state.failed) return this.props.children
    return <EditorLoadFailed error={this.state.error} onRetry={this.props.onRetry} />
  }
}

function EditorLoadFailed({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const { t } = useTranslation(PLATFORM_NS)
  return (
    <Unavailable
      compact
      icon="warning"
      title={t('editor.loadFailed.title')}
      hint={t('editor.loadFailed.hint')}
      error={error}
      onRetry={onRetry}
    />
  )
}
