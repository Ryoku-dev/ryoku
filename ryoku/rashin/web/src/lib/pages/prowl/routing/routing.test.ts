import { describe, expect, it } from "vitest";
import type { ModelRow, ProfileModel } from "../types";
import {
  buildMembershipWrite,
  countUsableMembers,
  groupAvailableModels,
  toggleProvider,
} from "./routing";

function model(id: number, platform: string, available = true, enabled = true): ModelRow {
  return {
    id,
    platform,
    modelId: `model-${id}`,
    displayName: `Model ${id}`,
    intelligenceRank: id,
    speedRank: id,
    sizeLabel: "",
    rpmLimit: null,
    rpdLimit: null,
    tpmLimit: null,
    tpdLimit: null,
    monthlyTokenBudget: "",
    contextWindow: 128_000,
    enabled,
    supportsVision: false,
    supportsTools: true,
    priority: null,
    fallbackEnabled: false,
    source: "catalog",
    keyId: null,
    keyLabel: null,
    endpointScope: null,
    qualifiedModelId: null,
    hasOverrides: false,
    overrideFields: [],
    hasProvider: true,
    keyCount: available ? 1 : 0,
    access: "paid",
    keyless: false,
    available,
  };
}

function member(id: number): ProfileModel {
  return {
    model_db_id: id,
    priority: id,
    enabled: true,
    platform: "test",
    model_id: `model-${id}`,
    display_name: `Model ${id}`,
    intelligence_rank: id,
    speed_rank: id,
    size_label: "",
    rpm_limit: null,
    rpd_limit: null,
    tpm_limit: null,
    tpd_limit: null,
    monthly_token_budget: "",
  };
}

describe("routing set derivation", () => {
  it("counts usable members against connected-provider availability", () => {
    expect(countUsableMembers([member(1), member(2), member(3)], [
      model(1, "openai"),
      model(2, "anthropic", false),
      model(4, "openai"),
    ])).toEqual({ usable: 1, total: 3 });
  });

  it("groups only connected-provider models and starts every provider collapsed", () => {
    const models = [model(3, "openai"), model(1, "anthropic"), model(2, "openai"), model(4, "offline", false)];
    expect(groupAvailableModels(models).map((group) => ({
      provider: group.provider,
      collapsed: group.collapsed,
      ids: group.models.map((entry) => entry.id),
    }))).toEqual([
      { provider: "anthropic", collapsed: true, ids: [1] },
      { provider: "openai", collapsed: true, ids: [2, 3] },
    ]);
    expect(groupAvailableModels(models, { openai: false }).find((group) => group.provider === "openai")?.collapsed).toBe(false);
  });

  it("selects missing provider models, then clears the whole provider", () => {
    const provider = [model(2, "openai"), model(3, "openai")];
    expect(toggleProvider([1, 2], provider)).toEqual([1, 2, 3]);
    expect(toggleProvider([1, 2, 3, 8], provider)).toEqual([1, 8]);
  });

  it("marks selected globally disabled models for enable before the reorder", () => {
    expect(buildMembershipWrite([4, 2], [model(2, "openai"), model(4, "openai", true, false)])).toEqual({
      entries: [
        { modelDbId: 4, priority: 1, enabled: true },
        { modelDbId: 2, priority: 2, enabled: true },
      ],
      enableModelIds: [4],
    });
  });
});
