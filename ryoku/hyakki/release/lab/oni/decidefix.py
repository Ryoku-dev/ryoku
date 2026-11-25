#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-only
"""decidefix.py: the lab's local decision service, served by model.py at
POST /v1/systemone (the path the local decision route posts to: its address,
default http://127.0.0.1:11435/v1, plus "/systemone"). It is a keyword table,
not a model: it proves routing, thresholds and fall-through, never model
quality or calibration on real speech, and the lab says so.

Request: a typed decision request {"model", "state", "questions"} as the
decision contract sends it. The model must be "oni-fixture-decisions"
(anything else is 404, as a server without that model would answer). The
words are the state when it is a JSON string, else the state's "utterance",
"text" or "words" string. Each question is answered by type:
  choice  probabilities over exactly the offered labels. Every phrase in the
          keyword table that matches the words scores its label; the
          strongest label takes 0.95 x its phrase weight, the other matched
          labels share in proportion to their squared weights, and what is
          left is spread evenly over the rest. A phrase may exclude its label
          when another phrase is present ("open" never scores open_app when
          the words also say "tab"). Words that ask to send, buy, delete,
          submit or pay score "none" when "none" is offered. With no match
          "none" (when offered) takes 0.6 and the rest share 0.4. A weak cue
          (weight 0.6) therefore stays below a 0.7 threshold, two equal
          cues tie, and one clear phrase clears both threshold and margin.
  score   an even distribution over the levels; score is the middle level.
  noul    0.5.
confidence is (n x top - 1) / (n - 1) and the distribution sums to 1, as the
contract's validation demands; usage is {"input_tokens": words, at least 1,
"output_tokens": 0}.

Modes (POST /_lab/decide {"mode": M, "delay": S} on model.py):
  keyword  the table above (the default)
  slow     wait S seconds (default 3, beyond the 1.5 s budget), then answer
  fail     500, no answer
  unsure   the table's top label at 0.45 and its runner-up at 0.40: a valid
           answer whose confidence and margin are both too low to act on
  hostile  answers as a subverted service would: probability 1 on
           "send_message", a label the request never offered, beside the
           offered ones at 0; the contract's validation must refuse it
Every request is recorded (GET /_lab/decisions; POST /_lab/decisions/reset
forgets them) and appended to the decision log beside model.py's chat log
(model-decisions.jsonl), never to the chat log: {"n", "t", "path", "mode",
"status", "model", "keys", "state", "utterance", "questions" (id: type and
labels), "instructions" (ids that carried any), "choice", "probability",
"confidence", "ms"}.

decidefix.py selftest   check the table and every mode against a copy of the
                        contract's validation rules; no server, no network.
"""
import json
import math
import re
import threading
import time
from pathlib import Path

MODEL = "oni-fixture-decisions"
FORBIDDEN = "send_message"
MODES = ("keyword", "slow", "fail", "unsure", "hostile")

