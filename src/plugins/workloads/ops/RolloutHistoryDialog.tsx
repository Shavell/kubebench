import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { useWorkloadsTranslation } from '../i18n'
import type { ObjectKey } from './opsDialogs'
import { RolloutHistory } from './RolloutHistory'

/** The rollout history of a Deployment, opened from its row menu. */
export function RolloutHistoryDialog({ deployment, onClose }: { deployment: ObjectKey; onClose: () => void }) {
  const { t } = useWorkloadsTranslation()
  return (
    <Dialog
      open
      size="large"
      icon="history"
      title={t('workloadOps.rollout.historyTitle', { name: deployment.name })}
      onClose={onClose}
      closeOnBackdropClick
      footer={<Button onClick={onClose}>{t('common.close')}</Button>}
    >
      <RolloutHistory deployment={deployment} />
    </Dialog>
  )
}
