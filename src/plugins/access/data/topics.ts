/** The resources the access views are computed from, as live-update topics (design §7.10). */
export const RBAC_TOPICS = ['roles', 'rolebindings', 'clusterroles', 'clusterrolebindings', 'serviceaccounts']

/** Query scope of the access views. */
export const RBAC_SCOPE = { topics: RBAC_TOPICS }
