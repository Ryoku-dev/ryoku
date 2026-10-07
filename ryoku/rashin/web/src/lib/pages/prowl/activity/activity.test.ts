import { describe, expect, it } from "vitest";
import type { UsageSeriesPoint, UsageSummary } from "../types";
import { activityFigures, densifySeries, scaleSeries } from "./activity";

const summary: UsageSummary = {
  window: "24h",
  requests: 10,
  successes: 8,
  failures: 2,
  inputTokens: 700,
  outputTokens: 300,
  exactRequests: 5,
  estimatedRequests: 3,
  unavailableRequests: 2,
  costUsd: 1.25,
  costKnownRequests: 6,
  unknownCostRequests: 4,
  avgLatencyMs: 850,
  failoverRate: 0.2,
};

function point(requests: number, inputTokens: number, outputTokens: number, start = "2026-10-06T00:00:00Z"): UsageSeriesPoint {
  return {
    start,
    requests,
    inputTokens,
    outputTokens,
    estimatedRequests: 0,
    unavailableRequests: 0,
    costUsd: 0,
    costKnownRequests: 0,
  };
}

describe("activity summary", () => {
  it("derives success, token, estimate and priced shares without hiding unknowns", () => {
    expect(activityFigures(summary)).toEqual({
      successRate: 0.8,
      totalTokens: 1_000,
      estimatedShare: 0.3,
      costKnown: true,
      pricedShare: 0.6,
    });
  });

  it("uses zero ratios when a window is empty", () => {
    expect(activityFigures({ ...summary, requests: 0, successes: 0, exactRequests: 0, estimatedRequests: 0, unavailableRequests: 0, costKnownRequests: 0 })).toEqual({
      successRate: 0,
      totalTokens: 1_000,
      estimatedShare: 0,
      costKnown: false,
      pricedShare: 0,
    });
  });
});

describe("activity series densification", () => {
  it("fills missing hourly buckets in a sparse 24 hour response", () => {
    const dense = densifySeries([
      point(1, 10, 5, "2026-10-06T01:00:00Z"),
      point(3, 30, 15, "2026-10-06T12:00:00Z"),
      point(2, 20, 10, "2026-10-07T00:00:00Z"),
    ], "24h", new Date("2026-10-07T00:34:00Z"));

    expect(dense).toHaveLength(24);
    const first = dense[0];
    const second = dense[1];
    const middle = dense[11];
    const last = dense[23];
    expect(first).toBeDefined();
    expect(second).toBeDefined();
    expect(middle).toBeDefined();
    expect(last).toBeDefined();
    if (!first || !second || !middle || !last) return;
    expect(first.start).toBe("2026-10-06T01:00:00.000Z");
    expect(last.start).toBe("2026-10-07T00:00:00.000Z");
    expect([first.requests, second.requests, middle.requests, last.requests]).toEqual([1, 0, 3, 2]);
  });

  it("returns a complete zero-filled daily grid for an empty response", () => {
    const dense = densifySeries([], "7d", new Date("2026-10-07T19:20:00Z"));

    expect(dense).toHaveLength(7);
    const first = dense[0];
    const last = dense[6];
    expect(first).toBeDefined();
    expect(last).toBeDefined();
    if (!first || !last) return;
    expect(first.start).toBe("2026-10-01T00:00:00.000Z");
    expect(last.start).toBe("2026-10-07T00:00:00.000Z");
    expect(dense.every((bucket) =>
      bucket.requests === 0
      && bucket.inputTokens === 0
      && bucket.outputTokens === 0
    )).toBe(true);
  });

  it("aligns daily points and keeps only the selected window edges", () => {
    const dense = densifySeries([
      point(1, 0, 0, "2026-10-01T23:59:59Z"),
      point(2, 0, 0, "2026-10-02T18:45:00Z"),
      point(31, 0, 0, "2026-10-31T23:59:59Z"),
      point(32, 0, 0, "2026-11-01T00:00:00Z"),
    ], "30d", new Date("2026-10-31T23:59:59Z"));

    expect(dense).toHaveLength(30);
    expect(dense[0]).toMatchObject({ start: "2026-10-02T00:00:00.000Z", requests: 2 });
    expect(dense[29]).toMatchObject({ start: "2026-10-31T00:00:00.000Z", requests: 31 });
    expect(dense.reduce((total, bucket) => total + bucket.requests, 0)).toBe(33);
  });
});

describe("activity series scaling", () => {
  it("keeps a single non-zero bucket visible on independent request and token scales", () => {
    const scaled = scaleSeries([
      point(0, 0, 0),
      point(5, 100, 300),
      point(0, 0, 0),
    ], 120, 40);

    expect(scaled.requestBars.map(({ y, height }) => ({ y, height }))).toEqual([
      { y: 40, height: 0 },
      { y: 0, height: 40 },
      { y: 40, height: 0 },
    ]);
    expect(scaled.tokenPoints).toBe("20,40 60,0 100,40");
    expect(scaled.maxRequests).toBe(5);
    expect(scaled.maxTokens).toBe(400);
  });

  it("returns no geometry for an empty series", () => {
    expect(scaleSeries([], 100, 40)).toEqual({
      requestBars: [],
      tokenPoints: "",
      maxRequests: 0,
      maxTokens: 0,
    });
  });
});
