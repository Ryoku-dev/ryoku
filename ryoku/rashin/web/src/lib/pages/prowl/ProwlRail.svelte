<script lang="ts">
  import { untrack } from "svelte";
  import { ApiError } from "$lib/api/client";
  import { prowl } from "$lib/api/prowl";

  interface Props {
    active: string;
  }

  let { active }: Props = $props();
  let providerBadge = $state("");
  let routingBadge = $state("");
  let harnessBadge = $state("");
  let starting = $state(false);
  let retryTimer: number | undefined;

  const pages = [
    { id: "overview", label: "Overview", gloss: "総覧" },
    { id: "providers", label: "Providers", gloss: "供給" },
    { id: "routing", label: "Routing", gloss: "経路" },
    { id: "activity", label: "Activity", gloss: "活動" },
    { id: "projects", label: "Projects", gloss: "索引" },
    { id: "harnesses", label: "Harnesses", gloss: "連携" },
    { id: "toolkit", label: "Toolkit", gloss: "道具" },
  ] as const;

  async function loadBadges() {
    clearTimeout(retryTimer);
    const [directory, profiles, activeProfile, harnesses] = await Promise.allSettled([
      prowl.providers.directory(),
      prowl.routing.profiles(),
      prowl.routing.activeProfile(),
      prowl.setup.harnesses(),
    ] as const);

    if (directory.status === "fulfilled") {
      providerBadge = `${directory.value.counts.configured} connected`;
    }
    if (profiles.status === "fulfilled" && activeProfile.status === "fulfilled") {
      const selected = profiles.value.find((profile) => profile.id === activeProfile.value.activeProfileId);
      routingBadge = selected?.name ?? "Default";
    }
    if (harnesses.status === "fulfilled") {
      const detected = harnesses.value.harnesses.filter((harness) => harness.detected);
      const routed = detected.filter((harness) => harness.active).length;
      harnessBadge = `${routed}/${detected.length}`;
    }

    const failures = [directory, profiles, activeProfile, harnesses].filter(
      (result): result is PromiseRejectedResult => result.status === "rejected",
    );
    starting = failures.some((result) => result.reason instanceof ApiError && result.reason.code === "gateway_down");
    if (starting) retryTimer = window.setTimeout(() => void loadBadges(), 2_000);
  }

  $effect(() => {
    void active;
    untrack(() => { void loadBadges(); });
    return () => clearTimeout(retryTimer);
  });

  function badge(id: string): string {
    if (starting && (id === "providers" || id === "routing" || id === "harnesses")) return "starting";
    if (id === "providers") return providerBadge;
    if (id === "routing") return routingBadge;
    if (id === "harnesses") return harnessBadge;
    return "";
  }
</script>

<nav class="rail" aria-label="Prowl sections">
  <div class="rail-mark">
    <span class="t-mark">Prowl</span>
    <span class="rail-seal">徘徊</span>
  </div>
  <div class="rail-links">
    {#each pages as page (page.id)}
      {@const value = badge(page.id)}
      <a
        href="#/prowl/{page.id}"
        class="rail-link"
        class:on={active === page.id}
        aria-current={active === page.id ? "page" : undefined}
      >
        <span class="rail-name">{page.label}</span>
        <span class="rail-gloss">{page.gloss}</span>
        {#if value}<span class="rail-badge">{value}</span>{/if}
      </a>
    {/each}
  </div>
</nav>

<style>
  .rail {
    width: var(--rail-w);
    min-width: 0;
    padding: var(--s5) var(--s4);
    border-right: 1px solid var(--line-soft);
    overflow: auto;
  }
  .rail-mark {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    padding: 0 var(--s3) var(--s4);
    border-bottom: 1px solid var(--line-soft);
    margin-bottom: var(--s3);
  }
  .rail-seal { font-family: var(--jp); color: var(--ink-faint); font-size: var(--f-row); }
  .rail-links { display: flex; flex-direction: column; gap: 2px; }
  .rail-link {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    grid-template-areas: "name gloss" "badge badge";
    gap: 0 var(--s2);
    align-items: baseline;
    min-height: 48px;
    padding: var(--s2) var(--s3);
    border-radius: var(--radius);
    color: var(--ink-mute);
    transition: background-color var(--t-fast) var(--ease), color var(--t-fast) var(--ease);
  }
  .rail-link:hover { background: var(--tint5); color: var(--ink); }
  .rail-link.on { background: var(--bone); color: var(--ink-on-bone); }
  .rail-name { grid-area: name; font-size: var(--f-row); font-weight: 500; }
  .rail-gloss { grid-area: gloss; font-family: var(--jp); font-size: var(--f-micro); color: var(--ink-faint); }
  .rail-badge { grid-area: badge; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: var(--mono); font-size: var(--f-tiny); letter-spacing: var(--track-label); color: var(--ink-faint); }
  .rail-link.on .rail-gloss,
  .rail-link.on .rail-badge { color: color-mix(in srgb, var(--ink-on-bone) 62%, transparent); }

  @media (max-width: 900px) {
    .rail { width: 190px; padding-inline: var(--s3); }
    .rail-badge { display: none; }
  }
  @media (max-width: 680px) {
    .rail {
      width: 100%;
      padding: var(--s2) var(--s4);
      border-right: 0;
      border-bottom: 1px solid var(--line-soft);
      overflow-x: auto;
    }
    .rail-mark { display: none; }
    .rail-links { flex-direction: row; width: max-content; }
    .rail-link { display: flex; min-height: 36px; align-items: baseline; padding: var(--s1) var(--s3); }
    .rail-gloss { display: none; }
  }
</style>
