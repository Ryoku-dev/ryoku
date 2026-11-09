/*
 * Ryoku Discord Voice companion for Vencord
 * SPDX-License-Identifier: GPL-3.0-or-later
 *
 * Vesktop's local RPC proxy answers Rich Presence but not authenticated voice
 * RPC, so the Ryoku Game Bar needs this plugin to publish the call over a small
 * local socket. Official Discord needs none of this.
 */

import definePlugin, { PluginNative } from "@utils/types";
import { findByPropsLazy, findStoreLazy } from "@webpack";
import {
    ChannelRTCStore,
    ChannelStore,
    GuildMemberStore,
    SelectedChannelStore,
    UserStore,
    VoiceStateStore
} from "@webpack/common";

const Native = VencordNative.pluginHelpers.RyokuDiscordVoice as PluginNative<typeof import("./native")>;
const Audio = findByPropsLazy("toggleSelfMute", "toggleSelfDeaf");
const SpeakingStore = findStoreLazy("SpeakingStore");

let heartbeat: ReturnType<typeof setInterval> | undefined;
let running = false;
let busy = false;
let again = false;

function participant(userId: string, state: any, guildId?: string) {
    const user = UserStore.getUser(userId);
    if (!user)
        return null;
    return {
        id: user.id,
        username: user.username,
        nick: (guildId && GuildMemberStore.getNick(guildId, user.id)) || user.globalName || user.username,
        avatar: user.avatar || "",
        mute: Boolean(state.mute || state.selfMute),
        deaf: Boolean(state.deaf || state.selfDeaf),
        speaking: Boolean(state.speaking)
    };
}

function activeSpeakers(channelId: string | null | undefined) {
    const ids = new Set<string>();
    if (channelId) {
        for (const peer of ChannelRTCStore.getSpeakingParticipants(channelId) || []) {
            if (peer?.user?.id)
                ids.add(peer.user.id);
        }
    }
    const me = UserStore.getCurrentUser();
    if (me && SpeakingStore?.isSpeaking(me.id))
        ids.add(me.id);
    return ids;
}

function snapshot() {
    const channelId = SelectedChannelStore.getVoiceChannelId();
    const channel = channelId ? ChannelStore.getChannel(channelId) : null;
    const states = channelId ? VoiceStateStore.getVoiceStatesForChannel(channelId) || {} : {};
    const speaking = activeSpeakers(channelId);
    const me = UserStore.getCurrentUser();

    const users = Object.entries(states)
        .map(([userId, state]) => participant(userId, { ...(state as object), speaking: speaking.has(userId) }, channel?.guild_id))
        .filter(Boolean);

    const own = me ? VoiceStateStore.getVoiceStateForUser(me.id) : null;
    return {
        version: 1,
        backend: "vesktop",
        timestamp: Date.now(),
        user: me ? { id: me.id, username: me.username, avatar: me.avatar || "" } : null,
        channel: channel ? {
            id: channel.id,
            name: channel.name || "Voice channel",
            guild_id: channel.guild_id || ""
        } : null,
        users,
        mute: Boolean(own?.mute || own?.selfMute),
        deaf: Boolean(own?.deaf || own?.selfDeaf)
    };
}

function applyCommand(line: string) {
    if (!line)
        return;
    try {
        const command = JSON.parse(line);
        if (command.type !== "command")
            return;
        const current = snapshot();
        if (typeof command.mute === "boolean" && command.mute !== current.mute)
            Audio.toggleSelfMute();
        if (typeof command.deaf === "boolean" && command.deaf !== current.deaf)
            Audio.toggleSelfDeaf();
    } catch {
        // A malformed command is not worth tearing the socket down for.
    }
}

async function commandLoop() {
    while (running) {
        const line = await Native.nextCommand();
        if (running)
            applyCommand(line);
    }
}

async function publish() {
    if (busy) {
        again = true;
        return;
    }
    busy = true;
    try {
        do {
            again = false;
            await Native.publishState(JSON.stringify(snapshot()));
        } while (again);
    } finally {
        busy = false;
        again = false;
    }
}

export default definePlugin({
    name: "RyokuDiscordVoice",
    description: "Shares Vesktop voice state with the Ryoku Game Bar",
    authors: [{ name: "Ryoku", id: 0n }],
    enabledByDefault: true,

    flux: {
        VOICE_STATE_UPDATES() { void publish(); },
        AUDIO_TOGGLE_SELF_MUTE() { void publish(); },
        AUDIO_TOGGLE_SELF_DEAF() { void publish(); },
        SPEAKING() { void publish(); },
        STOP_SPEAKING() { void publish(); }
    },

    start() {
        running = true;
        void commandLoop();
        void publish();
        heartbeat = setInterval(() => void publish(), 4000);
    },

    stop() {
        running = false;
        if (heartbeat)
            clearInterval(heartbeat);
        heartbeat = undefined;
        void Native.disconnect();
    }
});
