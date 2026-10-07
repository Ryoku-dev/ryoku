function trimDecimal(value: number): string {
  return value.toFixed(1).replace(/\.0$/, "");
}

export function formatTokens(value: number | null | undefined): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return "Unknown";
  const absolute = Math.abs(value);
  if (absolute >= 1_000_000_000) return `${trimDecimal(value / 1_000_000_000)}b`;
  if (absolute >= 1_000_000) return `${trimDecimal(value / 1_000_000)}m`;
  if (absolute >= 1_000) return `${trimDecimal(value / 1_000)}k`;
  return Math.round(value).toString();
}

export function formatLatency(milliseconds: number | null | undefined): string {
  if (milliseconds === null || milliseconds === undefined || !Number.isFinite(milliseconds)) return "Unknown";
  if (Math.abs(milliseconds) < 1_000) return `${Math.round(milliseconds)} ms`;
  return `${trimDecimal(milliseconds / 1_000)} s`;
}

export function formatPercent(ratio: number | null | undefined): string {
  if (ratio === null || ratio === undefined || !Number.isFinite(ratio)) return "Unknown";
  return `${trimDecimal(ratio * 100)}%`;
}

export function formatCost(value: number | null | undefined, known = value !== null && value !== undefined): string {
  if (!known || value === null || value === undefined || !Number.isFinite(value)) return "Unknown";
  const digits = value !== 0 && Math.abs(value) < 0.01 ? 4 : 2;
  return `$${value.toFixed(digits)}`;
}

export function parseInstant(value: string | number | null | undefined): Date | null {
  if (value === null || value === undefined) return null;

  let timestamp: number;
  if (typeof value === "number") {
    timestamp = Math.abs(value) < 1_000_000_000_000 ? value * 1_000 : value;
  } else {
    const input = value.trim();
    if (!input) return null;
    if (/^[+-]?(?:\d+(?:\.\d+)?|\.\d+)$/.test(input)) {
      const numeric = Number(input);
      timestamp = Math.abs(numeric) < 1_000_000_000_000 ? numeric * 1_000 : numeric;
    } else {
      const sqliteUTC = /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(?:\.\d+)?$/;
      timestamp = Date.parse(sqliteUTC.test(input) ? `${input.replace(" ", "T")}Z` : input);
    }
  }

  if (!Number.isFinite(timestamp)) return null;
  const instant = new Date(timestamp);
  return Number.isNaN(instant.getTime()) ? null : instant;
}

export function relativeTime(value: string | number | null | undefined, now = new Date()): string {
  const instant = parseInstant(value);
  if (!instant) return "Unknown";
  const seconds = Math.round((now.getTime() - instant.getTime()) / 1_000);
  const future = seconds < 0;
  const elapsed = Math.abs(seconds);
  if (elapsed < 5) return "now";

  let amount: number;
  let unit: string;
  if (elapsed < 60) {
    amount = elapsed;
    unit = "s";
  } else if (elapsed < 3_600) {
    amount = Math.floor(elapsed / 60);
    unit = "m";
  } else if (elapsed < 86_400) {
    amount = Math.floor(elapsed / 3_600);
    unit = "h";
  } else {
    amount = Math.floor(elapsed / 86_400);
    unit = "d";
  }
  return future ? `in ${amount}${unit}` : `${amount}${unit} ago`;
}

export function maskKey(value: string | null | undefined): string {
  if (!value) return "Not issued";
  const separator = value.indexOf("-");
  const prefix = separator >= 0 ? value.slice(0, separator + 1) : "";
  if (value.length <= prefix.length + 4) return `${prefix}••••`;
  return `${prefix}••••${value.slice(-4)}`;
}
