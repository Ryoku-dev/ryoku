import { describe, expect, it } from "vitest";
import type { Project, ProjectStatus } from "../types";
import { deriveProjectState, semanticCoverage } from "./projects";

function status(embedded: number, chunks: number, complete = false): ProjectStatus {
  return {
    counts: {
      files: 4,
      symbols: 8,
      edges: 3,
      resources: 0,
      chunks,
      resolved_edges: 2,
      external_edges: 1,
      unresolved_edges: 0,
      langs: { typescript: 4 },
    },
    last_index: "2026-10-06T12:00:00Z",
    ai_enabled: true,
    savings: { queries: 1, answer_tokens: 12, saved_tokens: 40 },
    semantic: { chunks, embedded, remaining: Math.max(0, chunks - embedded), complete },
  };
}

function project(overrides: Partial<Project> = {}): Project {
  return {
    root: "/work/ryoku",
    name: "ryoku",
    state: "ready",
    embedModel: "static:potion-code-16M",
    ...overrides,
  };
}

describe("deriveProjectState", () => {
  it.each([
    "ready",
    "indexing",
    "semantic building",
    "not indexed",
    "error",
  ] as const)("keeps the API's %s state", (state) => {
    expect(deriveProjectState(project({ state }))).toBe(state);
  });

  it("treats a running job as indexing", () => {
    expect(deriveProjectState(project({
      state: "not indexed",
      job: { kind: "index", startedAt: "2026-10-06T12:00:00Z" },
    }))).toBe("indexing");
  });

  it("surfaces project and completed-job failures", () => {
    expect(deriveProjectState(project({ error: "index failed" }))).toBe("error");
    expect(deriveProjectState(project({
      job: { kind: "reindex", startedAt: "2026-10-06T12:00:00Z", error: "embed failed" },
    }))).toBe("error");
  });
});

describe("semanticCoverage", () => {
  it("calculates partial coverage and remaining chunks", () => {
    expect(semanticCoverage(status(3, 8))).toEqual({
      embedded: 3,
      chunks: 8,
      remaining: 5,
      percent: 38,
      label: "3 of 8 chunks embedded, 5 remaining",
    });
  });

  it("reports complete coverage at the boundary", () => {
    expect(semanticCoverage(status(8, 8, true))).toEqual({
      embedded: 8,
      chunks: 8,
      remaining: 0,
      percent: 100,
      label: "8 of 8 chunks embedded",
    });
  });

  it("handles empty and out-of-range counters", () => {
    expect(semanticCoverage()).toMatchObject({ embedded: 0, chunks: 0, remaining: 0, percent: 0 });
    expect(semanticCoverage(status(12, 8))).toMatchObject({ embedded: 8, chunks: 8, remaining: 0, percent: 100 });
    expect(semanticCoverage(status(-2, 8))).toMatchObject({ embedded: 0, chunks: 8, remaining: 8, percent: 0 });
  });
});
