import type {
  AccountUsage,
  DirectoryProvider,
  KeyActivity,
  KeyView,
  LoginRow,
  SignInPlatform,
} from "../types";

export const KEYLESS_SENTINEL_MASK = "****ey";

export type ProviderFilter = "all" | "free" | "credits" | "paid" | "keyless" | "sign-in";
export type KeylessState = "not-keyless" | "unconnected" | "free-tier" | "keyed";

export interface ProviderBundle {
  provider: DirectoryProvider;
  platform: string;
  keys: KeyView[];
  signIn?: SignInPlatform;
  login?: LoginRow;
  connected: boolean;
}

export interface ProviderGroups {
  connected: ProviderBundle[];
  available: ProviderBundle[];
}

const tierOrder: Record<string, number> = {
  free: 0,
  "permanent-free": 0,
  credits: 1,
  "renewable-credits": 1,
  oauth: 2,
  local: 3,
  paid: 4,
};

export function providerPlatform(provider: DirectoryProvider): string {
  return provider.platform || provider.id;
}

function compareProviders(a: ProviderBundle, b: ProviderBundle): number {
  const tier = (tierOrder[a.provider.class] ?? 4) - (tierOrder[b.provider.class] ?? 4);
  if (tier !== 0) return tier;
  if (a.provider.freeModels !== b.provider.freeModels) return b.provider.freeModels - a.provider.freeModels;
  return a.provider.name.localeCompare(b.provider.name, undefined, { sensitivity: "base" });
}

function syntheticProvider(id: string, name: string, platform: string, providerKeys: KeyView[], adapter: boolean): DirectoryProvider {
  return {
    id,
    name,
    class: adapter ? "paid" : "custom",
    friction: adapter ? "API key" : "custom endpoint",
    apiKeyUrl: "",
    docsUrl: "",
    freeModels: 0,
    maxContext: 0,
    modalities: ["chat"],
    routable: true,
    probe: "",
    note: adapter ? "Stored provider credentials" : "OpenAI-compatible endpoint",
    keyCount: providerKeys.length,
    enabledKeyCount: providerKeys.filter((key) => key.enabled).length,
    healthyKeyCount: providerKeys.filter((key) => key.status === "healthy").length,
    errorKeyCount: providerKeys.filter((key) => key.status === "error").length,
    ready: providerKeys.some((key) => key.enabled && key.status !== "error"),
    modelCount: providerKeys.reduce((count, key) => count + (key.models?.length ?? 0), 0),
    adapter,
    platform,
    keyless: false,
    configured: true,
  };
}

