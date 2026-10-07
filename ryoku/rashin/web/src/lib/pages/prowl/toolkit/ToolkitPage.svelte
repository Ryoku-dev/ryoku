<script lang="ts">
  import Page from "$lib/app/Page.svelte";
  import Card from "$lib/ui/Card.svelte";
  import IconButton from "$lib/ui/IconButton.svelte";

  interface Props {
    params: string[];
  }

  let { params }: Props = $props();
  let copied = $state("");
  let copyError = $state("");

  const commands = [
    { name: "Project map", command: "prowl overview", description: "Languages, subsystems, entrypoints, hotspots" },
    { name: "Find and read", command: "prowl search / find / def / peek", description: "Cited code without whole-file reads" },
    { name: "Trace dependencies", command: "prowl references / callers / impact", description: "Uses, relationships, and change blast radius" },
    { name: "Project health", command: "prowl status / doctor", description: "Index freshness, coverage, and structural risks" },
    { name: "Current work", command: "prowl wip / changed / history", description: "Unfinished work and affected code" },
    { name: "Bounded context", command: "prowl context search / brief", description: "Small question-shaped context packets" },
    { name: "Documentation", command: "prowl docs add / list / refresh", description: "Indexed external docs beside project code" },
    { name: "Durable knowledge", command: "prowl knowledge", description: "Reviewed decisions, concepts, and gotchas" },
    { name: "Large-change review", command: "prowl review plan", description: "Bounded review units with a coverage gate" },
    { name: "Agent setup", command: "prowl init / skills", description: "Project index, rules, MCP, and client skills" },
    { name: "Model gateway", command: "prowl gateway", description: "Providers, routing sets, and usage" },
    { name: "Editor services", command: "prowl serve / lsp", description: "MCP and language-server integrations" },
    { name: "Capability finder", command: "prowl capabilities search", description: "The right workflow for an intent" },
  ] as const;

  async function copy(command: string) {
    copyError = "";
    try {
      await navigator.clipboard.writeText(command);
      copied = command;
      window.setTimeout(() => {
        if (copied === command) copied = "";
      }, 1_500);
    } catch {
      copyError = "The command could not be copied. Select it in the table and copy it manually.";
    }
  }

</script>

<Page title="Toolkit" gloss="道具" lead="The Prowl command crib, ready to copy into a terminal.">
  <div class="toolkit-grid">
    {#if copyError}<p class="copy-error" role="alert">{copyError}</p>{/if}

    <Card title="How Prowl indexes" gloss="索引">
      <div class="pipeline" aria-label="Prowl indexing pipeline">
        <span>tree-sitter</span><i aria-hidden="true"></i>
        <span>SQLite FTS5</span><i aria-hidden="true"></i>
        <span>potion-code-16M</span><i aria-hidden="true"></i>
        <span>sqlite-vec</span><i aria-hidden="true"></i>
        <span>rank fusion</span>
      </div>
      <p class="explainer-copy">
        Tree-sitter extracts the structure, then SQLite FTS5 keeps lexical search fast.
        The potion-code-16M static embedding model is compiled into Prowl and stores vectors
        through sqlite-vec. Reciprocal-rank fusion combines both result lists. A local model
        is optional and is used only for smart reranking.
      </p>
    </Card>

    <Card title="Command crib" gloss="早見" pad={false}>
      <div class="command-table" role="table" aria-label="Prowl commands">
        <div class="command-head" role="row">
          <span role="columnheader">Function</span>
          <span role="columnheader">Start here</span>
          <span role="columnheader">What it gives you</span>
          <span aria-hidden="true"></span>
        </div>
        {#each commands as item (item.name)}
          <div class="command-row" role="row">
            <span class="command-name" role="cell">{item.name}</span>
            <span role="cell"><code>{item.command}</code></span>
            <span class="command-description" role="cell">{item.description}</span>
            <IconButton
              icon={copied === item.command ? "check" : "copy"}
              label={copied === item.command ? "Copied" : `Copy ${item.name}`}
              onclick={() => void copy(item.command)}
            />
          </div>
        {/each}
      </div>
    </Card>
  </div>
</Page>

<style>
  .toolkit-grid { display: grid; grid-template-columns: 1fr; gap: var(--s4); align-content: start; }
  .copy-error { color: var(--alert); padding: var(--s2) var(--s3); border-left: 1px solid var(--alert); }
  .explainer-copy { max-width: var(--measure); color: var(--ink-dim); line-height: 1.6; }
  .pipeline { display: grid; grid-template-columns: auto minmax(18px, 1fr) auto minmax(18px, 1fr) auto minmax(18px, 1fr) auto minmax(18px, 1fr) auto; align-items: center; gap: var(--s2); margin-bottom: var(--s4); }
  .pipeline span { padding: var(--s2) var(--s3); border: 1px solid var(--line); border-radius: var(--radius); color: var(--ink); font-family: var(--mono); font-size: var(--f-small); white-space: nowrap; }
  .pipeline i { position: relative; height: 1px; background: var(--line-strong); }
  .pipeline i::after { content: ""; position: absolute; right: 0; top: -2px; width: 5px; height: 5px; border-top: 1px solid var(--ink-mute); border-right: 1px solid var(--ink-mute); transform: rotate(45deg); }
  .command-table { min-width: 0; }
  .command-head,
  .command-row { display: grid; grid-template-columns: minmax(130px, .7fr) minmax(220px, 1fr) minmax(260px, 1.35fr) 34px; gap: var(--s3); align-items: center; padding: var(--s2) var(--s5); }
  .command-head { border-bottom: 1px solid var(--line); color: var(--ink-faint); font-family: var(--mono); font-size: var(--f-micro); letter-spacing: var(--track-label); text-transform: uppercase; }
  .command-row { min-height: 48px; border-bottom: 1px solid var(--line-soft); }
  .command-row:last-child { border-bottom: 0; }
  .command-row:hover { background: var(--tint5); }
  .command-name { color: var(--ink); font-weight: 500; }
  .command-row code { overflow-wrap: anywhere; color: var(--ink-dim); font-size: var(--f-small); }
  .command-description { color: var(--ink-mute); font-size: var(--f-small); }

  @media (max-width: 1160px) {
    .pipeline { grid-template-columns: 1fr; align-items: stretch; }
    .pipeline i { width: 1px; height: 14px; margin-left: var(--s5); }
    .pipeline i::after { right: -2px; top: auto; bottom: 0; transform: rotate(135deg); }
    .command-head { display: none; }
    .command-row { grid-template-columns: minmax(0, 1fr) auto; gap: var(--s1) var(--s3); padding-block: var(--s3); }
    .command-name, .command-row code, .command-description { grid-column: 1; }
    .command-row :global(button) { grid-column: 2; grid-row: 1 / span 3; }
  }
</style>
