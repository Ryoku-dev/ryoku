import { ApiError } from "./client";
import type {
  ActiveProfileResponse,
  ActiveProfileRequest,
  AddKeyRequest,
  AddKeyResponse,
  AddProjectRequest,
  CacheStats,
  CancelSignInResponse,
  CatalogResponse,
  CatalogModality,
  CatalogProvidersResponse,
  CatalogUseRequest,
  CatalogUseResponse,
  ChainEntryUpdate,
  ChainPresetsResponse,
  ClearCooldownsResponse,
  ClearPenaltyInspectorResponse,
  CodeQuery,
  CodeReposResponse,
  CodeResult,
  CreatedChainResponse,
  CreateFromPresetRequest,
  CreateFromSelectionRequest,
  CreateProfileRequest,
  CustomKeyRequest,
  CustomKeyResponse,
  CustomProbeRequest,
  CustomProbeResponse,
  DirectoryResponse,
  DiscoverModelsResponse,
  ForgetLoginResponse,
  FreeTierResponse,
  FallbackEntry,
  HarnessesResponse,
  HarnessResponse,
  HarnessSetupRequest,
  HealthCheckResponse,
  HealthResponse,
  ImportPreviewResponse,
  ImportSelectedRequest,
  ImportSelectedResponse,
  InstallSkillsRequest,
  InstallSkillsResponse,
  KeyActivityResponse,
  KeyProvidersResponse,
  KeyView,
  LoginMutationResponse,
  LoginsResponse,
  LoginUsageResponse,
  LogsQuery,
  LogsResponse,
  ModelRow,
  ModelUpdate,
  PenaltyInspectorResponse,
  Profile,
  ProfileModel,
  ProjectRequest,
  ProjectResponse,
  ProjectsResponse,
  ProwlErrorBody,
  ProviderProbeResponse,
  RateLimitUsageResponse,
  RemoveProjectResponse,
  RevealKeyResponse,
  RoutingState,
  RoutingUpdate,
  SettingsVersion,
  SignInPlatformsResponse,
  SignInRequest,
  SignInSession,
  SortFallbackResponse,
  SuccessResponse,
  TogglePlatformRequest,
  TogglePlatformResponse,
  TokenUsageResponse,
  UnifiedKey,
  UpdateCheck,
  UpdateCheckSetting,
  UpdateKeyRequest,
  UpdateProfileRequest,
  UpdateRelease,
  UpdateStatus,
  UsageRequestsQuery,
  UsageRequestsResponse,
  UsagePeriod,
  UsageResponse,
} from "$lib/pages/prowl/types";

export type * from "$lib/pages/prowl/types";

type Method = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

function query(path: string, values: object): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(values)) {
    if (value !== undefined && value !== null && value !== "") params.set(key, String(value));
  }
  const encoded = params.toString();
  return encoded ? `${path}?${encoded}` : path;
}

export async function prowlRequest<T>(method: Method, path: string, body?: unknown): Promise<T> {
  const target = "/api/prowl/" + path.replace(/^\/+/, "");
  const isForm = typeof FormData !== "undefined" && body instanceof FormData;
  const response = await fetch(target, {
    method,
    headers: {
      accept: "application/json",
      ...(isForm ? {} : { "content-type": "application/json" }),
      "X-Rashin-Client": "console",
    },
    body: body === undefined ? undefined : isForm ? body : JSON.stringify(body),
  });
  const parsed = (await response.json().catch(() => ({}))) as Partial<ProwlErrorBody> & T;
  if (!response.ok) {
    throw new ApiError(target, response.status, parsed.error?.message, parsed.error?.code);
  }
  return parsed;
}

const get = <T>(path: string) => prowlRequest<T>("GET", path);
const post = <T>(path: string, body?: unknown) => prowlRequest<T>("POST", path, body);
const put = <T>(path: string, body?: unknown) => prowlRequest<T>("PUT", path, body);
const patch = <T>(path: string, body?: unknown) => prowlRequest<T>("PATCH", path, body);
const del = <T>(path: string) => prowlRequest<T>("DELETE", path);

