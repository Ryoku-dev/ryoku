import type { ChainEntryUpdate, ModelRow, ProfileModel } from "../types";

export interface ModelGroup {
  provider: string;
  models: ModelRow[];
  collapsed: boolean;
}

export interface UsableCount {
  usable: number;
  total: number;
}

export interface MembershipWrite {
  entries: ChainEntryUpdate[];
  enableModelIds: number[];
}

export function countUsableMembers(members: ProfileModel[], models: ModelRow[]): UsableCount {
  const available = new Set(models.filter((model) => model.available).map((model) => model.id));
  return {
    usable: members.reduce((count, member) => count + Number(available.has(member.model_db_id)), 0),
    total: members.length,
  };
}

export function groupAvailableModels(
  models: ModelRow[],
  collapsed: Readonly<Record<string, boolean>> = {},
): ModelGroup[] {
  const byProvider = new Map<string, ModelRow[]>();
  for (const model of models) {
    if (!model.available) continue;
    const group = byProvider.get(model.platform) ?? [];
    group.push(model);
    byProvider.set(model.platform, group);
  }

  return [...byProvider.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([provider, providerModels]) => ({
      provider,
      collapsed: collapsed[provider] ?? true,
      models: providerModels.sort((a, b) =>
        a.intelligenceRank - b.intelligenceRank || a.displayName.localeCompare(b.displayName)),
    }));
}


export function toggleModel(order: number[], modelId: number): number[] {
  return order.includes(modelId) ? order.filter((id) => id !== modelId) : [...order, modelId];
}

export function toggleProvider(order: number[], providerModels: ModelRow[]): number[] {
  const ids = providerModels.map((model) => model.id);
  if (ids.length === 0) return [...order];
  const selected = new Set(order);
  const allSelected = ids.every((id) => selected.has(id));
  if (allSelected) {
    const removed = new Set(ids);
    return order.filter((id) => !removed.has(id));
  }
  return [...order, ...ids.filter((id) => !selected.has(id))];
}

export function setAllShown(order: number[], shown: ModelRow[], selected: boolean): number[] {
  const shownIds = shown.map((model) => model.id);
  const shownSet = new Set(shownIds);
  if (!selected) return order.filter((id) => !shownSet.has(id));
  const current = new Set(order);
  return [...order, ...shownIds.filter((id) => !current.has(id))];
}

export function reorderMember(
  order: number[],
  modelId: number,
  offset: -1 | 1,
  visibleOrder: number[] = order,
): number[] {
  const visibleFrom = visibleOrder.indexOf(modelId);
  const visibleTo = visibleFrom + offset;
  if (visibleFrom < 0 || visibleTo < 0 || visibleTo >= visibleOrder.length) return [...order];
  const targetId = visibleOrder[visibleTo];
  if (targetId === undefined) return [...order];
  const from = order.indexOf(modelId);
  const to = order.indexOf(targetId);
  if (from < 0 || to < 0) return [...order];
  const next = [...order];
  next[from] = targetId;
  next[to] = modelId;
  return next;
}

export function buildMembershipWrite(order: number[], models: ModelRow[]): MembershipWrite {
  const byId = new Map(models.map((model) => [model.id, model]));
  return {
    entries: order.map((modelDbId, index) => ({ modelDbId, priority: index + 1, enabled: true })),
    enableModelIds: order.filter((id) => byId.get(id)?.enabled === false),
  };
}
