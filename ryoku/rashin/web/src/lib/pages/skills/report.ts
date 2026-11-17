import type { Skill } from "./harnesses";

export interface ToolFamily {
  family: string;
  tools: string[];
}

export interface SkillsReport {
  counts: Record<string, number>;
  categories: Array<{ name: string; skills: Skill[] }>;
  toolbelt: ToolFamily[];
}

// the daemon encodes an empty Go slice as null
export function readSkillsReport(raw: Partial<SkillsReport> | null | undefined): SkillsReport {
  return {
    counts: raw?.counts || {},
    categories: (raw?.categories || []).map((category) => ({ ...category, skills: category.skills || [] })),
    toolbelt: (raw?.toolbelt || []).map((family) => ({ ...family, tools: family.tools || [] })),
  };
}
