<script lang="ts">
  import { onMount } from "svelte";
  import Page from "$lib/app/Page.svelte";
  import { api, type Json } from "$lib/api/client";
  import { prowl } from "$lib/api/prowl";
  import { machine } from "$lib/state/machine.svelte";
  import { theme } from "$lib/state/theme.svelte";
  import { router } from "$lib/app/router.svelte";
  import Hero from "$lib/pages/overview/Hero.svelte";
  import VitalsStrip from "$lib/pages/overview/VitalsStrip.svelte";
  import HealthBand from "$lib/pages/overview/HealthBand.svelte";
  import CodeCard from "$lib/pages/overview/CodeCard.svelte";
  import VaultCard from "$lib/pages/overview/VaultCard.svelte";
  import SystemSummary from "$lib/pages/overview/SystemSummary.svelte";
  import type { SystemInventory } from "$lib/pages/system/types";
  import { normalizeInventory } from "$lib/pages/system/types";
  import type {
    CodeStatus,
    DoctorScan,
    FixAnswer,
    FixRequest,
    FixState,
    ProwlDaemonStatus,
    ProwlHit,
    Vitals,
  } from "$lib/pages/overview/types";
  import { attentionFindings } from "$lib/pages/overview/types";

  let inventory = $state<SystemInventory | null>(null);
  let systemLoading = $state(true);
  let systemError = $state("");
  let doctor = $state<DoctorScan | null>(null);
  let doctorLoading = $state(true);
  let prowlDaemon = $state<ProwlDaemonStatus | null>(null);
  let codeStatus = $state<CodeStatus | null>(null);
  let codeLoading = $state(true);
  let codeError = $state("");
  let hits = $state<ProwlHit[] | null>(null);
  let searching = $state(false);
  let searchError = $state("");
  let fixStates = $state<Record<string, FixState>>({});
  let copied = $state("");
  let reindexing = $state(false);
  let reindexMessage = $state("");
  let reindexFailed = $state(false);
  let fallbackVitals = $state<Vitals | null>(null);
  let streamFresh = $state(true);
  let lastStreamVitals: Vitals | null = null;
  let streamSeenAt = 0;
  let pollingVitals = false;

  const overviewIssues = $derived(attentionFindings(doctor, false));
  const displayedVitals = $derived(streamFresh ? machine.vitals ?? fallbackVitals : fallbackVitals ?? machine.vitals);

  $effect(() => {
    const frame = machine.vitals;
    if (frame && frame !== lastStreamVitals) {
      lastStreamVitals = frame;
      streamSeenAt = Date.now();
      streamFresh = true;
    }
  });

  async function loadFallbackVitals(): Promise<void> {
    if (pollingVitals) return;
    pollingVitals = true;
    try {
      fallbackVitals = (await api.vitals()) as unknown as Vitals;
    } catch {
      // Keep the last good frame while the daemon reconnects.
    } finally {
      pollingVitals = false;
    }
  }

  async function loadSystem(quiet = false): Promise<void> {
    if (!quiet) systemLoading = true;
    try {
      inventory = normalizeInventory((await api.system()) as unknown as SystemInventory);
      systemError = "";
    } catch (error) {
      if (!quiet || !inventory) systemError = error instanceof Error ? error.message : "The daemon is not answering.";
    } finally {
      systemLoading = false;
    }
  }

  async function loadDoctor(refresh = false): Promise<void> {
    doctorLoading = true;
    try {
      doctor = (await api.doctor(refresh)) as unknown as DoctorScan;
    } catch (error) {
      doctor = {
        findings: [],
        collectedAt: new Date().toISOString(),
        error: error instanceof Error ? error.message : "The daemon is not answering.",
      };
    } finally {
      doctorLoading = false;
    }
  }

  async function loadCode(): Promise<void> {
    codeLoading = true;
    codeError = "";
    codeStatus = null;
    try {
      const status = await api.status();
      prowlDaemon = (status as { prowl?: ProwlDaemonStatus }).prowl ?? null;
      if (prowlDaemon?.installed && prowlDaemon.running) {
        codeStatus = await prowl.code.status({ repo: prowlDaemon.repo }) as unknown as CodeStatus;
      }
    } catch (error) {
      codeError = error instanceof Error ? error.message : "The index report could not be read.";
    } finally {
      codeLoading = false;
    }
  }

  async function searchCode(query: string): Promise<void> {
    searching = true;
    searchError = "";
    hits = null;
    try {
      hits = await prowl.code.search({ q: query, repo: prowlDaemon?.repo }) as unknown as ProwlHit[];
    } catch (error) {
      searchError = error instanceof Error ? error.message : "Search did not answer.";
    } finally {
      searching = false;
    }
  }

  async function runFix(key: string, request: FixRequest): Promise<void> {
    fixStates = { ...fixStates, [key]: { phase: "opening", message: "Opening the agent" } };
    try {
      const answer = (await api.fix(request as unknown as Json)) as unknown as FixAnswer;
      const where = answer.harness ? ` with ${answer.harness}` : "";
      fixStates = { ...fixStates, [key]: { phase: "opened", message: `Opened in a terminal${where}` } };
    } catch (error) {
      fixStates = {
        ...fixStates,
        [key]: { phase: "failed", message: error instanceof Error ? error.message : "The agent could not be opened." },
      };
    }
  }

  async function copyRemedy(key: string, text: string): Promise<void> {
    try {
      await navigator.clipboard.writeText(text);
      copied = key;
      window.setTimeout(() => {
        if (copied === key) copied = "";
      }, 1200);
    } catch {
      copied = "";
    }
  }

  async function reindex(): Promise<void> {
    reindexing = true;
    reindexMessage = "";
    reindexFailed = false;
    try {
      await api.reindex();
      await machine.refreshStatus();
      reindexMessage = "Index refreshed";
    } catch (error) {
      reindexFailed = true;
      reindexMessage = error instanceof Error ? error.message : "Reindexing failed.";
    } finally {
      reindexing = false;
    }
  }

  onMount(() => {
    void loadSystem();
    void loadDoctor();
    void loadCode();
    const systemTimer = window.setInterval(() => {
      if (!document.hidden) void loadSystem(true);
    }, 30000);
    const vitalsTimer = window.setInterval(() => {
      streamFresh = !!streamSeenAt && Date.now() - streamSeenAt < 5000;
      if (!streamFresh && !document.hidden) void loadFallbackVitals();
    }, 2000);
    if (!machine.vitals) void loadFallbackVitals();
    return () => {
      clearInterval(systemTimer);
      clearInterval(vitalsTimer);
    };
  });
