export type JsonPrimitive = string | number | boolean | null;
export type JsonValue = JsonPrimitive | JsonValue[] | { [key: string]: JsonValue };

export interface ProwlErrorBody {
  error: {
    type?: string;
    message?: string;
    code?: string;
  };
}

export interface ProwlDaemonStatus {
  installed: boolean;
  bin: string;
  version: string;
  running: boolean;
  port: number;
  url: string;
  repo: string;
  error: string;
}

export interface SettingsVersion {
  version: string | null;
}

export interface UpdateCheckSetting {
  enabled: boolean;
}

export interface UnifiedKey {
  apiKey: string;
}

export interface UpdateChange {
  sha: string;
  message: string;
  date?: string;
}

export interface UpdateCheck {
  status: string;
  installation: string;
  localSha: string | null;
  checkedAt: string;
  version: string | null;
  remoteSha?: string;
  remoteMessage?: string;
  remoteDate?: string;
  changes?: UpdateChange[];
}
export interface UpdateStatus {
  status: string;
  installation: string;
  localSha: string | null;
  lastChecked: string | null;
  version: string | null;
}


export interface UpdateRelease {
  disabled?: boolean;
  tagName?: string;
  body?: string | null;
  htmlUrl?: string;
  publishedAt?: string | null;
}

export interface KeyCooldown {
  modelId: string;
  expiresAtMs: number;
  remainingMs: number;
}

export interface CustomKeyModel {
  id: number;
  kind: string;
  modelId: string;
  displayName: string;
  family: string | null;
}

export interface KeyView {
  id: number;
  platform: string;
  label: string;
  maskedKey: string;
  baseUrl: string | null;
  status: string;
  enabled: boolean;
  keyless: boolean;
  exportable: boolean;
  createdAt: string;
  lastCheckedAt: string | null;
  lastHealthError: string | null;
  modelScope: string[] | null;
  maskedProxyUrl: string;
  models?: CustomKeyModel[];
  cooldowns: KeyCooldown[];
}

export interface KeyProvider {
  platform: string;
  name: string;
  keyless: boolean;
  configured: boolean;
  keyCount: number;
  enabledKeyCount: number;
  modelCount: number;
}

export interface KeyProvidersResponse {
  providers: KeyProvider[];
  summary: Record<string, number>;
}

export interface AddKeyRequest {
  platform: string;
  key: string;
  label?: string;
  proxyUrl?: string;
}

export interface AddKeyResponse {
  id: number;
  platform: string;
  label: string;
  maskedKey: string;
  maskedProxyUrl?: string;
  status: string;
  enabled: boolean;
  modelsAvailable: number;
  notice?: string;
}

export interface UpdateKeyRequest {
  label?: string;
  enabled?: boolean;
  baseUrl?: string;
  modelScope?: string[] | null;
  proxyUrl?: string;
}

export interface SuccessResponse {
  success: boolean;
}

export interface RevealKeyResponse {
  key: string;
}

export interface ClearCooldownsResponse {
  cleared: number;
}

export interface TogglePlatformRequest {
  enabled: boolean;
}

export interface TogglePlatformResponse extends SuccessResponse {
  enabled: boolean;
  updatedKeys: number;
}

export interface DirectoryCounts {
  total: number;
  configured: number;
  routable: number;
  ready: number;
  free: number;
  credits: number;
  paid: number;
  oauth: number;
  local: number;
  freeModels: number;
}

export interface DirectoryProvider {
  id: string;
  name: string;
  class: string;
  friction: string;
  apiKeyUrl: string;
  docsUrl: string;
  freeModels: number;
  maxContext: number;
  modalities: string[] | null;
  routable: boolean;
  probe: string;
  note: string;
  keyCount: number;
  enabledKeyCount: number;
  healthyKeyCount: number;
  errorKeyCount: number;
  ready: boolean;
  modelCount: number;
  adapter: boolean;
  platform: string;
  keyless: boolean;
  configured: boolean;
}

