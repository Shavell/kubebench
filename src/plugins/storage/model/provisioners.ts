/**
 * Storage provisioners and CSI drivers the wizards recognise, with the
 * StorageClass parameters and PV fields each expects. Plain data: names,
 * parameter keys and examples are the drivers' own; the explanations are
 * i18n keys (`hints.<id>.*`) in the plugin namespace.
 */
export interface ParameterHint {
  key: string
  example: string
  /** i18n key explaining the parameter. */
  note: string
}

export interface ProvisionerHint {
  id: string
  /** Provisioner (StorageClass) and CSI driver name. */
  name: string
  /** Product name, shown untranslated. */
  title: string
  /** i18n key of a one-line note. */
  note: string
  parameters: ParameterHint[]
  /** Binding mode the driver works best with. */
  bindingMode: 'Immediate' | 'WaitForFirstConsumer'
  /** Whether the driver can expand volumes (allowVolumeExpansion makes sense). */
  expansion: boolean
  /** A CSI driver (usable as a PV's CSI source); false for in-tree style provisioners. */
  csi: boolean
  /** Example volumeHandle of a pre-provisioned volume. */
  volumeHandle?: string
  fsType?: string
}

const param = (id: string, key: string, example: string, noteKey: string): ParameterHint => ({
  key,
  example,
  note: `hints.${id}.params.${noteKey}`,
})

export const PROVISIONERS: ProvisionerHint[] = [
  {
    id: 'awsEbs',
    name: 'ebs.csi.aws.com',
    title: 'AWS EBS',
    note: 'hints.awsEbs.note',
    parameters: [
      param('awsEbs', 'type', 'gp3', 'type'),
      param('awsEbs', 'iops', '3000', 'iops'),
      param('awsEbs', 'throughput', '125', 'throughput'),
      param('awsEbs', 'encrypted', 'true', 'encrypted'),
      param('awsEbs', 'csi.storage.k8s.io/fstype', 'ext4', 'fstype'),
    ],
    bindingMode: 'WaitForFirstConsumer',
    expansion: true,
    csi: true,
    volumeHandle: 'vol-0123456789abcdef0',
    fsType: 'ext4',
  },
  {
    id: 'gcePd',
    name: 'pd.csi.storage.gke.io',
    title: 'Google Persistent Disk',
    note: 'hints.gcePd.note',
    parameters: [
      param('gcePd', 'type', 'pd-balanced', 'type'),
      param('gcePd', 'replication-type', 'none', 'replication'),
      param('gcePd', 'csi.storage.k8s.io/fstype', 'ext4', 'fstype'),
    ],
    bindingMode: 'WaitForFirstConsumer',
    expansion: true,
    csi: true,
    volumeHandle: 'projects/<project>/zones/<zone>/disks/<disk>',
    fsType: 'ext4',
  },
  {
    id: 'azureDisk',
    name: 'disk.csi.azure.com',
    title: 'Azure Disk',
    note: 'hints.azureDisk.note',
    parameters: [
      param('azureDisk', 'skuName', 'Premium_LRS', 'skuName'),
      param('azureDisk', 'cachingMode', 'ReadOnly', 'cachingMode'),
      param('azureDisk', 'csi.storage.k8s.io/fstype', 'ext4', 'fstype'),
    ],
    bindingMode: 'WaitForFirstConsumer',
    expansion: true,
    csi: true,
    volumeHandle: '/subscriptions/<subscription>/resourceGroups/<group>/providers/Microsoft.Compute/disks/<disk>',
    fsType: 'ext4',
  },
  {
    id: 'longhorn',
    name: 'driver.longhorn.io',
    title: 'Longhorn',
    note: 'hints.longhorn.note',
    parameters: [
      param('longhorn', 'numberOfReplicas', '3', 'replicas'),
      param('longhorn', 'staleReplicaTimeout', '2880', 'staleTimeout'),
      param('longhorn', 'dataLocality', 'best-effort', 'dataLocality'),
    ],
    bindingMode: 'Immediate',
    expansion: true,
    csi: true,
    volumeHandle: '<longhorn-volume-name>',
    fsType: 'ext4',
  },
  {
    id: 'localPath',
    name: 'rancher.io/local-path',
    title: 'Local Path Provisioner',
    note: 'hints.localPath.note',
    parameters: [param('localPath', 'nodePath', '/var/lib/rancher/k3s/storage', 'nodePath')],
    bindingMode: 'WaitForFirstConsumer',
    expansion: false,
    csi: false,
  },
  {
    id: 'noProvisioner',
    name: 'kubernetes.io/no-provisioner',
    title: 'Static (no provisioner)',
    note: 'hints.noProvisioner.note',
    parameters: [],
    bindingMode: 'WaitForFirstConsumer',
    expansion: false,
    csi: false,
  },
  {
    id: 'csiHostpath',
    name: 'hostpath.csi.k8s.io',
    title: 'CSI hostpath driver',
    note: 'hints.csiHostpath.note',
    parameters: [],
    bindingMode: 'WaitForFirstConsumer',
    expansion: true,
    csi: true,
    volumeHandle: '<volume-id>',
  },
  {
    id: 'csiNfs',
    name: 'nfs.csi.k8s.io',
    title: 'CSI NFS driver',
    note: 'hints.csiNfs.note',
    parameters: [
      param('csiNfs', 'server', 'nfs.example.internal', 'server'),
      param('csiNfs', 'share', '/exports', 'share'),
      param('csiNfs', 'subDir', '${pvc.metadata.namespace}/${pvc.metadata.name}', 'subDir'),
    ],
    bindingMode: 'Immediate',
    expansion: false,
    csi: true,
    volumeHandle: 'nfs.example.internal#exports#data#',
  },
]

const byName = new Map(PROVISIONERS.map((p) => [p.name, p]))

/** The hint for a provisioner or CSI driver name, if it is a known one. */
export function provisionerHint(name: string): ProvisionerHint | undefined {
  return byName.get(name.trim())
}
