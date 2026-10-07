import { describe, expect, it } from "vitest";
import { formatCost, formatLatency, formatPercent, formatTokens, maskKey, parseInstant, relativeTime } from "./format";

describe("Prowl formatting", () => {
  it("compacts token counts at each unit boundary", () => {
    expect(formatTokens(0)).toBe("0");
    expect(formatTokens(999)).toBe("999");
    expect(formatTokens(1_000)).toBe("1k");
    expect(formatTokens(12_300)).toBe("12.3k");
    expect(formatTokens(1_000_000)).toBe("1m");
    expect(formatTokens(null)).toBe("Unknown");
  });

  it("switches latency from milliseconds to seconds at one second", () => {
    expect(formatLatency(0)).toBe("0 ms");
    expect(formatLatency(999)).toBe("999 ms");
    expect(formatLatency(1_000)).toBe("1 s");
    expect(formatLatency(1_550)).toBe("1.6 s");
    expect(formatLatency(undefined)).toBe("Unknown");
  });

  it("formats ratios as percentages without hiding overages", () => {
    expect(formatPercent(0)).toBe("0%");
    expect(formatPercent(0.123)).toBe("12.3%");
    expect(formatPercent(1)).toBe("100%");
    expect(formatPercent(1.05)).toBe("105%");
    expect(formatPercent(null)).toBe("Unknown");
  });

  it("separates free cost from unknown cost", () => {
    expect(formatCost(0)).toBe("$0.00");
    expect(formatCost(0.0012)).toBe("$0.0012");
    expect(formatCost(12.345)).toBe("$12.35");
    expect(formatCost(0, false)).toBe("Unknown");
    expect(formatCost(null)).toBe("Unknown");
  });

  it("parses every timestamp shape as the same UTC instant", () => {
    const expected = "2026-10-07T00:15:03.000Z";
    expect(parseInstant("2026-10-07T00:15:03Z")?.toISOString()).toBe(expected);
    expect(parseInstant("2026-10-07 00:15:03")?.toISOString()).toBe(expected);
    expect(parseInstant("1791332103")?.toISOString()).toBe(expected);
    expect(parseInstant(1_791_332_103)?.toISOString()).toBe(expected);
    expect(parseInstant(1_791_332_103_000)?.toISOString()).toBe(expected);
    expect(parseInstant("1791332103000")?.toISOString()).toBe(expected);
    expect(parseInstant("not-a-date")).toBeNull();
  });

  it("uses stable relative-time boundaries for RFC3339 values", () => {
    const now = new Date("2026-10-06T12:00:00Z");
    expect(relativeTime("2026-10-06T11:59:56Z", now)).toBe("now");
    expect(relativeTime("2026-10-06T11:59:55Z", now)).toBe("5s ago");
    expect(relativeTime("2026-10-06T11:59:00Z", now)).toBe("1m ago");
    expect(relativeTime("2026-10-06T11:00:00Z", now)).toBe("1h ago");
    expect(relativeTime("2026-10-05T12:00:00Z", now)).toBe("1d ago");
    expect(relativeTime("2026-10-06T12:05:00Z", now)).toBe("in 5m");
    expect(relativeTime("not-a-date", now)).toBe("Unknown");
  });

  it("masks a unified key while keeping its family and final four", () => {
    expect(maskKey("prowlag-0123456789ab12")).toBe("prowlag-••••ab12");
    expect(maskKey("abcd1234")).toBe("••••1234");
    expect(maskKey("")).toBe("Not issued");
  });
});
