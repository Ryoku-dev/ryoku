<script lang="ts">
  import { untrack } from "svelte";
  import Page from "$lib/app/Page.svelte";
  import { router } from "$lib/app/router.svelte";
  import { prowl } from "$lib/api/prowl";
  import Button from "$lib/ui/Button.svelte";
  import Dialog from "$lib/ui/Dialog.svelte";
  import Empty from "$lib/ui/Empty.svelte";
  import Field from "$lib/ui/Field.svelte";
  import AddProjectDialog from "./AddProjectDialog.svelte";
  import ProjectCard from "./ProjectCard.svelte";
  import ProjectDetail from "./ProjectDetail.svelte";
  import type { Project } from "../types";
  import type { ProjectOverview } from "./projects";

  interface Props { params: string[] }
  let { params }: Props = $props();

  let projects = $state<Project[]>([]);
  let loading = $state(true);
  let error = $state("");
  let actionError = $state("");
  let addError = $state("");
  let addOpen = $state(false);
  let adding = $state(false);
  let busyRoot = $state("");
  let removeProject = $state<Project | null>(null);
  let removeOpen = $state(false);
  let removing = $state(false);
  let overview = $state<ProjectOverview | null>(null);
  let overviewError = $state("");
  let copiedRoot = $state("");
  let filter = $state("");
  let pollGeneration = 0;

  const selectedRoot = $derived(params[0] ?? "");
  const selectedProject = $derived(projects.find((project) => project.root === selectedRoot) ?? null);
  const ready = $derived(projects.filter((project) => project.state === "ready" || project.state === "semantic building").length);
  const filteredProjects = $derived.by(() => {
    const query = filter.trim().toLowerCase();
    return query
      ? projects.filter((project) => project.name.toLowerCase().includes(query) || project.root.toLowerCase().includes(query))
      : projects;
  });

  function upsert(project: Project) {
    const index = projects.findIndex((row) => row.root === project.root);
    projects = index < 0
      ? [...projects, project].sort((a, b) => a.root.localeCompare(b.root))
      : projects.map((row) => row.root === project.root ? project : row);
  }

  async function loadProjects() {
    loading = projects.length === 0;
    error = "";
    try {
      projects = (await prowl.projects.list()).projects;
    } catch (reason) {
      error = reason instanceof Error
        ? `${reason.message} Check that Prowl's gateway is running, then retry.`
        : "Projects could not be read. Check that Prowl's gateway is running, then retry.";
    } finally {
      loading = false;
    }
  }

  async function loadOverview(root: string) {
    overview = null;
    overviewError = "";
    try {
      overview = await prowl.code.overview({ repo: root }) as unknown as ProjectOverview;
    } catch (reason) {
      overviewError = reason instanceof Error ? reason.message : "Prowl did not return a repository map.";
    }
  }

  async function pollProject(root: string) {
    const generation = ++pollGeneration;
    while (generation === pollGeneration) {
      await new Promise((resolve) => window.setTimeout(resolve, 1_000));
      if (generation !== pollGeneration) return;
      try {
        const rows = (await prowl.projects.list()).projects;
        projects = rows;
        const current = rows.find((project) => project.root === root);
        if (current && (!current.job || current.job.error || current.state === "error")) {
          if (current.job?.error || current.error) actionError = current.job?.error || current.error || "Indexing failed.";
          if (selectedRoot === root && current.status) await loadOverview(root);
          busyRoot = "";
          return;
        }
        actionError = "";
      } catch (reason) {
        actionError = reason instanceof Error
          ? `${reason.message} Prowl will keep checking the indexing job.`
          : "The indexing job could not be checked. Prowl will keep trying.";
      }
    }
  }

  async function addProject(root: string, integrations: string[]) {
    if (adding) return;
    adding = true;
    addError = "";
    actionError = "";
    try {
      const response = await prowl.projects.add({ root, integrations });
      upsert(response.project);
      addOpen = false;
      busyRoot = response.project.root;
      void pollProject(response.project.root);
    } catch (reason) {
      addError = reason instanceof Error
        ? `${reason.message} Check the path and try again.`
        : "The project was not added. Check the path and try again.";
    } finally {
      adding = false;
    }
  }

  async function reindex(root: string) {
    if (busyRoot) return;
    busyRoot = root;
    actionError = "";
    try {
      const response = await prowl.projects.reindex({ root });
      upsert(response.project);
      void pollProject(root);
    } catch (reason) {
      actionError = reason instanceof Error
        ? `${reason.message} Check the project path, then retry.`
        : "Reindexing did not start. Check the project path, then retry.";
      busyRoot = "";
    }
  }
  function askRemove(project: Project) {
    removeProject = project;
    removeOpen = true;
  }


  async function confirmRemove() {
    if (!removeProject || removing) return;
    const project = removeProject;
    removing = true;
    actionError = "";
    try {
      await prowl.projects.remove(project.root);
      projects = projects.filter((row) => row.root !== project.root);
      removeOpen = false;
      if (selectedRoot === project.root) router.go("prowl", "projects");
    } catch (reason) {
      actionError = reason instanceof Error
        ? `${reason.message} Wait for indexing to finish, then retry.`
        : "The project was not removed. Wait for indexing to finish, then retry.";
    } finally {
      removing = false;
    }
  }

  function openProject(root: string) {
    router.go("prowl", "projects", root);
  }

  async function copyPath(root: string) {
    try {
      await navigator.clipboard.writeText(root);
      copiedRoot = root;
      window.setTimeout(() => {
        if (copiedRoot === root) copiedRoot = "";
      }, 1_500);
    } catch {
      actionError = "The project path could not be copied. Select the path and copy it manually.";
    }
  }
  $effect(() => {
    if (!removeOpen) removeProject = null;
  });


  $effect(() => {
    const root = params[0] ?? "";
    untrack(() => {
      void loadProjects().then(() => {
        if (root && projects.some((project) => project.root === root)) void loadOverview(root);
        const running = projects.find((project) => project.job && !project.job.error);
        if (running) {
          busyRoot = running.root;
          void pollProject(running.root);
        }
      });
    });
    return () => { pollGeneration++; };
  });
