import type {
  AccessRequest,
  BindingGraph,
  ReviewRequest,
  ReviewResult,
  RiskyRole,
  Subject,
  SubjectDetail,
  SubjectSummary,
  WhoCanResult,
} from './types'

/**
 * The access plugin's data capability (design §7.8). Everything is read
 * only; `review` asks the cluster's authorizer through a
 * SubjectAccessReview (rate-limited per user on the server).
 */
export interface AccessProvider {
  listSubjects(): Promise<SubjectSummary[]>
  getSubject(subject: Subject): Promise<SubjectDetail>
  whoCan(request: AccessRequest): Promise<WhoCanResult>
  listRiskyRoles(): Promise<RiskyRole[]>
  /** `namespace` empty: every binding. `includeSystem` keeps system:… bindings. */
  graph(namespace: string, includeSystem: boolean): Promise<BindingGraph>
  review(request: ReviewRequest): Promise<ReviewResult>
}
