#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""Frozen-snapshot acceptance orchestration; real nested UI/audio sessions.

suite WM OUT checkout|package runs phase8..17 afresh, then five distinct
sessions. index OUT CELL... admits exactly checkout/package x niri/hyprland
with identical frozen sources and binaries. A green cell is NOT matrix parity.
No live-session mode exists. All evidence and temporary files are private
/dev/shm; archive them only after teardown. Detailed feature safety remains
in the composed gates, not duplicated as permissive persona approximations.

The model is a deterministic local HTTP fixture, speech is synthesized into a
private microphone, and the target is an actual GTK application. This proves
interaction, routing and refusal, NOT model quality, human speech accuracy,
production billing, real hardware or a live development-desktop deployment.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import sys
import time

sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[4]
PERSONAS = ("keyboard-first", "mouse-first", "voice-first", "interrupt-heavy", "second-language")
BINS = ("oni", "hyakki", "hyakki-app", "oni-surface")
PHASES = tuple(range(8, 18))
EXCLUDED = ("autostart", "quiet-watch", "tuck", "opacity", "actual whole-desktop Hands")


def stamp():
    import datetime
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def dump(path, value):
    path = Path(path)
    part = path.with_name(path.name + ".part")
    part.write_text(json.dumps(value, indent=2, ensure_ascii=False, default=str) + "\n")
    os.replace(part, path)


def sha(path):
    h = hashlib.sha256()
    with Path(path).open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(",", ":")).encode()).hexdigest()


def private_out(path):
    path = Path(path).absolute()
    # A symlinked parent must not redirect output into the checkout or a
    # session home. Existing evidence is never replaced by a retry.
    if not path.resolve().is_relative_to(Path("/dev/shm")) or path.resolve() == Path("/dev/shm"):
        raise RuntimeError("OUT must be a fresh private directory under /dev/shm")
    if path.exists() or path.is_symlink():
        raise RuntimeError(f"refusing existing OUT: {path}")
    path.mkdir(parents=True, mode=0o700)
    os.chmod(path, 0o700)
    return path.resolve()


def identity():
    """Hash actual corresponding source, including untracked inputs/deletions.

    Read-only git inventory is used instead of assuming HEAD describes WIP.
    Runtime/build/cache outputs are excluded explicitly; an inventory records
    the exact boundary. Binary hashes do not pretend to prove build provenance.
    """
    r = subprocess.run(["git", "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--",
                        "ryoku/hyakki", "ryoku/i18n", "ryoku/wm/oniwindow",
                        "ryoku/wm/reflex", "docs", ":(top,glob)*"],
                       cwd=ROOT, capture_output=True, check=True)
    rows = []
    for raw in sorted(set(r.stdout.split(b"\0")) - {b""}):
        rel = os.fsdecode(raw)
        p = Path(rel)
        if any(x in ("__pycache__", ".cache", "build", "node_modules", ".git") for x in p.parts):
            continue
        full = ROOT / p
        if full.is_symlink():
            if full.is_dir():
                raise RuntimeError(f"source directory symlink cannot define a frozen inventory: {rel}")
            rows.append({"path": rel, "symlink": os.readlink(full),
                         "target_sha256": sha(full) if full.is_file() else None})
        elif full.is_file():
            rows.append({"path": rel, "sha256": sha(full), "mode": stat.S_IMODE(full.stat().st_mode)})
        elif not full.exists():
            rows.append({"path": rel, "deleted": True})
        else:
            raise RuntimeError(f"unhashable source input {rel}")
    if not rows:
        raise RuntimeError("no corresponding source inventory")
    binary_dir = Path(os.environ["HYAKKI_BIN"]).resolve(strict=True)
    binaries = {}
    for name in BINS:
        p = binary_dir / name
        if not p.is_file() or not os.access(p, os.X_OK):
            raise RuntimeError(f"missing executable {p}")
        binaries[name] = sha(p)
    for p in sorted(binary_dir.glob("*.test")):
        if not p.is_file() or not os.access(p, os.X_OK):
            raise RuntimeError(f"invalid feature gate test binary {p}")
        binaries[p.name] = sha(p)
    revision = subprocess.run(["git", "rev-parse", "HEAD"], cwd=ROOT, capture_output=True, text=True, check=True).stdout.strip()
    return {"source_root": str(ROOT), "head": revision, "source_scope": "ryoku/hyakki,ryoku/i18n,docs,root files; git cached+untracked nonignored; excludes build/cache/node_modules",
            "source_sha256": digest(rows), "source_files": rows, "binary_root": str(binary_dir),
            "binaries": binaries, "binary_sha256": digest(binaries)}


def stable(before):
    after = identity()
    if before["source_sha256"] != after["source_sha256"] or before["binaries"] != after["binaries"]:
        raise RuntimeError("source or binaries changed: this entire run is invalid; freeze and rerun ALL gates")


def receipt(path):
    value = json.loads(Path(path).read_text())
    steps = value.get("steps")
    if value.get("pass") is not True or not isinstance(steps, list) or not steps:
        raise RuntimeError(f"nonpassing or empty receipt: {path}")
    if any(s.get("pass") is not True for s in steps):
        raise RuntimeError(f"failed/malformed step: {path}")
    return value


