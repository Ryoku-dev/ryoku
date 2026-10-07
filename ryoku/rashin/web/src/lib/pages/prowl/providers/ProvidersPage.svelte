<script lang="ts">
  import { untrack } from "svelte";
  import Page from "$lib/app/Page.svelte";
  import { ApiError, api } from "$lib/api/client";
  import { prowl } from "$lib/api/prowl";
  import Button from "$lib/ui/Button.svelte";
  import Card from "$lib/ui/Card.svelte";
  import Chip from "$lib/ui/Chip.svelte";
  import Dialog from "$lib/ui/Dialog.svelte";
  import Empty from "$lib/ui/Empty.svelte";
  import Field from "$lib/ui/Field.svelte";
  import Lamp from "$lib/ui/Lamp.svelte";
  import { formatTokens, parseInstant, relativeTime } from "../format";
  import type {
    AccountUsage,
    CustomModelInput,
    CustomProbeResponse,
    DirectoryResponse,
    DiscoverModelsResponse,
    HealthResponse,
    ImportPreviewResponse,
    ImportSelectedKey,
    KeyActivity,
    KeyView,
    LoginRow,
    ProviderProbeResponse,
    SignInPlatform,
    SignInSession,
  } from "../types";
  import ConnectProviderDialog from "./ConnectProviderDialog.svelte";
  import CustomEndpointDialog from "./CustomEndpointDialog.svelte";
  import ImportKeysDialog from "./ImportKeysDialog.svelte";
  import ProviderManageDialog from "./ProviderManageDialog.svelte";
  import SignInDialog from "./SignInDialog.svelte";
  import SubscriptionAccounts from "./SubscriptionAccounts.svelte";
  import {
    activityByKey,
    deriveKeylessState,
    filterProviders,
    groupProviders,
    providerTier,
    type ProviderBundle,
    type ProviderFilter,
  } from "./providers";

  interface Props {
    params: string[];
  }

  interface Confirmation {
    title: string;
    description: string;
    verb: string;
    run: () => Promise<void>;
  }

  const FILTERS: Array<{ value: ProviderFilter; label: string }> = [
    { value: "all", label: "All" },
    { value: "free", label: "Free" },
    { value: "credits", label: "Credits" },
    { value: "paid", label: "Paid" },
    { value: "keyless", label: "Keyless" },
    { value: "sign-in", label: "Sign-in" },
  ];

  let { params }: Props = $props();
  let directory = $state<DirectoryResponse | null>(null);
  let keys = $state<KeyView[]>([]);
  let activity = $state<KeyActivity[]>([]);
  let health = $state<HealthResponse | null>(null);
  let logins = $state<LoginRow[]>([]);
  let accounts = $state<AccountUsage[]>([]);
  let signInPlatforms = $state<SignInPlatform[]>([]);
  let loading = $state(true);
  let starting = $state(false);
  let error = $state("");
  let actionError = $state("");
  let notice = $state("");
  let busy = $state("");
  let retryTimer: number | undefined;
  let firstLoad = true;
  let lastRouteParam = "";

  let query = $state("");
  let filter = $state<ProviderFilter>("all");
  let connectOpen = $state(false);
  let connectBundle = $state<ProviderBundle | null>(null);
  let connectError = $state("");
  let manageOpen = $state(false);
  let selectedProviderID = $state("");
  let providerProbe = $state<ProviderProbeResponse | null>(null);
  let probeLoading = $state(false);
  let probeRequest = 0;
  let revealed = $state<Record<number, string>>({});
  const revealTimers = new Map<number, number>();

  let signInOpen = $state(false);
  let signInSession = $state<SignInSession | null>(null);
  let signInPlatform = $state<SignInPlatform | null>(null);
  let signInError = $state("");
  let signInStartedAt = 0;
  let signInTimer: number | undefined;

  let customOpen = $state(false);
  let customDiscovery = $state<DiscoverModelsResponse | null>(null);
  let customProbe = $state<CustomProbeResponse | null>(null);
  let customBusy = $state("");
  let customError = $state("");

  let importOpen = $state(false);
  let importPreview = $state<ImportPreviewResponse | null>(null);
  let importBusy = $state("");
  let importError = $state("");

  let confirmation = $state<Confirmation | null>(null);
  let confirmOpen = $state(false);
  let confirmBusy = $state(false);

  const groups = $derived(groupProviders(directory?.providers ?? [], keys, signInPlatforms, logins));
  const available = $derived(filterProviders(groups.available, query, filter));
  const activityMap = $derived(activityByKey(activity));
  const healthByKey = $derived.by(() => {
    const byKey = new Map<number, HealthResponse["keys"][number]>();
    for (const key of health?.keys ?? []) byKey.set(key.id, key);
    return byKey;
  });
  const selectedBundle = $derived(
    groups.connected.find((row) => row.provider.id === selectedProviderID || row.platform === selectedProviderID)
      ?? groups.available.find((row) => row.provider.id === selectedProviderID || row.platform === selectedProviderID)
      ?? null,
  );

  function errorText(reason: unknown, fallback: string): string {
    return reason instanceof Error ? `${reason.message} ${fallback}` : fallback;
  }

  function routeParam(): string {
    if (!params[0]) return "";
    try {
      return decodeURIComponent(params[0]);
    } catch {
      return params[0];
    }
  }

  async function load(showLoading = false) {
    clearTimeout(retryTimer);
    loading = showLoading;
    starting = false;
    error = "";

    const results = await Promise.allSettled([
      prowl.providers.directory(),
      prowl.keys.list(),
      prowl.keys.activity(),
      prowl.health.read(),
      prowl.logins.list(),
      prowl.logins.usage(),
      prowl.logins.platforms(),
    ] as const);
    const [directoryResult, keysResult, activityResult, healthResult, loginsResult, usageResult, platformsResult] = results;
    if (directoryResult.status === "fulfilled") directory = directoryResult.value;
    if (keysResult.status === "fulfilled") keys = keysResult.value;
    if (activityResult.status === "fulfilled") activity = activityResult.value.activity;
    if (healthResult.status === "fulfilled") health = healthResult.value;
    if (loginsResult.status === "fulfilled") logins = loginsResult.value.logins;
    if (usageResult.status === "fulfilled") accounts = usageResult.value.accounts;
    if (platformsResult.status === "fulfilled") signInPlatforms = platformsResult.value.platforms;

    const failures = results.filter((result): result is PromiseRejectedResult => result.status === "rejected");
    if (failures.some((result) => result.reason instanceof ApiError && result.reason.code === "gateway_down")) {
      starting = true;
      error = "Rashin is starting Prowl. This page will retry automatically.";
      retryTimer = window.setTimeout(() => void load(), 2_000);
    } else if (failures[0]) {
      error = errorText(failures[0].reason, "Check Prowl's gateway, then retry.");
    }
    const target = routeParam();
    if (target && manageOpen && providerProbe === null && !probeLoading) {
      const loadedGroups = groupProviders(directory?.providers ?? [], keys, signInPlatforms, logins);
      const bundle = loadedGroups.connected.find((row) => row.provider.id === target || row.platform === target)
        ?? loadedGroups.available.find((row) => row.provider.id === target || row.platform === target);
      if (bundle?.connected && !bundle.provider.id.startsWith("custom:")) void probeProvider(bundle);
    }
    loading = false;
  }

  async function announceRouting(prefix: string) {
    try {
      const result = await api.routeHarnesses();
      const routed = result.routed.length
        ? ` Rashin now routes ${result.routed.join(", ")} through Prowl.`
        : " No harnesses were waiting to route through Prowl.";
      const pending = result.pending.length
        ? ` Still pending: ${result.pending.map((row) => `${row.id} (${row.reason})`).join(", ")}.`
        : "";
      notice = `${prefix}${routed}${pending}`;
    } catch (reason) {
      notice = `${prefix} The provider is ready, but Rashin could not check waiting harnesses. ${reason instanceof Error ? reason.message : "Retry from Harnesses."}`;
    }
  }

  function openConnect(bundle: ProviderBundle) {
    connectBundle = bundle;
    connectError = "";
    connectOpen = true;
  }

  async function connectKey(apiKey: string, label: string) {
    const bundle = connectBundle;
    if (!bundle) return;
    busy = "connect";
    connectError = "";
    try {
      if (bundle.provider.keyless) {
        for (const replacement of bundle.keys) await prowl.keys.remove(replacement.id);
      }
      await prowl.keys.add({ platform: bundle.platform, key: apiKey, label: label || undefined });
      connectOpen = false;
      await announceRouting(apiKey ? `${bundle.provider.name} key added.` : `${bundle.provider.name} free tier enabled.`);
      await load();
    } catch (reason) {
      connectError = errorText(reason, apiKey ? "The key was not added. Check it and retry." : "The free tier was not enabled. Retry from this dialog.");
    } finally {
      busy = "";
    }
  }

  function clearSignInTimer() {
    clearTimeout(signInTimer);
    signInTimer = undefined;
  }

  async function startSignIn(platform: SignInPlatform) {
    clearSignInTimer();
    busy = `signin:${platform.id}`;
    signInError = "";
    signInPlatform = platform;
    try {
      signInSession = await prowl.logins.signin.start({ provider: platform.id });
      signInStartedAt = Date.now();
      signInOpen = true;
      signInTimer = window.setTimeout(() => void pollSignIn(), 2_000);
    } catch (reason) {
      signInError = errorText(reason, "The sign-in flow did not start. Check Prowl, then retry.");
      actionError = signInError;
    } finally {
      busy = "";
    }
  }

  async function pollSignIn() {
    const active = signInSession;
    if (!active) return;
    try {
      const session = await prowl.logins.signin.status(active.id);
      if (!signInSession || signInSession.id !== session.id) return;
      signInError = "";
      signInSession = session;
      if (session.state === "complete") {
        clearSignInTimer();
        busy = `enroll:${session.provider}`;
        try {
          await prowl.logins.enroll(session.provider);
          signInSession = null;
          signInOpen = false;
          await announceRouting(`${signInPlatform?.name || session.provider} signed in and enrolled.`);
          await load();
        } catch (reason) {
          signInSession = null;
          signInOpen = false;
          actionError = errorText(reason, "Sign-in completed, but the account was not enrolled. Use Enroll in Subscription accounts to retry.");
          await load();
        } finally {
          busy = "";
        }
        return;
      }
      if (session.state === "failed" || session.state === "cancelled") {
        clearSignInTimer();
        signInError = session.error || `Sign-in ${session.state}. Start it again when you are ready.`;
        return;
      }
      if (Date.now() - signInStartedAt > 15 * 60_000) {
        clearSignInTimer();
        signInError = "Sign-in timed out. Cancel it and start again.";
        return;
      }
    } catch (reason) {
      signInError = errorText(reason, "Prowl will retry this sign-in check automatically.");
      if (Date.now() - signInStartedAt > 15 * 60_000) {
        clearSignInTimer();
        signInError = errorText(reason, "Sign-in timed out. Cancel it and start again.");
        return;
      }
    }
    signInTimer = window.setTimeout(() => void pollSignIn(), 2_000);
  }

  async function cancelSignIn() {
    const active = signInSession;
    clearSignInTimer();
    signInSession = null;
    signInOpen = false;
    if (!active) return;
    busy = `signin:${active.provider}`;
    try {
      await prowl.logins.signin.cancel(active.id);
      notice = "Sign-in cancelled. No account or credential was changed.";
    } catch (reason) {
      actionError = errorText(reason, "Prowl could not cancel the sign-in. It will expire automatically.");
    } finally {
      busy = "";
    }
  }

  async function enrollLogin(login: LoginRow) {
    busy = `enroll:${login.id}`;
    actionError = "";
    try {
      const result = await prowl.logins.enroll(login.id);
      await announceRouting(`${login.name} enrolled with ${result.models ?? login.offered} models.`);
      await load();
    } catch (reason) {
      actionError = errorText(reason, "The account was not enrolled. Sign in again, then retry.");
    } finally {
      busy = "";
    }
  }

  async function withdrawLogin(login: LoginRow) {
    busy = `withdraw:${login.id}`;
    actionError = "";
    try {
      await prowl.logins.withdraw(login.id);
      notice = `${login.name} withdrew from routing. Its browser login remains stored.`;
      await load();
    } catch (reason) {
      actionError = errorText(reason, "The account is still routing. Retry the withdrawal.");
    } finally {
      busy = "";
    }
  }

  function askConfirmation(value: Confirmation) {
    confirmation = value;
    confirmOpen = true;
  }

  function confirmForget(login: LoginRow) {
    askConfirmation({
      title: `Forget ${login.name}`,
      description: "Its models leave routing. Your actual account is untouched; only Prowl's stored login is deleted.",
      verb: "Forget login",
      run: async () => {
        await prowl.logins.forget(login.id);
        notice = `${login.name} login forgotten.`;
        await load();
      },
    });
  }

  async function runConfirmation() {
    if (!confirmation) return;
    confirmBusy = true;
    actionError = "";
    try {
      await confirmation.run();
      confirmOpen = false;
      confirmation = null;
    } catch (reason) {
      actionError = errorText(reason, "Nothing was removed. Check Prowl, then retry.");
    } finally {
      confirmBusy = false;
    }
  }

  function openManage(bundle: ProviderBundle) {
    lastRouteParam = bundle.provider.id;
    selectedProviderID = bundle.provider.id;
    providerProbe = null;
    manageOpen = true;
    window.location.hash = `#/prowl/providers/${encodeURIComponent(bundle.provider.id)}`;
    if (bundle.connected && !bundle.provider.id.startsWith("custom:")) void probeProvider(bundle);
  }

  function closeManage() {
    clearReveals();
    probeRequest += 1;
    probeLoading = false;
    manageOpen = false;
    lastRouteParam = "";
    if (window.location.hash.startsWith("#/prowl/providers/")) window.location.hash = "#/prowl/providers";
  }

  async function probeProvider(bundle = selectedBundle) {
    if (!bundle || bundle.provider.id.startsWith("custom:")) return;
    const request = ++probeRequest;
    probeLoading = true;
    actionError = "";
    try {
      const response = await prowl.providers.probe(bundle.provider.id);
      if (request === probeRequest) providerProbe = response;
    } catch (reason) {
      if (request === probeRequest) actionError = errorText(reason, "Prowl could not read this provider's quota. Routing may still work.");
    } finally {
      if (request === probeRequest) probeLoading = false;
    }
  }

  async function checkKey(key: KeyView) {
    busy = `Checking ${key.label || key.maskedKey}`;
    actionError = "";
    try {
      const result = await prowl.health.check(key.id);
      notice = `${key.platform} health check finished: ${result.status}.`;
      await load();
    } catch (reason) {
      actionError = errorText(reason, "The health check did not finish. Retry from the provider panel.");
    } finally {
      busy = "";
    }
  }

  async function checkAll() {
    busy = "check-all";
    actionError = "";
    try {
      await prowl.health.checkAll();
      notice = "Every connected key was checked. Health statuses are current.";
      await load();
    } catch (reason) {
      actionError = errorText(reason, "Prowl could not check every key. Retry when the gateway is ready.");
    } finally {
      busy = "";
    }
  }

  async function toggleKey(key: KeyView) {
    busy = `${key.enabled ? "Pausing" : "Resuming"} ${key.id}`;
    actionError = "";
    try {
      await prowl.keys.update(key.id, { enabled: !key.enabled });
      notice = `${key.label || key.maskedKey} ${key.enabled ? "paused" : "resumed"}.`;
      await load();
    } catch (reason) {
      actionError = errorText(reason, `The key is still ${key.enabled ? "enabled" : "paused"}. Retry from the provider panel.`);
    } finally {
      busy = "";
    }
  }

  async function toggleProvider(bundle: ProviderBundle) {
    if (bundle.signIn?.signed_in) {
      if (bundle.login) {
        if (bundle.login.enrolled) await withdrawLogin(bundle.login);
        else await enrollLogin(bundle.login);
        return;
      }
      busy = `enroll:${bundle.signIn.id}`;
      actionError = "";
      try {
        const result = await prowl.logins.enroll(bundle.signIn.id);
        await announceRouting(`${bundle.provider.name} enrolled with ${result.models ?? bundle.provider.modelCount} models.`);
        await load();
      } catch (reason) {
        actionError = errorText(reason, "The signed-in account was not enrolled. Sign in again, then retry.");
      } finally {
        busy = "";
      }
      return;
    }
    const enabled = bundle.keys.some((key) => key.enabled);
    busy = `${enabled ? "pause" : "resume"}:${bundle.platform}`;
    actionError = "";
    try {
      await Promise.all(bundle.keys.filter((key) => key.enabled === enabled).map((key) => prowl.keys.update(key.id, { enabled: !enabled })));
      notice = `${bundle.provider.name} ${enabled ? "paused; its models left routing" : "resumed"}.`;
      await load();
    } catch (reason) {
      actionError = errorText(reason, `Some ${bundle.provider.name} keys may not have changed. Open Manage to check each key.`);
      await load();
    } finally {
      busy = "";
    }
  }

  function removeKey(key: KeyView) {
    askConfirmation({
      title: `Remove ${key.label || key.maskedKey}`,
      description: "Routing stops using this credential immediately. Other keys on the provider stay connected.",
      verb: "Remove key",
      run: async () => {
        await prowl.keys.remove(key.id);
        clearReveal(key.id);
        notice = `${key.platform} key removed.`;
        await load();
      },
    });
  }

  function disconnectProvider(bundle: ProviderBundle) {
    const loginKeyID = bundle.login?.keyId;
    askConfirmation({
      title: `Disconnect ${bundle.provider.name}`,
      description: bundle.signIn?.signed_in
        ? "The stored login and pasted keys are removed from Prowl. Your actual account is untouched."
        : bundle.provider.keyless
          ? "Its free-tier or keyed connection leaves routing immediately."
          : `Every stored key (${bundle.keys.length}) is removed and its models leave routing.`,
      verb: "Disconnect",
      run: async () => {
        if (bundle.signIn?.signed_in) await prowl.logins.forget(bundle.signIn.id);
        for (const key of bundle.keys) {
          if (key.id !== loginKeyID) await prowl.keys.remove(key.id);
        }
        notice = `${bundle.provider.name} disconnected.`;
        if (selectedProviderID === bundle.provider.id) closeManage();
        await load();
      },
    });
  }

  async function revealKey(key: KeyView) {
    actionError = "";
    try {
      const result = await prowl.keys.reveal(key.id);
      clearReveal(key.id);
      revealed = { ...revealed, [key.id]: result.key };
      revealTimers.set(key.id, window.setTimeout(() => clearReveal(key.id), 30_000));
    } catch (reason) {
      actionError = errorText(reason, "The key stayed masked. Retry from the provider panel.");
    }
  }

  function clearReveal(id: number) {
    const timer = revealTimers.get(id);
    if (timer !== undefined) clearTimeout(timer);
    revealTimers.delete(id);
    const next = { ...revealed };
    delete next[id];
    revealed = next;
  }

  function clearReveals() {
    for (const timer of revealTimers.values()) clearTimeout(timer);
    revealTimers.clear();
    revealed = {};
  }

  async function clearCooldowns(key: KeyView) {
    busy = "Clearing cooldowns";
    actionError = "";
    try {
      const result = await prowl.keys.clearCooldowns(key.id);
      notice = `${result.cleared} cooldown${result.cleared === 1 ? "" : "s"} cleared for ${key.label || key.maskedKey}.`;
      await load();
    } catch (reason) {
      actionError = errorText(reason, "Cooldowns were not cleared. Retry from the provider panel.");
    } finally {
      busy = "";
    }
  }

  async function renameKey(key: KeyView, label: string) {
    busy = `Renaming ${key.id}`;
    actionError = "";
    try {
      await prowl.keys.update(key.id, { label });
      notice = label ? `Key renamed to ${label}.` : "Key label cleared.";
      await load();
    } catch (reason) {
      actionError = errorText(reason, "The key label did not change. Retry from the provider panel.");
    } finally {
      busy = "";
    }
  }

  function resetCustom() {
    customDiscovery = null;
    customProbe = null;
    customError = "";
  }

  async function discoverCustom(baseUrl: string, apiKey: string) {
    customBusy = "discover";
    customError = "";
    customDiscovery = null;
    customProbe = null;
    try {
      customDiscovery = await prowl.keys.custom.discoverModels({ baseUrl, apiKey });
    } catch (reason) {
      customError = errorText(reason, "No models were discovered. Check the base URL and key, then retry.");
    } finally {
      customBusy = "";
    }
  }

  async function probeCustom(baseUrl: string, apiKey: string) {
    customBusy = "probe";
    customProbe = null;
    customError = "";
    try {
      customProbe = await prowl.keys.custom.probe({ baseUrl, apiKey });
    } catch (reason) {
      customError = errorText(reason, "The endpoint did not answer the probe. Check its URL, key, and model access.");
    } finally {
      customBusy = "";
    }
  }

  async function addCustom(baseUrl: string, apiKey: string, label: string, models: CustomModelInput[]) {
    customBusy = "add";
    customError = "";
    try {
      const result = await prowl.keys.custom.add({ baseUrl, apiKey, label: label || undefined, models });
      customOpen = false;
      await announceRouting(`Custom endpoint added with ${result.models?.length ?? 0} models.`);
      await load();
    } catch (reason) {
      customError = errorText(reason, "The endpoint was not added. Probe it again, then retry.");
    } finally {
      customBusy = "";
    }
  }

  function resetImport() {
    importPreview = null;
    importError = "";
  }

  async function previewImport(files: File[]) {
    importBusy = "preview";
    importPreview = null;
    importError = "";
    const form = new FormData();
    for (const file of files) form.append("files", file, file.name);
    try {
      importPreview = await prowl.keys.import.preview(form);
    } catch (reason) {
      importError = errorText(reason, "Prowl could not read the export. Choose another file or check its format.");
    } finally {
      importBusy = "";
    }
  }

  async function importSelected(selected: ImportSelectedKey[]) {
    importBusy = "import";
    importError = "";
    try {
      const result = await prowl.keys.import.selected({ keys: selected });
      if (result.imported === 0) {
        importError = result.errors.length
          ? result.errors.map((row) => `${row.key}: ${row.error}`).join(" ")
          : "No selected keys were imported. Review the provider choices, then retry.";
        return;
      }
      importOpen = false;
      await announceRouting(`${result.imported} credential${result.imported === 1 ? "" : "s"} imported${result.modelsRegistered ? ` with ${result.modelsRegistered} custom models` : ""}.`);
      if (result.errors.length) actionError = `${result.errors.length} selected row${result.errors.length === 1 ? "" : "s"} could not be imported: ${result.errors.map((row) => row.key).join(", ")}.`;
      await load();
    } catch (reason) {
      importError = errorText(reason, "The selected keys were not imported. Review them, then retry.");
    } finally {
      importBusy = "";
    }
  }

  function coolingText(key: KeyView): string {
    const stats = activityMap.get(key.id);
    const coolingUntil = parseInstant(stats?.coolingUntil);
    if (coolingUntil && coolingUntil.getTime() > Date.now()) return relativeTime(stats?.coolingUntil);
    const expires = key.cooldowns.reduce((latest, cooldown) => Math.max(latest, cooldown.expiresAtMs), 0);
    const expiresAt = parseInstant(expires);
    return expiresAt && expiresAt.getTime() > Date.now() ? relativeTime(expires) : "-";
  }

  $effect(() => {
    const target = routeParam();
    untrack(() => {
      if (target !== lastRouteParam) {
        providerProbe = null;
        probeRequest += 1;
        probeLoading = false;
      }
      lastRouteParam = target;
      selectedProviderID = target;
      manageOpen = Boolean(target);
      void load(firstLoad);
      firstLoad = false;
    });
    return () => {
      clearTimeout(retryTimer);
      clearSignInTimer();
      clearReveals();
    };
  });
