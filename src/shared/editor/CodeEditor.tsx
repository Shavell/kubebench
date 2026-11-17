import clsx from 'clsx'
import { LazyEditor } from './LazyEditor'
import { importWithMonacoMessages } from './monacoLocale'
import type { CodeEditorProps } from './types'
import './editor.css'

const loadCodeEditor = () => importWithMonacoMessages(() => import('./MonacoCodeEditor'))

/**
 * Monaco code editor for YAML, JSON, shell, Markdown and plain text (design §6.6).
 * Monaco is bundled locally and loaded on first use, so it stays out of the
 * main chunk; the theme follows the app (vs-dark / vs).
 *
 *   <CodeEditor value={yaml} onChange={setYaml} ariaLabel={name} />
 */
export function CodeEditor({ height = '100%', className, ...props }: CodeEditorProps) {
  return (
    <div className={clsx('kb-editor', className)} style={{ height }}>
      <LazyEditor load={loadCodeEditor} props={props} />
    </div>
  )
}