def artifacts(out):
    rows = []
    for p in sorted(Path(out).rglob("*")):
        if p.is_symlink():
            raise RuntimeError(f"evidence contains symlink: {p}")
        if p.is_file() and p.name not in ("artifacts.json", "phase18.json", "matrix.json"):
            rows.append({"path": str(p.relative_to(out)), "bytes": p.stat().st_size, "sha256": sha(p)})
    dump(Path(out) / "artifacts.json", rows)
    return digest(rows)


def command(argv, logfile, timeout=10800):
    import signal
    start = time.monotonic()
    with Path(logfile).open("wb") as stream:
        process = subprocess.Popen(list(map(str, argv)), stdout=stream, stderr=subprocess.STDOUT,
                                   stdin=subprocess.DEVNULL, start_new_session=True,
                                   env={**os.environ, "PYTHONDONTWRITEBYTECODE": "1"})
        try:
            rc = process.wait(timeout=timeout)
            return {"command": list(map(str, argv)), "rc": rc, "seconds": time.monotonic() - start}
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGTERM)
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait()
            return {"command": list(map(str, argv)), "rc": 124, "seconds": time.monotonic() - start, "error": "deadline"}


def suite(wm, where, execution_root):
    out = private_out(where)
    result = {"started_at": stamp(), "wm": wm, "execution_root": execution_root, "steps": [], "pass": False,
              "matrix_complete": False, "excluded_by_contract": EXCLUDED,
              "limits": ["Nested checkout-backed lab is not live development-desktop proof.",
                         "Package-stage root is not a fresh native package installation; phase17 packaging receipts remain separate.",
                         "Deterministic model/synthetic speech do not measure model quality or human speech accuracy.",
                         "HTTP 402 fixture proves provider refusal/no local-feature lockout, not production Account binding or billing.",
                         "Translation completeness is deferred; second-language input is tested with English control labels.",
                         "The nested niri winit backend has one output; multi-output is exercised in the Hyprland cells only."]}
    frozen = None
    try:
        if not os.environ.get("ONI_LAB_ID"):
            raise RuntimeError("set a unique ONI_LAB_ID; owner allows at most two lab slots machine-wide")
        import re
        lab_id = os.environ["ONI_LAB_ID"]
        if not re.fullmatch("[a-z0-9]{1,3}", lab_id):
            raise RuntimeError("ONI_LAB_ID must be 1–3 lowercase alphanumeric characters")
        lab_path = Path("/dev/shm") / f"oni-lab-{lab_id}"
        if os.environ.get("ONI_LAB", str(lab_path)) != str(lab_path) or lab_path.exists() or lab_path.is_symlink():
            raise RuntimeError("lab path disagrees with ID or is already occupied; never tear down somebody else's lab")
        if not os.environ.get("HYAKKI_BIN") or not Path(os.environ["HYAKKI_BIN"]).is_absolute():
            raise RuntimeError("HYAKKI_BIN must select exact frozen binaries by absolute path")
        if not os.environ.get("ONI_LAB_SECOND_LANGUAGE_STT_MODEL"):
            raise RuntimeError("stage a multilingual STT model and set ONI_LAB_SECOND_LANGUAGE_STT_MODEL (not an .en model)")
        if os.environ["ONI_LAB_SECOND_LANGUAGE_STT_MODEL"].endswith(".en"):
            raise RuntimeError("second-language session cannot use an English-only model")
        frozen = identity()
        if execution_root == "package":
            package = Path(os.environ["ONI_LAB_PACKAGE_ROOT"]).resolve(strict=True)
            if not Path(frozen["binary_root"]).is_relative_to(package) or package.is_relative_to(ROOT):
                raise RuntimeError("package root must be an external staged payload containing HYAKKI_BIN")
            result["package_root"] = str(package)
        dump(out / "snapshot.json", frozen)
        result["snapshot"] = {k: frozen[k] for k in ("head", "source_sha256", "binary_sha256", "binaries", "binary_root")}
        # Always run the complete ordered feature suite, even after an ordinary
        # red gate. A changed snapshot stops immediately: mixed proof is invalid.
        for phase in PHASES:
            stable(frozen)
            name = f"phase{phase}"
            argv = ["sh", HERE / f"{name}.sh", wm, out / name]
            row = {"name": name, **command(argv, out / f"{name}.log")}
            try:
                v = receipt(out / name / f"{name}.json")
                row.update(pass_=row["rc"] == 0, limits=v.get("limits", []), receipt_sha256=sha(out / name / f"{name}.json"))
                row["pass"] = row.pop("pass_")
            except (OSError, ValueError, RuntimeError) as e:
                row.update({"pass": False, "error": str(e)})
            cleanup = command(["sh", HERE / "down.sh"], out / f"{name}-down.txt", timeout=180)
            if cleanup["rc"]:
                row.update({"pass": False, "teardown_error": cleanup})
            result["steps"].append(row)
            stable(frozen)
            dump(out / "phase18.json", result)
        for persona in PERSONAS:
            stable(frozen)
            dst = out / persona
            dst.mkdir(mode=0o700)
            row = {"name": persona, **command(["sh", HERE / "phase18.sh", "session", wm, dst, persona], out / f"{persona}.log")}
            try:
                v = receipt(dst / "persona.json")
                row.update({"pass": row["rc"] == 0, "receipt_sha256": sha(dst / "persona.json"), "limits": v.get("limits", [])})
            except (OSError, ValueError, RuntimeError) as e:
                row.update({"pass": False, "error": str(e)})
            cleanup = command(["sh", HERE / "down.sh"], out / f"{persona}-down.txt", timeout=180)
            if cleanup["rc"]:
                row.update({"pass": False, "teardown_error": cleanup})
            result["steps"].append(row)
            stable(frozen)
            dump(out / "phase18.json", result)
        result["pass"] = len(result["steps"]) == len(PHASES) + len(PERSONAS) and all(x["pass"] for x in result["steps"])
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError) as e:
        result["error"] = str(e)
        result["pass"] = False
    finally:
        # Teardown failures never disappear behind a green child receipt.
        if frozen is not None:
            row = command(["sh", HERE / "down.sh"], out / "final-down.txt", timeout=180)
            if row["rc"]:
                result["pass"] = False
                result["teardown_error"] = row
            try:
                stable(frozen)
            except Exception as e:
                result["pass"] = False
                result["snapshot_error"] = str(e)
        result["finished_at"] = stamp()
        result["artifacts_sha256"] = artifacts(out)
        dump(out / "phase18.json", result)
    print(json.dumps({"pass": result["pass"], "report": str(out / "phase18.json"), "matrix_complete": False}))
    return 0 if result["pass"] else 1


