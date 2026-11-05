// One connection per conversation lane. Frames from each /ws/chat hub fold
// into the shared reducer independently, so the two transcripts never leak
// into one another.

import { JsonSocket } from "$lib/api/socket";
import { applyEvent, initialState, type ChatState } from "$chatstate";
import type { ApprovalsMode, PromptImage, WsIn, WsOut } from "./protocol";

export type ChatLane = "ryoku" | "chat";

export class ChatStore {
  state = $state.raw<ChatState>(initialState());
  connected = $state(false);
  draft = $state("");
  loadingSessionId = $state("");
  readonly lane: ChatLane;
  private outbox: WsIn[] = [];
  private socket: JsonSocket<WsOut> | null = null;

  constructor(lane: ChatLane) {
    this.lane = lane;
    this.draft = localStorage.getItem(this.preferenceKey("draft")) ?? "";
  }

  private preferenceKey(name: string): string {
    return `rashin.${this.lane}.${name}`;
  }

  open(): void {
    if (this.socket) return;
    this.socket = new JsonSocket<WsOut>({
      path: `/ws/chat?lane=${this.lane}`,
      onFrame: (f) => this.apply(f),
      onOpen: () => {
        this.connected = true;
        const queued = this.outbox;
        this.outbox = [];
        for (const q of queued) this.socket?.send(q);
      },
      onClose: () => {
        this.connected = false;
      },
    });
    this.socket.open();
  }

  close(): void {
    this.socket?.close();
    this.socket = null;
    this.connected = false;
  }

  apply(frame: WsOut): void {
    this.state = applyEvent(this.state, frame);
    if (frame.type === "replay_end" && frame.sessionId === this.loadingSessionId) {
      this.loadingSessionId = "";
    }
  }

  private post(cmd: WsIn): void {
    if (!this.socket?.send(cmd)) this.outbox.push(cmd);
  }

  send(text: string, images: PromptImage[] = []): void {
    const trimmed = text.trim();
    if (trimmed.length === 0 && images.length === 0) return;
    this.apply({ type: "user", text: trimmed, images });
    this.post({ type: "user", text: trimmed, images: images.length ? images : undefined });
    this.setDraft("");
  }
  handoff(askId: string, question: string, answer: string, run: boolean): void {
    this.post({ type: "handoff", askId, question, answer, run });
  }


  setDraft(text: string): void {
    this.draft = text;
    if (text) localStorage.setItem(this.preferenceKey("draft"), text);
    else localStorage.removeItem(this.preferenceKey("draft"));
  }

  cancel(): void {
    this.post({ type: "cancel" });
  }

  newChat(): void {
    this.post({ type: "new" });
  }

  loadSessions(): void {
    this.post({ type: "history" });
  }

  switchSession(id: string): void {
    if (!id) return;
    this.loadingSessionId = id;
    this.apply({ type: "replay_start", sessionId: id });
    this.post({ type: "load", sessionId: id });
  }

  setModel(id: string): void {
    if (id && (id !== this.state.currentModel || id !== this.state.pick)) this.post({ type: "set_model", modelId: id });
  }

  setApprovals(mode: ApprovalsMode): void {
    this.post({ type: "approvals", mode });
  }

  answerPermission(requestId: string, optionId: string): void {
    this.post({ type: "permission", requestId, optionId });
  }
}

export const ryoku = new ChatStore("ryoku");
export const chat = new ChatStore("chat");