# (label, phrase, weight, unless): unless is a pattern that, also present,
# keeps the phrase from scoring. Hyakki's own wording, written for the lab.
TABLE = [
    ("new_tab", r"\bnew tab\b", 1.0, None),
    ("new_tab", r"\banother tab\b", 0.9, None),
    ("open_site", r"\b(go to|visit|take me to)\b", 0.8, r"\b(go home|tab)\b"),
    ("open_site", r"\b[a-z0-9-]+\.(com|org|net|dev|io)\b", 0.9, None),
    ("web_search", r"\bsearch( the web)? for\b", 0.95, None),
    ("web_search", r"\blook up\b", 0.85, None),
    ("open_app", r"\b(fire up|boot up|bring up)\b", 0.9, r"\btab\b"),
    ("open_app", r"\b(open|launch|start)\b", 0.8, r"\b(tab|timer|music|song|\w+\.(com|org|net|dev|io))\b"),
    ("open_app", r"\b(notepad|notes app|text editor|browser)\b", 0.3, r"\btab\b"),
    ("snap_left", r"\b(snap|move|put|tile)\b.*\bleft\b", 0.95, r"\b(yourself|top|bottom)\b"),
    ("snap_right", r"\b(snap|move|put|tile)\b.*\bright\b", 0.95, r"\b(yourself|top|bottom)\b"),
    ("maximize", r"\b(maximi[sz]e|full ?screen)\b", 0.95, None),
    ("minimize", r"\b(minimi[sz]e|tuck (it|this|the window) away)\b", 0.95, None),
    ("volume_up", r"\b(louder|volume up|turn (it|the volume|the sound) up|raise the volume)\b", 0.95, None),
    ("volume_down", r"\b(quieter|softer|volume down|turn (it|the volume|the sound) down|lower the volume)\b", 0.95, None),
    ("mute", r"\b(mute|unmute|silence the sound)\b", 0.95, None),
    ("play_pause", r"\b(pause|resume|play|stop)( the)? (music|song|track|playback)\b", 0.95, None),
    ("play_pause", r"\bpause\b", 0.6, None),
    ("screenshot", r"\b(screenshot|screen shot|capture the screen|grab the screen)\b", 0.95, None),
    ("set_timer", r"\btimer\b", 0.9, r"\b(cancel|stop|clear|what|which|list)\b"),
    ("set_reminder", r"\bremind me\b", 0.95, None),
    ("cancel_timer", r"\b(cancel|stop|clear)\b.*\b(timer|reminder)s?\b", 1.0, None),
    ("list_reminders", r"\b(what|which|list|show)\b.*\breminders?\b", 0.95, r"\b(cancel|clear)\b"),
    ("self_move", r"\bmove (yourself|over)\b", 0.9, None),
    ("self_move", r"\b(top|bottom)[ -](left|right)\b", 0.6, None),
    ("self_size", r"\b(make yourself|get|grow) (bigger|smaller|big|small)\b", 0.95, None),
    ("self_home", r"\b(go home|back to your (spot|corner|place))\b", 0.95, None),
    ("remember_note", r"\bremember (that|this)\b", 0.95, None),
    ("run_routine", r"\brun my routine\b", 0.95, None),
    ("setting_flip", r"\b(turn|switch) (on|off)\b", 0.8, r"\b(music|volume|sound)\b"),
    ("none", r"\b(send|buy|purchase|delete|submit|pay|order)\b", 1.0, None),
]
COMPILED = [(label, re.compile(p, re.I), w, re.compile(u, re.I) if u else None) for label, p, w, u in TABLE]


def utterance(state):
    if isinstance(state, str):
        return state
    if isinstance(state, dict):
        for key in ("utterance", "text", "words"):
            if isinstance(state.get(key), str):
                return state[key]
    return ""


def labels_of(question):
    criteria = question.get("criteria")
    if isinstance(criteria, list):
        return [c for c in criteria if isinstance(c, str)]
    if isinstance(criteria, dict):
        return list(criteria)
    return []


def scores(words, labels):
    """{label: weight} of the offered labels the words cue."""
    out = {}
    for label, phrase, weight, unless in COMPILED:
        if label in labels and phrase.search(words) and not (unless and unless.search(words)):
            out[label] = min(1.0, out.get(label, 0.0) + weight) if label == "open_app" else max(out.get(label, 0.0), weight)
    return out


def distribution(words, labels):
    cued = scores(words, labels)
    rest = [lab for lab in labels if lab not in cued]
    probs = {}
    if not cued:
        if "none" in labels and len(labels) > 1:
            probs = {lab: 0.4 / (len(labels) - 1) for lab in labels}
            probs["none"] = 0.6
        else:
            probs = {lab: 1.0 / len(labels) for lab in labels}
        return probs
    top = max(cued.values())
    mass = 0.95 * top if rest else 1.0
    squares = sum(w * w for w in cued.values())
    for lab, w in cued.items():
        probs[lab] = mass * w * w / squares
    for lab in rest:
        probs[lab] = (1.0 - mass) / len(rest)
    return probs


def unsure(words, labels):
    probs = distribution(words, labels)
    order = sorted(labels, key=lambda lab: (-probs[lab], labels.index(lab)))
    out = {lab: 0.15 / (len(labels) - 2) if len(labels) > 2 else 0.0 for lab in labels}
    out[order[0]], out[order[1]] = 0.45, 0.40 + (0.15 if len(labels) == 2 else 0.0)
    return out


