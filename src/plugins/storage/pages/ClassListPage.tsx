import { useTranslation } from 'react-i18next'
import { usePluginQuery } from '@/data/pluginData'
import { EmptyState } from '@/shared/EmptyState'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { storageData } from '../data'
import { NS } from '../i18n'
import { openClassWizard } from '../tabs'
import { ClassesTable } from './ClassesTable'
import { WriteButton } from './CreateButton'
import '@/shared/tabPage.css'
import '../storage.css'

/** StorageClasses, with the default class marked. */
export function ClassListPage() {
  const { t } = useTranslation(NS)
  const query = usePluginQuery(() => storageData().overview(), [])

  const body = () => {
    if (query.loading && !query.data) return <div className="page-loading">{t('classes.loading')}</div>
    if (!query.data) return <Unavailable code={query.errorCode} reason={query.error} title={t('classes.unavailable')} />
    const { classes, defaultClass } = query.data
    if (classes.length === 0) return <EmptyState icon="layers" title={t('classes.empty')} hint={t('classes.emptyHint')} />
    return (
      <div className="page__scroll">
        <ClassesTable classes={classes} defaultClass={defaultClass} />
      </div>
    )
  }

  return (
    <div className="page">
      <Toolbar title={t('classes.title')}>
        <WriteButton label={t('actions.newClass')} icon="add" variant="primary" onClick={openClassWizard} />
      </Toolbar>
      {body()}
    </div>
  )
}
