<script lang="ts">
  import { untrack } from "svelte";
  import Page from "$lib/app/Page.svelte";
  import { prowl } from "$lib/api/prowl";
  import Button from "$lib/ui/Button.svelte";
  import Card from "$lib/ui/Card.svelte";
  import Dialog from "$lib/ui/Dialog.svelte";
  import Empty from "$lib/ui/Empty.svelte";
  import Seg from "$lib/ui/Seg.svelte";
  import { formatCost, formatLatency, formatPercent, formatTokens } from "../format";
  import type { ServerLog, UsagePeriod, UsageRequestRow, UsageResponse } from "../types";
  import ActivityChart from "./ActivityChart.svelte";
  import GatewayLogs from "./GatewayLogs.svelte";
  import RequestsTable from "./RequestsTable.svelte";
  import { activityFigures } from "./activity";

  interface Props { params: string[]; }
  let { params }: Props = $props();
  let period = $state<UsagePeriod>("24h");
  let usage = $state<UsageResponse | null>(null);
  let requests = $state<UsageRequestRow[]>([]);
  let logs = $state<ServerLog[]>([]);
  let failuresOnly = $state(false);
  let loading = $state(true);
  let requestsLoading = $state(true);
  let logsLoading = $state(true);
  let clearing = $state(false);
  let clearOpen = $state(false);
  let error = $state("");
  let actionError = $state("");
  let copied = $state(false);

  const figures = $derived(usage ? activityFigures(usage.summary) : null);
  const platforms = $derived(usage?.platforms ?? []);

  function errorMessage(reason: unknown, fallback: string): string {
    return reason instanceof Error ? `${reason.message} ${fallback}` : fallback;
  }

  async function loadUsage(nextPeriod = period) {
    loading = true;
    error = "";
    try {
      usage = await prowl.activity.usage(nextPeriod);
      period = nextPeriod;
    } catch (reason) {
      error = errorMessage(reason, "Check that Prowl's gateway is running, then retry.");
    } finally {
      loading = false;
    }
  }

  async function loadRequests(nextFailures = failuresOnly) {
    requestsLoading = true;
    actionError = "";
    try {
      requests = (await prowl.activity.requests({ limit: 100, failures: nextFailures })).requests;
      failuresOnly = nextFailures;
    } catch (reason) {
      actionError = errorMessage(reason, "The recent request trail is unavailable. Retry after checking the gateway.");
    } finally {
      requestsLoading = false;
    }
  }

  async function loadLogs() {
    logsLoading = true;
    actionError = "";
    try {
      logs = (await prowl.activity.logs({ limit: 200 })).logs;
    } catch (reason) {
      actionError = errorMessage(reason, "The gateway log tail is unavailable. Retry after checking the gateway.");
    } finally {
      logsLoading = false;
    }
  }

  async function load() {
    await Promise.all([loadUsage(), loadRequests(), loadLogs()]);
  }

  async function clearLogs() {
    clearing = true;
    actionError = "";
    try {
      await prowl.activity.clearLogs();
      logs = [];
      clearOpen = false;
    } catch (reason) {
      actionError = errorMessage(reason, "The logs were not cleared. Check the gateway and retry.");
    } finally {
      clearing = false;
    }
  }

  async function copyError(row: UsageRequestRow) {
    actionError = "";
    try {
      await navigator.clipboard.writeText(`${row.platform}/${row.model} [${row.errorKind || row.outcome}]\n${row.error || ""}`);
      copied = true;
      window.setTimeout(() => (copied = false), 1_500);
    } catch {
      actionError = "The error could not be copied. Select its text and copy it manually.";
    }
  }

  $effect(() => {
    const routeKey = params.join("/");
    untrack(() => {
      void routeKey;
      void Promise.all([loadUsage("24h"), loadRequests(false), loadLogs()]);
    });
  });
</script>

