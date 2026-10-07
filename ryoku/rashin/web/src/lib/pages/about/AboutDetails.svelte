<script lang="ts">
  import { router } from "$lib/app/router.svelte";
  import Button from "$lib/ui/Button.svelte";
  import Card from "$lib/ui/Card.svelte";
  import Chip from "$lib/ui/Chip.svelte";
  import Empty from "$lib/ui/Empty.svelte";
  import Fold from "$lib/ui/Fold.svelte";
  import type { AboutInfo, AboutManifest } from "./types";

  interface Props {
    about: AboutInfo | null;
    manifest: AboutManifest | null;
    error?: string;
  }

  let { about, manifest, error = "" }: Props = $props();
  const paths = $derived(manifest ? [manifest.skill, manifest.prowl, ...(manifest.vault || [])] : []);

  const commands = [
    ["hermes -h", "Explore every agent command"],
    ["hermes gateway", "Connect messaging gateways"],
    ["hermes model", "Switch the resident model"],
    ["hermes tools", "Choose agent toolsets"],
    ["ryoku-rashin -h", "Inspect daemon verbs"],
    ["prowl overview", "Map code in the current repo"],
  ];
</script>

{#if error && !about}
  <Empty title="About data is unavailable" body="Start the Rashin daemon, then return here for build and path details." />
{:else if about}
  <div class="about-grid">
    <Card class="intro">
      <div class="intro-copy">
        <span class="seal t-jp">力</span>
        <div>
          <span class="t-label">Local agent OS</span>
          <h2>Rashin keeps the machine legible to every agent.</h2>
          <p>Its maintained map replaces repeated discovery, while the resident session and this console share one transcript.</p>
        </div>
      </div>
      <div class="build">
        <div><span class="t-label">Build</span><strong>v{about.version || "unknown"}</strong></div>
        <div><span class="t-label">Loopback</span><strong>:{about.port || 3600}</strong></div>
        <a href={`http://127.0.0.1:${about.port || 3600}`} target="_blank" rel="noreferrer">Open dashboard</a>
      </div>
    </Card>

    <Card title="The pieces" gloss="構成">
      <div class="pieces">
        <div><span>Vault</span><small>The map agents read</small><code>{about.vault || "Not set"}</code></div>
        <div><span>Hermes</span><small>The resident agent</small><code>{about.hermes.installed ? [about.hermes.version && `v${about.hermes.version}`, about.hermes.model, about.hermes.provider].filter(Boolean).join(" / ") : "Not installed"}</code></div>
        <div><span>Prowl</span><small>Code intelligence</small><Chip tone={about.prowl.installed ? "line" : "quiet"}>{about.prowl.installed ? "Installed" : "Absent"}</Chip></div>
        <div><span>Privacy</span><small>Loopback only</small><code>127.0.0.1:{about.port || 3600}</code></div>
      </div>
    </Card>

    <Card title="Quick start" gloss="開始">
      <ol>
        <li><span>1</span><p>Enable Rashin in Ryoku Settings under Advanced.</p></li>
        <li><span>2</span><p>Connect an installed harness through Prowl.</p></li>
        <li><span>3</span><p>Chat here, or run the agent inside the vault.</p></li>
      </ol>
      <Button variant="plate" size="sm" onclick={() => router.go("prowl", "harnesses")}>Open harnesses</Button>
    </Card>

    {#if manifest}
      <Card title="Paths and links" gloss="経路" class="wide">
        <Fold>
          {#snippet summary()}<span>{paths.length} local resources exposed to agents</span>{/snippet}
          <div class="paths">
            {#each paths as item (item.path)}
              <div class:missing={!item.exists}>
                <span>{item.label}</span><code>{item.path}</code><small>{item.desc}</small>
              </div>
            {/each}
          </div>
        </Fold>
      </Card>
    {/if}

    <Card title="Command crib" gloss="端末" class="wide">
      <Fold>
        {#snippet summary()}<span>{commands.length} useful commands</span>{/snippet}
        <div class="commands">
          {#each commands as command (command[0])}<div><code>$ {command[0]}</code><span>{command[1]}</span></div>{/each}
        </div>
      </Fold>
    </Card>
  </div>
{/if}

<style>
  .about-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--s4); align-items: start; }
  :global(.about-grid > .intro), :global(.about-grid > .wide) { grid-column: 1 / -1; }
  .intro-copy { display: flex; align-items: flex-start; gap: var(--s5); }
  .seal { display: grid; place-items: center; width: var(--s7); height: var(--s7); flex: none; border-radius: var(--radius); background: var(--alert); color: var(--ink-on-bone); font-size: 28px; }
  h2 { max-width: 25ch; margin-top: var(--s1); color: var(--ink); font-family: var(--display); font-size: 28px; font-weight: 400; line-height: 1.18; }
  .intro-copy p { max-width: 68ch; margin-top: var(--s3); color: var(--ink-mute); }
  .build { display: grid; grid-template-columns: auto auto 1fr; gap: var(--s5); align-items: end; margin-top: var(--s5); padding-top: var(--s4); border-top: 1px solid var(--line-soft); }
  .build > div { display: grid; gap: var(--s1); }
  .build strong { color: var(--ink); font-family: var(--mono); font-size: var(--f-row); font-weight: 400; }
  .build a { justify-self: end; color: var(--ink); border-bottom: 1px solid var(--line-strong); font-size: var(--f-small); }
  .pieces { display: grid; }
  .pieces > div { display: grid; grid-template-columns: 72px 1fr auto; align-items: center; gap: var(--s3); min-height: 40px; border-bottom: 1px solid var(--line-soft); }
  .pieces > div:last-child { border: 0; }
  .pieces > div > span { color: var(--ink); font-weight: 500; }
  small { color: var(--ink-mute); font-size: var(--f-small); }
  code { max-width: 260px; overflow: hidden; color: var(--ink-dim); font-size: var(--f-micro); text-overflow: ellipsis; white-space: nowrap; }
  ol { display: grid; gap: var(--s3); margin-bottom: var(--s4); }
  li { display: flex; gap: var(--s3); align-items: flex-start; }
  li > span { display: grid; place-items: center; width: 22px; height: 22px; flex: none; border: 1px solid var(--line); border-radius: 50%; color: var(--ink-faint); font-family: var(--mono); font-size: var(--f-tiny); }
  li p { color: var(--ink-dim); }
  .paths, .commands { display: grid; margin-top: var(--s3); border-top: 1px solid var(--line-soft); }
  .paths > div { display: grid; grid-template-columns: 130px minmax(180px, 0.8fr) minmax(240px, 1.2fr); gap: var(--s3); align-items: center; min-height: 40px; border-bottom: 1px solid var(--line-soft); }
  .paths > div > span { color: var(--ink); }
  .paths .missing { opacity: 0.55; }
  .commands { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 var(--s5); }
  .commands > div { display: grid; grid-template-columns: minmax(140px, auto) 1fr; gap: var(--s3); min-height: 40px; align-items: center; border-bottom: 1px solid var(--line-soft); }
  .commands span { color: var(--ink-mute); font-size: var(--f-small); }
  @media (max-width: 850px) { .about-grid { grid-template-columns: 1fr; } :global(.about-grid > .intro), :global(.about-grid > .wide) { grid-column: auto; } .paths > div { grid-template-columns: 1fr; gap: var(--s1); padding: var(--s2) 0; } .commands { grid-template-columns: 1fr; } }
  @media (max-width: 560px) { .intro-copy { flex-direction: column; } .build { grid-template-columns: 1fr 1fr; } .build a { grid-column: 1 / -1; justify-self: start; } .pieces > div { grid-template-columns: 1fr auto; padding: var(--s2) 0; } .pieces small { grid-column: 1 / -1; grid-row: 2; } }
</style>
