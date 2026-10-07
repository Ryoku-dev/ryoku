import type { UsagePeriod, UsageSeriesPoint, UsageSummary } from "../types";
import { parseInstant } from "../format";

export interface ActivityFigures {
  successRate: number;
  totalTokens: number;
  estimatedShare: number;
  costKnown: boolean;
  pricedShare: number;
}

export interface ScaledBar {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface ScaledSeries {
  requestBars: ScaledBar[];
  tokenPoints: string;
  maxRequests: number;
  maxTokens: number;
}

const WINDOW_GRID: Record<UsagePeriod, { buckets: number; bucketMs: number }> = {
  "24h": { buckets: 24, bucketMs: 60 * 60 * 1_000 },
  "7d": { buckets: 7, bucketMs: 24 * 60 * 60 * 1_000 },
  "30d": { buckets: 30, bucketMs: 24 * 60 * 60 * 1_000 },
};

export function activityFigures(summary: UsageSummary): ActivityFigures {
  const measuredRequests = summary.exactRequests + summary.estimatedRequests + summary.unavailableRequests;
  return {
    successRate: summary.requests > 0 ? summary.successes / summary.requests : 0,
    totalTokens: summary.inputTokens + summary.outputTokens,
    estimatedShare: measuredRequests > 0 ? summary.estimatedRequests / measuredRequests : 0,
    costKnown: summary.costKnownRequests > 0,
    pricedShare: summary.requests > 0 ? summary.costKnownRequests / summary.requests : 0,
  };
}

export function densifySeries(series: UsageSeriesPoint[], window: UsagePeriod, now = new Date()): UsageSeriesPoint[] {
  const { buckets, bucketMs } = WINDOW_GRID[window];
  const currentStart = Math.floor(now.getTime() / bucketMs) * bucketMs;
  const firstStart = currentStart - (buckets - 1) * bucketMs;
  const pointsByStart = new Map<number, UsageSeriesPoint>();

  for (const point of series) {
    const timestamp = parseInstant(point.start)?.getTime();
    if (timestamp === undefined) continue;
    const start = Math.floor(timestamp / bucketMs) * bucketMs;
    if (start < firstStart || start > currentStart) continue;
    pointsByStart.set(start, { ...point, start: new Date(start).toISOString() });
  }

  const dense = new Array<UsageSeriesPoint>(buckets);
  for (let index = 0; index < buckets; index += 1) {
    const start = firstStart + index * bucketMs;
    dense[index] = pointsByStart.get(start) ?? {
      start: new Date(start).toISOString(),
      requests: 0,
      inputTokens: 0,
      outputTokens: 0,
      estimatedRequests: 0,
      unavailableRequests: 0,
      costUsd: 0,
      costKnownRequests: 0,
    };
  }
  return dense;
}

export function scaleSeries(series: UsageSeriesPoint[], width: number, height: number): ScaledSeries {
  if (series.length === 0 || width <= 0 || height <= 0) {
    return { requestBars: [], tokenPoints: "", maxRequests: 0, maxTokens: 0 };
  }

  let maxRequests = 0;
  let maxTokens = 0;
  for (const point of series) {
    maxRequests = Math.max(maxRequests, point.requests);
    maxTokens = Math.max(maxTokens, point.inputTokens + point.outputTokens);
  }

  const bucketWidth = width / series.length;
  const barWidth = bucketWidth * 0.64;
  const requestBars = new Array<ScaledBar>(series.length);
  const tokenPoints = new Array<string>(series.length);
  let index = 0;
  for (const point of series) {
    const requests = Math.max(0, point.requests);
    const tokens = Math.max(0, point.inputTokens + point.outputTokens);
    const requestHeight = maxRequests === 0 ? 0 : (requests / maxRequests) * height;
    const x = index * bucketWidth + bucketWidth / 2;
    requestBars[index] = {
      x: x - barWidth / 2,
      y: height - requestHeight,
      width: barWidth,
      height: requestHeight,
    };
    const tokenY = maxTokens === 0 ? height : height - (tokens / maxTokens) * height;
    tokenPoints[index] = `${x},${tokenY}`;
    index += 1;
  }

  return {
    requestBars,
    tokenPoints: tokenPoints.join(" "),
    maxRequests,
    maxTokens,
  };
}
