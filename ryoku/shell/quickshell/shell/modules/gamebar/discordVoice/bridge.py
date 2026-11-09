#!/usr/bin/env python3
"""Ryoku Discord voice bridge.

The shell speaks newline-delimited JSON on stdin and stdout. Voice state comes
from one of two places: Discord's native RPC socket for the official client, or
a companion plugin inside Vesktop that pushes the same shape over a local Unix
socket. Only the Python standard library is used, and the OAuth token is never
printed.
"""

from __future__ import annotations

import asyncio
import json
import os
import signal
import stat
import struct
import sys
import urllib.request
from pathlib import Path
from typing import Any

APP_ID = "207646673902501888"  # Discord StreamKit, the public voice RPC client
SCOPES = ("rpc", "rpc.voice.read", "rpc.voice.write")
TOKEN_ENDPOINT = "https://streamkit.discord.com/overlay/token"
AUTHORIZE_TIMEOUT = 12.0
COMPANION_WRITE_TIMEOUT = 2.0
COMPANION_LIMIT = 512 * 1024
COMPANION_NAME = "ryoku-discord-voice.sock"
VOICE_EVENTS = ("VOICE_STATE_CREATE", "VOICE_STATE_UPDATE", "VOICE_STATE_DELETE",
                "SPEAKING_START", "SPEAKING_STOP")

HANDSHAKE, FRAME, CLOSE, PING, PONG = range(5)


def emit(kind: str, **fields: Any) -> None:
    print(json.dumps({"type": kind, **fields}, separators=(",", ":"), ensure_ascii=True), flush=True)


