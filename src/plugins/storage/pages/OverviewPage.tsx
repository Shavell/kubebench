import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { usePluginQuery } from '@/data/pluginData'
import { formatBytes, formatNumber } from '@/i18n/format'
import { chartColor } from '@/shared/charts/chartTheme'
import { StatTile } from '@/shared/charts/StatTile'
import { UsageBar } from '@/shared/charts/UsageBar'
import { EmptyState } from '@/shared/EmptyState'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { storageData } from '../data'
import type { CsiDriverSummary } from '../data/types'
import { NS } from '../i18n'
import { provisionerHint } from '../model/provisioners'
import { openClassWizard, openPvcWizard, openPvWizard } from '../tabs'
import { ClassesTable } from './ClassesTable'
import { WriteButton } from './CreateButton'
import '@/shared/tabPage.css'
import '../storage.css'

const GIB = 2 ** 30
const gib = (bytes: number) => Math.round((bytes / GIB) * 10) / 10

/** Storage at a glance: counts, capacity, classes (with the default) and CSI drivers. */
export function OverviewPage() {
  const { t } = useTranslation(NS)
  const overview = usePluginQuery(() => storageData().overview(), [])
  const drivers = usePluginQuery(() => storageData().listCsiDrivers(), [])

  const driverColumns = useMemo<Column<CsiDriverSummary>[]>(
    () => [
      {
        key: 'name',
        header: t('columns.name'),
        sortValue: (d) => d.name,
        render: (d) => {
          const hint = provisionerHint(d.name)
          return (
            <>
              <span className="storage-mono">{d.name}</span>
              {hint && <span className="storage-muted"> · {hint.title}</span>}
            </>
          )
        },
      },
      { key: 'attach', header: t('csi.attachRequired'), render: (d) => (d.attachRequired ? t('common.yes') : t('common.no')) },
      { key: 'podInfo', header: t('csi.podInfoOnMount'), render: (d) => (d.podInfoOnMount ? t('common.yes') : t('common.no')) },
      { key: 'capacity', header: t('csi.storageCapacity'), render: (d) => (d.storageCapacity ? t('common.yes') : t('common.no')) },
      { key: 'fsGroup', header: t('csi.fsGroupPolicy'), render: (d) => d.fsGroupPolicy },
      { key: 'modes', header: t('csi.lifecycleModes'), render: (d) => d.lifecycleModes.join(', ') },
      { key: 'age', header: t('columns.age'), render: (d) => d.age },
    ],
    [t],
  )

  const actions = (
    <>
      <WriteButton label={t('actions.newPvc')} icon="add" onClick={() => openPvcWizard()} variant="primary" />
      <WriteButton label={t('actions.newPv')} icon="add" onClick={openPvWizard} />
      <WriteButton label={t('actions.newClass')} icon="add" onClick={openClassWizard} />
    </>
  )

  if (overview.loading && !overview.data) return <div className="page-loading">{t('overview.loading')}</div>
  const data = overview.data
  if (!data) {
    return (
      <div className="page">
        <Toolbar title={t('overview.title')}>{actions}</Toolbar>
        <Unavailable code={overview.errorCode} reason={overview.error} title={t('overview.unavailable')} />
      </div>
    )
  }

  const { pvcs, pvs, capacity } = data
  return (
    <div className="page">
      <Toolbar title={t('overview.title')}>{actions}</Toolbar>
      <div className="page__scroll storage-overview">
        <div className="storage-tiles">
          <StatTile
            icon="save"
            label={t('overview.tiles.pvcs')}
            value={formatNumber(pvcs.total)}
            detail={t('overview.tiles.pvcsDetail', { bound: pvcs.bound, pending: pvcs.pending, lost: pvcs.lost })}
          />
          <StatTile
            icon="save-all"
            label={t('overview.tiles.pvs')}
            value={formatNumber(pvs.total)}
            detail={t('overview.tiles.pvsDetail', { available: pvs.available, bound: pvs.bound, released: pvs.released })}
          />
          <StatTile
            icon="layers"
            label={t('overview.tiles.classes')}
            value={formatNumber(data.classes.length)}
            detail={data.defaultClass ? t('overview.tiles.defaultClass', { name: data.defaultClass }) : t('overview.tiles.noDefault')}
          />
          <StatTile
            icon="plug"
            label={t('overview.tiles.csiDrivers')}
            value={formatNumber(data.csiDrivers)}
            detail={data.snapshotsAvailable ? t('overview.tiles.snapshotsOn') : t('overview.tiles.snapshotsOff')}
          />
        </div>

        <section>
          <h2 className="storage-section-title">{t('overview.capacity')}</h2>
          <div className="storage-capacity">
            <UsageBar
              label={t('overview.bars.requested')}
              value={gib(capacity.requestedBytes)}
              total={gib(capacity.provisionedBytes)}
              unit="GiB"
              color={chartColor('blue')}
            />
            <UsageBar
              label={t('overview.bars.available')}
              value={gib(capacity.availableBytes)}
              total={gib(capacity.provisionedBytes)}
              unit="GiB"
              color={chartColor('green')}
            />
          </div>
          <div className="storage-muted">
            {t('overview.capacityDetail', {
              requested: formatBytes(capacity.requestedBytes),
              provisioned: formatBytes(capacity.provisionedBytes),
            })}
          </div>
        </section>

        <section>
          <h2 className="storage-section-title">{t('classes.title')}</h2>
          {data.classes.length ? (
            <ClassesTable classes={data.classes} defaultClass={data.defaultClass} />
          ) : (
            <EmptyState icon="layers" title={t('classes.empty')} hint={t('classes.emptyHint')} />
          )}
        </section>

        <section>
          <h2 className="storage-section-title">{t('csi.title')}</h2>
          {drivers.loading && !drivers.data ? (
            <div className="storage-muted">{t('csi.loading')}</div>
          ) : drivers.error ? (
            <Unavailable compact code={drivers.errorCode} reason={drivers.error} title={t('csi.unavailable')} />
          ) : drivers.data && drivers.data.length === 0 ? (
            <EmptyState icon="plug" title={t('csi.empty')} />
          ) : (
            <Table columns={driverColumns} rows={drivers.data ?? []} rowKey={(d) => d.name} />
          )}
        </section>
      </div>
    </div>
  )
}