export function groupProviders(
  directory: DirectoryProvider[],
  keys: KeyView[],
  signInPlatforms: SignInPlatform[],
  logins: LoginRow[],
): ProviderGroups {
  const loginByID = new Map<string, LoginRow>();
  const linkedKeyIDs = new Set<number>();
  for (const login of logins) {
    loginByID.set(login.id.toLowerCase(), login);
    if (login.keyId !== undefined) linkedKeyIDs.add(login.keyId);
  }
  const keysByPlatform = new Map<string, KeyView[]>();
  for (const key of keys) {
    if (linkedKeyIDs.has(key.id)) continue;
    const id = key.platform.toLowerCase();
    const platformKeys = keysByPlatform.get(id);
    if (platformKeys) platformKeys.push(key);
    else keysByPlatform.set(id, [key]);
  }

  const signInByRoute = new Map<string, SignInPlatform>();
  for (const platform of signInPlatforms) {
    signInByRoute.set((platform.routes_to || platform.id).toLowerCase(), platform);
  }
  const seen = new Set<string>();
  const bundles: ProviderBundle[] = [];

  for (const provider of directory) {
    const platform = providerPlatform(provider);
    const normalized = platform.toLowerCase();
    const signIn = signInByRoute.get(normalized);
    const providerKeys = keysByPlatform.get(normalized) ?? [];
    seen.add(normalized);
    bundles.push({
      provider,
      platform,
      keys: providerKeys,
      signIn,
      login: signIn ? loginByID.get(signIn.id.toLowerCase()) : undefined,
      connected: providerKeys.length > 0 || Boolean(signIn?.signed_in),
    });
  }

  for (const signIn of signInPlatforms) {
    const platform = signIn.routes_to || signIn.id;
    const normalized = platform.toLowerCase();
    if (!signIn.signed_in || seen.has(normalized)) continue;
    bundles.push({
      provider: {
        id: signIn.id,
        name: signIn.name,
        class: "oauth",
        friction: "registration",
        apiKeyUrl: "",
        docsUrl: "",
        freeModels: 0,
        maxContext: 0,
        modalities: [],
        routable: Boolean(signIn.routes_to),
        probe: "",
        note: "",
        keyCount: 0,
        enabledKeyCount: 0,
        healthyKeyCount: 0,
        errorKeyCount: 0,
        ready: false,
        modelCount: 0,
        adapter: Boolean(signIn.routes_to),
        platform,
        keyless: false,
        configured: true,
      },
      platform,
      keys: keysByPlatform.get(normalized) ?? [],
      signIn,
      login: loginByID.get(signIn.id.toLowerCase()),
      connected: true,
    });
    seen.add(normalized);
  }

  for (const [normalized, providerKeys] of keysByPlatform) {
    if (seen.has(normalized)) continue;
    const platform = providerKeys[0]?.platform ?? normalized;
    if (normalized !== "custom") {
      bundles.push({
        provider: syntheticProvider(platform, platform, platform, providerKeys, true),
        platform,
        keys: providerKeys,
        connected: true,
      });
      continue;
    }

    const endpointKeys = new Map<string, KeyView[]>();
    for (const key of providerKeys) {
      const endpoint = key.baseUrl || `key:${key.id}`;
      const grouped = endpointKeys.get(endpoint);
      if (grouped) grouped.push(key);
      else endpointKeys.set(endpoint, [key]);
    }
    for (const [endpoint, customKeys] of endpointKeys) {
      const name = customKeys[0]?.label || customKeys[0]?.baseUrl || "Custom endpoint";
      bundles.push({
        provider: syntheticProvider(`custom:${endpoint}`, name, platform, customKeys, false),
        platform,
        keys: customKeys,
        connected: true,
      });
    }
  }

  bundles.sort(compareProviders);
  return {
    connected: bundles.filter((provider) => provider.connected),
    available: bundles.filter((provider) => !provider.connected),
  };
}

export function providerTier(provider: DirectoryProvider): "free" | "credits" | "paid" | "sign-in" | "local" | "custom" {
  switch (provider.class) {
    case "free":
    case "permanent-free":
      return "free";
    case "credits":
    case "renewable-credits":
      return "credits";
    case "oauth":
      return "sign-in";
    case "local":
      return "local";
    case "custom":
      return "custom";
    default:
      return "paid";
  }
}

export function filterProviders(providers: ProviderBundle[], query: string, filter: ProviderFilter): ProviderBundle[] {
  const needle = query.trim().toLowerCase();
  return providers.filter(({ provider, signIn }) => {
    const matchesFilter = filter === "all"
      || (filter === "keyless" && provider.keyless)
      || (filter === "sign-in" && Boolean(signIn))
      || providerTier(provider) === filter;
    if (!matchesFilter) return false;
    if (!needle) return true;
    return [
      provider.name,
      provider.id,
      provider.platform,
      provider.note,
      provider.class,
      provider.friction,
      ...(provider.modalities ?? []),
    ].some((value) => value.toLowerCase().includes(needle));
  });
}

export function isKeylessSentinel(key: KeyView): boolean {
  return key.maskedKey === KEYLESS_SENTINEL_MASK;
}

export function deriveKeylessState(provider: DirectoryProvider, keys: KeyView[]): KeylessState {
  if (!provider.keyless) return "not-keyless";
  if (keys.length === 0) return "unconnected";
  return keys.every(isKeylessSentinel) ? "free-tier" : "keyed";
}

export function activityByKey(activity: KeyActivity[]): Map<number, KeyActivity> {
  const byKey = new Map<number, KeyActivity>();
  for (const row of activity) byKey.set(row.keyId, row);
  return byKey;
}

export interface CapacityWindow {
  provider: string;
  account: string;
  key: string;
  label: string;
  utilization: number;
  resetsAt?: string;
}

export function tightestCapacity(accounts: AccountUsage[]): CapacityWindow | null {
  let tightest: CapacityWindow | null = null;
  for (const account of accounts) {
    for (const window of account.windows ?? []) {
      const utilization = Math.min(100, Math.max(0, window.utilization));
      if (tightest && utilization <= tightest.utilization) continue;
      tightest = {
        provider: account.provider,
        account: account.name,
        key: window.key,
        label: window.label,
        utilization,
        resetsAt: window.resetsAt,
      };
    }
  }
  return tightest;
}
