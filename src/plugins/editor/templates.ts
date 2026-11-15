/**
 * Starting points of the New resource tab. Each renders a ready-to-apply
 * manifest for a namespace; its label is the `newResource.templates.<id>`
 * i18n key. Names are placeholders to change before applying.
 */
export interface ResourceTemplate {
  id: string
  /** The kind it creates first: a list's New button picks the template by it. */
  kind: string
  render: (namespace: string) => string
}

const container = `        - name: my-app
          image: nginx:1.27
          ports:
            - containerPort: 80
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              memory: 128Mi`

export const TEMPLATES: ResourceTemplate[] = [
  {
    id: 'deployment',
    kind: 'Deployment',
    render: (ns) => `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: ${ns}
  labels:
    app: my-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
${container}
`,
  },
  {
    id: 'service',
    kind: 'Service',
    render: (ns) => `apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: ${ns}
spec:
  type: ClusterIP
  selector:
    app: my-app
  ports:
    - name: http
      port: 80
      targetPort: 80
`,
  },
  {
    id: 'configMap',
    kind: 'ConfigMap',
    render: (ns) => `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
  namespace: ${ns}
data:
  APP_MODE: production
  LOG_LEVEL: info
`,
  },
  {
    id: 'secret',
    kind: 'Secret',
    render: (ns) => `apiVersion: v1
kind: Secret
metadata:
  name: my-secret
  namespace: ${ns}
type: Opaque
stringData:
  username: admin
  password: change-me
`,
  },
  {
    id: 'ingress',
    kind: 'Ingress',
    render: (ns) => `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  namespace: ${ns}
spec:
  rules:
    - host: my-app.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: my-app
                port:
                  number: 80
`,
  },
  {
    id: 'job',
    kind: 'Job',
    render: (ns) => `apiVersion: batch/v1
kind: Job
metadata:
  name: my-job
  namespace: ${ns}
spec:
  backoffLimit: 2
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: main
          image: busybox:1.37
          command: ["sh", "-c", "echo Hello from KubeBench"]
`,
  },
  {
    id: 'cronJob',
    kind: 'CronJob',
    render: (ns) => `apiVersion: batch/v1
kind: CronJob
metadata:
  name: my-cronjob
  namespace: ${ns}
spec:
  schedule: "*/30 * * * *"
  concurrencyPolicy: Forbid
  jobTemplate:
    spec:
      backoffLimit: 2
      template:
        spec:
          restartPolicy: OnFailure
          containers:
            - name: main
              image: busybox:1.37
              command: ["sh", "-c", "date; echo Hello from KubeBench"]
`,
  },
  {
    id: 'namespace',
    kind: 'Namespace',
    render: () => `apiVersion: v1
kind: Namespace
metadata:
  name: my-namespace
`,
  },
  {
    id: 'persistentVolumeClaim',
    kind: 'PersistentVolumeClaim',
    render: (ns) => `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-data
  namespace: ${ns}
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
`,
  },
  {
    id: 'serviceAccount',
    kind: 'ServiceAccount',
    render: (ns) => `apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-service-account
  namespace: ${ns}
`,
  },
  {
    id: 'roleBinding',
    kind: 'Role',
    render: (ns) => `apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: pod-reader
  namespace: ${ns}
rules:
  - apiGroups: [""]
    resources: ["pods", "pods/log"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: pod-reader
  namespace: ${ns}
subjects:
  - kind: ServiceAccount
    name: my-service-account
    namespace: ${ns}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: pod-reader
`,
  },
]

/** The template for `kind` (a RoleBinding starts from Role + RoleBinding), else the Deployment one. */
export function templateFor(kind: string | undefined): ResourceTemplate {
  const wanted = kind === 'RoleBinding' ? 'Role' : kind
  return TEMPLATES.find((tpl) => tpl.kind === wanted) ?? TEMPLATES[0]
}

export function templateById(id: string): ResourceTemplate {
  return TEMPLATES.find((tpl) => tpl.id === id) ?? TEMPLATES[0]
}
