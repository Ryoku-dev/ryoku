<script lang="ts">
  import Button from "$lib/ui/Button.svelte";
  import Dialog from "$lib/ui/Dialog.svelte";
  import Field from "$lib/ui/Field.svelte";

  interface Integration {
    id: string;
    label: string;
    detail: string;
  }

  interface Props {
    open: boolean;
    busy?: boolean;
    error?: string;
    onadd: (root: string, integrations: string[]) => void;
  }

  const integrations: Integration[] = [
    { id: "agents", label: "Agent guidance", detail: "AGENTS.md" },
    { id: "agent-skills", label: "Agent Skills", detail: ".agents/skills" },
    { id: "generic-mcp", label: "Generic MCP", detail: ".mcp.json" },
    { id: "claude", label: "Claude Code", detail: "skills" },
    { id: "cursor", label: "Cursor", detail: "MCP" },
    { id: "factory", label: "Factory", detail: "MCP" },
    { id: "helix", label: "Helix", detail: "editor command" },
    { id: "neovim", label: "Neovim", detail: "editor command" },
    { id: "omp", label: "OMP", detail: "MCP and skills" },
    { id: "opencode", label: "OpenCode", detail: "MCP" },
    { id: "vscode", label: "VS Code", detail: "MCP" },
  ];

  let { open = $bindable(false), busy = false, error = "", onadd }: Props = $props();
  let root = $state("");
  let selected = $state<string[]>([]);
  const valid = $derived(root.trim().startsWith("/") && root.trim() !== "/");

  function toggle(id: string) {
    selected = selected.includes(id) ? selected.filter((value) => value !== id) : [...selected, id];
  }

  function submit(event: SubmitEvent) {
    event.preventDefault();
    if (valid && !busy) onadd(root.trim(), selected);
  }

  $effect(() => {
    if (!open) {
      root = "";
      selected = [];
    }
  });
</script>

<Dialog bind:open title="Add a project" description="Register a local folder and build its first code index." width={680}>
  <form class="add-project" onsubmit={submit}>
    <Field
      label="Absolute project path"
      hint="Choose the project root, not your home folder or the filesystem root."
      placeholder="/home/you/Work/project"
      mono
      autofocus
      bind:value={root}
    />

    <fieldset>
      <legend>Optional integrations</legend>
      <p>None are selected by default. Prowl still keeps its derived index out of Git.</p>
      <div class="integration-grid">
        {#each integrations as integration (integration.id)}
          <button
            type="button"
            class:selected={selected.includes(integration.id)}
            aria-pressed={selected.includes(integration.id)}
            onclick={() => toggle(integration.id)}
          >
            <span>{integration.label}</span>
            <small>{integration.detail}</small>
          </button>
        {/each}
      </div>
    </fieldset>

    {#if error}<p class="form-error" role="alert">{error}</p>{/if}
  </form>

  {#snippet footer()}
    <Button variant="quiet" onclick={() => (open = false)}>Cancel</Button>
    <Button variant="plate" busy={busy} armed={valid} onclick={() => onadd(root.trim(), selected)}>Add and index</Button>
  {/snippet}
</Dialog>

<style>
  .add-project { display: grid; gap: var(--s5); }
  fieldset { min-width: 0; border: 0; }
  legend { color: var(--ink); font-size: var(--f-row); font-weight: 500; }
  fieldset > p { margin-top: var(--s1); color: var(--ink-mute); font-size: var(--f-small); }
  .integration-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--s2); margin-top: var(--s3); }
  .integration-grid button { display: grid; grid-template-columns: 1fr auto; align-items: baseline; gap: var(--s2); min-height: var(--row-h); padding: 0 var(--s3); border: 1px solid var(--line-soft); border-radius: var(--radius); color: var(--ink-dim); text-align: left; }
  .integration-grid button:hover { border-color: var(--line-strong); background: var(--tint5); }
  .integration-grid button.selected { border-color: var(--bone); background: var(--bone); color: var(--ink-on-bone); }
  .integration-grid span { font-size: var(--f-small); font-weight: 500; }
  .integration-grid small { color: var(--ink-faint); font-family: var(--mono); font-size: var(--f-tiny); }
  .integration-grid button.selected small { color: color-mix(in srgb, var(--ink-on-bone) 65%, transparent); }
  .form-error { color: var(--alert); font-size: var(--f-small); }
  @media (max-width: 600px) { .integration-grid { grid-template-columns: 1fr; } }
</style>
