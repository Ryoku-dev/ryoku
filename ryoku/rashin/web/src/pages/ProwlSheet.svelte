<script lang="ts">
  import { router } from "$lib/app/router.svelte";
  import ProwlRail from "$lib/pages/prowl/ProwlRail.svelte";
  import OverviewPage from "$lib/pages/prowl/overview/OverviewPage.svelte";
  import ProvidersPage from "$lib/pages/prowl/providers/ProvidersPage.svelte";
  import RoutingPage from "$lib/pages/prowl/routing/RoutingPage.svelte";
  import ActivityPage from "$lib/pages/prowl/activity/ActivityPage.svelte";
  import ProjectsPage from "$lib/pages/prowl/projects/ProjectsPage.svelte";
  import HarnessesPage from "$lib/pages/prowl/harnesses/HarnessesPage.svelte";
  import ToolkitPage from "$lib/pages/prowl/toolkit/ToolkitPage.svelte";

  const PAGES = {
    overview: OverviewPage,
    providers: ProvidersPage,
    routing: RoutingPage,
    activity: ActivityPage,
    projects: ProjectsPage,
    harnesses: HarnessesPage,
    toolkit: ToolkitPage,
  } as const;

  const active = $derived.by((): keyof typeof PAGES => {
    const requested = router.rest[0];
    return requested && requested in PAGES ? requested as keyof typeof PAGES : "overview";
  });
  const params = $derived(router.rest.slice(1));
  const View = $derived(PAGES[active]);
</script>

<div class="prowl-sheet">
  <ProwlRail {active} />
  <section class="prowl-page" aria-label="Prowl {active}">
    {#key active}
      <View {params} />
    {/key}
  </section>
</div>

<style>
  .prowl-sheet {
    display: grid;
    grid-template-columns: var(--rail-w) minmax(0, 1fr);
    height: 100%;
    min-height: 0;
  }
  .prowl-page { min-width: 0; min-height: 0; }
  @media (max-width: 900px) { .prowl-sheet { grid-template-columns: 190px minmax(0, 1fr); } }
  @media (max-width: 680px) {
    .prowl-sheet { grid-template-columns: 1fr; grid-template-rows: auto minmax(0, 1fr); }
  }
</style>
