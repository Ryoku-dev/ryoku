// Hash routes keep the console compatible with the daemon's static server
// and the companion window. A sheet is the first segment; the rest belongs
// to that sheet.

export interface Sheet {
  id: string;
  label: string;
  /** the real Japanese word, sealed beside the Latin name */
  gloss: string;
}

export const SHEETS: readonly Sheet[] = [
  { id: "ryoku", label: "Ryoku", gloss: "力" },
  { id: "chat", label: "Chat", gloss: "対話" },
  { id: "wiki", label: "Wiki", gloss: "手引" },
  { id: "overview", label: "Overview", gloss: "概要" },
  { id: "system", label: "System", gloss: "演算" },
  { id: "vault", label: "Vault", gloss: "書庫" },
  { id: "memory", label: "Memory", gloss: "記憶" },
  { id: "skills", label: "Skills", gloss: "技" },
  { id: "prowl", label: "Prowl", gloss: "徘徊" },
  { id: "about", label: "About", gloss: "案内" },
];

export const DEFAULT_SHEET = "ryoku";

function parse(hash: string): { sheet: string; rest: string[] } {
  const parts = hash.replace(/^#\/?/, "").split("/").filter(Boolean).map(decodeURIComponent);
  const sheet = parts[0] && SHEETS.some((s) => s.id === parts[0]) ? parts[0] : DEFAULT_SHEET;
  return { sheet, rest: parts.slice(1) };
}

export class Router {
  sheet = $state(DEFAULT_SHEET);
  rest = $state<string[]>([]);

  start(): void {
    const sync = () => {
      const requested = location.hash.replace(/^#\/?/, "").split("/")[0];
      if (requested === "ask") {
        history.replaceState(null, "", "#/ryoku");
        this.sheet = "ryoku";
        this.rest = [];
        return;
      }
      const { sheet, rest } = parse(location.hash);
      this.sheet = sheet;
      this.rest = rest;
    };
    sync();
    window.addEventListener("hashchange", sync);
  }

  go(sheet: string, ...rest: string[]): void {
    location.hash = "#/" + [sheet, ...rest].map(encodeURIComponent).join("/");
  }
}

export const router = new Router();