<Page title="Gateway activity" gloss="稼働" lead="Traffic, accounting quality, route outcomes, and the gateway's own warning tail.">
  {#snippet tools()}
    <Seg
      label="Activity window"
      options={[{ value: "24h", label: "24h" }, { value: "7d", label: "7d" }, { value: "30d", label: "30d" }]}
      value={period}
      onchange={(value) => void loadUsage(value as UsagePeriod)}
    />
    <Button icon="refresh" busy={loading || requestsLoading || logsLoading} onclick={() => void load()}>Refresh</Button>
  {/snippet}

  {#if loading && !usage}
    <Empty title="Reading gateway activity" body="Prowl is loading traffic, model usage, recent requests, and gateway logs." />
  {:else if error && !usage}
    <div class="notice" role="alert"><span>{error}</span><Button size="sm" onclick={() => void load()}>Retry</Button></div>
  {:else if usage && figures}
    <div class="activity-layout">
      {#if error}<p class="action-error" role="alert">{error}</p>{/if}
      {#if actionError}<p class="action-error" role="alert">{actionError}</p>{/if}
      {#if copied}<p class="notice-inline" role="status">Error copied.</p>{/if}

      <Card title={`Summary · ${period}`} gloss="集計">
        <div class="summary-grid">
          <div class="kpi"><span class="t-value">{formatTokens(usage.summary.requests)}</span><span class="t-label">Requests</span></div>
          <div class="kpi"><span class="t-value">{formatPercent(figures.successRate)}</span><span class="t-label">Success</span><small>{usage.summary.failures} failed</small></div>
          <div class="kpi"><span class="t-value">{formatTokens(usage.summary.inputTokens)}</span><span class="t-label">Tokens in</span></div>
          <div class="kpi"><span class="t-value">{formatTokens(usage.summary.outputTokens)}</span><span class="t-label">Tokens out</span><small>{formatPercent(figures.estimatedShare)} estimated</small></div>
          <div class="kpi"><span class="t-value">{formatCost(usage.summary.costUsd, figures.costKnown)}</span><span class="t-label">Known cost</span><small>{usage.summary.costKnownRequests} priced ({formatPercent(figures.pricedShare)}) · {usage.summary.unknownCostRequests} unknown</small></div>
          <div class="kpi"><span class="t-value">{formatLatency(usage.summary.avgLatencyMs)}</span><span class="t-label">Average latency</span></div>
          <div class="kpi"><span class="t-value">{formatPercent(usage.summary.failoverRate)}</span><span class="t-label">Failover rate</span></div>
          <div class="kpi"><span class="t-value">{usage.summary.unavailableRequests}</span><span class="t-label">Usage unavailable</span><small>{usage.summary.exactRequests} exact · {usage.summary.estimatedRequests} estimated</small></div>
        </div>
      </Card>

      <Card title="Traffic over time" gloss="推移" lead="Solid requests and dashed token volume use independent scales.">
        <ActivityChart series={usage.series} window={period} />
      </Card>

      <div class="rollups">
        <Card title="By platform" gloss="提供">
          {#if platforms.length === 0}
            <Empty title="No platform traffic" body="Platform totals appear after the first routed request in this window." />
          {:else}
            <div class="table-wrap"><table class="data"><thead><tr><th>Platform</th><th class="r">Requests</th><th class="r">Errors</th><th class="r">Tokens</th><th class="r">Average</th></tr></thead><tbody>
              {#each platforms as platform (platform.platform)}
                <tr><td class="ink">{platform.platform}</td><td class="r">{platform.requests}</td><td class="r">{platform.errors}</td><td class="r">{formatTokens(platform.tokens)}</td><td class="r">{formatLatency(platform.avgMs)}</td></tr>
              {/each}
            </tbody></table></div>
          {/if}
        </Card>

        <Card title="By model" gloss="模型">
          {#if usage.models.length === 0}
            <Empty title="No model traffic" body="Model totals appear after the first routed request in this window." />
          {:else}
            <div class="table-wrap"><table class="data models"><thead><tr><th>Provider · model</th><th class="r">Requests</th><th class="r">Tokens</th><th class="r">Cost</th><th class="r">Average</th></tr></thead><tbody>
              {#each usage.models as model (`${model.platform}/${model.model}`)}
                {@const tokens = model.inputTokens + model.outputTokens}
                <tr>
                  <td class="route"><strong>{model.platform}</strong><span>{model.model}</span></td>
                  <td class="r">{model.requests}{model.errors ? ` / ${model.errors} failed` : ""}</td>
                  <td class="r mono">{model.unavailableRequests === model.requests ? "—" : `${model.estimatedRequests ? "~" : ""}${formatTokens(tokens)}`}</td>
                  <td class="r mono">{formatCost(model.costUsd, model.costKnownRequests > 0)}</td>
                  <td class="r mono">{formatLatency(model.avgMs)}</td>
                </tr>
              {/each}
            </tbody></table></div>
          {/if}
        </Card>
      </div>

      <RequestsTable
        rows={requests}
        {failuresOnly}
        loading={requestsLoading}
        onfilter={(value) => void loadRequests(value)}
        oncopy={(row) => void copyError(row)}
      />
      <GatewayLogs {logs} loading={logsLoading} onclear={() => (clearOpen = true)} />
    </div>
  {/if}
</Page>

<Dialog bind:open={clearOpen} title="Clear gateway logs" description="This removes Prowl's durable warning and error tail.">
  <p class="dialog-copy">Request history and usage totals stay intact. Cleared gateway events cannot be restored.</p>
  {#snippet footer()}
    <Button variant="quiet" onclick={() => (clearOpen = false)}>Keep logs</Button>
    <Button variant="plate" busy={clearing} autofocus onclick={() => void clearLogs()}>Clear logs</Button>
  {/snippet}
</Dialog>

<style>
  .activity-layout { display: grid; gap: var(--s4); align-content: start; }
  .notice { display: flex; align-items: center; justify-content: space-between; gap: var(--s4); padding: var(--s3) var(--s4); border: 1px solid var(--line); border-radius: var(--radius); color: var(--ink-dim); }
  .action-error { padding: var(--s2) var(--s3); border-left: 1px solid var(--alert); color: var(--alert); }
  .notice-inline { color: var(--ink-dim); font-size: var(--f-small); }
  .summary-grid { display: grid; grid-template-columns: repeat(4, minmax(110px, 1fr)); gap: var(--s5); }
  .kpi small { color: var(--ink-faint); font-size: var(--f-small); }
  .rollups { display: grid; grid-template-columns: minmax(440px, .7fr) minmax(670px, 1.3fr); gap: var(--s4); align-items: start; }
  .table-wrap { overflow-x: auto; }
  .models { min-width: 620px; }
  .route { max-width: 260px; }
  .route strong, .route span { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .route strong { color: var(--ink); font-weight: 500; text-transform: capitalize; }
  .route span { margin-top: 2px; color: var(--ink-mute); font-family: var(--mono); font-size: var(--f-tiny); }
  .dialog-copy { color: var(--ink-dim); max-width: var(--measure); }
  @media (max-width: 1500px) { .rollups { grid-template-columns: 1fr; } }
  @media (max-width: 1050px) { .summary-grid { grid-template-columns: repeat(2, minmax(110px, 1fr)); } }
  @media (max-width: 620px) { .summary-grid { grid-template-columns: 1fr 1fr; gap: var(--s4); } }
</style>
