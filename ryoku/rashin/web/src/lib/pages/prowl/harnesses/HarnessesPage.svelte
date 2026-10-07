<script lang="ts">
  import { untrack } from "svelte";
  import Page from "$lib/app/Page.svelte";
  import { api } from "$lib/api/client";
  import { prowl } from "$lib/api/prowl";
  import type { Harness } from "$lib/pages/skills/harnesses";
  import Button from "$lib/ui/Button.svelte";
  import Empty from "$lib/ui/Empty.svelte";
  import Lamp from "$lib/ui/Lamp.svelte";
  import ChatAgentPicker from "./ChatAgentPicker.svelte";
  import HarnessCard from "./HarnessCard.svelte";
  import type { ChatAgentInfo } from "./harnesses";
  import { mergeHarnesses } from "./harnesses";
  import type { HarnessesResponse } from "../types";

  interface Props { params: string[] }
  let { params }: Props = $props();

  let ledger = $state<Harness[]>([]);
  let setup = $state<HarnessesResponse | null>(null);
  let chatAgents = $state<ChatAgentInfo[]>([]);
  let loading = $state(true);
  let error = $state("");
  let chatError = $state("");
  let actionError = $state("");
  let notice = $state("");
  let noticePending = $state(false);
  let noticeProviders = $state(false);
  let busy = $state<Record<string, string>>({});
  let switching = $state(false);

  const harnesses = $derived(mergeHarnesses(ledger, setup?.harnesses ?? []));
  const pickerAgents = $derived(chatAgents);
  const installed = $derived(harnesses.filter((row) => row.installed).length);
  const active = $derived(harnesses.filter((row) => row.active).length);
  const pending = $derived(harnesses.filter((row) => row.pending).length);

  function setBusy(id: string, action: string) {
    busy = { ...busy, [id]: action };
  }

  function clearBusy(id: string) {
    const next = { ...busy };
    delete next[id];
    busy = next;
  }

  async function load() {
    loading = harnesses.length === 0;
    error = "";
    const [ledgerResult, setupResult, chatResult] = await Promise.allSettled([
      api.harnesses(),
      prowl.setup.harnesses(),
      api.chatAgents(),
    ] as const);

    if (ledgerResult.status === "fulfilled") {
      ledger = ((ledgerResult.value as { harnesses?: Harness[] }).harnesses ?? []);
    }
    if (setupResult.status === "fulfilled") setup = setupResult.value;
    if (chatResult.status === "fulfilled") {
      chatAgents = chatResult.value as unknown as ChatAgentInfo[];
      chatError = "";
    } else {
      chatError = "Rashin did not return its chat harnesses. Check the daemon, then retry.";
    }

    const primaryFailure = ledgerResult.status === "rejected" ? ledgerResult.reason
      : setupResult.status === "rejected" ? setupResult.reason
      : null;
    if (primaryFailure) {
      error = primaryFailure instanceof Error
        ? `${primaryFailure.message} Check the Rashin and Prowl services, then retry.`
        : "Harness setup could not be read. Check the Rashin and Prowl services, then retry.";
    }
    loading = false;
  }

  async function connect(id: string) {
    if (busy[id]) return;
    setBusy(id, "connect");
    actionError = "";
    notice = "";
    try {
      const result = await api.wire(id);
      noticePending = Boolean(result.pending);
      noticeProviders = Boolean(result.pending);
      notice = result.pending
        ? result.reason || "The harness is connected, but Prowl needs an available provider before it can route."
        : "Harness connected and routed through Prowl.";
      await load();
    } catch (reason) {
      actionError = reason instanceof Error
        ? `${reason.message} Check the harness configuration, then retry.`
        : "The harness was not connected. Check its configuration, then retry.";
    } finally {
      clearBusy(id);
    }
  }

  async function disconnect(id: string) {
    if (busy[id]) return;
    setBusy(id, "disconnect");
    actionError = "";
    notice = "";
    try {
      await api.unwire(id);
      noticePending = false;
      noticeProviders = false;
      notice = "Harness disconnected. Its previous model was restored where the setting was still Prowl-managed.";
      await load();
    } catch (reason) {
      actionError = reason instanceof Error
        ? `${reason.message} Check the harness configuration, then retry.`
        : "The harness was not disconnected. Check its configuration, then retry.";
    } finally {
      clearBusy(id);
    }
  }

  async function routeNow(id: string) {
    if (busy[id]) return;
    setBusy(id, "route");
    actionError = "";
    notice = "";
    try {
      const result = await api.routeHarnesses();
      const stillPending = result.pending.find((row) => row.id === id);
      noticePending = Boolean(stillPending);
      noticeProviders = Boolean(stillPending);
      notice = stillPending
        ? stillPending.reason
        : result.routed.includes(id)
          ? "Harness is now routed through Prowl."
          : "Connected harnesses were checked. This harness did not need a routing change.";
      await load();
    } catch (reason) {
      actionError = reason instanceof Error
        ? `${reason.message} Open Providers and make the active set routable, then retry.`
        : "The harness is still pending. Open Providers and make the active set routable, then retry.";
    } finally {
      clearBusy(id);
    }
  }

  async function installSkills(id: string) {
    if (busy[id]) return;
    setBusy(id, "skills");
    actionError = "";
    notice = "";
    try {
      const result = await prowl.setup.installSkills({ clients: [id] });
      noticePending = result.conflicts > 0;
      noticeProviders = false;
      notice = result.conflicts > 0
        ? `${result.message}. Conflicting files were left untouched; review them before retrying.`
        : result.message;
      await load();
    } catch (reason) {
      actionError = reason instanceof Error
        ? `${reason.message} Resolve any edited skill files, then retry.`
        : "Skills were not installed. Resolve any edited skill files, then retry.";
    } finally {
      clearBusy(id);
    }
  }

  async function switchAgent(id: string) {
    if (switching) return;
    switching = true;
    chatError = "";
    try {
      const result = await api.setChatAgent(id) as unknown as {
        agents: ChatAgentInfo[];
        routing: { active: boolean; pending: boolean; reason?: string };
      };
      chatAgents = result.agents;
      return result;
    } catch (reason) {
      chatError = reason instanceof Error
        ? `${reason.message} Keep the harness connected, then retry.`
        : "The chat agent did not change. Keep the harness connected, then retry.";
    } finally {
      switching = false;
    }
  }

  $effect(() => {
    void params.join("/");
    untrack(() => { void load(); });
  });
