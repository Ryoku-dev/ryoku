import type { Project, ProjectStatus } from "../types";

export type ProjectState = Project["state"];

export interface ProjectOverview {
  counts?: ProjectStatus["counts"];
  roles?: Record<string, number>;
  docs?: string[];
  entrypoints?: string[];
  entrypoint_count?: number;
  clusters?: Array<{ label: string; lang: string; files: number }>;
  keybinds?: number;
  hotspots?: Array<{ file: string; in: number }>;
}

export interface SemanticCoverage {
  embedded: number;
  chunks: number;
  remaining: number;
  percent: number;
  label: string;
}

export function deriveProjectState(project: Project): ProjectState {
  if (project.error || project.job?.error) return "error";
  if (project.job) return "indexing";
  return project.state;
}

export function semanticCoverage(status?: ProjectStatus): SemanticCoverage {
  const chunks = Math.max(0, status?.semantic.chunks ?? 0);
  const embedded = Math.min(chunks, Math.max(0, status?.semantic.embedded ?? 0));
  const remaining = Math.max(0, chunks - embedded);
  const percent = chunks === 0 ? 0 : Math.round((embedded / chunks) * 100);
  const label = chunks === 0
    ? "No embeddable chunks"
    : status?.semantic.complete || embedded === chunks
      ? `${embedded} of ${chunks} chunks embedded`
      : `${embedded} of ${chunks} chunks embedded, ${remaining} remaining`;
  return { embedded, chunks, remaining, percent, label };
}

export function stateLabel(state: ProjectState): string {
  switch (state) {
    case "semantic building": return "Semantic building";
    case "not indexed": return "Not indexed";
    case "ready": return "Ready";
    case "indexing": return "Indexing";
    case "error": return "Error";
  }
}

export function projectBusy(project: Project): boolean {
  return deriveProjectState(project) === "indexing";
}
