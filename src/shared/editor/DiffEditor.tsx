import clsx from 'clsx'
import { LazyEditor } from './LazyEditor'
import { importWithMonacoMessages } from './monacoLocale'
import type { DiffEditorProps } from './types'
import './editor.css'

const loadDiffEditor = () => importWithMonacoMessages(() => import('./MonacoDiffEditor'))

/**
 * Monaco diff view (design §6.6), e.g. live object vs edited YAML before an
 * apply, or the values of two Helm revisions. Whitespace changes count.
 *
 *   <DiffEditor original={live} modified={edited} originalTitle={t('live')} modifiedTitle={t('edited')} />
 */
export function DiffEditor({ height = '100%', className, ...props }: DiffEditorProps) {
  return (
    <div className={clsx('kb-editor', className)} style={{ height }}>
      <LazyEditor load={loadDiffEditor} props={props} />
    </div>
  )
}