export interface DirectoryResponse {
  providers: DirectoryProvider[];
  counts: DirectoryCounts;
  sources: Record<string, string>;
}

export interface ProviderProbeResponse {
  provider: string;
  kind: string;
  published: boolean;
  message: string;
  limit: number | null;
  remaining: number | null;
  resetAt: string | null;
  window?: string;
}

export interface CustomModelInput {
  model: string;
  displayName?: string;
  supportsTools?: boolean;
  supportsVision?: boolean;
}

export interface CustomKeyRequest {
  baseUrl: string;
  keyId?: number;
  model?: string;
  models?: CustomModelInput[];
  displayName?: string;
  apiKey?: string;
  label?: string;
  supportsTools?: boolean;
  supportsVision?: boolean;
}

export interface CustomRegisteredModel {
  modelDbId: number;
  model: string;
  displayName: string;
  supportsTools: boolean;
  supportsVision: boolean;
  created: boolean;
}

export interface CustomKeyResponse {
  success: boolean;
  keyId: number;
  platform: string;
  baseUrl: string;
  model?: string;
  displayName?: string;
  models?: CustomRegisteredModel[];
  supportsTools?: boolean;
  supportsVision?: boolean;
}

export interface CustomProbeRequest {
  baseUrl: string;
  keyId?: number;
  apiKey?: string;
}

export interface CustomProbeResponse {
  modelId: string;
  latencyMs: number;
  reasoning?: boolean;
  toolCalls?: boolean;
}

export interface DiscoveredModel {
  id: string;
  ownedBy: string | null;
  registered: boolean;
  contextWindow?: number | null;
  priceNote?: string | null;
  isFree?: boolean | null;
  vision?: boolean | null;
  kind?: string | null;
}

export interface DiscoverModelsResponse {
  baseUrl: string;
  keyId: number | null;
  models: DiscoveredModel[];
  total: number;
  registeredCount: number;
}

export interface ImportModel {
  id: string;
  supportsTools?: boolean;
  supportsVision?: boolean;
}

export interface ImportPreviewKey {
  keyName: string;
  keyValue: string;
  detectedPlatform: string | null;
  prefix: string;
  baseUrl?: string | null;
  models?: ImportModel[];
  isDuplicate: boolean;
}


export interface ImportPreviewResponse {
  keys: ImportPreviewKey[];
  total: number;
  skipped: string[];
  duplicates: number;
}

export interface ImportSelectedKey {
  keyName: string;
  keyValue: string;
  platform: string;
  baseUrl?: string;
  models?: ImportModel[];
}

export interface ImportSelectedRequest {
  keys: ImportSelectedKey[];
}

export interface ImportSelectedResponse {
  imported: number;
  skipped: string[];
  errors: Array<{ key: string; error: string }>;
  total: number;
  modelsRegistered: number;
}

export interface LoginRow {
  id: string;
  name: string;
  kind: string;
  detail: string;
  enrolled: boolean;
  offered: number;
  models: number;
  keyId?: number;
}

export interface LoginsResponse {
  logins: LoginRow[];
}

export interface LoginMutationResponse {
  enrolled: boolean;
  keyId?: number;
  models?: number;
}

export interface SignInPlatform {
  id: string;
  name: string;
  kind: string;
  routes_to: string;
  account_hint: string;
  signed_in: boolean;
  broken: boolean;
  account: string;
}

export interface SignInPlatformsResponse {
  platforms: SignInPlatform[];
}

export interface SignInRequest {
  provider: string;
}

export interface SignInSession {
  id: string;
  provider: string;
  url: string;
  user_code: string;
  kind: string;
  state: string;
  error: string;
  account: string;
  expires_at: string;
}

export interface CancelSignInResponse {
  canceled: boolean;
}

export interface ForgetLoginResponse {
  removed: boolean;
}

export interface UsageWindow {
  key: string;
  label: string;
  utilization: number;
  resetsAt?: string;
  windowSeconds: number;
  tokensUsed: number;
}

