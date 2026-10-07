<!-- The console: one island bar, one sheet on the stage. Stores start here
     and live for the page; a sheet is a projection of them. -->
<script lang="ts">
  import { Tooltip } from "bits-ui";
  import TopBar from "$lib/app/TopBar.svelte";
  import { router } from "$lib/app/router.svelte";
  import { chat, ryoku } from "$lib/chat/store.svelte";
  import { machine } from "$lib/state/machine.svelte";
  import { theme } from "$lib/state/theme.svelte";
  import Offline from "$lib/app/Offline.svelte";
  import RyokuSheet from "./pages/RyokuSheet.svelte";
  import ChatSheet from "./pages/ChatSheet.svelte";
  import WikiSheet from "./pages/WikiSheet.svelte";
  import OverviewSheet from "./pages/OverviewSheet.svelte";
  import SystemSheet from "./pages/SystemSheet.svelte";
  import VaultSheet from "./pages/VaultSheet.svelte";
  import MemorySheet from "./pages/MemorySheet.svelte";
  import SkillsSheet from "./pages/SkillsSheet.svelte";
  import ProwlSheet from "./pages/ProwlSheet.svelte";
  import AboutSheet from "./pages/AboutSheet.svelte";

  const SHEET_VIEWS = {
    ryoku: RyokuSheet,
    chat: ChatSheet,
    wiki: WikiSheet,
    overview: OverviewSheet,
    system: SystemSheet,
    vault: VaultSheet,
    memory: MemorySheet,
    skills: SkillsSheet,
    prowl: ProwlSheet,
    about: AboutSheet,
  } as const;

  const View = $derived(SHEET_VIEWS[router.sheet as keyof typeof SHEET_VIEWS] ?? RyokuSheet);

  $effect(() => {
    router.start();
    theme.start();
    machine.start();
    ryoku.open();
    chat.open();
    return () => {
      ryoku.close();
      chat.close();
      machine.stop();
      theme.stop();
    };
  });
</script>

<Tooltip.Provider>
  <div class="console">
    <TopBar />
    <main class="stage">
      {#key router.sheet}
        <View />
      {/key}
    </main>
    <Offline />
  </div>
</Tooltip.Provider>

<style>
  .console {
    display: grid;
    grid-template-rows: auto minmax(0, 1fr);
    height: 100%;
    container-type: inline-size;
  }
  .stage { min-height: 0; position: relative; }
</style>