class VoiceBridge:
    def __init__(self) -> None:
        self.reader: asyncio.StreamReader | None = None
        self.writer: asyncio.StreamWriter | None = None
        self.rpc_path = ""
        self.nonce = 0
        self.pending: dict[str, str] = {}
        self.auth_tasks: dict[str, asyncio.Task[None]] = {}
        self.rejected: set[str] = set()
        self.users: dict[str, dict[str, Any]] = {}
        self.channel: dict[str, Any] | None = None
        self.subscribed_channel = ""
        self.muted = False
        self.deafened = False

        cache = Path(os.environ.get("XDG_CACHE_HOME") or (Path.home() / ".cache"))
        self.token_file = cache / "ryoku" / "discord-voice.json"

        runtime = os.environ.get("XDG_RUNTIME_DIR")
        self.companion_path = Path(runtime) / COMPANION_NAME if runtime else None
        self.companion_server: asyncio.Server | None = None
        self.companion: asyncio.StreamWriter | None = None
        self.companion_inode: int | None = None
        self.companion_active = False
        self.companion_digest = ""

    def token(self) -> str:
        try:
            os.chmod(self.token_file, 0o600)
            return str(json.loads(self.token_file.read_text())["access_token"])
        except (OSError, KeyError, ValueError):
            return ""

    def save_token(self, token: str) -> None:
        self.token_file.parent.mkdir(parents=True, exist_ok=True)
        fd = os.open(self.token_file, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            json.dump({"access_token": token}, handle)

    def clear_token(self) -> None:
        try:
            self.token_file.unlink()
        except OSError:
            pass

    def next_nonce(self) -> str:
        self.nonce += 1
        return str(self.nonce)

    async def send_frame(self, payload: dict[str, Any], opcode: int = FRAME) -> None:
        if not self.writer or self.writer.is_closing():
            return
        body = json.dumps(payload).encode()
        self.writer.write(struct.pack("<II", opcode, len(body)) + body)
        await self.writer.drain()

    async def command(self, name: str, args: dict[str, Any] | None = None) -> None:
        nonce = self.next_nonce()
        self.pending[nonce] = name
        payload: dict[str, Any] = {"cmd": name, "nonce": nonce}
        if args is not None:
            payload["args"] = args
        await self.send_frame(payload)

    async def subscribe(self, event: str, channel_id: str = "") -> None:
        nonce = self.next_nonce()
        self.pending[nonce] = "SUB_" + event
        payload: dict[str, Any] = {"cmd": "SUBSCRIBE", "evt": event, "nonce": nonce}
        if channel_id:
            payload["args"] = {"channel_id": channel_id}
        await self.send_frame(payload)

    async def unsubscribe_channel(self) -> None:
        channel_id = self.subscribed_channel
        self.subscribed_channel = ""
        if not channel_id:
            return
        for event in VOICE_EVENTS:
            nonce = self.next_nonce()
            self.pending[nonce] = "UNSUB_" + event
            await self.send_frame({
                "cmd": "UNSUBSCRIBE", "evt": event, "nonce": nonce,
                "args": {"channel_id": channel_id},
            })

    async def subscribe_channel(self, channel_id: str) -> None:
        if self.subscribed_channel == channel_id:
            return
        await self.unsubscribe_channel()
        for event in VOICE_EVENTS:
            await self.subscribe(event, channel_id)
        self.subscribed_channel = channel_id

    @staticmethod
    def candidates() -> list[str]:
        runtime = os.environ.get("XDG_RUNTIME_DIR", "/tmp")
        roots = (runtime, f"{runtime}/app/com.discordapp.Discord", "/tmp")
        return [f"{root}/discord-ipc-{index}" for root in roots for index in range(10)]

    async def connect(self) -> bool:
        if self.companion_active and self.companion and not self.companion.is_closing():
            return True
        if self.writer and not self.writer.is_closing():
            return True

        for path in self.candidates():
            if path in self.rejected or not os.path.exists(path):
                continue
            try:
                reader, writer = await asyncio.open_unix_connection(path)
                self.reader, self.writer, self.rpc_path = reader, writer, path
                await self.send_frame({"v": 1, "client_id": APP_ID}, HANDSHAKE)
                try:
                    opcode, ready = await asyncio.wait_for(
                        self.receive(reader), AUTHORIZE_TIMEOUT)
                except asyncio.TimeoutError as error:
                    raise ConnectionError("the RPC handshake timed out") from error
                if opcode == CLOSE or ready.get("evt") != "READY":
                    raise ConnectionError("the client rejected the RPC handshake")
                emit("backend", backend="discord")
                emit("connected", socket=path)
                asyncio.create_task(self.native_loop(reader, writer))
                token = self.token()
                if token:
                    await self.authenticate(token)
                else:
                    emit("auth_required")
                return True
            except (OSError, ConnectionError, asyncio.IncompleteReadError, ValueError):
                self.close_native()

        emit("unavailable", message="Discord is not running")
        return False

    async def start_companion_server(self) -> None:
        if self.companion_path is None:
            return
        try:
            current = self.companion_path.lstat()
            if not stat.S_ISSOCK(current.st_mode) or current.st_uid != os.getuid():
                emit("companion_error", message="Refusing an unsafe Ryoku voice socket path")
                return
            self.companion_path.unlink()
        except FileNotFoundError:
            pass

        previous = os.umask(0o177)
        try:
            self.companion_server = await asyncio.start_unix_server(
                self.companion_client, path=self.companion_path, limit=COMPANION_LIMIT)
        finally:
            os.umask(previous)
        self.companion_inode = self.companion_path.stat().st_ino

    async def companion_client(self, reader: asyncio.StreamReader,
                               writer: asyncio.StreamWriter) -> None:
        if self.companion and self.companion is not writer:
            self.companion.close()
        self.companion = writer
        try:
            while not writer.is_closing():
                try:
                    line = await reader.readline()
                except ValueError:
                    continue
                if not line:
                    break
                try:
                    message = json.loads(line)
                except (ValueError, TypeError):
                    continue
                if not isinstance(message, dict):
                    continue
                state = message.get("state") if message.get("type") == "state" else None
                if isinstance(state, dict):
                    self.companion_state(state)
        except OSError:
            pass
        finally:
            if self.companion is writer:
                self.companion = None
                if self.companion_active:
                    self.companion_active = False
                    self.companion_digest = ""
                    self.channel = None
                    self.users.clear()
                    emit("disconnected", reason="The Vesktop companion stopped")
                    await self.connect()
            writer.close()
            try:
                await asyncio.wait_for(writer.wait_closed(), COMPANION_WRITE_TIMEOUT)
            except (asyncio.TimeoutError, OSError):
                pass

    def companion_state(self, state: dict[str, Any], force: bool = False) -> None:
        if state.get("version") != 1 or state.get("backend") != "vesktop":
            return
        stable = {key: value for key, value in state.items() if key != "timestamp"}
        digest = json.dumps(stable, sort_keys=True, separators=(",", ":"))
        if not force and digest == self.companion_digest:
            return

        self.close_native()
        for task in self.auth_tasks.values():
            task.cancel()
        self.auth_tasks.clear()
        self.pending.clear()

        self.companion_active = True
        self.companion_digest = digest
        self.channel = state.get("channel") or None
        self.users = {str(user.get("id")): user for user in state.get("users") or []
                      if isinstance(user, dict) and user.get("id")}
        self.muted = bool(state.get("mute"))
        self.deafened = bool(state.get("deaf"))

        emit("backend", backend="vesktop")
        emit("authenticated", user=state.get("user") or {})
        emit("voice_channel", channel=self.channel, users=list(self.users.values()))
        emit("voice_settings", mute=self.muted, deaf=self.deafened)

    async def companion_command(self, **fields: bool) -> None:
        if not self.companion or self.companion.is_closing():
            return
        writer = self.companion
        writer.write((json.dumps({"type": "command", **fields}, separators=(",", ":")) + "\n").encode())
        try:
            await asyncio.wait_for(writer.drain(), COMPANION_WRITE_TIMEOUT)
        except (asyncio.TimeoutError, OSError):
            emit("companion_error", message="The Vesktop companion stopped reading")
            writer.close()

    def close_native(self) -> None:
        if self.writer:
            self.writer.close()
        self.reader = None
        self.writer = None
        self.rpc_path = ""
        self.subscribed_channel = ""

    async def receive(self, reader: asyncio.StreamReader | None = None) -> tuple[int, dict[str, Any]]:
        source = reader or self.reader
        if source is None:
            raise ConnectionError("no Discord socket")
        header = await source.readexactly(8)
        opcode, length = struct.unpack("<II", header)
        body = await source.readexactly(length)
        return opcode, json.loads(body.decode())

    async def authenticate(self, token: str) -> None:
        nonce = self.next_nonce()
        self.pending[nonce] = "AUTHENTICATE"
        await self.send_frame({"cmd": "AUTHENTICATE", "args": {"access_token": token}, "nonce": nonce})

    async def authorize(self) -> None:
        if self.companion_active:
            return
        # connect() also succeeds when the companion took over while the RPC
        # candidate was opening; nothing native is left to authorize then.
        if not await self.connect() or self.companion_active:
            return
        emit("authorizing")
        nonce = self.next_nonce()
        self.pending[nonce] = "AUTHORIZE"
        await self.send_frame({"cmd": "AUTHORIZE", "args": {
            "client_id": APP_ID, "scopes": list(SCOPES), "prompt": "none"
        }, "nonce": nonce})
        self.auth_tasks[nonce] = asyncio.create_task(self.authorize_timeout(nonce, self.rpc_path))

    def clear_auth_task(self, nonce: str) -> None:
        task = self.auth_tasks.pop(nonce, None)
        if task and task is not asyncio.current_task():
            task.cancel()

    async def authorize_timeout(self, nonce: str, path: str) -> None:
        await asyncio.sleep(AUTHORIZE_TIMEOUT)
        if self.pending.pop(nonce, None) != "AUTHORIZE":
            return
        self.auth_tasks.pop(nonce, None)
        self.rejected.add(path)
        self.close_native()
        if await self.connect():
            await self.authorize()
            return
        emit("error", message=(
            "This client does not support Discord voice authorization. "
            "Vesktop needs the Ryoku voice companion; use Install voice "
            "companion in the Voice widget."))

    @staticmethod
    def exchange(code: str) -> str:
        request = urllib.request.Request(
            TOKEN_ENDPOINT,
            data=json.dumps({"code": code}).encode(),
            headers={
                "Content-Type": "application/json",
                "User-Agent": "Ryoku/DiscordVoice",
            },
            method="POST",
        )
        with urllib.request.urlopen(request, timeout=10) as response:
            return str(json.loads(response.read().decode())["access_token"])

    async def native_loop(self, reader: asyncio.StreamReader,
                          writer: asyncio.StreamWriter) -> None:
        try:
            while not writer.is_closing():
                opcode, payload = await self.receive(reader)
                if opcode == PING:
                    await self.send_frame(payload, PONG)
                elif opcode == CLOSE:
                    break
                elif opcode == FRAME:
                    await self.frame(payload)
        except (OSError, asyncio.IncompleteReadError, ValueError):
            pass
        if self.writer is writer:
            for task in self.auth_tasks.values():
                task.cancel()
            self.auth_tasks.clear()
            self.pending.clear()
            self.close_native()
            self.users.clear()
            self.channel = None
            if not self.companion_active:
                emit("disconnected", reason="Discord RPC closed")

    async def frame(self, payload: dict[str, Any]) -> None:
        nonce = payload.get("nonce", "")
        event = payload.get("evt", "")
        if event == "ERROR":
            command = self.pending.pop(nonce, "")
            self.clear_auth_task(nonce)
            if command == "AUTHENTICATE":
                self.clear_token()
                emit("auth_required")
            elif command.startswith("UNSUB_"):
                # A best-effort cleanup of a channel we already left.
                return
            else:
                emit("error", message=(payload.get("data") or {}).get("message", "Discord RPC error"))
            return
        if nonce in self.pending:
            self.clear_auth_task(nonce)
            await self.response(self.pending.pop(nonce), payload.get("data") or {})
            return
        if payload.get("cmd") == "DISPATCH":
            await self.dispatch(event, payload.get("data") or {})

    async def response(self, command: str, data: dict[str, Any]) -> None:
        if command == "AUTHORIZE":
            code = data.get("code", "")
            if not code:
                emit("error", message="Discord authorization was not completed")
                return
            try:
                token = await asyncio.to_thread(self.exchange, code)
                self.save_token(token)
                await self.authenticate(token)
            except Exception:
                emit("error", message="Discord token exchange failed")
        elif command == "AUTHENTICATE":
            emit("authenticated", user=data.get("user", {}))
            await self.subscribe("VOICE_CHANNEL_SELECT")
            await self.subscribe("VOICE_SETTINGS_UPDATE")
            await self.command("GET_SELECTED_VOICE_CHANNEL")
        elif command == "GET_SELECTED_VOICE_CHANNEL":
            await self.select_channel(data if data and data.get("id") else None)
        elif command in ("GET_VOICE_SETTINGS", "SET_VOICE_SETTINGS"):
            self.muted = bool(data.get("mute"))
            self.deafened = bool(data.get("deaf"))
            emit("voice_settings", mute=self.muted, deaf=self.deafened)

    @staticmethod
    def voice_user(state: dict[str, Any]) -> dict[str, Any]:
        user = state.get("user", {})
        voice = state.get("voice_state", {})
        return {
            "id": user.get("id", ""),
            "username": user.get("username", ""),
            "nick": state.get("nick") or user.get("global_name") or user.get("username", ""),
            "avatar": user.get("avatar", ""),
            "mute": bool(voice.get("mute") or voice.get("self_mute")),
            "deaf": bool(voice.get("deaf") or voice.get("self_deaf")),
            "speaking": False,
        }

    async def select_channel(self, data: dict[str, Any] | None) -> None:
        if not data or not data.get("id"):
            await self.unsubscribe_channel()
            self.users.clear()
            self.channel = None
            emit("voice_channel", channel=None, users=[])
            return
        await self.subscribe_channel(data["id"])
        self.users.clear()
        # A refresh can repeat only the channel id. Keep the name and guild that
        # this same channel already reported instead of degrading the label.
        previous = self.channel or {}
        same = previous.get("id") == data["id"]
        self.channel = {
            "id": data["id"],
            "name": data.get("name") or (previous.get("name") if same else "")
                or "Voice channel",
            "guild_id": data.get("guild_id") or (previous.get("guild_id") if same else ""),
        }
        for state in data.get("voice_states", []):
            user = self.voice_user(state)
            if user["id"]:
                self.users[user["id"]] = user
        emit("voice_channel", channel=self.channel, users=list(self.users.values()))
        await self.command("GET_VOICE_SETTINGS")

    async def dispatch(self, event: str, data: dict[str, Any]) -> None:
        if event == "VOICE_CHANNEL_SELECT":
            await self.command("GET_SELECTED_VOICE_CHANNEL")
        elif event in ("VOICE_STATE_CREATE", "VOICE_STATE_UPDATE"):
            user = self.voice_user(data)
            old = self.users.get(user["id"], {})
            user["speaking"] = old.get("speaking", False)
            if user["id"]:
                self.users[user["id"]] = user
            emit("voice_state", users=list(self.users.values()))
        elif event == "VOICE_STATE_DELETE":
            self.users.pop((data.get("user") or {}).get("id", ""), None)
            emit("voice_state", users=list(self.users.values()))
        elif event in ("SPEAKING_START", "SPEAKING_STOP"):
            user_id = data.get("user_id", "")
            if user_id in self.users:
                self.users[user_id]["speaking"] = event == "SPEAKING_START"
            emit("voice_state", users=list(self.users.values()))
        elif event == "VOICE_SETTINGS_UPDATE":
            self.muted = bool(data.get("mute"))
            self.deafened = bool(data.get("deaf"))
            emit("voice_settings", mute=self.muted, deaf=self.deafened)

    async def input_loop(self) -> None:
        loop = asyncio.get_running_loop()
        while True:
            line = await loop.run_in_executor(None, sys.stdin.readline)
            if not line:
                return
            try:
                message = json.loads(line)
                command = message.get("cmd")
                if command == "connect":
                    await self.connect()
                elif command == "authorize":
                    await self.authorize()
                elif command == "set_voice_settings":
                    fields = {key: bool(message[key]) for key in ("mute", "deaf") if key in message}
                    if self.companion_active:
                        await self.companion_command(**fields)
                    elif self.writer:
                        await self.command("SET_VOICE_SETTINGS", fields)
                elif command == "disconnect":
                    self.close_native()
            except (ValueError, OSError):
                emit("error", message="Invalid bridge command")

    async def shutdown(self) -> None:
        self.close_native()
        if self.companion:
            self.companion.close()
        if self.companion_server:
            self.companion_server.close()
            await self.companion_server.wait_closed()
        if self.companion_path and self.companion_inode is not None:
            try:
                if self.companion_path.stat().st_ino == self.companion_inode:
                    self.companion_path.unlink()
            except OSError:
                pass


async def main() -> None:
    bridge = VoiceBridge()
    loop = asyncio.get_running_loop()
    task = asyncio.current_task()
    if task is not None:
        loop.add_signal_handler(signal.SIGTERM, task.cancel)
    await bridge.start_companion_server()
    emit("ready")
    interrupted = False
    try:
        await bridge.input_loop()
    except asyncio.CancelledError:
        interrupted = True
    finally:
        await bridge.shutdown()
        loop.remove_signal_handler(signal.SIGTERM)
    if interrupted:
        # The stdin read sits in an executor thread that stays blocked while the
        # parent keeps the pipe open. The socket is already unlinked, so leave.
        os._exit(0)


if __name__ == "__main__":
    asyncio.run(main())
