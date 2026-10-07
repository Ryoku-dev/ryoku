<script lang="ts">
  import { untrack } from "svelte";
  import Page from "$lib/app/Page.svelte";
  import { api, type QuickResponse } from "$lib/api/client";
  import { prowl } from "$lib/api/prowl";
  import Button from "$lib/ui/Button.svelte";
  import Card from "$lib/ui/Card.svelte";
  import Chip from "$lib/ui/Chip.svelte";
  import Dialog from "$lib/ui/Dialog.svelte";
  import Empty from "$lib/ui/Empty.svelte";
  import Field from "$lib/ui/Field.svelte";
  import Select from "$lib/ui/Select.svelte";
  import Switch from "$lib/ui/Switch.svelte";
  import type {
    ActiveProfileResponse,
    ChainPreset,
    ModelRow,
    Profile,
    ProfileModel,
    RoutingState,
  } from "../types";
  import RashinLanesCard from "./RashinLanesCard.svelte";
  import SetEditor from "./SetEditor.svelte";
  import { buildMembershipWrite, countUsableMembers } from "./routing";

  interface Props { params: string[]; }
  type FormMode = "" | "create" | "rename" | "delete";

  const STRATEGIES = [
    { value: "efficient", label: "Efficient", hint: "Right-size each prompt" },
    { value: "smartest", label: "Smartest", hint: "Capability first" },
    { value: "balanced", label: "Balanced", hint: "Blend capability, reliability and speed" },
    { value: "fastest", label: "Fastest", hint: "Observed latency first" },
    { value: "reliable", label: "Most reliable", hint: "Observed success rate first" },
    { value: "priority", label: "Priority", hint: "Use the saved model order" },
    { value: "custom", label: "Custom", hint: "Use the configured custom weights" },
  ];
  const SET_STRATEGIES = [{ value: "", label: "Global default", hint: "Follow the setting below" }, ...STRATEGIES];
  const KEY_STRATEGIES = [
    { value: "auto", label: "Automatic", hint: "Let the router choose" },
    { value: "least-remaining", label: "Most headroom", hint: "Prefer the key with more allowance" },
  ];

  let { params }: Props = $props();
  let profiles = $state<Profile[]>([]);
  let activeProfileId = $state<number | null>(null);
  let presets = $state<ChainPreset[]>([]);
  let models = $state<ModelRow[]>([]);
  let membersByProfile = $state<Record<number, ProfileModel[]>>({});
  let routing = $state<RoutingState | null>(null);
  let quick = $state<QuickResponse | null>(null);
  let order = $state<number[]>([]);
  let savedOrder = $state<number[]>([]);
  let loading = $state(true);
  let saving = $state(false);
  let actionBusy = $state("");
  let quickBusy = $state(false);
  let error = $state("");
  let actionError = $state("");
  let quickError = $state("");
  let formMode = $state<FormMode>("");
  let formOpen = $state(false);
  let formProfile = $state<Profile | null>(null);
  let formName = $state("");
  let libraryQuery = $state("");
  let settingsStrategy = $state("balanced");
  let settingsExplore = $state(false);
  let settingsKeyStrategy = $state("auto");
  let settingsCooldown = $state("");

  const setId = $derived.by(() => {
    if (!params[0]) return null;
    const id = Number(params[0]);
    return Number.isInteger(id) && id > 0 ? id : null;
  });
  const editingProfile = $derived(setId === null ? null : profiles.find((profile) => profile.id === setId) ?? null);
  const editorDirty = $derived(order.join(",") !== savedOrder.join(",")
    || order.some((id) => models.find((model) => model.id === id)?.enabled === false));
  const shownProfiles = $derived.by(() => {
    const needle = libraryQuery.trim().toLowerCase();
    return needle ? profiles.filter((profile) => profile.name.toLowerCase().includes(needle)) : profiles;
  });
  const shownPresets = $derived.by(() => {
    const needle = libraryQuery.trim().toLowerCase();
    return needle ? presets.filter((preset) => [preset.name, preset.description, ...preset.requirements]
      .some((value) => value.toLowerCase().includes(needle))) : presets;
  });

  function errorMessage(reason: unknown, fallback: string): string {
    return reason instanceof Error ? `${reason.message} ${fallback}` : fallback;
  }

  async function load() {
    loading = true;
    error = "";
    actionError = "";
    try {
      void loadQuick();
      const [nextProfiles, active, nextPresets, nextModels, nextRouting] = await Promise.all([
        prowl.routing.profiles(),
        prowl.routing.activeProfile(),
        prowl.routing.presets(),
        prowl.routing.models(),
        prowl.routing.settings(),
      ]);
      const memberLists = await Promise.all(nextProfiles.map(async (profile) => [profile.id, await prowl.routing.profileModels(profile.id)] as const));
      profiles = nextProfiles;
      activeProfileId = (active as ActiveProfileResponse).activeProfileId;
      presets = nextPresets.presets;
      models = nextModels;
      membersByProfile = Object.fromEntries(memberLists);
      routing = nextRouting;
      settingsStrategy = nextRouting.strategy;
      settingsExplore = nextRouting.exploreEnabled;
      settingsKeyStrategy = nextRouting.keySelectionStrategy;
      settingsCooldown = nextRouting.cooldownCeilingMs === null ? "" : String(nextRouting.cooldownCeilingMs);
      if (setId !== null) {
        const nextOrder = (membersByProfile[setId] ?? []).map((member) => member.model_db_id);
        order = nextOrder;
        savedOrder = [...nextOrder];
      }
    } catch (reason) {
      error = errorMessage(reason, "Check that Prowl's gateway is running, then retry.");
    } finally {
      loading = false;
    }
  }

  async function loadQuick() {
    try {
      quick = await api.quick();
      quickError = "";
    } catch (reason) {
      quickError = errorMessage(reason, "Start the Rashin service, then retry.");
    }
  }

  async function setQuick(route: string) {
    quickBusy = true;
    quickError = "";
    try {
      quick = await api.setQuick(route);
    } catch (reason) {
      quickError = errorMessage(reason, "The fast lane did not change. Check its provider and try again.");
    } finally {
      quickBusy = false;
    }
  }

  async function activate(profile: Profile) {
    actionBusy = `activate-${profile.id}`;
    actionError = "";
    try {
      activeProfileId = (await prowl.routing.setActiveProfile(profile.id)).activeProfileId;
      await loadQuick();
    } catch (reason) {
      actionError = errorMessage(reason, "The active set did not change. Retry after checking the gateway.");
    } finally {
      actionBusy = "";
    }
  }

  async function setStrategy(profile: Profile, strategy: string) {
    actionBusy = `strategy-${profile.id}`;
    actionError = "";
    try {
      const updated = await prowl.routing.updateProfile(profile.id, { strategy });
      profiles = profiles.map((row) => row.id === updated.id ? updated : row);
    } catch (reason) {
      actionError = errorMessage(reason, "The set strategy did not save. Retry after checking the gateway.");
    } finally {
      actionBusy = "";
    }
  }

  function openCreate() {
    formOpen = true;
    formMode = "create";
    formProfile = null;
    formName = "";
  }

  function openRename(profile: Profile) {
    formOpen = true;
    formMode = "rename";
    formProfile = profile;
    formName = profile.name;
  }

  function openDelete(profile: Profile) {
    if (profile.name.toLowerCase() === "default") return;
    formOpen = true;
    formMode = "delete";
    formProfile = profile;
    formName = profile.name;
  }

  async function submitForm() {
    actionBusy = formMode;
    actionError = "";
    try {
      if (formMode === "create") {
        const created = await prowl.routing.createProfile({ name: formName, empty: true });
        formOpen = false;
        formMode = "";
        location.hash = `#/prowl/routing/${created.id}`;
        return;
      }
      if (formMode === "rename" && formProfile) {
        const updated = await prowl.routing.updateProfile(formProfile.id, { name: formName });
        profiles = profiles.map((row) => row.id === updated.id ? updated : row);
      }
      if (formMode === "delete" && formProfile) {
        const deletingProfile = formProfile;
        await prowl.routing.deleteProfile(deletingProfile.id);
        profiles = profiles.filter((row) => row.id !== deletingProfile.id);
        const remainingMembers = { ...membersByProfile };
        delete remainingMembers[deletingProfile.id];
        membersByProfile = remainingMembers;
        if (activeProfileId === deletingProfile.id) activeProfileId = null;
      }
      formOpen = false;
      formMode = "";
    } catch (reason) {
      actionError = errorMessage(reason, "Nothing changed. Correct the name or check the gateway, then retry.");
    } finally {
      actionBusy = "";
    }
  }

  async function createPreset(preset: ChainPreset, activateCreated: boolean) {
    actionBusy = `preset-${preset.id}-${activateCreated}`;
    actionError = "";
    try {
      const created = await prowl.routing.createFromPreset(preset.id, { activate: activateCreated });
      if (activateCreated) activeProfileId = created.id;
      await load();
      location.hash = `#/prowl/routing/${created.id}`;
    } catch (reason) {
      actionError = errorMessage(reason, "The preset was not created. Rename the existing set with that name or retry.");
    } finally {
      actionBusy = "";
    }
  }

  async function saveEditor() {
    if (!editingProfile) return;
    saving = true;
    actionError = "";
    try {
      const write = buildMembershipWrite(order, models);
      await Promise.all(write.enableModelIds.map((id) => prowl.routing.updateModel(id, { enabled: true })));
      await prowl.routing.reorderProfile(editingProfile.id, write.entries);
      const [nextMembers, nextModels] = await Promise.all([
        prowl.routing.profileModels(editingProfile.id),
        prowl.routing.models(),
      ]);
      membersByProfile = { ...membersByProfile, [editingProfile.id]: nextMembers };
      models = nextModels;
      order = nextMembers.map((member) => member.model_db_id);
      savedOrder = [...order];
      profiles = profiles.map((profile) => profile.id === editingProfile.id ? { ...profile, modelCount: order.length } : profile);
      await loadQuick();
    } catch (reason) {
      actionError = errorMessage(reason, "The set remains unsaved. Check the gateway and try again.");
    } finally {
      saving = false;
    }
  }

  async function saveGlobalSettings() {
    if (!routing) return;
    saving = true;
    actionError = "";
    const cooldown = settingsCooldown.trim() === "" ? null : Number(settingsCooldown);
    if (cooldown !== null && (!Number.isInteger(cooldown) || cooldown < 60_000 || cooldown > 86_400_000)) {
      actionError = "Cooldown ceiling must be blank or between 60000 and 86400000 milliseconds.";
      saving = false;
      return;
    }
    try {
      await prowl.routing.saveSettings({
        strategy: settingsStrategy,
        exploreEnabled: settingsExplore,
        keySelectionStrategy: settingsKeyStrategy,
        cooldownCeilingMs: cooldown,
      });
      routing = await prowl.routing.settings();
      settingsStrategy = routing.strategy;
      settingsExplore = routing.exploreEnabled;
      settingsKeyStrategy = routing.keySelectionStrategy;
      settingsCooldown = routing.cooldownCeilingMs === null ? "" : String(routing.cooldownCeilingMs);
    } catch (reason) {
      actionError = errorMessage(reason, "The routing settings remain unsaved. Check the values and retry.");
    } finally {
      saving = false;
    }
  }

  $effect(() => {
    const routeKey = params.join("/");
    untrack(() => {
      void routeKey;
      void load();
    });
  });