</script>

<Page title="Harnesses" gloss="接続" lead="Connect local coding agents to Prowl and choose the one Rashin uses for chat.">
  {#if loading}
    <Empty title="Inspecting coding harnesses" body="Rashin is matching installed agents with Prowl's routing and skills ledger." />
  {:else if error && harnesses.length === 0}
    <Empty title="Harness setup is unavailable" body={error}>
      {#snippet action()}<Button variant="plate" onclick={() => void load()}>Retry</Button>{/snippet}
    </Empty>
  {:else}
    <div class="harnesses-page">
      {#if setup && !setup.routable}
        <div class="route-banner">
          <Lamp state="bad" />
          <div><strong>Prowl cannot route yet</strong><p>{setup.reason || "Connect a provider and add an available model to the active set."}</p></div>
          <a href="#/prowl/providers">Open Providers</a>
        </div>
      {/if}
      {#if error}<p class="action-error" role="alert">{error}</p>{/if}
      {#if setup?.skillsError}<p class="action-error" role="alert">Skills could not be checked: {setup.skillsError}. Resolve the Prowl skills error, then reload.</p>{/if}
      {#if actionError}<p class="action-error" role="alert">{actionError}</p>{/if}
      {#if notice}
        <p class:pending={noticePending} class="notice" aria-live="polite">
          {notice}
          {#if noticeProviders}<a href="#/prowl/providers">Open Providers</a>{/if}
        </p>
      {/if}

      <div class="summary">
        <div><strong>{installed}</strong><span>detected</span></div>
        <div><strong>{active}</strong><span>routed</span></div>
        <div><strong>{pending}</strong><span>pending</span></div>
        <p>Connect writes only Prowl and Rashin-owned settings. Disconnect restores the previous model.</p>
      </div>

      {#if harnesses.length}
        <div class="harness-grid">
          {#each harnesses as harness (harness.id)}
            <HarnessCard
              {harness}
              routable={setup?.routable ?? false}
              busy={busy[harness.id]}
              onconnect={(id) => void connect(id)}
              ondisconnect={(id) => void disconnect(id)}
              onroute={(id) => void routeNow(id)}
              onskills={(id) => void installSkills(id)}
            />
          {/each}
        </div>
      {:else}
        <Empty title="No supported harnesses found" body="Install a supported coding harness, then reload this page to connect it." />
      {/if}

      <ChatAgentPicker agents={pickerAgents} busy={switching} error={chatError} onswitch={switchAgent} />
    </div>
  {/if}
</Page>

<style>
  .harnesses-page { display: grid; gap: var(--s4); }
  .route-banner { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: var(--s3); padding: var(--s3) var(--s4); border: 1px solid var(--line-strong); border-radius: var(--radius); }
  .route-banner strong { color: var(--ink); font-weight: 500; }
  .route-banner p { margin-top: var(--s1); color: var(--ink-mute); font-size: var(--f-small); }
  .route-banner a, .notice a { color: var(--ink); font-size: var(--f-small); text-decoration: underline; text-underline-offset: 3px; }
  .action-error, .notice { padding: var(--s3); border: 1px solid color-mix(in srgb, var(--alert) 45%, transparent); border-radius: var(--radius); color: var(--alert); font-size: var(--f-small); }
  .notice { border-color: var(--line-soft); color: var(--ink-dim); }
  .notice.pending { border-color: color-mix(in srgb, var(--alert) 45%, transparent); color: var(--alert); }
  .notice a { margin-left: var(--s2); }
  .summary { display: grid; grid-template-columns: auto auto auto minmax(0, 1fr); align-items: center; gap: var(--s5); padding: var(--s4) var(--s5); border: 1px solid var(--line-soft); border-radius: var(--radius); }
  .summary > div { display: flex; align-items: baseline; gap: var(--s2); }
  .summary strong { color: var(--ink); font-family: var(--display); font-size: 32px; font-weight: 400; }
  .summary span, .summary p { color: var(--ink-mute); font-size: var(--f-small); }
  .harness-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 410px), 1fr)); gap: var(--s4); align-items: start; }
  @media (max-width: 720px) {
    .route-banner { grid-template-columns: auto 1fr; }
    .route-banner a { grid-column: 2; }
    .summary { grid-template-columns: repeat(3, auto); }
    .summary p { grid-column: 1 / -1; }
  }
</style>