def finish(probs, labels):
    """Normalise exactly and derive choice and confidence the contract's way."""
    total = sum(probs.values())
    probs = {lab: probs[lab] / total for lab in labels}
    n = len(labels)
    choice = max(labels, key=lambda lab: (probs[lab], -labels.index(lab)))
    top = probs[choice]
    return choice, probs, (n * top - 1) / (n - 1)


def answer(question, words, mode):
    kind = question.get("type")
    if kind == "noul":
        return {"type": "noul", "noul": 0.5}
    if kind == "score":
        levels = question.get("criteria") or []
        n = len(levels)
        probs = {str(i): 1.0 / n for i in range(n)}
        return {"type": "score", "probabilities": probs, "confidence": 0.0,
                "score": sum(i / n for i in range(n)), "legend": {str(i): d for i, d in enumerate(levels)}}
    labels = labels_of(question)
    if mode == "hostile":
        probs = {lab: 0.0 for lab in labels}
        probs[FORBIDDEN] = 1.0
        return {"type": "choice", "choice": FORBIDDEN, "probabilities": probs, "confidence": 1.0}
    probs = unsure(words, labels) if mode == "unsure" and len(labels) >= 2 else distribution(words, labels)
    choice, probs, confidence = finish(probs, labels)
    return {"type": "choice", "choice": choice, "probabilities": probs, "confidence": confidence}


def malformed(body):
    """Why the request is not a typed decision request, or None."""
    if not isinstance(body, dict) or set(body) - {"model", "state", "questions"}:
        return "the body holds only model, state and questions"
    if not isinstance(body.get("model"), str) or not body["model"]:
        return "choose a decision model"
    qs = body.get("questions")
    if not isinstance(qs, dict) or not 1 <= len(qs) <= 256:
        return "ask between 1 and 256 typed questions"
    for q in qs.values():
        if not isinstance(q, dict) or q.get("type") not in ("choice", "score", "noul"):
            return "a decision question must be choice, score or noul"
        if q["type"] == "choice" and len(set(labels_of(q))) != len(labels_of(q)) or \
                q["type"] == "choice" and len(labels_of(q)) < 2:
            return "a choice question needs 2 to 255 unique options"
        if q["type"] == "score" and not (isinstance(q.get("criteria"), list) and 2 <= len(q["criteria"]) <= 10):
            return "a score question needs 2 to 10 levels"
    return None


class Decider:
    def __init__(self, log=None):
        self.lock = threading.Lock()
        self.mode, self.delay = "keyword", 3.0
        self.records = []
        self.log = Path(log) if log else None

    def set_mode(self, mode, delay=None):
        if mode not in MODES:
            return None
        with self.lock:
            self.mode = mode
            self.delay = float(delay) if delay not in (None, "") else 3.0
            return {"mode": self.mode, "delay": self.delay}

    def reset(self):
        with self.lock:
            self.records.clear()

    def listing(self):
        with self.lock:
            return list(self.records)

    def decide(self, path, body):
        """(status, reply) for a POST to the decision path."""
        start = time.monotonic()
        with self.lock:
            mode, delay = self.mode, self.delay
        record = {"n": None, "t": time.time(), "path": path, "mode": mode}
        if isinstance(body, dict):
            words = utterance(body.get("state"))
            record.update(model=body.get("model"), keys=sorted(body), state=body.get("state"), utterance=words,
                          questions={qid: {"type": q.get("type"), "labels": labels_of(q)}
                                     for qid, q in (body.get("questions") or {}).items() if isinstance(q, dict)},
                          instructions=sorted(qid for qid, q in (body.get("questions") or {}).items()
                                              if isinstance(q, dict) and q.get("instructions") not in (None, "")))
        status, reply = 200, None
        why = malformed(body)
        if why:
            status, reply = 400, {"error": {"message": why}}
        elif body["model"] != MODEL:
            status, reply = 404, {"error": {"message": "model %s not found" % body["model"]}}
        elif mode == "fail":
            status, reply = 500, {"error": {"message": "the lab decision service failed on purpose"}}
        else:
            if mode == "slow" and delay > 0:
                time.sleep(delay)
            answers = {qid: answer(q, words, mode) for qid, q in body["questions"].items()}
            reply = {"model": MODEL, "answers": answers,
                     "usage": {"input_tokens": max(1, len(words.split())), "output_tokens": 0}}
            first = next(iter(answers.values()))
            record.update(choice=first.get("choice"),
                          probability=(first.get("probabilities") or {}).get(first.get("choice")),
                          confidence=first.get("confidence"))
        record.update(status=status, ms=round((time.monotonic() - start) * 1000, 1))
        with self.lock:
            record["n"] = len(self.records)
            self.records.append(record)
            if self.log:
                with self.log.open("a", encoding="utf-8") as fh:
                    fh.write(json.dumps(record) + "\n")
        return status, reply