export interface AccountUsage {
  provider: string;
  name: string;
  windows?: UsageWindow[];
  balance?: number;
  unit?: string;
  cadence?: string;
  note?: string;
  needsSignIn?: boolean;
  error?: string;
}

export interface LoginUsageResponse {
  accounts: AccountUsage[];
}

export interface QuotaSignal {
  platform: string;
  keyId: number;
  metric: string;
  limit: number | null;
  remaining: number | null;
  resetAt: string | null;
  source: string;
  observedAt: string;
}

export interface HealthPlatform {
  platform: string;
  hasProvider: boolean;
  totalKeys: number;
  healthyKeys: number;
  errorKeys: number;
  enabledKeys: number;
}

export interface HealthKey {
  id: number;
  platform: string;
  label: string;
  status: string;
  enabled: boolean;
  lastCheckedAt: string | null;
  lastHealthError: string | null;
}

export interface HealthDegradation {
  healthyProviders: number;
  totalProviders: number;
  ratio: number;
  state: string;
  degradedAt: string | null;
  pendingForMs?: number;
}

export interface HealthResponse {
  platforms: HealthPlatform[];
  keys: HealthKey[];
  quotaStates: QuotaSignal[];
  degradation: HealthDegradation;
}

export interface HealthCheckResponse {
  keyId: number;
  status: string;
}

export interface KeyActivity {
  keyId: number;
  served: number;
  failed: number;
  tokens: number;
  hops: number;
  avgMs: number;
  windowHours: number;
  lastUsedAt: number | null;
  lastError?: string;
  lastErrorKind?: string;
  lastErrorAt: number | null;
  coolingUntil: number | null;
  coolingModels: number;
}

export interface KeyActivityResponse {
  activity: KeyActivity[];
  windowHours: number;
}

export interface Weights {
  reliability: number;
  speed: number;
  intelligence: number;
}

export interface BenchmarkStatus {
  source: string;
  configured: boolean;
  status: string;
  lastSuccess: string | null;
  matched: number;
  available: number;
  error: string;
  attribution: string;
  indexVersion: number;
}

export interface RoutingScore {
  modelDbId: number;
  platform: string;
  modelId: string;
  displayName: string;
  enabled: boolean;
  reliability: number;
  speed: number;
  intelligence: number;
  headroom: number;
  rateLimit: number;
  score: number;
  totalRequests: number;
}

export interface RoutingState {
  strategy: string;
  weights: Weights | null;
  customWeights: Weights;
  exploreEnabled: boolean;
  peakHoursAdjust: boolean;
  peakStartHour: number;
  peakEndHour: number;
  peakTimezone: string;
  peakAdjusted: boolean;
  keySelectionStrategy: string;
  cooldownCeilingMs: number | null;
  benchmark: BenchmarkStatus;
  scores: RoutingScore[];
  presets?: JsonValue[];
}

export interface RoutingUpdate {
  strategy?: string;
  weights?: Weights | null;
  exploreEnabled?: boolean;
  peakHoursAdjust?: boolean;
  peakStartHour?: number;
  peakEndHour?: number;
  peakTimezone?: string;
  keySelectionStrategy?: string;
  cooldownCeilingMs?: number | null;
}

export interface Profile {
  id: number;
  name: string;
  strategy: string;
  emoji: string;
  color: string;
  type: string;
  is_favorite: number;
  sort_order: number;
  auto_sort: string | null;
  layout_config: string | null;
  auto_include_new_models: number;
  modelCount: number;
  created_at: string;
}

export interface ActiveProfileResponse {
  activeProfileId: number | null;
}
export interface ActiveProfileRequest {
  profileId: number | null;
}


export interface CreateProfileRequest {
  name: string;
  sourceProfileId?: number;
  empty?: boolean;
  strategy?: string;
}

export interface UpdateProfileRequest {
  name?: string;
  strategy?: string;
}

