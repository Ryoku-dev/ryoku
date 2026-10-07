<script lang="ts">
  import type { UsagePeriod, UsageSeriesPoint } from "../types";
  import { formatTokens, relativeTime } from "../format";
  import { densifySeries, scaleSeries } from "./activity";

  interface Props { series: UsageSeriesPoint[]; window: UsagePeriod; }
  let { series, window }: Props = $props();
  const denseSeries = $derived(densifySeries(series, window));
  const chart = $derived(scaleSeries(denseSeries, 1000, 220));
  const hasTraffic = $derived(chart.maxRequests > 0 || chart.maxTokens > 0);
  const first = $derived(denseSeries[0]);
</script>

{#if !hasTraffic}
  <p class="chart-empty">No traffic in this window</p>
{:else}
  <div class="chart-head">
    <span><i class="request-key"></i> Requests, peak {chart.maxRequests}</span>
    <span><i class="token-key"></i> Tokens, peak {formatTokens(chart.maxTokens)}</span>
  </div>
  <div class="chart-frame">
    <svg viewBox="0 0 1000 240" preserveAspectRatio="none" role="img" aria-label="Requests and token volume over time">
      <line x1="0" y1="0" x2="1000" y2="0" class="grid-line" />
      <line x1="0" y1="110" x2="1000" y2="110" class="grid-line" />
      <line x1="0" y1="220" x2="1000" y2="220" class="grid-line" />
      {#each chart.requestBars as bar}
        <rect x={bar.x} y={bar.y} width={bar.width} height={bar.height} class="request-bar" />
      {/each}
      <polyline points={chart.tokenPoints} class="token-line" vector-effect="non-scaling-stroke" />
    </svg>
  </div>
  <div class="axis"><span>Window start · {relativeTime(first?.start)}</span><span>Now</span></div>
{/if}

<style>
  .chart-head { display: flex; flex-wrap: wrap; justify-content: space-between; gap: var(--s2) var(--s4); margin-bottom: var(--s3); color: var(--ink-mute); font-size: var(--f-small); }
  .chart-head span { display: inline-flex; align-items: center; gap: var(--s2); }
  .chart-head i { display: inline-block; background: var(--bone); }
  .chart-head .request-key { width: 8px; height: 8px; }
  .chart-head .token-key { width: 18px; height: 0; border-top: 1px dashed var(--ink-mute); background: transparent; }
  .chart-frame { height: 230px; overflow: hidden; }
  svg { width: 100%; height: 100%; overflow: visible; }
  .grid-line { stroke: var(--line-soft); stroke-width: 1; }
  .request-bar { fill: var(--bone); opacity: .72; }
  .token-line { fill: none; stroke: var(--ink-mute); stroke-width: 1.5; stroke-dasharray: 5 5; stroke-linecap: square; stroke-linejoin: bevel; }
  .axis { display: flex; justify-content: space-between; gap: var(--s3); margin-top: var(--s2); color: var(--ink-faint); font-family: var(--mono); font-size: var(--f-tiny); }
  .chart-empty { display: flex; align-items: center; min-height: 180px; color: var(--ink-mute); }
</style>