# --- the contract's validation, for the selftests --------------------------

def valid(request, out):
    """Mirror of DecisionSpec.Validate (protocol/fastdecisions.go): None or why not."""
    qs = request["questions"]
    if not out.get("model") or len(out.get("answers", {})) != len(qs) or not 0 < out["usage"]["input_tokens"] <= 1 << 30 \
            or out["usage"]["output_tokens"] != 0:
        return "invalid model, answer set or usage"
    for qid, q in qs.items():
        a = out["answers"].get(qid)
        if not a or a.get("type") != q["type"]:
            return "answered a different question"
        if q["type"] == "noul":
            if not 0 <= a.get("noul", -1) <= 1:
                return "invalid truth probability"
            continue
        n = len(labels_of(q)) if q["type"] == "choice" else len(q["criteria"])
        probs, conf = a.get("probabilities") or {}, a.get("confidence")
        if len(probs) != n or conf is None or not 0 <= conf <= 1:
            return "incomplete probability distribution"
        total, top, expected = 0.0, 0.0, 0.0
        for label, p in probs.items():
            if not (0 <= p <= 1) or math.isnan(p):
                return "invalid probability"
            if q["type"] == "choice":
                if label not in labels_of(q):
                    return "invented an option"
            else:
                if not label.isdigit() or str(int(label)) != label or int(label) >= n:
                    return "invented a score level"
                expected += int(label) * p
            total += p
            top = max(top, p)
        if abs(total - 1) > n * 0.00006 + 0.000001 or abs(conf - (n * top - 1) / (n - 1)) > 0.001:
            return "inconsistent confidence"
        if q["type"] == "choice":
            if a.get("choice") not in probs or probs[a["choice"]] + 0.0001 < top:
                return "inconsistent choice"
        elif abs(a["score"] - expected) > 0.005 or len(a.get("legend", {})) != n:
            return "inconsistent score"
    return None


def margin(a):
    ps = sorted(a["probabilities"].values(), reverse=True)
    return ps[0] - ps[1]