export interface ProfileModel {
  model_db_id: number;
  priority: number;
  enabled: boolean;
  platform: string;
  model_id: string;
  display_name: string;
  intelligence_rank: number;
  speed_rank: number;
  size_label: string;
  rpm_limit: number | null;
  rpd_limit: number | null;
  tpm_limit: number | null;
  tpd_limit: number | null;
  monthly_token_budget: string;
}

export interface ChainEntryUpdate {
  modelDbId: number;
  priority: number;
  enabled: boolean;
}

export interface ChainPreset {
  id: string;
  name: string;
  description: string;
  group: string;
  requirements: string[];
  models: number;
  strategy: string;
}

export interface ChainPresetsResponse {
  presets: ChainPreset[];
}

export interface CreateFromPresetRequest {
  name?: string;
  activate: boolean;
}

export interface CreateFromSelectionRequest {
  name: string;
  modelDbIds: number[];
  activate: boolean;
}

export interface CreatedChainResponse {
  id: number;
  name: string;
  models: number;
  callAs: string;
  active: boolean;
}

export interface ModelRow {
  id: number;
  platform: string;
  modelId: string;
  displayName: string;
  intelligenceRank: number;
  speedRank: number;
  sizeLabel: string;
  rpmLimit: number | null;
  rpdLimit: number | null;
  tpmLimit: number | null;
  tpdLimit: number | null;
  monthlyTokenBudget: string;
  contextWindow: number | null;
  enabled: boolean;
  supportsVision: boolean;
  supportsTools: boolean;
  priority: number | null;
  fallbackEnabled: boolean;
  source: string;
  keyId: number | null;
  keyLabel: string | null;
  endpointScope: string | null;
  qualifiedModelId: string | null;
  hasOverrides: boolean;
  overrideFields: string[];
  hasProvider: boolean;
  keyCount: number;
  access: string;
  keyless: boolean;
  available: boolean;
}

export type CatalogModality = "chat" | "embeddings" | "image" | "video" | "audio";

export interface CatalogUseRequest {
  modelDbIds: number[];
  use: boolean;
}
export interface CatalogUseResponse {
  inGateway: number;
}


export interface CatalogProvider {
  platform: string;
  label: string;
  access: string;
  hasKey: boolean;
  keyStatus: string;
  models: number;
  routableModels: number;
  inGatewayModels: number;
  latencyMs: number | null;
  reliability: number | null;
  requests: number | null;
  quotaRemaining: number | null;
  quotaLimit: number | null;
  requiresCard: boolean;
  signupUrl: string | null;
  docsUrl: string | null;
  source: string;
}

export interface CatalogProvidersResponse {
  total: number;
  providers: CatalogProvider[];
}

export interface ModelUpdate {
  displayName?: string;
  intelligenceRank?: number;
  speedRank?: number;
  sizeLabel?: string;
  rpmLimit?: number | null;
  rpdLimit?: number | null;
  tpmLimit?: number | null;
  tpdLimit?: number | null;
  monthlyTokenBudget?: string;
  contextWindow?: number | null;
  enabled?: boolean;
  supportsVision?: boolean;
  supportsTools?: boolean;
  fallbackEnabled?: boolean;
}

export interface FallbackEntry extends Omit<ModelRow, "id" | "priority" | "fallbackEnabled" | "hasProvider" | "access" | "available" | "qualifiedModelId"> {
  modelDbId: number;
  priority: number;
  effectivePriority: number;
  penalty: number;
  rateLimitHits: number;
  monthlyTokenBudgetTokens: number;
  tier: string;
  paidInputPerM: number | null;
  paidOutputPerM: number | null;
}
export interface TokenUsageResponse {
  totalBudget: number;
  totalUsed: number;
  models: TokenUsageModel[];
}

export interface RateLimitUsageResponse {
  generatedAtMs: number;
  rows: RateLimitUsage[];
}

export interface PenaltyInspectorResponse {
  generatedAtMs: number;
  lookbackMinutes: number;
  rows: PenaltyInspector[];
}

export interface ClearPenaltyInspectorResponse {
  cooldowns: number;
  penalties: number;
  failureWindows: number;
}


