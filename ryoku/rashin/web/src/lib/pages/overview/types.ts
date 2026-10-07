import type { Vitals } from "$lib/state/machine.svelte";
import type { ProjectStatus } from "$lib/pages/prowl/types";

export type { Vitals };

export interface DoctorFinding {
  name: string;
  status: "ok" | "note" | "fixed" | "todo" | "warn" | "fail" | string;
  detail: string;
  remedy?: string;
}

export interface DoctorScan {
  findings: DoctorFinding[] | null;
  collectedAt: string;
  report?: string;
  error?: string;
}

export interface ProwlDaemonStatus {
  installed: boolean;
  bin?: string;
  version?: string;
  running: boolean;
  port?: number;
  url?: string;
  repo?: string;
  error?: string;
}

export type CodeStatus = ProjectStatus;

export interface ProwlHit {
  file: string;
  start_line: number;
  end_line: number;
  snippet?: string;
}

export interface FixRequest {
  kind: "tip" | "doctor" | "app";
  id?: string;
  name?: string;
  app?: string;
  note?: string;
}

export interface FixAnswer {
  ok: boolean;
  display?: string;
  harness?: string;
}

export type FixPhase = "idle" | "opening" | "opened" | "failed";

export interface FixState {
  phase: FixPhase;
  message?: string;
}

export function formatBytes(value: number): string {
  if (!Number.isFinite(value) || value < 0) return "—";
  const gib = value / (1024 * 1024 * 1024);
  return `${gib.toFixed(1)} GiB`;
}

export function formatUptime(seconds: number): string {
  let value = Math.max(0, Math.floor(Number(seconds) || 0));
  const days = Math.floor(value / 86400);
  value %= 86400;
  const hours = Math.floor(value / 3600);
  const minutes = Math.floor((value % 3600) / 60);
  const parts: string[] = [];
  if (days) parts.push(`${days}d`);
  if (hours || days) parts.push(`${hours}h`);
  parts.push(`${minutes}m`);
  return parts.join(" ");
}

export function formatTokens(value: number): string {
  const n = Number(value) || 0;
  if (n >= 1e9) return `${(n / 1e9).toFixed(n >= 1e10 ? 0 : 1)}B`;
  if (n >= 1e6) return `${(n / 1e6).toFixed(n >= 1e7 ? 0 : 1)}M`;
  if (n >= 1e3) return `${(n / 1e3).toFixed(n >= 1e4 ? 0 : 1)}K`;
  return String(n);
}

export function percent(used: number, total: number): number {
  if (!Number.isFinite(used) || !Number.isFinite(total) || total <= 0) return 0;
  return Math.max(0, Math.min(100, (used / total) * 100));
}

export function repoName(path: string | undefined): string {
  const clean = (path ?? "").replace(/\/+$/, "");
  return clean.split("/").pop() || clean;
}

export function attentionFindings(scan: DoctorScan | null, includeTodo = true): DoctorFinding[] {
  const rank: Record<string, number> = { fail: 0, warn: 1, todo: 2 };
  return (scan?.findings ?? [])
    .filter((finding) => finding.status in rank && (includeTodo || finding.status !== "todo"))
    .sort((a, b) => (rank[a.status] ?? 99) - (rank[b.status] ?? 99));
}