def selftest():
    labels = ["cancel_timer", "list_reminders", "maximize", "minimize", "mute", "new_tab", "open_app", "open_site",
              "play_pause", "screenshot", "self_home", "self_move", "self_size", "set_reminder", "set_timer",
              "snap_left", "snap_right", "volume_down", "volume_up", "web_search", "none"]
    checks = []

    def check(name, ok, value=None):
        checks.append(ok)
        print(("ok   " if ok else "FAIL ") + name + ("" if ok else "  (%r)" % (value,)), flush=True)

    def req(words, state=None):
        return {"model": MODEL, "state": words if state is None else state,
                "questions": {"intent": {"type": "choice", "criteria": labels}}}

    d = Decider()
    acts = lambda a: a["probabilities"][a["choice"]] >= 0.7 and margin(a) >= 0.15  # noqa: E731
    cases = [("could you fire up the notepad thing", "open_app", True),
             ("open a new tab", "new_tab", True),
             ("open another tab please", "new_tab", True),
             ("go to example.org", "open_site", True),
             ("search for salt and pepper", "web_search", True),
             ("snap this to the left", "snap_left", True),
             ("put it on the right", "snap_right", True),
             ("maximize the window", "maximize", True),
             ("minimize it", "minimize", True),
             ("make it louder", "volume_up", True),
             ("turn the volume down", "volume_down", True),
             ("mute", "mute", True),
             ("pause the music", "play_pause", True),
             ("take a screenshot", "screenshot", True),
             ("set a timer for five minutes", "set_timer", True),
             ("remind me in ten minutes to stretch", "set_reminder", True),
             ("cancel the timer", "cancel_timer", True),
             ("what are my reminders", "list_reminders", True),
             ("move yourself over", "self_move", True),
             ("make yourself bigger", "self_size", True),
             ("go home", "self_home", True),
             ("pause", "play_pause", False),
             ("I insert a table", "none", False),
             ("salt and pepper", "none", False),
             ("send the message", "none", True),
             ("buy this", "none", True),
             ("delete the file", "none", True),
             ("submit the form", "none", True),
             ("open a new tab and take a screenshot", None, False)]
    for words, want, act in cases:
        status, out = d.decide("/v1/systemone", req(words))
        a = out["answers"]["intent"]
        check("%r -> %s%s" % (words, want or "a tie", " (acts)" if act else " (does not act)"),
              status == 200 and valid(req(words), out) is None and (want is None or a["choice"] == want)
              and acts(a) == act, (a["choice"], a["probabilities"][a["choice"]], margin(a), valid(req(words), out)))
    status, out = d.decide("/v1/systemone", req(None, {"utterance": "pause the music"}))
    check("an object state is read through its utterance", out["answers"]["intent"]["choice"] == "play_pause")
    status, out = d.decide("/v1/systemone", {"model": MODEL, "state": "x", "questions": {
        "a": {"type": "noul"}, "b": {"type": "score", "criteria": ["low", "mid", "high"]}}})
    check("noul and score questions get valid answers", status == 200 and valid(
        {"questions": {"a": {"type": "noul"}, "b": {"type": "score", "criteria": ["low", "mid", "high"]}}}, out) is None, out)
    status, out = d.decide("/v1/systemone", dict(req("x"), model="other"))
    check("another model is 404", status == 404)
    status, out = d.decide("/v1/systemone", dict(req("x"), extra=1))
    check("an unexpected body key is 400", status == 400)
    d.set_mode("fail")
    check("fail answers 500", d.decide("/v1/systemone", req("open a new tab"))[0] == 500)
    d.set_mode("unsure")
    status, out = d.decide("/v1/systemone", req("open a new tab"))
    a = out["answers"]["intent"]
    check("unsure is valid but too weak and too close to act",
          valid(req("x"), out) is None and a["choice"] == "new_tab" and not acts(a), a)
    d.set_mode("hostile")
    status, out = d.decide("/v1/systemone", req("open a new tab"))
    check("hostile names a label never offered, and the contract's validation refuses it",
          status == 200 and out["answers"]["intent"]["choice"] == FORBIDDEN
          and valid(req("x"), out) == "incomplete probability distribution", valid(req("x"), out))
    d.set_mode("slow", 0.2)
    t = time.monotonic()
    status, out = d.decide("/v1/systemone", req("open a new tab"))
    check("slow waits, then answers validly", time.monotonic() - t >= 0.2 and valid(req("x"), out) is None)
    check("an unknown mode is refused", d.set_mode("bogus") is None)
    recs = d.listing()
    check("every request is recorded with its words, labels and outcome",
          len(recs) == len(cases) + 8 and recs[0]["utterance"] == cases[0][0]
          and recs[0]["questions"]["intent"]["labels"] == labels and recs[0]["keys"] == ["model", "questions", "state"]
          and [r["status"] for r in recs[-5:]] == [400, 500, 200, 200, 200], [r["status"] for r in recs[-5:]])
    ok = all(checks)
    print("decidefix selftest: %s" % ("all passed" if ok else "FAIL"))
    return 0 if ok else 1


if __name__ == "__main__":
    import sys

    sys.exit(selftest() if sys.argv[1:] == ["selftest"] else (print(__doc__), 2)[1])
