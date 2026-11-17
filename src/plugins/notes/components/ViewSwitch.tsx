import clsx from 'clsx'
import { Icon } from '@/shared/Icon'
import { useNotesTranslation } from '../i18n'
import type { EditorView } from '../model/editorSessions'

const VIEWS: { view: EditorView; icon: string }[] = [
  { view: 'edit', icon: 'edit' },
  { view: 'split', icon: 'split-horizontal' },
  { view: 'preview', icon: 'open-preview' },
]

/** Edit / Split / Preview, as a segmented control of icon buttons. */
export function ViewSwitch({ view, onChange }: { view: EditorView; onChange: (view: EditorView) => void }) {
  const { t } = useNotesTranslation()
  return (
    <div className="kb-notes-views" role="group" aria-label={t('editor.views.label')}>
      {VIEWS.map((v) => (
        <button
          key={v.view}
          type="button"
          className={clsx('kb-notes-views__button', view === v.view && 'kb-notes-views__button--active')}
          aria-pressed={view === v.view}
          title={t(`editor.views.${v.view}`)}
          aria-label={t(`editor.views.${v.view}`)}
          onClick={() => onChange(v.view)}
        >
          <Icon name={v.icon} size={14} />
        </button>
      ))}
    </div>
  )
}