</script>

<Page title="Project indexes" gloss="索引" lead="Registered repositories Prowl keeps current for cited code answers.">
  {#if loading}
    <Empty title="Reading project indexes" body="Prowl is checking every registered repository." />
  {:else if error && projects.length === 0}
    <Empty title="Projects are unavailable" body={error}>
      {#snippet action()}<Button variant="plate" onclick={() => void loadProjects()}>Retry</Button>{/snippet}
    </Empty>
  {:else}
    <div class="projects-page">
      {#if error}<p class="action-error" role="alert">{error}</p>{/if}
      {#if actionError}<p class="action-error" role="alert">{actionError}</p>{/if}

      {#if selectedRoot}
        {#if selectedProject}
          <ProjectDetail
            project={selectedProject}
            {overview}
            {overviewError}
            busy={busyRoot === selectedProject.root}
            copied={copiedRoot === selectedProject.root}
            onback={() => router.go("prowl", "projects")}
            oncopy={(root) => void copyPath(root)}
            onreindex={(root) => void reindex(root)}
            onremove={askRemove}
          />
        {:else}
          <Empty title="Project not found" body="This repository is no longer registered. Return to the project list and choose another index.">
            {#snippet action()}<Button variant="plate" onclick={() => router.go("prowl", "projects")}>All projects</Button>{/snippet}
          </Empty>
        {/if}
      {:else}
        <div class="summary">
          <div><strong>{projects.length}</strong><span>registered</span></div>
          <div><strong>{ready}</strong><span>ready</span></div>
          <p>Adding a project indexes it in the background. Source files stay in place.</p>
          <Button variant="plate" onclick={() => { addError = ""; addOpen = true; }}>Add project</Button>
        </div>

        {#if projects.length}
          <div class="project-filter">
            <Field label="Filter projects" placeholder="Name or absolute path" bind:value={filter} />
          </div>
        {/if}

        {#if filteredProjects.length}
          <div class="project-grid">
            {#each filteredProjects as project (project.root)}
              <ProjectCard
                {project}
                busy={busyRoot === project.root}
                copied={copiedRoot === project.root}
                onopen={openProject}
                oncopy={(root) => void copyPath(root)}
                onreindex={(root) => void reindex(root)}
                onremove={askRemove}
              />
            {/each}
          </div>
        {:else if projects.length}
          <Empty title="No matching projects" body="Try a shorter name or clear the project filter." />
        {:else}
          <Empty title="No projects indexed" body="Add an absolute project path. Prowl will register it and build the first index without changing source files." />
        {/if}

        <a class="toolkit-link" href="#/prowl/toolkit">How Prowl indexes and queries a project</a>
      {/if}
    </div>
  {/if}
</Page>

<AddProjectDialog bind:open={addOpen} busy={adding} error={addError} onadd={(root, integrations) => void addProject(root, integrations)} />

<Dialog
  bind:open={removeOpen}
  title="Remove this project?"
  description="Prowl will unregister the project only. Its files and .prowl directory stay on disk."
>
  <p class="remove-copy"><strong>{removeProject?.name}</strong><code>{removeProject?.root}</code></p>
  {#snippet footer()}
    <Button variant="quiet" onclick={() => (removeOpen = false)}>Keep project</Button>
    <Button variant="plate" autofocus busy={removing} onclick={() => void confirmRemove()}>Remove project</Button>
  {/snippet}
</Dialog>

<style>
  .projects-page { display: grid; gap: var(--s4); }
  .action-error { padding: var(--s3); border: 1px solid color-mix(in srgb, var(--alert) 45%, transparent); border-radius: var(--radius); color: var(--alert); font-size: var(--f-small); }
  .summary { display: grid; grid-template-columns: auto auto minmax(0, 1fr) auto; align-items: center; gap: var(--s5); padding: var(--s4) var(--s5); border: 1px solid var(--line-soft); border-radius: var(--radius); }
  .summary > div { display: flex; align-items: baseline; gap: var(--s2); }
  .summary strong { color: var(--ink); font-family: var(--display); font-size: 32px; font-weight: 400; }
  .summary span, .summary p { color: var(--ink-mute); font-size: var(--f-small); }
  .project-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 430px), 1fr)); gap: var(--s4); align-items: start; }
  .project-filter { max-width: 420px; }
  .toolkit-link { justify-self: start; color: var(--ink-dim); font-size: var(--f-small); text-decoration: underline; text-decoration-color: var(--line-strong); text-underline-offset: 3px; }
  .toolkit-link:hover { color: var(--ink); }
  .remove-copy { display: grid; gap: var(--s2); }
  .remove-copy strong { color: var(--ink); font-weight: 500; }
  .remove-copy code { overflow-wrap: anywhere; color: var(--ink-mute); font-size: var(--f-micro); }
  @media (max-width: 760px) {
    .summary { grid-template-columns: auto auto 1fr; }
    .summary p { grid-column: 1 / -1; grid-row: 2; }
  }
  @media (max-width: 520px) {
    .summary { grid-template-columns: 1fr 1fr; }
  }
</style>
