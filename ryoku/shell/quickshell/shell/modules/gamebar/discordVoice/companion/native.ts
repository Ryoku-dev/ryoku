/*
 * Ryoku Discord Voice companion for Vencord
 * SPDX-License-Identifier: GPL-3.0-or-later
 */

import type { IpcMainInvokeEvent } from "electron";
import { createConnection, Socket } from "net";
import { join } from "path";

const runtime = process.env.XDG_RUNTIME_DIR || "";
const socketPath = runtime ? join(runtime, "ryoku-discord-voice.sock") : "";

let socket: Socket | undefined;
let connecting = false;
let latest = "";
let incoming = "";
const queue: string[] = [];
const waiters: Array<(line: string) => void> = [];

function deliver(line: string) {
    const waiter = waiters.shift();
    if (waiter) waiter(line);
    else queue.push(line);
}

function reset() {
    incoming = "";
    connecting = false;
}

function ensure() {
    if (!socketPath || connecting)
        return;
    if (socket && !socket.destroyed)
        return;

    connecting = true;
    incoming = "";
    const candidate = createConnection(socketPath);
    socket = candidate;
    candidate.setEncoding("utf8");

    candidate.on("connect", () => {
        connecting = false;
        if (latest)
            candidate.write(latest);
    });

    candidate.on("data", (chunk: string) => {
        incoming += chunk;
        let newline = incoming.indexOf("\n");
        while (newline >= 0) {
            const line = incoming.slice(0, newline);
            incoming = incoming.slice(newline + 1);
            if (line)
                deliver(line);
            newline = incoming.indexOf("\n");
        }
    });

    candidate.on("error", () => {
        reset();
        candidate.destroy();
    });

    candidate.on("close", () => {
        reset();
        if (socket === candidate)
            socket = undefined;
    });
}

export function publishState(_: IpcMainInvokeEvent, json: string) {
    latest = JSON.stringify({ type: "state", state: JSON.parse(json) }) + "\n";
    ensure();
    if (socket?.writable && !connecting)
        socket.write(latest);
}

export function nextCommand(_: IpcMainInvokeEvent): Promise<string> {
    const queued = queue.shift();
    if (queued !== undefined)
        return Promise.resolve(queued);
    return new Promise(resolve => waiters.push(resolve));
}

export function disconnect(_: IpcMainInvokeEvent) {
    socket?.destroy();
    socket = undefined;
    reset();
    for (const waiter of waiters.splice(0))
        waiter("");
}
