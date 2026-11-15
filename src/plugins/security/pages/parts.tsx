import { useTranslation } from 'react-i18next'
import { usePermission } from '@/core/permissions'
import { Badge } from '@/shared/Badge'
import { Button } from '@/shared/Button'
import { Checkbox } from '@/shared/form'
import { downloadReport, runScan } from '../actions'
import type { SecurityReport, SecurityResource, Severity } from '../data/types'
import { NS } from '../i18n'
import { SEVERITY_TONES } from '../model/severity'
import { useSecurityStore } from '../store'
import { openResource } from '../tabs'
import { resourceLabel, useSecurityTexts } from './texts'

export function SeverityBadge({ severity }: { severity: Severity }) {
  const { severity: label } = useSecurityTexts()
  return <Badge tone={SEVERITY_TONES[severity]}>{label(severity)}</Badge>
}

/** The object of a finding as a link that opens it. */
export function ResourceLink({ resource }: { resource: SecurityResource }) {
  const { t } = useTranslation(NS)
  return (
    <button
      type="button"
      className="security-link"
      title={t('actions.openResource')}
      onClick={(event) => {
        event.stopPropagation()
        openResource(resource)
      }}
    >
      {resourceLabel(resource)}
    </button>
  )
}

/**
 * "Include system namespaces" and Scan: a scan only reads, so every role
 * may run one. `compact` leaves the option out (it keeps its last value).
 */
export function ScanControls({ primary = true, compact = false }: { primary?: boolean; compact?: boolean }) {
  const { t } = useTranslation(NS)
  const includeSystem = useSecurityStore((s) => s.includeSystem)
  const scanning = useSecurityStore((s) => s.scanning)
  const setIncludeSystem = useSecurityStore((s) => s.setIncludeSystem)
  return (
    <>
      {!compact && (
        <Checkbox
          checked={includeSystem}
          onChange={setIncludeSystem}
          disabled={scanning}
          label={t('scan.includeSystem')}
          title={t('scan.includeSystemHint')}
        />
      )}
      <Button variant={primary ? 'primary' : 'secondary'} icon="play" busy={scanning} onClick={() => void runScan()}>
        {scanning ? t('scan.running') : t('scan.run')}
      </Button>
    </>
  )
}

/** Markdown and CSV downloads of the report. */
export function ExportButtons({ report }: { report: SecurityReport }) {
  const { t } = useTranslation(NS)
  return (
    <>
      <Button variant="secondary" icon="export" title={t('export.markdownHint')} onClick={() => void downloadReport(report, 'md')}>
        {t('export.markdown')}
      </Button>
      <Button variant="secondary" icon="export" title={t('export.csvHint')} onClick={() => void downloadReport(report, 'csv')}>
        {t('export.csv')}
      </Button>
    </>
  )
}

/** A mutation button: disabled, with the reason as tooltip, without the write permission. */
export function WriteButton({ label, icon, onClick }: { label: string; icon?: string; onClick: () => void }) {
  const { allowed, reason } = usePermission('write')
  return (
    <Button
      variant="secondary"
      icon={icon}
      disabled={!allowed}
      title={reason ?? undefined}
      onClick={(event) => {
        event.stopPropagation()
        onClick()
      }}
    >
      {label}
    </Button>
  )
}