</script>

{#if setId !== null}
  <SetEditor
    profile={editingProfile}
    {models}
    {order}
    {loading}
    {saving}
    dirty={editorDirty}
    error={actionError || error}
    onchange={(next) => (order = next)}
    onsave={() => void saveEditor()}
    onretry={() => void load()}
  />
{:else}
  <Page title="Routing sets" gloss="経路" lead="Build named model sets, choose how each set routes, and keep Rashin's two lanes on the same gateway.">
    {#snippet tools()}
      <Button icon="refresh" busy={loading} onclick={() => void load()}>Refresh</Button>
      <Button icon="plus" variant="plate" onclick={openCreate}>New empty set</Button>
    {/snippet}
    {#if loading}
      <Empty title="Reading routing sets" body="Prowl is loading saved sets, presets, connected models, and routing policy." />
    {:else if error}
      <div class="notice" role="alert"><span>{error}</span><Button size="sm" onclick={() => void load()}>Retry</Button></div>
    {:else}
      <div class="routing-layout">
        {#if actionError}<p class="action-error" role="alert">{actionError}</p>{/if}
        <div class="library-search"><Field label="Search sets and presets" placeholder="Name or requirement" bind:value={libraryQuery} /></div>

        <Card title="Your sets" gloss="組合" lead="Usable counts include only members on connected providers." pad={false}>
          {#if shownProfiles.length === 0}
            <Empty title={libraryQuery ? "No sets match" : "No routing sets"} body={libraryQuery ? "Clear the search or use another name." : "Create an empty set or start from a preset below."} />
          {:else}
            <div class="set-list">
              {#each shownProfiles as profile (profile.id)}
                {@const counts = countUsableMembers(membersByProfile[profile.id] ?? [], models)}
                {@const active = activeProfileId === profile.id}
                {@const isDefault = profile.name.toLowerCase() === "default"}
                <article class:active class="set-row">
                  <a class="set-name" href={`#/prowl/routing/${profile.id}`}>
                    <strong>{profile.name}</strong>
                    <span>{counts.usable} of {counts.total} usable</span>
                  </a>
                  <span class="active-slot">{#if active}<Chip tone="plate">Active</Chip>{/if}</span>
                  <Select
                    label={`Strategy for ${profile.name}`}
                    options={SET_STRATEGIES}
                    value={profile.strategy}
                    size="sm"
                    disabled={actionBusy === `strategy-${profile.id}`}
                    onchange={(value) => void setStrategy(profile, value)}
                  />
                  <Button size="sm" variant={active ? "quiet" : "line"} armed={!active} busy={actionBusy === `activate-${profile.id}`} onclick={() => void activate(profile)}>Activate</Button>
                  <Button size="sm" variant="quiet" onclick={() => openRename(profile)}>Rename</Button>
                  <Button size="sm" variant="quiet" armed={!isDefault} onclick={() => openDelete(profile)}>Delete</Button>
                </article>
              {/each}
            </div>
          {/if}
        </Card>

        <Card title="Presets" gloss="雛形" lead="Counts are the models each preset can route through right now." pad={false}>
          {#if shownPresets.length === 0}
            <Empty title="No presets match" body="Clear the search or use a capability named in a preset requirement." />
          {:else}
            <div class="preset-grid">
              {#each shownPresets as preset (preset.id)}
                <article class="preset">
                  <header><div><h3>{preset.name}</h3><span>{preset.models} routable</span></div><Chip tone="quiet">{preset.strategy}</Chip></header>
                  <p>{preset.description}</p>
                  <ul>{#each preset.requirements as requirement}<li>{requirement}</li>{/each}</ul>
                  <div class="preset-actions">
                    <Button size="sm" armed={preset.models > 0} busy={actionBusy === `preset-${preset.id}-false`} onclick={() => void createPreset(preset, false)}>Create</Button>
                    <Button size="sm" variant="plate" armed={preset.models > 0} busy={actionBusy === `preset-${preset.id}-true`} onclick={() => void createPreset(preset, true)}>Create and activate</Button>
                  </div>
                </article>
              {/each}
            </div>
          {/if}
        </Card>

        <RashinLanesCard {quick} busy={quickBusy} error={quickError} onset={(route) => void setQuick(route)} />

        {#if routing}
          <Card title="Global routing" gloss="方針" lead="Sets may override the strategy. These values govern every set that follows the default.">
            <div class="settings-grid">
              <div class="setting">
                <span class="t-label">Default strategy</span>
                <Select label="Default routing strategy" options={STRATEGIES} value={settingsStrategy} onchange={(value) => (settingsStrategy = value)} />
              </div>
              <div class="setting">
                <span class="t-label">Key selection</span>
                <Select label="Key selection strategy" options={KEY_STRATEGIES} value={settingsKeyStrategy} onchange={(value) => (settingsKeyStrategy = value)} />
              </div>
              <Field label="Cooldown ceiling (milliseconds)" type="number" min="60000" max="86400000" step="1000" placeholder="No ceiling" bind:value={settingsCooldown} mono />
              <div class="switch-setting">
                <div><p class="t-row">Explore healthy routes</p><p class="t-small">Occasionally sample alternatives so routing evidence stays current.</p></div>
                <Switch checked={settingsExplore} label="Explore healthy routes" onchange={(value) => (settingsExplore = value)} />
              </div>
            </div>
            <div class="benchmark">
              <span class="t-label">Benchmark source</span>
              <dl>
                <div><dt>Source</dt><dd>{routing.benchmark.source || "Built in"}</dd></div>
                <div><dt>Status</dt><dd>{routing.benchmark.status || (routing.benchmark.configured ? "Configured" : "Not configured")}</dd></div>
                <div><dt>Coverage</dt><dd>{routing.benchmark.matched} of {routing.benchmark.available} models</dd></div>
                <div><dt>Last success</dt><dd>{routing.benchmark.lastSuccess || "Not yet"}</dd></div>
                {#if routing.benchmark.attribution}<div><dt>Attribution</dt><dd>{routing.benchmark.attribution}</dd></div>{/if}
              </dl>
              {#if routing.benchmark.error}<p class="benchmark-error">{routing.benchmark.error}</p>{/if}
            </div>
            <div class="settings-actions"><Button variant="plate" busy={saving} onclick={() => void saveGlobalSettings()}>Save routing settings</Button></div>
          </Card>
        {/if}
      </div>
    {/if}
  </Page>
{/if}

<Dialog bind:open={formOpen} title={formMode === "create" ? "New routing set" : formMode === "rename" ? "Rename routing set" : "Delete routing set"} description={formMode === "delete" ? "Its saved model selection is removed. This cannot be undone." : "Use letters, digits, hyphens or underscores, up to 20 characters."}>
  {#if formMode === "delete"}
    <p class="dialog-copy">Delete <strong>{formProfile?.name}</strong>?</p>
  {:else}
    <Field label="Set name" bind:value={formName} maxlength={20} autocomplete="off" />
  {/if}
  {#snippet footer()}
    <Button variant="quiet" onclick={() => { formOpen = false; formMode = ""; }}>Cancel</Button>
    <Button
      variant="plate"
      busy={actionBusy === formMode}
      armed={formMode === "delete" || formName.trim().length > 0}
      autofocus
      onclick={() => void submitForm()}
    >{formMode === "delete" ? "Delete set" : formMode === "rename" ? "Rename set" : "Create set"}</Button>
  {/snippet}
</Dialog>

<style>
  .routing-layout { display: grid; gap: var(--s4); align-content: start; }
  .notice { display: flex; align-items: center; justify-content: space-between; gap: var(--s4); padding: var(--s3) var(--s4); border: 1px solid var(--line); border-radius: var(--radius); color: var(--ink-dim); }
  .action-error { padding: var(--s2) var(--s3); border-left: 1px solid var(--alert); color: var(--alert); }
  .library-search { max-width: 440px; }
  .set-list { display: flex; flex-direction: column; }
  .set-row { display: grid; grid-template-columns: minmax(150px, 1fr) auto minmax(150px, auto) auto auto auto; align-items: center; gap: var(--s2); min-height: 54px; padding: var(--s2) var(--s4); border-bottom: 1px solid var(--line-soft); }
  .set-row:last-child { border-bottom: 0; }
  .set-row.active { border-left: 2px solid var(--bone); padding-left: calc(var(--s4) - 2px); }
  .set-name { min-width: 0; }
  .set-name strong, .set-name span { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .set-name strong { color: var(--ink); font-size: var(--f-row); font-weight: 500; }
  .set-name span { margin-top: 2px; color: var(--ink-mute); font-size: var(--f-small); }
  .preset-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 300px), 1fr)); }
  .preset { display: flex; flex-direction: column; min-height: 220px; padding: var(--s4); border-right: 1px solid var(--line-soft); border-bottom: 1px solid var(--line-soft); }
  .preset header { display: flex; align-items: flex-start; justify-content: space-between; gap: var(--s3); }
  .preset h3 { color: var(--ink); font-size: var(--f-row); font-weight: 500; }
  .preset header span { color: var(--ink-faint); font-family: var(--mono); font-size: var(--f-micro); }
  .preset > p { margin-top: var(--s3); color: var(--ink-mute); font-size: var(--f-small); line-height: 1.5; }
  .preset ul { display: flex; flex-wrap: wrap; gap: var(--s1) var(--s3); margin-top: var(--s3); color: var(--ink-dim); font-size: var(--f-small); }
  .preset li::before { content: "·"; margin-right: var(--s1); color: var(--ink-faint); }
  .preset-actions { display: flex; flex-wrap: wrap; gap: var(--s2); margin-top: auto; padding-top: var(--s4); }
  .settings-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: var(--s4); }
  .setting { display: flex; flex-direction: column; align-items: flex-start; gap: var(--s1); }
  .switch-setting { grid-column: 1 / -1; display: flex; align-items: center; justify-content: space-between; gap: var(--s4); padding-top: var(--s3); border-top: 1px solid var(--line-soft); }
  .benchmark { margin-top: var(--s5); padding-top: var(--s4); border-top: 1px solid var(--line-soft); }
  .benchmark dl { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--s2) var(--s5); margin-top: var(--s3); }
  .benchmark dl div { display: flex; justify-content: space-between; gap: var(--s3); padding-bottom: var(--s2); border-bottom: 1px solid var(--line-soft); }
  .benchmark dt { color: var(--ink-mute); }
  .benchmark dd { margin: 0; color: var(--ink); text-align: right; }
  .benchmark-error { margin-top: var(--s3); color: var(--alert); font-size: var(--f-small); }
  .settings-actions { display: flex; justify-content: flex-end; margin-top: var(--s4); }
  .dialog-copy { color: var(--ink-dim); }
  .dialog-copy strong { color: var(--ink); font-weight: 500; }
  @media (max-width: 1100px) {
    .set-row { grid-template-columns: minmax(140px, 1fr) auto minmax(140px, auto) auto; }
    .set-row > :global(button:nth-last-child(-n+2)) { grid-row: 2; }
    .settings-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  }
  @media (max-width: 720px) {
    .set-row { grid-template-columns: minmax(0, 1fr) auto; }
    .set-row > :global(.sel), .set-row > :global(button) { grid-column: auto; }
    .settings-grid, .benchmark dl { grid-template-columns: 1fr; }
  }
</style>
