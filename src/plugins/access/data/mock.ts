import { ApiError } from '@/data/apiClient'
import { reasonError } from '@/data/reasonError'
import { delay, delayReject } from '@/data/mock/delay'
import { identityOf, RbacEngine } from './mockEngine'
import { mockRbac } from './mockRbac'
import type { AccessProvider } from './provider'
import type { AccessRequest, Grant, ReviewResult, Subject } from './types'

/**
 * Mock access data over the demo cluster's RBAC (mockRbac), evaluated by
 * the same rules as the server. Reviews are answered by that evaluation,
 * with the reason text the API server's RBAC authorizer gives.
 */

/** The server's refusal of field, with the same reason code (design §3.3). */
const badRequest = (field: string, reason: string) => reasonError(400, 'bad_request', reason, { field })

function requestProblem(r: AccessRequest): ApiError | null {
  if (!r.verb) return badRequest('verb', 'field.required')
  if (r.path) {
    if (r.resource || r.subresource || r.group || r.namespace || r.name) return badRequest('path', 'access.non_resource_fields')
    return r.path.startsWith('/') ? null : badRequest('path', 'access.path_slash')
  }
  return r.resource ? null : badRequest('resource', 'access.resource_required')
}

function subjectProblem(s: Subject): ApiError | null {
  if (!s.name) return badRequest('name', 'field.required')
  if (s.kind === 'ServiceAccount' && !s.namespace) return badRequest('namespace', 'field.required')
  if (s.kind !== 'ServiceAccount' && s.namespace) return badRequest('namespace', 'access.namespace_only_service_account')
  return null
}

/** The RBAC authorizer's wording, e.g. `RBAC: allowed by RoleBinding "web/shop" of Role "web"`. */
function reasonOf(grant: Grant): string {
  const binding = grant.binding.namespace ? `${grant.binding.name}/${grant.binding.namespace}` : grant.binding.name
  return `RBAC: allowed by ${grant.binding.kind} "${binding}" of ${grant.role.kind} "${grant.role.name}"`
}

export function createMockAccess(): AccessProvider {
  const engine = new RbacEngine(mockRbac)
  return {
    listSubjects: () => delay(engine.subjects()),
    getSubject: (subject) => {
      const problem = subjectProblem(subject)
      return problem ? delayReject(problem) : delay(engine.subject(subject))
    },
    whoCan: (request) => {
      const problem = requestProblem(request)
      return problem ? delayReject(problem) : delay({ request, subjects: engine.whoCan(request) })
    },
    listRiskyRoles: () => delay(engine.risky()),
    graph: (namespace, includeSystem) => delay(engine.graph(namespace, includeSystem)),
    review: (request) => {
      const { subject, ...access } = request
      const problem = subjectProblem(subject) ?? requestProblem(access)
      if (problem) return delayReject(problem)
      const id = identityOf(subject)
      const grants = engine.allows(subject, access)
      const result: ReviewResult = { allowed: grants.length > 0, groups: id.groups, grants }
      if (id.user) result.user = id.user
      if (grants.length) result.reason = reasonOf(grants[0])
      return delay(result)
    },
  }
}
