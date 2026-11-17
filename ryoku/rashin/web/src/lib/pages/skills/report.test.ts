import { describe, expect, it } from "vitest";
import { readSkillsReport } from "./report";

describe("readSkillsReport", () => {
  it("turns the daemon's null lists into empty ones", () => {
    const report = readSkillsReport({
      counts: { agent: 0 },
      categories: null as never,
      toolbelt: null as never,
    });
    expect(report.categories.reduce((sum, category) => sum + category.skills.length, 0)).toBe(0);
    expect(report.toolbelt).toEqual([]);
  });

  it("keeps a category whose skills came back null", () => {
    const report = readSkillsReport({
      categories: [{ name: "general", skills: null as never }],
      toolbelt: [{ family: "Web", tools: null as never }],
    });
    expect(report.categories).toEqual([{ name: "general", skills: [] }]);
    expect(report.toolbelt).toEqual([{ family: "Web", tools: [] }]);
  });
});
