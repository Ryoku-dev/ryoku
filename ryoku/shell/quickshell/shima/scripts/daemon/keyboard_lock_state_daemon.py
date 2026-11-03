#!/usr/bin/env python3

import argparse
import asyncio
import atexit
import fcntl
import json
import os
from pathlib import Path
import signal
import sys
import time

from evdev import InputDevice, ecodes, list_devices

IGNORED_NAME_PARTS = ("ydotool", "virtual")
RELEVANT_KEY_CODES = {ecodes.KEY_CAPSLOCK, ecodes.KEY_NUMLOCK}
RELEVANT_LED_CODES = {ecodes.LED_CAPSL, ecodes.LED_NUML}


def _monitor_paths():
    runtime_dir = Path(os.environ.get("XDG_RUNTIME_DIR") or "/tmp")
    prefix = "inir-keyboard-lock-state" if runtime_dir != Path("/tmp") else f"inir-{os.getuid()}-keyboard-lock-state"
    return runtime_dir / f"{prefix}.pid", runtime_dir / f"{prefix}.lock"


def _is_owned_monitor(pid):
    if pid <= 1 or pid == os.getpid():
        return False

    try:
        cmdline = Path(f"/proc/{pid}/cmdline").read_bytes().replace(b"\0", b" ").decode("utf-8", "replace")
    except OSError:
        return False

    return "keyboard_lock_state_daemon.py" in cmdline and "--once" not in cmdline


def claim_monitor_process():
    """Ensure only the newest persistent monitor for this user stays alive."""
    pid_path, lock_path = _monitor_paths()
    lock_path.parent.mkdir(parents=True, exist_ok=True)

    with lock_path.open("a+") as lock_file:
        fcntl.flock(lock_file.fileno(), fcntl.LOCK_EX)

        old_pid = 0
        try:
            old_pid = int(pid_path.read_text().strip())
        except (OSError, ValueError):
            pass

        if _is_owned_monitor(old_pid):
            try:
                os.kill(old_pid, signal.SIGTERM)
            except ProcessLookupError:
                pass

            deadline = time.monotonic() + 1.0
            while _is_owned_monitor(old_pid) and time.monotonic() < deadline:
                time.sleep(0.05)

            if _is_owned_monitor(old_pid):
                try:
                    os.kill(old_pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass

        pid_path.write_text(f"{os.getpid()}\n")

    def cleanup_pid_file():
        try:
            if pid_path.read_text().strip() == str(os.getpid()):
                pid_path.unlink()
        except OSError:
            pass

    atexit.register(cleanup_pid_file)


class KeyboardLockMonitor:
    def __init__(self):
        self.devices = {}
        self.tasks = {}
        self.last_state = None
        self.inspected = set()  # readable device nodes opened by the last refresh, keyboards or not

    def _is_candidate(self, dev):
        name = (dev.name or "").lower()
        if any(part in name for part in IGNORED_NAME_PARTS):
            return False

        caps = dev.capabilities()
        key_caps = set(caps.get(ecodes.EV_KEY, []))
        led_caps = set(caps.get(ecodes.EV_LED, []))
        return bool(key_caps & RELEVANT_KEY_CODES) and bool(led_caps & RELEVANT_LED_CODES)

    def _aggregate(self, values, previous):
        if not values:
            return previous if previous is not None else False

        true_count = sum(1 for value in values if value)
        false_count = len(values) - true_count
        if true_count == false_count:
            return previous if previous is not None else False
        return true_count > false_count

    def _snapshot(self):
        caps_values = []
        num_values = []

        for dev in self.devices.values():
            try:
                active_leds = set(dev.leds())
            except OSError:
                continue

            caps_values.append(ecodes.LED_CAPSL in active_leds)
            num_values.append(ecodes.LED_NUML in active_leds)

        if not caps_values and not num_values:
            return None

        previous_caps = self.last_state["caps"] if self.last_state is not None else None
        previous_num = self.last_state["num"] if self.last_state is not None else None
        return {
            "caps": self._aggregate(caps_values, previous_caps),
            "num": self._aggregate(num_values, previous_num),
            "devices": len(self.devices),
        }

    async def emit_state(self, force=False):
        state = self._snapshot()
        if state is None:
            return

        next_state = {"caps": state["caps"], "num": state["num"]}
        if force or next_state != self.last_state:
            print(json.dumps({"type": "state", **state}), flush=True)
        self.last_state = next_state

    async def refresh_devices(self):
        discovered = {}
        inspected = set()
        for path in list_devices():
            try:
                dev = InputDevice(path)
            except OSError:
                continue
            inspected.add(path)

            try:
                if not self._is_candidate(dev):
                    dev.close()
                    continue
            except OSError:
                dev.close()
                continue

            discovered[path] = dev

        removed_paths = [path for path in self.devices.keys() if path not in discovered]
        for path in removed_paths:
            task = self.tasks.pop(path, None)
            if task is not None:
                task.cancel()
            try:
                self.devices[path].close()
            except OSError:
                pass
            self.devices.pop(path, None)

        for path, dev in discovered.items():
            if path in self.devices:
                dev.close()
                continue

            self.devices[path] = dev
            self.tasks[path] = asyncio.create_task(self.monitor_device(path))
        self.inspected = inspected

    def _forget(self, path):
        """A keyboard that went away: the next refresh opens whatever takes its node."""
        self.tasks.pop(path, None)
        dev = self.devices.pop(path, None)
        if dev is not None:
            try:
                dev.close()
            except OSError:
                pass
        self.inspected.discard(path)

    async def monitor_device(self, path):
        dev = self.devices[path]
        try:
            async for event in dev.async_read_loop():
                if event.type == ecodes.EV_LED and event.code in RELEVANT_LED_CODES:
                    await self.emit_state()
                    continue

                if event.type == ecodes.EV_KEY and event.code in RELEVANT_KEY_CODES and event.value == 0:
                    await asyncio.sleep(0.03)
                    await self.emit_state()
        except asyncio.CancelledError:
            return
        except OSError:
            self._forget(path)
            return

    async def run(self):
        await self.refresh_devices()
        if not self.devices:
            return 1

        await self.emit_state(force=True)

        while True:
            await asyncio.sleep(5)
            # Opening every input device to read its capabilities is the costly part: only when the readable set
            # changed (a hotplug, udev granting the seat's access after creating a node, a keyboard that left).
            if set(list_devices()) != self.inspected:
                await self.refresh_devices()
            if self.devices:
                await self.emit_state()

    async def run_once(self):
        await self.refresh_devices()
        if not self.devices:
            return 1

        await self.emit_state(force=True)
        return 0

    async def close(self):
        for task in self.tasks.values():
            task.cancel()
        for task in list(self.tasks.values()):
            try:
                await task
            except asyncio.CancelledError:
                pass
        for dev in self.devices.values():
            try:
                dev.close()
            except OSError:
                pass


async def async_main(run_once):
    monitor = KeyboardLockMonitor()
    try:
        return await (monitor.run_once() if run_once else monitor.run())
    finally:
        await monitor.close()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--once", action="store_true")
    args = parser.parse_args()

    if not args.once:
        claim_monitor_process()

    try:
        code = asyncio.run(async_main(args.once))
    except KeyboardInterrupt:
        code = 0
    except Exception as exc:
        print(json.dumps({"type": "error", "message": str(exc)}), file=sys.stderr, flush=True)
        code = 1

    raise SystemExit(code)


if __name__ == "__main__":
    main()