export interface SortFallbackResponse extends SuccessResponse {
  preset: string;
}

export interface TokenUsageModel {
  modelDbId: number;
  displayName: string;
  platform: string;
  modelId: string;
  budget: number;
  used: number;
  enabled: boolean;
  rpmLimit: number | null;
  rpdLimit: number | null;
  tpmLimit: number | null;
  tpdLimit: number | null;
}

export interface RateLimitWindow {
  used: number;
  limit: number;
}

export interface RateLimitUsage {
  modelDbId: number;
  platform: string;
  modelId: string;
  rpm: RateLimitWindow | null;
  rpd: RateLimitWindow | null;
  tpm: RateLimitWindow | null;
}

export interface PenaltyInspector {
  modelDbId: number | null;
  platform: string;
  modelId: string;
  displayName: string;
  enabled: boolean;
  fallbackEnabled: boolean;
  priority: number | null;
  penalty: { hits: number; value: number; rateLimitFactor: number };
  cooldowns: Array<{ keyId: number; keyLabel: string | null; keyStatus: string | null; expiresAtMs: number; expiresInMs: number }>;
  recentErrors: Array<{ id: number; keyId: number | null; keyLabel: string | null; error: string; latencyMs: number; createdAt: string }>;
  recentErrorCount: number;
  reasons: string[];
}

export type UsagePeriod = "24h" | "7d" | "30d";

export interface UsageSummary {
  window: string;
  requests: number;
  successes: number;
  failures: number;
  inputTokens: number;
  outputTokens: number;
  exactRequests: number;
  estimatedRequests: number;
  unavailableRequests: number;
  costUsd: number;
  costKnownRequests: number;
  unknownCostRequests: number;
  avgLatencyMs: number;
  failoverRate: number;
}

export interface UsagePlatform {
  platform: string;
  requests: number;
  errors: number;
  tokens: number;
  avgMs: number;
}

export interface UsageModel {
  platform: string;
  model: string;
  requests: number;
  errors: number;
  inputTokens: number;
  outputTokens: number;
  estimatedRequests: number;
  unavailableRequests: number;
  costUsd: number;
  costKnownRequests: number;
  avgMs: number;
}

export interface UsageSeriesPoint {
  start: string;
  requests: number;
  inputTokens: number;
  outputTokens: number;
  estimatedRequests: number;
  unavailableRequests: number;
  costUsd: number;
  costKnownRequests: number;
}

export interface UsageResponse {
  summary: UsageSummary;
  platforms: UsagePlatform[] | null;
  models: UsageModel[];
  series: UsageSeriesPoint[];
}

export interface UsageRequestRow {
  id: number;
  createdAt: string;
  platform: string;
  model: string;
  outcome: string;
  status: number;
  inputTokens: number;
  outputTokens: number;
  estimated: boolean;
  usageQuality: string;
  costUsd: number | null;
  costKnown: boolean;
  latencyMs: number;
  attempts: number;
  routedFrom: string | null;
  class: string | null;
  effort: string | null;
  error: string | null;
  errorKind: string | null;
}

export interface UsageRequestsQuery {
  limit?: number;
  failures?: boolean;
}

export interface UsageRequestsResponse {
  requests: UsageRequestRow[];
}

export interface ServerLog {
  id: number;
  level: string;
  ts: number;
  source: string;
  provider: string;
  model: string;
  event: string;
  message: string;
}

export interface LogsQuery {
  sinceId?: number;
  limit?: number;
  q?: string;
  provider?: string;
}

export interface LogsResponse {
  logs: ServerLog[];
  maxId: number;
}


export interface CatalogCapabilities {
  tools: boolean;
  vision: boolean;
  reasoning: boolean;
}

export interface CatalogProviderModel {
  modelDbId: number;
  platform: string;
  label: string;
  hasKey: boolean;
  keyStatus: string;
  priceIn: number | null;
  priceOut: number | null;
  contextWindow: number | null;
  latencyMs: number | null;
  throughput: number | null;
  reliability: number | null;
  requests: number | null;
  inGateway: boolean;
}