def matrix(where, cells):
    out = private_out(where)
    result = {"pass": False, "cells": [], "started_at": stamp()}
    try:
        if len(cells) != 4 or len({str(Path(p).resolve()) for p in cells}) != 4:
            raise RuntimeError("index requires four distinct cell directories")
        seen, snapshots = set(), set()
        for cell in cells:
            path = Path(cell).resolve() / "phase18.json"
            v = receipt(path)
            key = (v["execution_root"], v["wm"])
            if key in seen:
                raise RuntimeError(f"duplicate matrix cell {key}")
            seen.add(key)
            snapshots.add((v["snapshot"]["source_sha256"], v["snapshot"]["binary_sha256"]))
            if [r["name"] for r in v["steps"]] != [f"phase{x}" for x in PHASES] + list(PERSONAS):
                raise RuntimeError("narrowed or reordered suite is not a final acceptance receipt")
            # Rehash receipts/artifacts, so copying a stale green index over a
            # failed rerun or dropping recordings cannot admit a matrix.
            recorded = json.loads((path.parent / "artifacts.json").read_text())
            if digest(recorded) != v["artifacts_sha256"]:
                raise RuntimeError(f"artifact manifest changed: {cell}")
            for a in recorded:
                p = path.parent / a["path"]
                if not p.resolve().is_relative_to(path.parent) or p.is_symlink() or sha(p) != a["sha256"]:
                    raise RuntimeError(f"artifact missing/changed: {p}")
            result["cells"].append({"root": key[0], "wm": key[1], "report": str(path), "sha256": sha(path), "limits": v["limits"]})
        if seen != {(r, w) for r in ("checkout", "package") for w in ("niri", "hyprland")} or len(snapshots) != 1:
            raise RuntimeError("all four root/compositor cells must use the SAME exact frozen source and binaries")
        current = identity()
        if (current["source_sha256"], current["binary_sha256"]) not in snapshots:
            raise RuntimeError("source/binaries changed since the cells ran; rerun the ENTIRE suite")
        result["pass"] = True
        result["scope"] = "four isolated cells; NOT live desktop, hardware, production billing, native package installation or owner acceptance"
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError) as e:
        result["error"] = str(e)
    dump(out / "matrix.json", result)
    print(json.dumps(result, indent=2))
    return 0 if result["pass"] else 1


