import { describe, expect, it } from "vitest";
import type { AccountUsage, DirectoryProvider, KeyView, SignInPlatform, UsageWindow } from "../types";
import { deriveKeylessState, filterProviders, groupProviders, tightestCapacity, type ProviderFilter } from "./providers";

function provider(overrides: Partial<DirectoryProvider> & Pick<DirectoryProvider, "id" | "name">): DirectoryProvider {
  return {
    class: "paid",
    friction: "card",
    apiKeyUrl: "",
    docsUrl: "",
    freeModels: 0,
    maxContext: 0,
    modalities: ["chat"],
    routable: true,
    probe: "",
    note: "",
    keyCount: 0,
    enabledKeyCount: 0,
    healthyKeyCount: 0,
    errorKeyCount: 0,
    ready: false,
    modelCount: 3,
    adapter: true,
    platform: overrides.id,
    keyless: false,
    configured: false,
    ...overrides,
  };
}

function key(platform: string, overrides: Partial<KeyView> = {}): KeyView {
  return {
    id: 1,
    platform,
    label: "Main",
    maskedKey: "sk-…1234",
    baseUrl: null,
    status: "healthy",
    enabled: true,
    keyless: false,
    exportable: true,
    createdAt: "2026-10-06T00:00:00Z",
    lastCheckedAt: null,
    lastHealthError: null,
    modelScope: [],
    maskedProxyUrl: "",
    cooldowns: [],
    ...overrides,
  };
}

function signIn(overrides: Partial<SignInPlatform> = {}): SignInPlatform {
  return {
    id: "claude",
    name: "Claude",
    kind: "oauth",
    routes_to: "anthropic",
    account_hint: "",
    signed_in: true,
    broken: false,
    account: "user@example.test",
    ...overrides,
  };
}

function account(name: string, windows?: UsageWindow[]): AccountUsage {
  return {
    provider: "claude",
    name,
    unit: "requests",
    cadence: "",
    windows,
    note: "",
    needsSignIn: false,
    error: "",
  };
}

describe("groupProviders", () => {
  it("puts connected providers first and keeps tier ordering inside each group", () => {
    const groups = groupProviders([
      provider({ id: "paid", name: "Zed Paid" }),
      provider({ id: "free-low", name: "A Free", class: "free", freeModels: 1 }),
      provider({ id: "free-high", name: "B Free", class: "free", freeModels: 8 }),
      provider({ id: "credits", name: "Credits", class: "credits" }),
    ], [key("paid")], [], []);

    expect(groups.connected.map((row) => row.provider.id)).toEqual(["paid"]);
    expect(groups.available.map((row) => row.provider.id)).toEqual(["free-high", "free-low", "credits"]);
  });

  it("joins a signed-in account by routes_to and preserves a signed-in platform missing from the directory", () => {
    const groups = groupProviders(
      [provider({ id: "anthropic", name: "Anthropic", class: "oauth" })],
      [key("claude", { id: 50 })],
      [signIn(), signIn({ id: "codex", name: "Codex", routes_to: "openai" })],
      [{ id: "claude", name: "Claude", kind: "oauth", detail: "", enrolled: true, offered: 4, models: 4, keyId: 50 }],
    );

    expect(groups.connected.map((row) => row.platform)).toEqual(["anthropic", "openai"]);
    expect(groups.connected[0]?.login?.enrolled).toBe(true);
    expect(groups.connected[0]?.keys).toEqual([]);
  });

  it("keeps connected custom endpoints even though they are not directory entries", () => {
    const groups = groupProviders([], [
      key("custom", { id: 8, label: "Lab", baseUrl: "https://lab.example/v1" }),
      key("custom", { id: 9, label: "Remote", baseUrl: "https://remote.example/v1" }),
    ], [], []);

    expect(groups.connected.map((row) => row.provider.name)).toEqual(["Lab", "Remote"]);
    expect(groups.connected.every((row) => row.keys.length === 1)).toBe(true);
  });
});

describe("filterProviders", () => {
  const rows = groupProviders([
    provider({ id: "kilo", name: "Kilo", class: "free", keyless: true, note: "No signup" }),
    provider({ id: "credits", name: "Cloud Credits", class: "renewable-credits" }),
    provider({ id: "claude", name: "Claude", class: "oauth" }),
    provider({ id: "paid", name: "Metered API", class: "paid" }),
  ], [], [signIn({ signed_in: false, routes_to: "claude" })], []).available;

  it.each<[ProviderFilter, string[]]>([
    ["free", ["kilo"]],
    ["credits", ["credits"]],
    ["paid", ["paid"]],
    ["keyless", ["kilo"]],
    ["sign-in", ["claude"]],
  ])("applies the %s filter", (filter, expected) => {
    expect(filterProviders(rows, "", filter).map((row) => row.provider.id)).toEqual(expected);
  });

  it("searches provider metadata after applying a chip", () => {
    expect(filterProviders(rows, "no signup", "keyless").map((row) => row.provider.id)).toEqual(["kilo"]);
    expect(filterProviders(rows, "metered", "free")).toEqual([]);
  });
});

describe("deriveKeylessState", () => {
  const kilo = provider({ id: "kilo", name: "Kilo", keyless: true });

  it("distinguishes no row, the free sentinel, and a real key", () => {
    expect(deriveKeylessState(kilo, [])).toBe("unconnected");
    expect(deriveKeylessState(kilo, [key("kilo", { keyless: true, maskedKey: "****ey" })])).toBe("free-tier");
    expect(deriveKeylessState(kilo, [key("kilo", { keyless: true })])).toBe("keyed");
  });

  it("does not classify an ordinary provider as keyless", () => {
    expect(deriveKeylessState(provider({ id: "paid", name: "Paid" }), [key("paid")])).toBe("not-keyless");
  });
});

describe("tightestCapacity", () => {
  it("returns no reading when providers publish no windows", () => {
    expect(tightestCapacity([])).toBeNull();
    expect(tightestCapacity([account("Work")])).toBeNull();
  });

  it("selects the most-utilized window and clamps published percentages", () => {
    const result = tightestCapacity([
      account("Work", [
        { key: "five-hour", label: "5 hour", utilization: 72, resetsAt: "2026-10-06T05:00:00Z", windowSeconds: 18_000, tokensUsed: 0 },
        { key: "weekly", label: "Weekly", utilization: 101, resetsAt: "2026-10-13T00:00:00Z", windowSeconds: 604_800, tokensUsed: 0 },
      ]),
      account("Personal", [
        { key: "daily", label: "Daily", utilization: -4, resetsAt: "2026-10-07T00:00:00Z", windowSeconds: 86_400, tokensUsed: 0 },
      ]),
    ]);

    expect(result).toEqual({
      provider: "claude",
      account: "Work",
      key: "weekly",
      label: "Weekly",
      utilization: 100,
      resetsAt: "2026-10-13T00:00:00Z",
    });
  });
});