</script>

<Page title="Providers" gloss="提供" lead="Connect credentials and subscriptions, then Prowl makes their models available to Rashin and every routed harness.">
  {#snippet tools()}
    <Button size="sm" busy={busy === "check-all"} onclick={() => void checkAll()}>Check all</Button>
    <Button size="sm" onclick={() => (importOpen = true)}>Import keys</Button>
    <Button size="sm" variant="plate" onclick={() => (customOpen = true)}>Custom endpoint</Button>
  {/snippet}

  {#if loading}
    <Empty title="Reading providers" body="Rashin is checking the provider directory, credentials, health, subscriptions, and recent key traffic." />
  {:else}
    <div class="providers-grid">
      {#if error}
        <div class="wide notice-block" class:starting><Lamp state={starting ? "busy" : "bad"} /><span>{error}</span>{#if !starting}<Button size="sm" onclick={() => void load()}>Retry</Button>{/if}</div>
      {/if}
      {#if notice}<div class="wide notice-block"><Lamp state="ok" /><span>{notice}</span><Button size="sm" variant="quiet" onclick={() => (notice = "")}>Dismiss</Button></div>{/if}
      {#if actionError}<p class="wide action-error" role="alert">{actionError}</p>{/if}

      <Card title="Connected" gloss="接続" lead={`${groups.connected.length} provider${groups.connected.length === 1 ? "" : "s"} currently hold a credential or browser login.`} pad={false}>
        {#if groups.connected.length === 0}
          <div class="card-pad"><Empty title="No providers connected" body="Choose a provider from the directory below. Keyless providers can start without an API key." /></div>
        {:else}
          <div class="connected-list">
            {#each groups.connected as bundle (bundle.provider.id)}
              {@const providerState = deriveKeylessState(bundle.provider, bundle.keys)}
              {@const accountConnection = Boolean(bundle.signIn?.signed_in)}
              {@const accountID = bundle.login?.id || bundle.signIn?.id || ""}
              {@const providerEnabled = accountConnection ? Boolean(bundle.login?.enrolled) : bundle.keys.some((key) => key.enabled)}
              {@const providerBusy = accountConnection
                ? busy === `${bundle.login?.enrolled ? "withdraw" : "enroll"}:${accountID}`
                : busy === `${providerEnabled ? "pause" : "resume"}:${bundle.platform}`}
              {@const modalities = (bundle.provider.modalities ?? []).filter(Boolean)}
              <article class="connected-provider">
                <header class="provider-head">
                  <div>
                    <div class="provider-title"><h3>{bundle.provider.name}</h3><Chip tone={providerEnabled ? "plate" : "quiet"}>{providerEnabled ? "Routing" : "Paused"}</Chip><Chip>{providerState === "free-tier" ? "Free tier" : providerTier(bundle.provider)}</Chip></div>
                    <p>{bundle.login?.enrolled ? bundle.login.models : bundle.provider.modelCount} models{#if modalities.length} · {modalities.join(", ")}{/if}</p>
                  </div>
                  <div class="provider-actions">
                    <Button size="sm" variant="plate" onclick={() => openManage(bundle)}>Manage</Button>
                    <Button size="sm" busy={providerBusy} onclick={() => void toggleProvider(bundle)}>{providerEnabled ? "Pause" : "Resume"}</Button>
                    {#if bundle.provider.adapter}<Button size="sm" onclick={() => openConnect(bundle)}>Add key</Button>{/if}
                    <Button size="sm" variant="quiet" onclick={() => disconnectProvider(bundle)}>Disconnect</Button>
                  </div>
                </header>

                {#if bundle.keys.length}
                  <div class="key-table-wrap">
                    <table class="data keys-table">
                      <thead><tr><th>Key</th><th>Health</th><th>State</th><th class="r">Served</th><th class="r">Failed</th><th class="r">Tokens</th><th class="error-col">Error / cooldown</th></tr></thead>
                      <tbody>
                        {#each bundle.keys as key (key.id)}
                          {@const stats = activityMap.get(key.id)}
                          {@const healthKey = healthByKey.get(key.id)}
                          {@const lastError = stats?.lastError || healthKey?.lastHealthError || key.lastHealthError}
                          {@const cooling = coolingText(key)}
                          <tr>
                            <td class="ink"><strong>{key.label || "Unlabelled"}</strong><code>{key.maskedKey}</code></td>
                            <td><span class:error-state={(healthKey?.status ?? key.status) === "error"}>{healthKey?.status ?? key.status}</span><small>{relativeTime(healthKey?.lastCheckedAt ?? key.lastCheckedAt)}</small></td>
                            <td>{key.enabled ? "Enabled" : "Paused"}</td>
                            <td class="r">{stats?.served ?? 0}</td>
                            <td class="r">{stats?.failed ?? 0}</td>
                            <td class="r">{formatTokens(stats?.tokens ?? 0)}</td>
                            <td class="last-state error-col" class:error-state={Boolean(lastError)} title={lastError || undefined}>{lastError || "-"}<small>{cooling === "-" ? "" : `Cooling ${cooling}`}</small></td>
                          </tr>
                        {/each}
                      </tbody>
                    </table>
                  </div>
                {:else}
                  <p class="account-only">Connected by browser sign-in. Manage the provider or use Subscription accounts below to change routing.</p>
                {/if}
              </article>
            {/each}
          </div>
        {/if}
      </Card>

      <SubscriptionAccounts
        logins={logins}
        {accounts}
        platforms={signInPlatforms}
        {busy}
        error=""
        onenroll={(login) => void enrollLogin(login)}
        onwithdraw={(login) => void withdrawLogin(login)}
        onforget={confirmForget}
        onsignin={(platform) => void startSignIn(platform)}
      />

      <Card title="Provider directory" gloss="一覧" lead="Free tiers lead the list. Search by provider, modality, or setup note.">
        <div class="directory-tools">
          <Field label="Search providers" type="search" bind:value={query} placeholder="Provider, modality, or note" />
          <div class="filter-chips" aria-label="Filter provider directory">
            {#each FILTERS as option (option.value)}
              <Button size="sm" variant={filter === option.value ? "plate" : "line"} onclick={() => (filter = option.value)}>{option.label}</Button>
            {/each}
          </div>
        </div>

        {#if available.length === 0}
          <Empty title="No providers match" body="Clear the search or choose All to see every provider that is still available to connect.">
            {#snippet action()}<Button size="sm" onclick={() => { query = ""; filter = "all"; }}>Show all providers</Button>{/snippet}
          </Empty>
        {:else}
          <div class="directory-grid">
            {#each available as bundle (bundle.platform)}
              {@const providerSignIn = bundle.signIn}
              <article class="directory-provider">
                <header><div><h3>{bundle.provider.name}</h3><p>{bundle.provider.note || `${(bundle.provider.modalities ?? []).join(", ") || "Model"} provider`}</p></div><Chip>{providerTier(bundle.provider)}</Chip></header>
                <dl>
                  <div><dt>Models</dt><dd>{bundle.provider.modelCount || bundle.provider.freeModels}</dd></div>
                  <div><dt>Free models</dt><dd>{bundle.provider.freeModels}</dd></div>
                  <div><dt>Setup</dt><dd>{bundle.provider.keyless ? "Key optional" : providerSignIn ? "Browser sign-in" : bundle.provider.friction || "API key"}</dd></div>
                </dl>
                <footer>
                  {#if bundle.provider.apiKeyUrl || bundle.provider.docsUrl}<a href={bundle.provider.apiKeyUrl || bundle.provider.docsUrl} target="_blank" rel="noreferrer">Signup page</a>{/if}
                  <Button size="sm" variant="quiet" onclick={() => openManage(bundle)}>Details</Button>
                  {#if !bundle.provider.adapter}
                    <Button size="sm" armed={false}>Adapter unavailable</Button>
                  {:else if providerSignIn}
                    <Button size="sm" variant="plate" busy={busy === `signin:${providerSignIn.id}`} onclick={() => void startSignIn(providerSignIn)}>Sign in</Button>
                  {:else}
                    <Button size="sm" variant="plate" onclick={() => openConnect(bundle)}>{bundle.provider.keyless ? "Connect" : "Add key"}</Button>
                  {/if}
                </footer>
              </article>
            {/each}
          </div>
        {/if}
      </Card>
    </div>
  {/if}
</Page>

<ConnectProviderDialog bind:open={connectOpen} bundle={connectBundle} busy={busy === "connect"} error={connectError} onconnect={(apiKey, label) => void connectKey(apiKey, label)} />
<ProviderManageDialog
  bind:open={manageOpen}
  bundle={selectedBundle}
  activity={activityMap}
  {health}
  probe={providerProbe}
  {probeLoading}
  {revealed}
  {busy}
  error={actionError}
  onclose={closeManage}
  onprobe={() => void probeProvider()}
  oncheck={(key) => void checkKey(key)}
  onreveal={(key) => void revealKey(key)}
  onclearcooldowns={(key) => void clearCooldowns(key)}
  ontoggle={(key) => void toggleKey(key)}
  onremove={removeKey}
  onrename={(key, label) => void renameKey(key, label)}
  onaddkey={() => {
    const bundle = selectedBundle;
    if (!bundle) return;
    closeManage();
    openConnect(bundle);
  }}
  onsignin={() => {
    const platform = selectedBundle?.signIn;
    if (!platform) return;
    closeManage();
    void startSignIn(platform);
  }}
/>
<SignInDialog bind:open={signInOpen} session={signInSession} providerName={signInPlatform?.name} busy={busy.startsWith("signin:")} error={signInError} oncancel={() => void cancelSignIn()} />
<CustomEndpointDialog bind:open={customOpen} discovery={customDiscovery} probe={customProbe} busy={customBusy} error={customError} onreset={resetCustom} ondiscover={(baseUrl, apiKey) => void discoverCustom(baseUrl, apiKey)} onprobe={(baseUrl, apiKey) => void probeCustom(baseUrl, apiKey)} onadd={(baseUrl, apiKey, label, models) => void addCustom(baseUrl, apiKey, label, models)} />
<ImportKeysDialog bind:open={importOpen} preview={importPreview} providers={directory?.providers ?? []} busy={importBusy} error={importError} onreset={resetImport} onpreview={(files) => void previewImport(files)} onimport={(selected) => void importSelected(selected)} />

<Dialog bind:open={confirmOpen} title={confirmation?.title ?? "Confirm action"} description={confirmation?.description}>
  <p class="confirm-copy">This action changes Prowl's stored connection immediately.</p>
  {#snippet footer()}
    <Button variant="quiet" onclick={() => (confirmOpen = false)}>Keep connection</Button>
    <Button variant="plate" busy={confirmBusy} autofocus onclick={() => void runConfirmation()}>{confirmation?.verb ?? "Confirm"}</Button>
  {/snippet}
</Dialog>

<style>
  .providers-grid { display: grid; grid-template-columns: 1fr; gap: var(--s4); align-content: start; }
  .wide { min-width: 0; }
  .notice-block { display: flex; align-items: center; gap: var(--s3); min-height: 44px; padding: var(--s2) var(--s3); border: 1px solid var(--line); border-radius: var(--radius); color: var(--ink-dim); }
  .notice-block span:nth-child(2) { flex: 1; }
  .notice-block.starting { border-style: dashed; }
  .action-error { color: var(--alert); padding: var(--s2) var(--s3); border-left: 1px solid var(--alert); }
  .card-pad { padding: var(--s4) var(--s5) var(--s5); }
  .connected-list { display: flex; flex-direction: column; }
  .connected-provider { padding: var(--s4) var(--s5); border-top: 1px solid var(--line-soft); }
  .connected-provider:first-child { border-top: 0; }
  .provider-head, .provider-title, .provider-actions, .directory-provider header, .directory-provider footer { display: flex; align-items: center; gap: var(--s2); }
  .provider-head { justify-content: space-between; align-items: flex-start; gap: var(--s4); }
  .provider-title h3, .directory-provider h3 { color: var(--ink); font-size: var(--f-row); font-weight: 500; }
  .provider-head p, .directory-provider header p { margin-top: var(--s1); color: var(--ink-mute); font-size: var(--f-small); }
  .provider-actions { flex-wrap: wrap; justify-content: flex-end; }
  .key-table-wrap { overflow-x: auto; margin-top: var(--s3); }
  .keys-table { min-width: 820px; }
  .keys-table th.r, .keys-table td.r { min-width: 72px; padding-right: var(--s3); }
  .keys-table .error-col { min-width: 180px; padding-left: var(--s4); }
  .keys-table strong, .keys-table code, .keys-table small { display: block; }
  .keys-table strong { font-weight: 500; }
  .keys-table code, .keys-table small { margin-top: 2px; color: var(--ink-faint); font-size: var(--f-tiny); }
  .keys-table .error-state { color: var(--alert); }
  .keys-table .last-state { max-width: 280px; overflow-wrap: anywhere; white-space: normal; }
  .keys-table .last-state small { color: var(--ink-mute); }
  .account-only { margin-top: var(--s3); color: var(--ink-mute); font-size: var(--f-small); }
  .directory-tools { display: grid; grid-template-columns: minmax(220px, .7fr) minmax(0, 1.3fr); align-items: end; gap: var(--s4); margin-bottom: var(--s5); }
  .filter-chips { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: var(--s2); }
  .directory-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 300px), 1fr)); gap: var(--s3); }
  .directory-provider { display: flex; flex-direction: column; min-height: 190px; padding: var(--s4); border: 1px solid var(--line-soft); border-radius: var(--radius); }
  .directory-provider header { justify-content: space-between; align-items: flex-start; }
  .directory-provider header p { max-width: 34ch; }
  .directory-provider dl { display: flex; flex-direction: column; margin-top: var(--s4); }
  .directory-provider dl div { display: flex; justify-content: space-between; gap: var(--s3); padding: var(--s1) 0; border-bottom: 1px solid var(--line-soft); font-size: var(--f-small); }
  .directory-provider dt { color: var(--ink-mute); }
  .directory-provider dd { margin: 0; color: var(--ink); text-align: right; }
  .directory-provider footer { justify-content: flex-end; flex-wrap: wrap; margin-top: auto; padding-top: var(--s4); }
  .directory-provider footer a { margin-right: auto; color: var(--ink-dim); font-size: var(--f-small); border-bottom: 1px solid var(--line-strong); }
  .directory-provider footer a:hover { color: var(--ink); border-color: var(--bone); }
  .confirm-copy { color: var(--ink-dim); max-width: var(--measure); }
  @media (max-width: 850px) {
    .provider-head { flex-direction: column; }
    .provider-actions { justify-content: flex-start; }
    .directory-tools { grid-template-columns: 1fr; }
    .filter-chips { justify-content: flex-start; }
  }
</style>
