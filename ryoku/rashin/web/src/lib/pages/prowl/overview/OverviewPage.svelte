<script lang="ts">
  import { untrack } from "svelte";
  import Page from "$lib/app/Page.svelte";
  import { ApiError, api } from "$lib/api/client";
  import { prowl } from "$lib/api/prowl";
  import Button from "$lib/ui/Button.svelte";
  import Card from "$lib/ui/Card.svelte";
  import Dialog from "$lib/ui/Dialog.svelte";
  import Empty from "$lib/ui/Empty.svelte";
  import IconButton from "$lib/ui/IconButton.svelte";
  import Icon from "$lib/ui/Icon.svelte";
  import Lamp from "$lib/ui/Lamp.svelte";
  import Switch from "$lib/ui/Switch.svelte";
  import { formatCost, formatLatency, formatPercent, formatTokens, maskKey, relativeTime } from "../format";
  import type {
    AccountUsage,
    ActiveProfileResponse,
    DirectoryResponse,
    HarnessesResponse,
    LoginUsageResponse,
    ModelRow,
    Profile,
    ProjectsResponse,
    ProwlDaemonStatus,
    UnifiedKey,
    UpdateCheckSetting,
    UsageRequestRow,
    UsageResponse,
  } from "../types";

  interface Props {
    params: string[];
  }

  let { params }: Props = $props();
  let daemon = $state<ProwlDaemonStatus | null>(null);
  let directory = $state<DirectoryResponse | null>(null);
  let profiles = $state<Profile[]>([]);
  let activeProfile = $state<ActiveProfileResponse | null>(null);
  let models = $state<ModelRow[]>([]);
  let usage = $state<UsageResponse | null>(null);
  let requests = $state<UsageRequestRow[]>([]);
  let key = $state("");
  let accounts = $state<AccountUsage[]>([]);
  let projects = $state<ProjectsResponse | null>(null);
  let harnesses = $state<HarnessesResponse | null>(null);
  let updateCheck = $state(false);
  let loading = $state(true);
  let starting = $state(false);
  let error = $state("");
  let actionError = $state("");
  let keyRevealed = $state(false);
  let regenerateOpen = $state(false);
  let regenerating = $state(false);
  let copied = $state("");
  let retryTimer: number | undefined;

  const activeSet = $derived(profiles.find((profile) => profile.id === activeProfile?.activeProfileId));
  const capacityWindows = $derived(accounts.flatMap((account) =>
    (account.windows ?? []).map((window) => ({ account: account.name, ...window })),
  ));
  const tightestWindow = $derived.by(() => {
    if (capacityWindows.length === 0) return null;
    return capacityWindows.reduce((tightest, window) => window.utilization > tightest.utilization ? window : tightest);
  });
  const connectedProviders = $derived(directory?.counts.configured ?? 0);
  const enabledModels = $derived(models.filter((model) => model.enabled && model.available).length);
  const indexedProjects = $derived(projects?.projects.filter((project) => project.state === "ready" || project.state === "semantic building").length ?? 0);
  const detectedHarnesses = $derived(harnesses?.harnesses.filter((harness) => harness.detected) ?? []);
  const routedHarnesses = $derived(detectedHarnesses.filter((harness) => harness.active).length);
  const allHarnessesRouted = $derived(detectedHarnesses.length > 0 && routedHarnesses === detectedHarnesses.length);
  const gatewayUrl = $derived(daemon?.url || "http://127.0.0.1:8788");
  const providerHealth = $derived(`${directory?.counts.ready ?? 0} provider${directory?.counts.ready === 1 ? "" : "s"} ready`);

  async function load() {
    clearTimeout(retryTimer);
    loading = !usage && !directory;
    starting = false;
    error = "";

    try {
      const status = await api.status();
      daemon = (status as { prowl?: ProwlDaemonStatus }).prowl ?? null;
    } catch {
      error = "Rashin did not answer. Start the Rashin service, then retry.";
    }

    const results = await Promise.allSettled([
      prowl.providers.directory(),
      prowl.routing.profiles(),
      prowl.routing.activeProfile(),
      prowl.routing.models(),
      prowl.activity.usage("24h"),
      prowl.activity.requests({ limit: 5 }),
      prowl.settings.apiKey(),
      prowl.logins.usage(),
      prowl.projects.list(),
      prowl.setup.harnesses(),
      prowl.settings.updateCheck(),
    ] as const);

    const [directoryResult, profilesResult, activeResult, modelsResult, usageResult, requestsResult,
      keyResult, accountsResult, projectsResult, harnessesResult, updateResult] = results;
    if (directoryResult.status === "fulfilled") directory = directoryResult.value as DirectoryResponse;
    if (profilesResult.status === "fulfilled") profiles = profilesResult.value as Profile[];
    if (activeResult.status === "fulfilled") activeProfile = activeResult.value as ActiveProfileResponse;
    if (modelsResult.status === "fulfilled") models = modelsResult.value as ModelRow[];
    if (usageResult.status === "fulfilled") usage = usageResult.value as UsageResponse;
    if (requestsResult.status === "fulfilled") requests = requestsResult.value.requests;
    if (keyResult.status === "fulfilled") key = (keyResult.value as UnifiedKey).apiKey;
    if (accountsResult.status === "fulfilled") accounts = (accountsResult.value as LoginUsageResponse).accounts;
    if (projectsResult.status === "fulfilled") projects = projectsResult.value as ProjectsResponse;
    if (harnessesResult.status === "fulfilled") harnesses = harnessesResult.value as HarnessesResponse;
    if (updateResult.status === "fulfilled") updateCheck = (updateResult.value as UpdateCheckSetting).enabled;

    const failures = results.filter(
      (result): result is PromiseRejectedResult => result.status === "rejected",
    );
    const gatewayDown = failures.find(
      (result) => result.reason instanceof ApiError && result.reason.code === "gateway_down",
    );
    if (gatewayDown) {
      starting = true;
      error = "Rashin is starting Prowl. This page will retry automatically.";
      retryTimer = window.setTimeout(() => void load(), 2_000);
    } else if (failures[0]) {
      const reason = failures[0].reason;
      error = reason instanceof Error
        ? `${reason.message} Check Prowl's gateway, then retry.`
        : "Prowl did not answer. Check the gateway, then retry.";
    }
    loading = false;
  }

  async function copy(value: string, name: string) {
    actionError = "";
    try {
      await navigator.clipboard.writeText(value);
      copied = name;
      window.setTimeout(() => {
        if (copied === name) copied = "";
      }, 1_500);
    } catch {
      actionError = `${name} could not be copied. Select it and copy it manually.`;
    }
  }

  async function regenerateKey() {
    regenerating = true;
    actionError = "";
    try {
      key = (await prowl.settings.regenerateApiKey()).apiKey;
      keyRevealed = true;
      regenerateOpen = false;
    } catch (reason) {
      actionError = reason instanceof Error
        ? `${reason.message} Check that the Prowl gateway is running, then retry.`
        : "The unified key did not change. Check the Prowl gateway, then retry.";
    } finally {
      regenerating = false;
    }
  }

  async function setUpdateCheck(enabled: boolean) {
    actionError = "";
    try {
      updateCheck = (await prowl.settings.setUpdateCheck(enabled)).enabled;
    } catch (reason) {
      actionError = reason instanceof Error
        ? `${reason.message} Check that the Prowl gateway is running, then retry.`
        : "The update-check setting did not save. Check the Prowl gateway, then retry.";
    }
  }

  $effect(() => {
    const routeKey = params.join("/");
    untrack(() => {
      void routeKey;
      void load();
    });
    return () => clearTimeout(retryTimer);
  });