export const prowl = {
  settings: {
    version: () => get<SettingsVersion>("settings/version"),
    updateCheck: () => get<UpdateCheckSetting>("settings/update-check"),
    setUpdateCheck: (enabled: boolean) => put<UpdateCheckSetting>("settings/update-check", { enabled }),
    apiKey: () => get<UnifiedKey>("settings/api-key"),
    regenerateApiKey: () => post<UnifiedKey>("settings/api-key/regenerate"),
    release: () => get<UpdateRelease>("update/release"),
    checkUpdate: () => get<UpdateCheck>("update/check"),
    updateStatus: () => get<UpdateStatus>("update/status"),
    freeTier: () => get<FreeTierResponse>("free-tier"),
    cacheStats: () => get<CacheStats>("cache/stats"),
  },

  providers: {
    directory: () => get<DirectoryResponse>("providers/directory"),
    probe: (id: string) => get<ProviderProbeResponse>(`providers/${encodeURIComponent(id)}/probe`),
  },

  keys: {
    list: () => get<KeyView[]>("keys"),
    providers: () => get<KeyProvidersResponse>("keys/providers"),
    add: (request: AddKeyRequest) => post<AddKeyResponse>("keys", request),
    update: (id: number, request: UpdateKeyRequest) => patch<SuccessResponse>(`keys/${id}`, request),
    remove: (id: number) => del<SuccessResponse>(`keys/${id}`),
    reveal: (id: number) => post<RevealKeyResponse>(`keys/${id}/reveal`),
    clearCooldowns: (id: number) => del<ClearCooldownsResponse>(`keys/${id}/cooldowns`),
    setPlatform: (platform: string, request: TogglePlatformRequest) =>
      patch<TogglePlatformResponse>(`keys/platform/${encodeURIComponent(platform)}`, request),
    activity: () => get<KeyActivityResponse>("keys/activity"),
    custom: {
      add: (request: CustomKeyRequest) => post<CustomKeyResponse>("keys/custom", request),
      probe: (request: CustomProbeRequest) => post<CustomProbeResponse>("keys/custom/probe", request),
      discoverModels: (request: CustomProbeRequest) =>
        post<DiscoverModelsResponse>("keys/custom/discover-models", request),
    },
    import: {
      preview: (request: FormData) => prowlRequest<ImportPreviewResponse>("POST", "keys/preview", request),
      selected: (request: ImportSelectedRequest) =>
        post<ImportSelectedResponse>("keys/import-selected", request),
    },
  },

  logins: {
    list: () => get<LoginsResponse>("logins"),
    enroll: (id: string) => post<LoginMutationResponse>(`logins/${encodeURIComponent(id)}/enroll`),
    withdraw: (id: string) => del<LoginMutationResponse>(`logins/${encodeURIComponent(id)}/enroll`),
    usage: () => get<LoginUsageResponse>("logins/usage"),
    platforms: () => get<SignInPlatformsResponse>("logins/platforms"),
    forget: (id: string) => del<ForgetLoginResponse>(`logins/${encodeURIComponent(id)}/forget`),
    signin: {
      start: (request: SignInRequest) => post<SignInSession>("signin", request),
      status: (id: string) => get<SignInSession>(`signin/${encodeURIComponent(id)}`),
      cancel: (id: string) => del<CancelSignInResponse>(`signin/${encodeURIComponent(id)}`),
    },
  },

  health: {
    read: () => get<HealthResponse>("health"),
    check: (keyId: number) => post<HealthCheckResponse>(`health/check/${keyId}`),
    checkAll: () => post<SuccessResponse>("health/check-all"),
  },

  routing: {
    profiles: () => get<Profile[]>("profiles"),
    activeProfile: () => get<ActiveProfileResponse>("profiles/active"),
    setActiveProfile: (profileId: ActiveProfileRequest["profileId"]) =>
      post<ActiveProfileResponse>("profiles/active", { profileId } satisfies ActiveProfileRequest),
    createProfile: (request: CreateProfileRequest) => post<Profile>("profiles", request),
    profileModels: (id: number) => get<ProfileModel[]>(`profiles/${id}/models`),
    reorderProfile: (id: number, entries: ChainEntryUpdate[]) =>
      put<SuccessResponse>(`profiles/${id}/reorder`, entries),
    updateProfile: (id: number, request: UpdateProfileRequest) => patch<Profile>(`profiles/${id}`, request),
    deleteProfile: (id: number) => del<SuccessResponse>(`profiles/${id}`),
    presets: () => get<ChainPresetsResponse>("profiles/presets"),
    createFromPreset: (id: string, request: CreateFromPresetRequest) =>
      post<CreatedChainResponse>(`profiles/presets/${encodeURIComponent(id)}`, request),
    createFromSelection: (request: CreateFromSelectionRequest) =>
      post<CreatedChainResponse>("profiles/from-selection", request),
    models: () => get<ModelRow[]>("models"),
    updateModel: (id: number, request: ModelUpdate) => patch<SuccessResponse>(`models/${id}`, request),
    deleteModel: (id: number) => del<SuccessResponse>(`models/${id}`),
    deleteCustomModel: (id: number) => del<SuccessResponse>(`models/custom/${id}`),
    fallback: (profile?: number) => get<FallbackEntry[]>(query("fallback", { profile })),
    saveFallback: (entries: ChainEntryUpdate[]) => put<SuccessResponse>("fallback", entries),
    sortFallback: (preset: "intelligence" | "speed" | "budget") =>
      post<SortFallbackResponse>(`fallback/sort/${preset}`),
    settings: () => get<RoutingState>("fallback/routing"),
    saveSettings: (request: RoutingUpdate) => put<RoutingState>("fallback/routing", request),
    tokenUsage: () => get<TokenUsageResponse>("fallback/token-usage"),
    rateLimitUsage: () => get<RateLimitUsageResponse>("fallback/rate-limit-usage"),
    penaltyInspector: () => get<PenaltyInspectorResponse>("fallback/penalty-inspector"),
    clearPenaltyInspector: () => del<ClearPenaltyInspectorResponse>("fallback/penalty-inspector"),
    catalog: (modality: CatalogModality = "chat") =>
      get<CatalogResponse>(query("catalog/models", { modality })),
    catalogProviders: () => get<CatalogProvidersResponse>("catalog/providers"),
    useCatalogModel: (request: CatalogUseRequest) => put<CatalogUseResponse>("catalog/use", request),
  },

  activity: {
    usage: (window: UsagePeriod = "24h") =>
      get<UsageResponse>(query("usage/summary", { window })),
    requests: (options: UsageRequestsQuery = {}) =>
      get<UsageRequestsResponse>(query("usage/requests", {
        limit: options.limit,
        failures: options.failures ? 1 : undefined,
      })),
    logs: (options: LogsQuery = {}) =>
      get<LogsResponse>(query("logs", options)),
    clearLogs: () => del<SuccessResponse>("logs"),
  },

  projects: {
    list: () => get<ProjectsResponse>("projects"),
    add: (request: AddProjectRequest) => post<ProjectResponse>("projects", request),
    reindex: (request: ProjectRequest) => post<ProjectResponse>("projects/reindex", request),
    remove: (root: string) => del<RemoveProjectResponse>(query("projects", { root })),
  },

  setup: {
    harnesses: () => get<HarnessesResponse>("setup/harnesses"),
    connectHarness: (id: string, request: HarnessSetupRequest = {}) =>
      post<HarnessResponse>(`setup/harnesses/${encodeURIComponent(id)}`, request),
    disconnectHarness: (id: string) => del<HarnessResponse>(`setup/harnesses/${encodeURIComponent(id)}`),
    installSkills: (request: InstallSkillsRequest = {}) =>
      post<InstallSkillsResponse>("setup/skills", request),
  },

  code: {
    repos: () => get<CodeReposResponse>("code/repos"),
    status: (params: CodeQuery = {}) => get<CodeResult>(query("code/status", params)),
    overview: (params: CodeQuery = {}) => get<CodeResult>(query("code/overview", params)),
    find: (params: CodeQuery) => get<CodeResult>(query("code/find", params)),
    def: (params: CodeQuery) => get<CodeResult>(query("code/def", params)),
    outline: (params: CodeQuery) => get<CodeResult>(query("code/outline", params)),
    references: (params: CodeQuery) => get<CodeResult>(query("code/references", params)),
    impact: (params: CodeQuery) => get<CodeResult>(query("code/impact", params)),
    search: (params: CodeQuery) => get<CodeResult>(query("code/search", params)),
    peek: (params: CodeQuery) => get<CodeResult>(query("code/peek", params)),
    history: (params: CodeQuery) => get<CodeResult>(query("code/history", params)),
    doctor: (params: CodeQuery = {}) => get<CodeResult>(query("code/doctor", params)),
  },
};