</script>

<Page title="Overview" gloss="概要" bare>
  <div class="overview">
    <Hero wallpaper={theme.wallpaper} vitals={displayedVitals} />
    <VitalsStrip vitals={displayedVitals} system={inventory} />

    <div class="body">
      <HealthBand
        scan={doctor}
        issues={overviewIssues}
        loading={doctorLoading}
        states={fixStates}
        {copied}
        onrefresh={() => void loadDoctor(true)}
        onfix={(key, request) => void runFix(key, request)}
        oncopy={(key, text) => void copyRemedy(key, text)}
        onopenDoctor={() => router.go("system", "doctor")}
      />

      <div class="instruments">
        <div class="code">
          <CodeCard
            daemon={prowlDaemon}
            status={codeStatus}
            loading={codeLoading}
            error={codeError}
            {hits}
            {searching}
            {searchError}
            onsearch={(query) => void searchCode(query)}
          />
        </div>
        <div class="side">
          <SystemSummary
            {inventory}
            loading={systemLoading}
            error={systemError}
            onopen={() => router.go("system")}
            onrefresh={() => void loadSystem()}
          />
          <VaultCard
            status={machine.status}
            {reindexing}
            message={reindexMessage}
            failed={reindexFailed}
            onreindex={() => void reindex()}
          />
        </div>
      </div>
    </div>
  </div>
</Page>

<style>
  .overview { height: 100%; overflow: auto; }
  .body { display: flex; flex-direction: column; gap: var(--s5); padding: var(--s5) var(--s7) var(--s6); }
  .instruments { display: grid; grid-template-columns: minmax(0, 1.45fr) minmax(300px, .75fr); gap: var(--s4); align-items: start; }
  .code { min-width: 0; align-self: stretch; }
  .side { display: flex; flex-direction: column; gap: var(--s4); min-width: 0; }
  @media (max-width: 980px) {
    .instruments { grid-template-columns: 1fr; }
    .side { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 300px), 1fr)); }
  }
  @media (max-width: 680px) { .body { padding-inline: var(--s5); } }
</style>