</script>

<Page title="Gateway overview" gloss="総覧" lead="One route for Rashin and every connected harness.">
  {#if loading}
    <Empty title="Reading Prowl" body="Rashin is checking the gateway, routing pool, traffic, and project index." />
  {:else}
    <div class="overview-grid">
      {#if error}
        <div class="wide notice" class:starting>
          <Lamp state={starting ? "busy" : "bad"} />
          <span>{error}</span>
          {#if !starting}<Button size="sm" onclick={() => void load()}>Retry</Button>{/if}
        </div>
      {/if}
      {#if actionError}<p class="wide action-error" role="alert">{actionError}</p>{/if}

      <Card title="Gateway" gloss="門" lead="The local inference endpoint Rashin owns.">
        <div class="gateway-state">
          <Lamp state={daemon?.running ? "ok" : starting ? "busy" : "idle"} />
          <div>
            <p class="t-row">{daemon?.running ? "Running" : starting ? "Starting" : "Stopped"}</p>
            <p class="t-small">{daemon?.version || "Version unavailable"}, port {daemon?.port ?? 8788}</p>
          </div>
        </div>
        <dl class="facts">
          <div><dt>Updates</dt><dd>Arrive with <code>ryoku update</code></dd></div>
          <div><dt>Health</dt><dd>{daemon?.running ? providerHealth : "Unavailable"}</dd></div>
          <div><dt>Models ready</dt><dd>{enabledModels}</dd></div>
        </dl>
        <div class="setting-row">
          <div>
            <p class="t-row">Check for Prowl updates</p>
            <p class="t-small">Only checks upstream when this is on.</p>
          </div>
          <Switch checked={updateCheck} label="Check for Prowl updates" onchange={(value) => void setUpdateCheck(value)} />
        </div>
      </Card>

      <Card title="Endpoint" gloss="接続" lead="Use this URL and key in OpenAI-compatible clients.">
        <div class="credential">
          <span class="t-label">Base URL</span>
          <code>{gatewayUrl}/v1</code>
          <IconButton icon="copy" label={copied === "Endpoint" ? "Copied" : "Copy endpoint"} onclick={() => void copy(`${gatewayUrl}/v1`, "Endpoint")} />
        </div>
        <div class="credential">
          <span class="t-label">Unified key</span>
          <code>{keyRevealed ? key : maskKey(key)}</code>
          <IconButton icon="eye" label={keyRevealed ? "Hide key" : "Reveal key"} active={keyRevealed} onclick={() => (keyRevealed = !keyRevealed)} />
          <IconButton icon="copy" label={copied === "Key" ? "Copied" : "Copy key"} armed={Boolean(key)} onclick={() => void copy(key, "Key")} />
        </div>
        <div class="credential-actions">
          <Button size="sm" onclick={() => (regenerateOpen = true)}>Regenerate key</Button>
        </div>
      </Card>

      <Card title="Ready to route" gloss="準備" lead="Each row opens the page that completes it.">
        <div class="checklist">
          <a href="#/prowl/providers" class:done={connectedProviders > 0}>
            <Icon name={connectedProviders > 0 ? "check" : "dot"} size={14} />
            <span><b>Provider connected</b><small>{connectedProviders} configured</small></span>
          </a>
          <a href="#/prowl/routing" class:done={Boolean(harnesses?.routable)}>
            <Icon name={harnesses?.routable ? "check" : "dot"} size={14} />
            <span><b>Active set routable</b><small>{activeSet?.name ?? "Default"}, {harnesses?.reason ?? `${enabledModels} usable models`}</small></span>
          </a>
          <a href="#/prowl/harnesses" class:done={allHarnessesRouted}>
            <Icon name={allHarnessesRouted ? "check" : "dot"} size={14} />
            <span><b>Harnesses routed</b><small>{routedHarnesses} of {detectedHarnesses.length} detected</small></span>
          </a>
          <a href="#/prowl/projects" class:done={indexedProjects > 0}>
            <Icon name={indexedProjects > 0 ? "check" : "dot"} size={14} />
            <span><b>A project indexed</b><small>{indexedProjects} ready</small></span>
          </a>
        </div>
      </Card>

      <Card title="Subscription capacity" gloss="容量" lead="Remaining headroom comes from each provider's own report.">
        {#if tightestWindow}
          {@const remaining = Math.max(0, 100 - tightestWindow.utilization)}
          <div class="capacity-head">
            <span class="t-value">{remaining.toFixed(0)}%</span>
            <span class="t-small">remaining, {tightestWindow.account} {tightestWindow.label}</span>
          </div>
          <div class="bar" class:alert={remaining < 15}><i style:width={`${remaining}%`}></i></div>
          <div class="capacity-lines">
            {#each accounts as account (account.provider)}
              {#if account.error}
                <div><span>{account.name}</span><span>{account.error} Sign in again from Providers.</span></div>
              {:else}
                {#each account.windows ?? [] as window (window.key)}
                  <div>
                    <span>{account.name}, {window.label}</span>
                    <span>{Math.max(0, 100 - window.utilization).toFixed(0)}% left{window.resetsAt ? `, resets ${relativeTime(window.resetsAt)}` : ""}</span>
                  </div>
                {/each}
                {#if account.balance != null}
                  <div><span>{account.name}</span><span>{account.balance} {account.unit ?? "credits"}</span></div>
                {/if}
              {/if}
            {/each}
          </div>
        {:else}
          <Empty title="No capacity report yet" body="Connect a subscription account in Providers to see its published windows here." />
        {/if}
      </Card>

      <Card title="Last 24 hours" gloss="通信" class="wide">
        <div class="traffic">
          <div class="kpi"><span class="t-value">{usage?.summary.requests ?? 0}</span><span class="t-label">Requests</span></div>
          <div class="kpi"><span class="t-value">{formatPercent(usage?.summary.requests ? usage.summary.successes / usage.summary.requests : 0)}</span><span class="t-label">Success</span></div>
          <div class="kpi"><span class="t-value">{formatTokens((usage?.summary.inputTokens ?? 0) + (usage?.summary.outputTokens ?? 0))}</span><span class="t-label">Tokens</span></div>
          <div class="kpi"><span class="t-value">{formatPercent(usage?.summary.failoverRate ?? 0)}</span><span class="t-label">Failover</span></div>
          <div class="kpi"><span class="t-value">{formatLatency(usage?.summary.avgLatencyMs ?? 0)}</span><span class="t-label">Latency</span></div>
          <div class="kpi"><span class="t-value">{formatCost(usage?.summary.costUsd, (usage?.summary.unknownCostRequests ?? 0) === 0)}</span><span class="t-label">Cost</span></div>
        </div>
      </Card>

      <Card title="Live route" gloss="流れ" lead="The five most recent requests, from requested route to the provider that answered." class="wide trace-card">
        {#if requests.length === 0}
          <Empty title="No requests have crossed Prowl" body="Send a Quick request or use a connected harness. The route will appear here." />
        {:else}
          <div class="route-trace">
            {#each requests as request (request.id)}
              <div class="trace-row">
                <span class="trace-time">{relativeTime(request.createdAt)}</span>
                <span class="trace-node">{request.routedFrom || "auto"}</span>
                <span class="trace-line" aria-hidden="true"><i></i></span>
                <span class="trace-node provider">{request.platform}</span>
                <span class="trace-line short" aria-hidden="true"><i></i></span>
                <span class="trace-model">{request.model}</span>
                <span class:failed={request.outcome !== "success"} class="trace-outcome">{request.outcome}</span>
                <span class="trace-latency">{formatLatency(request.latencyMs)}</span>
              </div>
            {/each}
          </div>
        {/if}
      </Card>
    </div>
  {/if}
</Page>

<Dialog bind:open={regenerateOpen} title="Regenerate the unified key" description="The current unified key stops working immediately.">
  <p class="dialog-copy">Connected harnesses use this machine's local token, so they keep working. Other clients using the unified key must receive the new value.</p>
  {#snippet footer()}
    <Button variant="quiet" onclick={() => (regenerateOpen = false)}>Cancel</Button>
    <Button variant="plate" busy={regenerating} autofocus onclick={() => void regenerateKey()}>Regenerate key</Button>
  {/snippet}
</Dialog>

<style>
  .overview-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--s4); align-content: start; }
  .overview-grid > :global(.wide) { grid-column: 1 / -1; }
  .notice { display: flex; align-items: center; gap: var(--s3); min-height: 44px; padding: var(--s2) var(--s3); border: 1px solid var(--line); border-radius: var(--radius); color: var(--ink-dim); }
  .notice span:nth-child(2) { flex: 1; }
  .notice.starting { border-style: dashed; }
  .action-error { color: var(--alert); padding: var(--s2) var(--s3); border-left: 1px solid var(--alert); }
  .gateway-state { display: flex; align-items: center; gap: var(--s3); margin-bottom: var(--s4); }
  .facts { display: grid; gap: var(--s2); }
  .facts div { display: flex; justify-content: space-between; gap: var(--s4); padding-bottom: var(--s2); border-bottom: 1px solid var(--line-soft); }
  .facts dt { color: var(--ink-mute); }
  .facts dd { margin: 0; color: var(--ink); text-align: right; }
  .facts code { font-size: var(--f-small); }
  .setting-row { display: flex; justify-content: space-between; align-items: center; gap: var(--s4); margin-top: var(--s4); padding-top: var(--s3); border-top: 1px solid var(--line-soft); }
  .credential { display: grid; grid-template-columns: 86px minmax(0, 1fr) auto auto; align-items: center; gap: var(--s2); min-height: 42px; border-bottom: 1px solid var(--line-soft); }
  .credential code { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--ink); font-size: var(--f-small); }
  .credential-actions { margin-top: var(--s4); }
  .checklist { display: flex; flex-direction: column; gap: 2px; }
  .checklist a { display: grid; grid-template-columns: 18px minmax(0, 1fr); gap: var(--s2); align-items: start; padding: var(--s2); border-radius: var(--radius); color: var(--ink-faint); }
  .checklist a:hover { background: var(--tint5); color: var(--ink-dim); }
  .checklist a.done { color: var(--ink); }
  .checklist b { display: block; font-weight: 500; }
  .checklist small { display: block; color: var(--ink-mute); font-size: var(--f-small); }
  .capacity-head { display: flex; align-items: baseline; gap: var(--s3); margin-bottom: var(--s2); }
  .capacity-lines { display: flex; flex-direction: column; margin-top: var(--s4); }
  .capacity-lines > div { display: flex; justify-content: space-between; gap: var(--s4); padding: var(--s2) 0; border-bottom: 1px solid var(--line-soft); color: var(--ink-dim); font-size: var(--f-small); }
  .capacity-lines > div span:last-child { color: var(--ink-mute); text-align: right; }
  .traffic { display: grid; grid-template-columns: repeat(6, minmax(90px, 1fr)); gap: var(--s4); }
  .route-trace { display: flex; flex-direction: column; }
  .trace-row { display: grid; grid-template-columns: 60px minmax(70px, .6fr) minmax(24px, .5fr) minmax(80px, .7fr) minmax(18px, .3fr) minmax(140px, 1.2fr) 64px 68px; align-items: center; min-height: 44px; border-bottom: 1px solid var(--line-soft); font-size: var(--f-small); }
  .trace-row:last-child { border-bottom: 0; }
  .trace-time, .trace-latency { font-family: var(--mono); font-size: var(--f-tiny); color: var(--ink-faint); }
  .trace-node { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--ink-dim); }
  .trace-node.provider { color: var(--ink); }
  .trace-line { position: relative; height: 1px; background: var(--line-strong); margin-inline: var(--s2); }
  .trace-line i { position: absolute; width: 5px; height: 5px; top: -2px; right: -1px; border-top: 1px solid var(--ink-mute); border-right: 1px solid var(--ink-mute); transform: rotate(45deg); }
  .trace-model { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: var(--mono); font-size: var(--f-small); color: var(--ink-dim); }
  .trace-outcome { color: var(--ink-mute); text-transform: capitalize; }
  .trace-outcome.failed { color: var(--alert); }
  .dialog-copy { color: var(--ink-dim); max-width: var(--measure); }

  @media (max-width: 1100px) {
    .overview-grid { grid-template-columns: 1fr; }
    .traffic { grid-template-columns: repeat(3, minmax(90px, 1fr)); }
    .trace-row { grid-template-columns: 54px minmax(64px, .5fr) 24px minmax(72px, .6fr) 18px minmax(100px, 1fr) 58px; }
    .trace-latency { display: none; }
  }
  @media (max-width: 760px) {
    .traffic { grid-template-columns: repeat(2, minmax(90px, 1fr)); }
    .credential { grid-template-columns: 72px minmax(0, 1fr) auto; }
    .credential > :global(button):last-child { grid-column: 3; }
    .trace-line.short, .trace-outcome { display: none; }
    .trace-row { grid-template-columns: 48px minmax(58px, .45fr) 20px minmax(66px, .55fr) minmax(90px, 1fr); }
  }
</style>