export interface CatalogModel {
  id: number;
  canonicalId: string;
  name: string;
  author: string;
  tier: string;
  contextWindow: number | null;
  priceIn: number | null;
  priceOut: number | null;
  caps: CatalogCapabilities;
  score: number | null;
  reliability: number | null;
  speed: number | null;
  intelligence: number | null;
  inGateway: boolean;
  routable: boolean;
  providers: CatalogProviderModel[];
}

export interface CatalogResponse {
  total: number;
  models: CatalogModel[];
  facets: JsonValue;
}


export interface FreeTierQuota {
  limit: number | null;
  remaining: number | null;
  resetAt: string | null;
  metric: string;
  keyCount: number;
}

export interface FreeTierPool {
  poolKey: string;
  platform: string;
  memberModelIds: string[];
  modelCount: number;
  disabledModelCount: number;
  keyCount: number;
  documentedBudget: number;
  bestLabel: string;
  kind: string;
  quota: FreeTierQuota | null;
}

export interface FreeTierResponse {
  generatedAt: string;
  summary: {
    poolCount: number;
    documentedMonthlyTokens: number;
    creditsBasedPools: number;
    unpublishedPools: number;
  };
  pools: FreeTierPool[];
}

export interface CacheStats {
  enabled: boolean;
  ttlSeconds: number;
  maxEntries: number;
  maxTemperature: number;
  entries: number;
  totalHits: number;
  estimatedRequestsSaved: number;
  savedPromptTokens: number;
  savedCompletionTokens: number;
  lookupHits: number;
  lookupMisses: number;
  hitRate: number;
  savedTokens: number;
}

export interface ProjectCounts {
  files: number;
  symbols: number;
  edges: number;
  resources: number;
  chunks: number;
  resolved_edges: number;
  external_edges: number;
  unresolved_edges: number;
  langs: Record<string, number>;
}

export interface ProjectStatus {
  counts: ProjectCounts;
  last_index: string;
  ai_enabled: boolean;
  savings: { queries: number; answer_tokens: number; saved_tokens: number };
  semantic: { chunks: number; embedded: number; remaining: number; complete: boolean };
}

export interface ProjectJob {
  kind: "index" | "reindex";
  startedAt: string;
  error?: string;
}

export interface Project {
  root: string;
  name: string;
  state: "ready" | "indexing" | "semantic building" | "not indexed" | "error";
  error?: string;
  status?: ProjectStatus;
  embedModel: string;
  job?: ProjectJob;
}

export interface ProjectsResponse {
  projects: Project[];
}

export interface AddProjectRequest {
  root: string;
  integrations?: string[];
}

export interface ProjectRequest {
  root: string;
}

export interface ProjectResponse {
  project: Project;
}

export interface RemoveProjectResponse {
  removed: boolean;
}

export interface HarnessRow {
  id: string;
  name: string;
  detected: boolean;
  injected: boolean;
  active: boolean;
  files: string[];
  note?: string;
  skills: "current" | "install" | "update" | "conflict" | "unsupported" | "error";
}

export interface HarnessesResponse {
  routable: boolean;
  reason?: string;
  harnesses: HarnessRow[];
  skillsError?: string;
}

export interface HarnessSetupRequest {
  activate?: boolean;
  force?: boolean;
}

export interface HarnessResponse {
  harness: HarnessRow;
}

export interface InstallSkillsRequest {
  clients?: string[];
}

export interface InstallSkillsResponse {
  installed: number;
  updated: number;
  unchanged: number;
  conflicts: number;
  message: string;
}

export interface CodeRepo {
  name: string;
  path: string;
}

export interface CodeReposResponse {
  repos: CodeRepo[];
}

export interface CodeQuery {
  repo?: string;
  q?: string;
  path?: string;
  start?: number;
  end?: number;
  limit?: number;
  smart?: boolean;
}

export type CodeResult = JsonValue;