def target(where, variant):
    """A real GTK target with ordinary keyboard accelerators, not a UI mock.

    Same rendered controls/state/Save implementation as the guide fixture.
    Shortcuts activate the real widgets, and state records their resulting
    application state. No daemon or persona outcomes are synthesized here.
    """
    import guidewin
    from gi.repository import Gdk, Gio, Gtk
    if variant not in guidewin.VARIANTS or not Path(where).resolve().is_relative_to(Path("/dev/shm")):
        raise RuntimeError("bad target variant/directory")
    Path(where).mkdir(parents=True, exist_ok=True)
    app = Gtk.Application(application_id="dev.ryoku.OniGuide", flags=Gio.ApplicationFlags.NON_UNIQUE)
    def activate(a):
        doc = guidewin.Doc(a, "dev.ryoku.OniGuide", "Oni guide Lindqvist ledger", where, variant)
        css = guidewin.css()
        Gtk.StyleContext.add_provider_for_display(Gdk.Display.get_default(), css, Gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
        keys = Gtk.EventControllerKey()
        def pressed(_controller, key, _code, mods):
            if mods & Gdk.ModifierType.CONTROL_MASK and key in (Gdk.KEY_s, Gdk.KEY_S):
                cid = "file" if doc.state()["page"] == "doc" else "save"
                control = next((w for k, _, _, w in doc.controls if k == cid and w.is_drawable()), None)
                if control is not None:
                    control.activate()
                    return True
            return False
        keys.connect("key-pressed", pressed)
        doc.win.add_controller(keys)
        a.persona_doc = doc
    app.connect("activate", activate)
    return app.run([])


def session(lab, out, persona):
    # Import the real lab only for a session; suite/index remain usable without
    # GTK or numpy installed and can explain missing prerequisites cleanly.
    import guidekit
    import labkit
    import reflexkit
    import voicekit
    from settingskit import Phase17

    class Persona(guidekit.Phase12):
        REPORT = "persona.json"

        def __init__(self):
            super().__init__(lab, out)
            self.watch = reflexkit.PausableWatch(self.env["LAB_ID"])
            self.ui = Phase17(lab, Path(out) / "ui")
            self.events = []
            self.session_recording = None
            self.previous_question = None
            self.results.update(persona=persona, habits={
                "keyboard-first": "one orientation click to field, typed Return submission, target Ctrl+S accelerators",
                "mouse-first": "drag to perch, visible Ask/consent/target/card clicks",
                "voice-first": "held speech for guide/next/stop; only target controls use pointer",
                "interrupt-heavy": "cancel disclosure, interrupt slow read, barge-in, recover and finish",
                "second-language": "Spanish typed question and multilingual held Spanish speech; English controls retained",
            }[persona], steps=[], measurements={}, confusion={"repeated_prompts": 0, "dismissals": 0, "error_hits": 0})

        def event(self, action, **details):
            self.events.append({"at": stamp(), "wall": time.time(), "action": action, **details})
            dump(self.out / "events.json", self.events)

        def final_of(self, held, timeout=None):
            result = super().final_of(held, timeout)
            partials = result.get("partials") or []
            end = result.get("end")
            values = {"hotkey_to_listening_s": result.get("listening", result["down"]) - result["down"],
                      "first_partial_s": partials[0][0] - result["down"] if partials else None,
                      "release_to_final_s": end[0] - result["up"] if end else None}
            self.event("speech-observation", fixture=result.get("fixture"), heard=result.get("final"), latency=values)
            self.results["measurements"][f"speech-{len(self.events)}"] = {"pass": end is not None, **values}
            return result

        def must(self, name, condition, value=None):
            row = {"name": name, "pass": bool(condition), "value": value}
            self.results["steps"].append(row)
            self.save()
            if not condition:
                raise RuntimeError(name)

        def capture(self, name):
            state = self.state()
            for output in state["outputs"]:
                if not output.get("enabled"):
                    continue
                # Whole nested output only, never the host session.
                box = [output["x"], output["y"], output["width"], output["height"]]
                self.shoot(f"{name}-{output['name']}", box)
            dump(self.out / f"{name}-state.json", state)

        def gw_open(self, variant="normal"):
            self.gw_n += 1
            d = self.out / "guidewin" / f"{self.gw_n:02d}-{variant}"
            self.sh("phase18.sh", "target", d, variant)
            deadline = time.monotonic() + 15
            while time.monotonic() < deadline:
                rec = self.gw_rec()
                if rec and (d / "state.json").is_file():
                    self.gwin = {"variant": variant, "dir": d, "id": rec["id"]}
                    self.model_guide(d / "state.json")
                    return rec
                time.sleep(0.1)
            raise RuntimeError("real persona target never mapped")

        def arrange(self):
            deadline = time.monotonic() + 20
            while True:
                try:
                    app = self.ui.app_window()
                    break
                except RuntimeError:
                    if time.monotonic() >= deadline:
                        raise
                    time.sleep(.2)
            self.sh("phase14.sh", "layout", app["id"])
            if self.wm == "hyprland":
                other = next(o for o in self.state()["outputs"] if o["enabled"] and o["name"] != self.env["LAB_OUTPUT"])
                self.gw("output", other["name"])
            else:
                output = self.output(self.env["LAB_OUTPUT"], self.state())
                target = self.gw_rec()
                self.gw("move", round(output["x"] + output["width"] - target["size"][0] - 24),
                        round(output["y"] + (output["height"] - target["size"][1]) / 2))

        def model_ready(self):
            self.must("loopback model present", self.model is not None)
            self.settings(master=True, inspect=True, disclosure="always")
            self.gsettings(enabled=True, keep=True, hints=False)
            self.model.mode("point")
            self.ensure_gw()
            self.arrange()
            if persona == "mouse-first":
                self.ensure_detached()
                self.wake()
                mark = self.ledger.mark()
                c = labkit.global_rect(self.current(), self.state())
                w = self.gw_rec()
                dest = (w["pos"][0] + w["size"][0] / 2, w["pos"][1] + 40)
                self.sh("input.sh", "drag", *map(round, labkit.centre(c)), *map(round, dest))
                self.must("pointer drag attaches to target", self.perch_line(mark, 20, w["id"]) is not None)
                self.event("drag-to-perch", destination=dest)
            else:
                self.perch_gw()
            self.capture("target-ready")

        def ui_question(self, question, keyboard=False, allow=True, expect_error=False, require_consent=True):
            self.ui.route("oni-screen")
            self.ui.click_label(self.previous_question or "What does this window do?")
            self.sh("input.sh", "key", "-M", "ctrl", "-k", "a", "-m", "ctrl")
            self.sh("input.sh", "type", question)
            self.previous_question = question
            mark, n0 = self.watch.mark(), len(self.requests())
            self.event("question", text=question, submit="Return" if keyboard else "Ask Oni")
            if keyboard:
                self.sh("input.sh", "key", "-k", "Return")
            else:
                self.ui.click_label("Ask Oni", "ask")
            card = self.watch.wait("consent", lambda d: True, mark, 20, None)
            if card[1] is not None:
                self.ui.click_label("Allow once" if allow else "Cancel", "disclosure")
            if require_consent:
                self.must("question has explicit disclosure", card[1] is not None, card[1])
            if not allow:
                self.results["confusion"]["dismissals"] += 1
                time.sleep(1)
                self.must("cancel sends no model request", len(self.requests()) == n0)
                self.capture("cancelled-disclosure")
                return
            deadline = time.monotonic() + 90
            seen = []
            while time.monotonic() < deadline:
                words, _ = self.ui.app_words("question-outcome")
                seen = [w.get("text", "") for w in words]
                text = " ".join(seen).lower()
                if ("could not ask" if expect_error else "oni's answer") in text or (not expect_error and "oni’s answer" in text):
                    break
                time.sleep(0.5)
            self.must("visible error" if expect_error else "visible screen answer",
                      ("could not ask" in text) if expect_error else ("oni's answer" in text or "oni’s answer" in text), seen)
            self.must("request traversed real provider" if not expect_error else "no successful late answer",
                      len(self.requests()) > n0 if not expect_error else "oni's answer" not in text,
                      self.requests()[n0:])
            if expect_error:
                self.results["confusion"]["error_hits"] += 1
            self.capture("question-error" if expect_error else "question-answer")

        def target_act(self, cid, keyboard=False):
            before = self.gw_state()
            self.event("target-action", control=cid, method="Ctrl+S" if keyboard else "pointer")
            if keyboard:
                self.gw("focus")
                self.sh("input.sh", "key", "-M", "ctrl", "-k", "s", "-m", "ctrl")
                changed = self.wait_state(lambda s: s["seq"] > before["seq"] and
                                          (s["page"] != before["page"] or s["saves"] > before["saves"]))
            else:
                changed = self.click_and_wait(cid, lambda s: s["page"] != before["page"] or s["saves"] > before["saves"])
            self.must(f"target {cid} changed through actual input", changed is not None, changed)

        def finish_guide(self, keyboard=False):
            self.ensure_gw()
            self.perch_gw()
            mark = self.ledger.mark()
            # Guide submission is a supported CLI user surface; target and
            # continuation are physical nested input, not daemon outcome hooks.
            call = self.guide("start", guidekit.GOAL_SAVE, answer="y\n")
            for cid in ("file", "save"):
                self.must("guide answered", (call.get("result") or {}).get("status") == "answered", call)
                target_line = self.wait_target(mark, 30)
                self.must(f"guide visibly lands at {cid}", target_line is not None, target_line)
                landed = self.landing(cid, target_line, mark, f"{persona}-{cid}")
                self.must(f"strand matches rendered {cid}", bool(landed.get("geometry")) and bool(landed.get("arrived")), landed)
                self.capture(f"guide-{cid}")
                self.target_act(cid, keyboard)
                mark = self.ledger.mark()
                call = self.guide("next", answer="y\n")
            self.must("actual target saved", (self.saved() or {}).get("control") == "save", self.saved())
            result = call.get("result") or {}
            offer = result.get("keep") or {}
            kept = self.keep_cli(offer["id"]) if offer.get("id") else {}
            self.must("explicit Keep persists real completed guide", bool((kept.get("result") or {}).get("kept")), kept)
            self.ensure_gw()
            self.perch_gw()
            n0 = len(self.requests())
            start = time.monotonic()
            replay = self.guide("start", guidekit.GOAL_SAVE, answer="y\n")
            self.must("kept replay needs no provider", len(self.requests()) == n0, replay)
            self.results["measurements"]["replay"] = {"pass": True, "seconds": time.monotonic() - start,
                                                          "model_requests": len(self.requests()) - n0}
            self.guide("stop")

        def mouse_guide(self):
            self.ensure_gw()
            self.perch_gw()
            self.ui.route("oni-screen")
            self.ui.click_label("Save this file", "mouse-goal")
            self.sh("input.sh", "type", guidekit.GOAL_SAVE)
            lm, wm = self.ledger.mark(), self.watch.mark()
            self.ui.click_label("Show me how", "mouse-start")
            for cid in ("file", "save"):
                consent = self.watch.wait("consent", lambda d: True, wm, 30, None)
                self.must(f"mouse {cid}: real app disclosure", consent[1] is not None, consent[1])
                self.ui.click_label("Allow once", f"mouse-allow-{cid}")
                target_line = self.wait_target(lm, 30)
                landed = self.landing(cid, target_line, lm, f"mouse-{cid}")
                self.must(f"mouse {cid}: rendered guide landing", landed.get("geometry") and landed.get("arrived"), landed)
                self.capture(f"mouse-{cid}")
                self.target_act(cid)
                lm, wm = self.ledger.mark(), self.watch.mark()
                self.ui.route("oni-screen")
                self.ui.click_label("Next", f"mouse-next-{cid}")
            self.must("pointer-led actual document saved", (self.saved() or {}).get("control") == "save", self.saved())
            self.ui.click_label("Keep it", "mouse-keep", scroll=False)
            self.must("pointer Keep persisted completed guide", self.entry() is not None, self.keeps())
            self.event("mouse-guide-complete", actions="Show me how / Allow once / File / Next / Allow once / Save / Next / Keep it")

        def voice_guide(self):
            self.settings(disclosure="always")
            recording = voicekit.Rec(self, "voice-guide", mic=True)
            try:
                mark, lm = self.watch.mark(), self.ledger.mark()
                heard = self.final_of(self.ghold("guide"))
                card = self.watch.wait("consent", lambda d: True, mark, 60, None)
                self.must("held guide speech asks permission", card[1] is not None, heard)
                self.final_of(self.ghold("yes"))  # picture disclosure only, never Hands approval
                for index, cid in enumerate(("file", "save"), 1):
                    target_line = self.wait_target(lm, 60)
                    self.must(f"spoken guide points step {index}", target_line is not None, target_line)
                    self.capture(f"spoken-{cid}")
                    self.target_act(cid)
                    lm, mark = self.ledger.mark(), self.watch.mark()
                    self.final_of(self.ghold("next"))
                    if index == 1:
                        card = self.watch.wait("consent", lambda d: True, mark, 60, None)
                        self.must("next picture has fresh disclosure", card[1] is not None)
                        self.final_of(self.ghold("yes"))
                self.must("voice-led session saves actual document", (self.saved() or {}).get("control") == "save", self.saved())
                self.final_of(self.ghold("stop"))
            finally:
                recording.stop()
            self.must("voice session has output PCM beyond sync", bool(recording.segments()), recording.segments())
            self.event("voice-led-guide", source="held chord FIFO + isolated microphone", heard=heard)

        def interrupt(self):
            self.ui_question("Where is the save button?", keyboard=True, allow=False)
            self.model.mode("slow", delay=8)
            mark = self.watch.mark()
            ask = self.guide_popen("start", guidekit.GOAL_SAVE)
            try:
                card = self.watch.wait("consent", lambda d: True, mark, 20, None)
                self.must("slow request waits for disclosure", card[1] is not None)
                n0 = len(self.requests())
                ask.stdin.write("y\n")
                ask.stdin.flush()
                deadline = time.monotonic() + 15
                while len(self.requests()) == n0 and time.monotonic() < deadline:
                    time.sleep(0.1)
                self.must("slow read actually entered provider before interruption", len(self.requests()) > n0)
                lm = self.ledger.mark()
                stopped = self.guide("stop")
                self.must("mid-read stop accepted", stopped["rc"] == 0, stopped)
                ask.communicate(timeout=30)
                time.sleep(9)
                self.must("late read cannot point after stop", not self.targets(lm), self.targets(lm))
                self.results["confusion"]["dismissals"] += 1
            finally:
                if ask.poll() is None:
                    ask.kill()
                    ask.communicate()
                self.model.mode("point")
            self.capture("interrupted-read")
            # A fresh press interrupts actual spoken output; PCM exposes a
            # failure even when the frame stream says idle.
            recording = voicekit.Rec(self, "barge", mic=True)
            try:
                held = self.hold("ask")
                self.final_of(held)
                self.speaking_after(held["mark"], 90, ack=False)
                wm = self.watch.mark()
                at = self.keys.send("down")
                frame = self.wait_voice(wm, lambda d: d.get("phase") == "listening", 5, "barge listening")
                self.keys.send("up")
                self.talk_cli("stop", verb="speak")
            finally:
                recording.stop()
            cut = recording.to_rec(at)
            self.must("barge-in seen and aligned in recorded PCM", frame[1] is not None and cut is not None, {"frame": frame, "cut": cut})
            self.must("barge-in leaves no continuing reply after 0.5s", recording.peak(cut + .5, cut + 1.0) < .02)
            self.event("barge-in", hotkey_wall=at)
            self.results["confusion"]["repeated_prompts"] += 1
            self.finish_guide(keyboard=True)

        def spanish(self):
            import re
            self.sh("phase18.sh", "locale", timeout=120)
            self.previous_question = None
            self.arrange()
            self.event("second-language-locale", locale="es_ES.UTF-8", source="private localedef output, app environment")
            self.ui_question("¿Dónde está el botón para guardar?", keyboard=True)
            mid = os.environ["ONI_LAB_SECOND_LANGUAGE_STT_MODEL"]
            engine = os.environ.get("ONI_LAB_SECOND_LANGUAGE_STT_ENGINE", "whispercpp")
            self.vset(stt={"engine": engine, "model": mid, "language": "es"})
            sentence = "La biblioteca abre mañana por la tarde."
            wav = self.lab / "voice/persona-es.wav"
            r = subprocess.run(["espeak-ng", "-v", "es", "-s", "135", "-w", str(wav), sentence],
                               check=True, capture_output=True, timeout=60,
                               env={"PATH": "/usr/bin:/bin", "HOME": str(self.lab / "home")})
            self.fx["fixtures"]["spanish"] = {"file": str(wav), "text": sentence}
            self.event("spanish-speech", text=sentence, stt={"engine": engine, "model": mid}, synthesis="espeak-ng -v es -w (no device)")
            recording = voicekit.Rec(self, "spanish", mic=True)
            try:
                utterance = self.final_of(self.hold("spanish"))
                final = utterance.get("final")
                heard = final[1] if final else ""
                text = re.sub(r"[^\w ]", "", heard.lower())
                self.must("multilingual STT understands actual Spanish PCM", "biblioteca" in text and "tarde" in text, utterance)
                self.talk_cli("stop", verb="speak")
            finally:
                recording.stop()
            self.must("Spanish microphone PCM recorded", recording.mic_pcm is not None and max(abs(recording.mic_pcm)) > .01)
            self.finish_guide(keyboard=True)

        def geometry(self):
            if self.wm == "hyprland":
                self.gw("output", self.env["LAB_OUTPUT"])
            for scale in (1, 1.25, 1.5, 2):
                self.sh("outputs.sh", "scale", self.env["LAB_OUTPUT"], scale)
                self.settle(.5, 20)
                for reduced in ("off", "on"):
                    self.settings(reduceMotion=reduced)
                    self.ensure_gw()
                    self.perch_gw()
                    mark = self.ledger.mark()
                    self.event("scale-task", scale=scale, reduceMotion=reduced)
                    call = self.guide("start", guidekit.GOAL_SAVE, answer="y\n")
                    line = self.wait_target(mark, 30)
                    self.must(f"{scale}/{reduced}: guide points", line is not None, call)
                    state = self.state()
                    output = self.output(self.current()["output"], state)
                    rect = labkit.global_rect(self.current(), state)
                    fits = rect[0] >= output["x"] - 1 and rect[1] >= output["y"] - 1 and rect[0] + rect[2] <= output["x"] + output["width"] + 1 and rect[1] + rect[3] <= output["y"] + output["height"] + 1
                    self.must(f"{scale}/{reduced}: sprite remains on output", fits, {"rect": rect, "output": output})
                    self.capture(f"scale-{scale}-{reduced}")
                    self.target_act("file", keyboard=True)
                    self.guide("stop")
                    self.ensure_gw()
            self.sh("outputs.sh", "scale", self.env["LAB_OUTPUT"], 1)
            self.settings(reduceMotion="off")
            others = [o for o in self.state()["outputs"] if o["enabled"] and o["name"] != self.env["LAB_OUTPUT"]]
            if self.wm == "hyprland":
                self.must("real second nested output exists", bool(others), others)
                self.gw("output", others[0]["name"])
                self.perch_gw()
                self.capture("second-output")
                self.must("perch follows target to second output", self.current()["output"] == others[0]["name"], self.current())
                self.ui_question("Where is the save button?", keyboard=True)
            else:
                self.results["limits"].append({"check": "multi-output", "reason": "nested niri winit exposes one output; no multi-output claim for this cell"})

        def local_drag(self, name):
            self.ensure_detached()
            self.wake()
            self.settle(.5, 20)
            state, output, sprite = self.where()
            right = sprite[0] < output["x"] + output["width"] / 2
            point = (round(output["x"] + output["width"] * (.8 if right else .2)),
                     round(output["y"] + output["height"] * .25))
            mark = self.ledger.mark()
            self.sh("input.sh", "drag", *map(round, labkit.centre(sprite)), *point)
            self.settle(.5, 20)
            moved = labkit.global_rect(self.current(), self.state())
            self.must(f"{name}: local character responds to pointer without provider",
                      abs(moved[0] - sprite[0]) + abs(moved[1] - sprite[1]) > 8,
                      {"before": sprite, "after": moved, "ledger": self.ledger.since(mark)})
            self.capture(f"{name}-local-drag")
            self.perch_gw()

        def failures(self):
            # Close ALL model-facing discovery routes through a private
            # loopback HTTP fixture. It forwards only to this lab model and
            # has no proxy/environment-based or arbitrary external target.
            from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
            import threading
            import urllib.request
            import urllib.error
            parent = self
            mode, rows = {"value": "offline"}, []
            opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
            class Handler(BaseHTTPRequestHandler):
                def log_message(self, *_):
                    pass
                def do_GET(self):
                    self.dispatch()
                def do_POST(self):
                    self.dispatch()
                def dispatch(self):
                    n = int(self.headers.get("Content-Length", "0"))
                    if n > 8 * 1024 * 1024:
                        self.send_error(413)
                        return
                    body = self.rfile.read(n) if n else None
                    current = mode["value"]
                    path = self.path.split("?", 1)[0]
                    if current == "offline":
                        status, data = 503, b'{"error":{"message":"Persona fixture is offline"}}'
                    elif current == "missing":
                        status, data = (200, b'{"object":"list","data":[]}') if path.endswith("/models") else (404, b'{"error":{"message":"No provider model available"}}')
                    elif current == "exhausted" and path.endswith("/chat/completions"):
                        status, data = 402, b'{"error":{"code":"insufficient_credits","message":"The credit fixture has no credits remaining"}}'
                    else:
                        if path not in ("/v1/models", "/models", "/props", "/v1/props", "/health", "/v1/chat/completions"):
                            self.send_error(404)
                            return
                        req = urllib.request.Request(parent.env["LAB_MODEL_URL"] + path, data=body,
                                                     headers={"Content-Type": "application/json"} if body else {})
                        try:
                            with opener.open(req, timeout=60) as r:
                                status, data = r.status, r.read()
                        except urllib.error.HTTPError as e:
                            status, data = e.code, e.read()
                    rows.append({"wall": time.time(), "mode": current, "method": self.command, "path": path, "status": status})
                    self.send_response(status)
                    self.send_header("Content-Type", "application/json")
                    self.send_header("Content-Length", str(len(data)))
                    self.end_headers()
                    self.wfile.write(data)
            server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
            server.daemon_threads = True
            thread = threading.Thread(target=server.serve_forever, daemon=True)
            thread.start()
            cfg = self.lab / "cfg/hyakki/hyakki.json"
            original = json.loads(cfg.read_text())
            def configure(local):
                self.ui.sh("phase17.sh", "ui", "close")
                self.watch.pause()
                try:
                    self.sh("ctl.sh", "surface", "stop")
                    self.sh("ctl.sh", "daemon", "stop")
                    value = json.loads(cfg.read_text())
                    value.setdefault("providers", {})["local"] = local
                    dump(cfg, value)
                    os.chmod(cfg, 0o600)
                    self.sh("ctl.sh", "daemon", "start")
                finally:
                    self.watch.resume()
                self.restart_surface()
                self.ui.sh("phase17.sh", "ui", "open")
                self.previous_question = None
                self.ensure_gw()
                self.arrange()
                self.perch_gw()
            try:
                base = f"http://127.0.0.1:{server.server_port}"
                for name in ("offline", "missing", "exhausted"):
                    mode["value"] = name
                    configure({"llamacpp": base + "/v1", "ollama-local": base + "/closed/v1"})
                    self.event("failure-injection", mode=name, endpoint=base, account="none")
                    # Missing/offline routes can refuse before capture; the
                    # real app must render the refusal, never an empty success.
                    t0 = time.monotonic()
                    self.ui_question("Where is the save button?", keyboard=True,
                                     expect_error=True, require_consent=False)
                    self.capture(f"failure-{name}")
                    self.local_drag(name)
                    self.target_act("file", keyboard=True)
                    self.target_act("save", keyboard=True)
                    self.must(f"{name}: local target still usable", self.saved() is not None)
                    self.results["measurements"][name] = {"pass": True, "seconds": time.monotonic() - t0}
                self.must("credit fixture really returned HTTP402", any(r["mode"] == "exhausted" and r["status"] == 402 for r in rows), rows)
            finally:
                try:
                    configure(original.get("providers", {}).get("local", {}))
                finally:
                    server.shutdown()
                    server.server_close()
                    thread.join(timeout=5)
                dump(self.out / "failure-http.json", rows)

        def execute(self):
            try:
                self.model_ready()
                details, ok = self.census11()
                self.must("isolated audio/chord setup", ok, details)
                self.vchildren.start()
                if persona in ("keyboard-first", "mouse-first"):
                    self.session_recording = voicekit.Rec(self, "session")
                if persona == "voice-first":
                    self.voice_guide()
                elif persona == "interrupt-heavy":
                    self.interrupt()
                elif persona == "second-language":
                    self.spanish()
                else:
                    self.ui_question("Where is the save button?", keyboard=persona == "keyboard-first")
                    if persona == "mouse-first":
                        self.mouse_guide()
                    else:
                        self.finish_guide(keyboard=True)
                # Extended failure/geometry tasks run once per fresh cell,
                # not five cosmetic reruns of the same command.
                if persona == "keyboard-first":
                    self.failures()
                if persona == "mouse-first":
                    self.geometry()
                self.capture("session-end")
                # Record real process counters (short sample, not an idle-CPU
                # acceptance target; full settlement ladders are composed).
                pids = labkit.lab_pids(self.env["LAB_ID"])
                before = {p: labkit.counters(p) for p in pids}
                time.sleep(2)
                after = {p: labkit.counters(p) for p in pids if Path(f"/proc/{p}").exists()}
                dump(self.out / "resources.json", {"seconds": 2, "before": before, "after": after,
                     "delta": {p: labkit.delta(before[p], after[p]) for p in after}})
            except Exception as e:
                self.results["steps"].append({"name": "session completion", "pass": False, "error": str(e)})
                try:
                    self.capture("failure")
                except Exception as capture_error:
                    self.results["capture_error"] = str(capture_error)
            finally:
                if self.session_recording is not None:
                    try:
                        self.session_recording.stop()
                    except Exception as e:
                        self.results["steps"].append({"name": "session PCM capture", "pass": False, "error": str(e)})
                if self.keys:
                    self.keys.send("up")
                    self.keys.close()
                self.sh("audio.sh", "stop", check=False)
                if self.watch.is_alive():
                    self.watch.stop()
                    self.watch.join(timeout=10)
                if self.clock:
                    self.clock.done.set()
                    self.clock.join(timeout=5)
                self.vchildren.done.set()
                if self.vchildren.is_alive():
                    self.vchildren.join(timeout=5)
                self.ui.children.done.set()
                self.ui.children.join(timeout=5)
                self.ui.log.close()
                dump(self.out / "frames.json", self.watch.lines)
                dump(self.out / "processes.json", self.vchildren.seen)
                dump(self.out / "model/requests.json", self.model.requests() if self.model else [])
                for p in (self.lab / "ledger").glob("*.jsonl"):
                    shutil.copyfile(p, self.out / "ledgers" / p.name)
                self.results["finished_at"] = stamp()
                self.save()
            return 0 if self.results["pass"] else 1

    for d in ("ui", "ledgers", "crops"):
        (Path(out) / d).mkdir(parents=True, exist_ok=True)
    return Persona().execute()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="verb", required=True)
    s = commands.add_parser("suite")
    s.add_argument("wm", choices=("niri", "hyprland"))
    s.add_argument("out")
    s.add_argument("root", choices=("checkout", "package"))
    s = commands.add_parser("index")
    s.add_argument("out")
    s.add_argument("cells", nargs="+")
    s = commands.add_parser("session")
    s.add_argument("lab")
    s.add_argument("out")
    s.add_argument("persona", choices=PERSONAS)
    s = commands.add_parser("target")
    s.add_argument("out")
    s.add_argument("variant")
    args = parser.parse_args()
    os.umask(0o077)
    if args.verb == "suite":
        return suite(args.wm, args.out, args.root)
    if args.verb == "index":
        return matrix(args.out, args.cells)
    if args.verb == "session":
        return session(args.lab, args.out, args.persona)
    return target(args.out, args.variant)


if __name__ == "__main__":
    sys.exit(main())
