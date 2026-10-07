import { api } from "$lib/api/client";
import type { SkillLike } from "./filter";

export interface Skill extends SkillLike {
  name: string;
  dir: string;
  description: string;
  origin: string;
}

export interface HarnessRouting {
  supported: boolean;
  injected: boolean;
  connected: boolean;
  active: boolean;
  pending: boolean;
  note?: string;
}

export interface Harness {
  id: string;
  name: string;
  present: boolean;
  home?: string;
  version?: string;
  model?: string;
  provider?: string;
  wired: boolean;
  skillCount: number;
  skills?: Skill[];
  sessions: number;
  lastActive?: string;
  note?: string;
  routing: HarnessRouting;
}

interface HarnessResponse {
  harnesses?: Harness[];
}

export const harnessCache: {
  data: Harness[] | null;
  at: number;
  promise: Promise<Harness[]> | null;
} = { data: null, at: 0, promise: null };

export function loadHarnesses(): Promise<Harness[]> {
  const now = Date.now();
  if (harnessCache.data && now - harnessCache.at < 60_000) return Promise.resolve(harnessCache.data);
  if (!harnessCache.promise) {
    harnessCache.promise = api.harnesses()
      .then((payload) => {
        const rows = (payload as unknown as HarnessResponse).harnesses || [];
        harnessCache.data = rows;
        harnessCache.at = Date.now();
        harnessCache.promise = null;
        return rows;
      })
      .catch((error: unknown) => {
        harnessCache.promise = null;
        throw error;
      });
  }
  return harnessCache.promise;
}
