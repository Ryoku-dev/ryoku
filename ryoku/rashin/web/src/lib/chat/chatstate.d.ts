// Types for the shell's chat reducer (ryoku/shell/quickshell/shell/services/
// lib/chatstate.js), aliased as $chatstate by vite.config.ts. The shapes
// mirror what applyEvent builds; the reducer itself stays plain JS so
// Quickshell and the node tests keep loading it unchanged.

import type { ApprovalsMode, BannerState, CommandInfo, ModelInfo, PermOption, PromptImage, SessionMeta, ToolDiff, WsOut } from "./protocol";

export interface MsgItem {
  kind: "msg";
  id: string;
  role: "user" | "agent";
  text: string;
  thought: string;
  /** continues the same reply below a tool call */
  cont: boolean;
  /** still streaming */
  open: boolean;
  failed: boolean;
  images: PromptImage[];
}

export interface ToolItem {
  kind: "tool";
  id: string;
  title: string;
  /** the tool category (read, edit, execute, ...) */
  kind2: string;
  status: string;
  input: string;
  output: string;
  diffs: ToolDiff[];
  auto: boolean;
}

export type Item = MsgItem | ToolItem;

export interface Permission {
  requestId: string;
  toolId: string;
  title: string;
  kind: string;
  input: string;
  options: PermOption[];
}

export interface ChatState {
  items: Item[];
  permissions: Permission[];
  banner: { state: BannerState; error: string };
  approvals: ApprovalsMode;
  busy: boolean;
  seq: number;
  models: ModelInfo[];
  currentModel: string;
  agent: string;
  prowl: "" | "active" | "pending";
  prowlReason: string;
  commands: CommandInfo[];
  session: { id: string; title: string };
  usage: { size: number; used: number } | null;
  history: SessionMeta[];
  replaying: boolean;
  activity: string;
}

export function initialState(): ChatState;
export function applyEvent(state: ChatState, ev: WsOut): ChatState;
