<script lang="ts">
  import { onMount } from "svelte";
  import Page from "$lib/app/Page.svelte";
  import { api } from "$lib/api/client";
  import { filterCategories, groupByOrigin } from "$lib/pages/skills/filter";
  import { loadHarnesses, type Harness, type Skill } from "$lib/pages/skills/harnesses";
  import { readSkillsReport, type SkillsReport } from "$lib/pages/skills/report";
  import SkillGroups from "$lib/pages/skills/SkillGroups.svelte";
  import Button from "$lib/ui/Button.svelte";
  import Chip from "$lib/ui/Chip.svelte";
  import Empty from "$lib/ui/Empty.svelte";
  import Field from "$lib/ui/Field.svelte";
  import Tabs from "$lib/ui/Tabs.svelte";

  interface TabSpec {
    value: string;
    label: string;
    count: number;
  }

  let hermes = $state<SkillsReport | null>(null);
  let harnesses = $state<Harness[]>([]);
  let active = $state("hermes");
  let query = $state("");
  let loading = $state(true);
  let refreshing = $state(false);
  let error = $state("");

  const tabs = $derived.by((): TabSpec[] => {
    const result: TabSpec[] = [];
    const present = harnesses.filter((harness) => harness.present);
    for (const harness of present) {
      const count = harness.id === "hermes" && hermes
        ? hermes.categories.reduce((sum, category) => sum + (category.skills || []).length, 0)
        : harness.skillCount || harness.skills?.length || 0;
      result.push({ value: harness.id, label: harness.name, count });
    }
    if (hermes && !result.some((tab) => tab.value === "hermes")) {
      result.unshift({
        value: "hermes",
        label: "Hermes",
        count: hermes.categories.reduce((sum, category) => sum + (category.skills || []).length, 0),
      });
    }
    return result;
  });

  function categoriesFor(id: string): Array<{ name: string; skills: Skill[] }> {
    if (id === "hermes") return hermes?.categories || [];
    const skills = harnesses.find((harness) => harness.id === id)?.skills || [];
    return skills.length > 25 ? groupByOrigin(skills) : [{ name: "Skills", skills }];
  }

  function originCounts(categories: Array<{ name?: string; skills?: Skill[] }>): Array<[string, number]> {
    const counts: Record<string, number> = {};
    for (const category of categories) {
      for (const skill of category.skills || []) {
        const origin = skill.origin || "agent";
        counts[origin] = (counts[origin] || 0) + 1;
      }
    }
    return Object.entries(counts).sort((a, b) => b[1] - a[1]);
  }

  async function load(force = false): Promise<void> {
    if (force) refreshing = true;
    else loading = true;
    error = "";
    try {
      const [skillsResult, harnessResult] = await Promise.allSettled([
        api.hermesSkills() as Promise<unknown>,
        loadHarnesses(),
      ]);
      if (skillsResult.status === "fulfilled") hermes = readSkillsReport(skillsResult.value as Partial<SkillsReport>);
      else hermes = null;
      if (harnessResult.status === "fulfilled") harnesses = harnessResult.value;
      else harnesses = [];
      if (!hermes && !harnesses.length) error = "The skills ledger is out of reach. Start the Rashin daemon, then refresh.";
      const available = tabs;
      if (!available.some((tab) => tab.value === active)) active = available[0]?.value || "hermes";
    } catch (cause) {
      error = cause instanceof Error ? cause.message : "The skills ledger could not be read.";
    } finally {
      loading = false;
      refreshing = false;
    }
  }

  function changeHarness(id: string): void {
    active = id;
    query = "";
  }

  onMount(() => {
    void load();
  });
</script>

{#snippet tools()}
  <Button icon="refresh" size="sm" busy={refreshing} onclick={() => load(true)}>Refresh</Button>
{/snippet}

<Page title="Skills" gloss="技" lead="The capabilities each installed agent can reach." {tools}>
  {#if loading}
    <p class="loading">Reading agent skills…</p>
  {:else if error}
    <Empty icon="tool" title="Skills unavailable" body={error} />
  {:else if !tabs.length}
    <Empty icon="tool" title="No harnesses found" body="Install an agent harness to see its skills and enabled tools." />
  {:else}
    <Tabs tabs={tabs} value={active} onchange={changeHarness}>
      {#snippet children(tabId)}
        {@const allCategories = categoriesFor(tabId)}
        {@const filtered = filterCategories(allCategories, query) as Array<{ name: string; skills: Skill[] }>}
        {@const total = allCategories.reduce((sum, category) => sum + (category.skills || []).length, 0)}
        {@const shown = filtered.reduce((sum, category) => sum + category.skills.length, 0)}
        <div class="tab-sheet">
          <div class="skills-toolbar">
            <Field label="Filter skills" placeholder="Name or description" bind:value={query} />
            <div class="counts" aria-label="Skill origins">
              {#if query}<Chip tone="quiet">{shown} of {total}</Chip>{/if}
              {#each originCounts(filtered) as [origin, count] (origin)}
                <Chip tone={origin === "bundled" || origin === "hub" ? "plate" : "line"} mono>{origin} {count}</Chip>
              {/each}
            </div>
          </div>

          <SkillGroups categories={filtered} {query} />

          {#if tabId === "hermes"}
            <section class="toolbelt">
              <header>
                <h2>Enabled toolbelt</h2>
                <span class="t-jp">道具</span>
              </header>
              {#if hermes?.toolbelt.length}
                <div class="tool-families">
                  {#each hermes.toolbelt as family (family.family)}
                    <div class="tool-family">
                      <span class="family-name">{family.family}</span>
                      <div class="tools">
                        {#each family.tools as tool (tool)}<Chip>{tool}</Chip>{/each}
                      </div>
                    </div>
                  {/each}
                </div>
              {:else}
                <p>No toolsets enabled.</p>
              {/if}
            </section>
          {/if}
        </div>
      {/snippet}
    </Tabs>
  {/if}
</Page>

<style>
  .loading { padding: var(--s5) 0; color: var(--ink-mute); }
  .tab-sheet { display: flex; flex-direction: column; gap: var(--s4); }
  .skills-toolbar { display: flex; align-items: flex-end; gap: var(--s4); }
  .skills-toolbar :global(.field) { width: min(340px, 100%); }
  .counts { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: var(--s2); margin-left: auto; padding-bottom: 6px; }
  .toolbelt { margin-top: var(--s3); padding-top: var(--s5); border-top: 1px solid var(--line); }
  .toolbelt header { display: flex; align-items: baseline; gap: var(--s2); margin-bottom: var(--s4); }
  .toolbelt h2 { color: var(--ink); font-size: var(--f-row); font-weight: 500; }
  .toolbelt > p { color: var(--ink-mute); font-size: var(--f-small); }
  .tool-families { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 220px), 1fr)); gap: var(--s4); }
  .tool-family { display: flex; flex-direction: column; gap: var(--s2); padding-top: var(--s3); border-top: 1px solid var(--line-soft); }
  .family-name { color: var(--ink-mute); font-family: var(--mono); font-size: var(--f-micro); letter-spacing: var(--track-label); text-transform: uppercase; }
  .tools { display: flex; flex-wrap: wrap; gap: var(--s1); }
  @media (max-width: 680px) {
    .skills-toolbar { align-items: stretch; flex-direction: column; }
    .skills-toolbar :global(.field) { width: 100%; }
    .counts { justify-content: flex-start; margin-left: 0; padding-bottom: 0; }
  }
</style>
