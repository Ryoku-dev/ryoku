package cdp

import "encoding/json"

// World is the isolated world every Hyakki function runs in: the page's own
// script cannot see its globals or call its binding.
const World = "hyakki"

// Binding is the one binding, present only in World. Its only payloads are
// "visible", "hidden" and "pressed:" followed by an Await token.
const Binding = "hyakkiSeen"

// Function names one compiled-in page function. Runtime.callFunctionOn is
// allowed only with one of these declarations, byte for byte.
type Function string

const (
	// FnDescribe reports what an element is, for the consent rules.
	FnDescribe Function = "describe"
	// FnPrepare focuses an editable element and selects its contents.
	FnPrepare Function = "prepare"
	// FnErase deletes the selection in the focused element.
	FnErase Function = "erase"
	// FnReadValue reads an element's value back.
	FnReadValue Function = "readValue"
	// FnTextLeaf reads a rendered text-only element without its inline AX boxes.
	FnTextLeaf Function = "textLeaf"
	// FnHit says whether a viewport point lands on the element.
	FnHit Function = "hit"
	// FnChoose picks one option of a select by its label.
	FnChoose Function = "choose"
	// FnScroll scrolls the page vertically (run on the world's context).
	FnScroll Function = "scroll"
	// FnWatch reports the page's visibility through Binding (run on the
	// world's context).
	FnWatch Function = "watch"
	// FnReady says the current document has finished parsing.
	FnReady Function = "ready"
	// FnAwait arms a one-shot watch on an element for the person's own
	// press: a trusted click on it or inside it, or, for a submit control,
	// a trusted submit of its form after a trusted Enter in that form. A
	// later arming disarms it.
	FnAwait Function = "await"
)

const watchSource = `function () {
  if (globalThis.__hyakkiWatch) { return true; }
  globalThis.__hyakkiWatch = true;
  const tell = () => {
    try { hyakkiSeen(document.visibilityState === "visible" ? "visible" : "hidden"); } catch (e) {}
  };
  document.addEventListener("visibilitychange", tell, true);
  tell();
  return true;
}`

var functions = map[Function]string{
	FnTextLeaf: `function (limit) {
  const el = this;
  if (!(el instanceof HTMLElement) || !el.isConnected || el.ownerDocument !== document ||
      el.childNodes.length !== 1 || el.firstChild.nodeType !== Node.TEXT_NODE ||
      !Number.isSafeInteger(limit) || limit < 0) { return { ok: false }; }
  const style = getComputedStyle(el);
  if (style.display === "none" || style.visibility !== "visible" ||
      style.webkitTextSecurity !== "none") { return { ok: false }; }
  for (const pseudo of ["::before", "::after"]) {
    const content = getComputedStyle(el, pseudo).content;
    if (content !== "none" && content !== "normal" && content !== '""' && content !== "''") {
      return { ok: false };
    }
  }
  if (limit === 0) { return { ok: true, text: "", runes: 0, truncated: true }; }
  const omitted = /[\p{Cc}\p{White_Space}\p{Bidi_Control}\uFFFD\uD800-\uDFFF]/u;
  let text = "", runes = 0, space = false;
  for (const ch of el.innerText) {
    if (omitted.test(ch)) {
      space = text.length > 0;
      continue;
    }
    const need = space ? 2 : 1;
    if (runes + need > limit) { return { ok: true, text, runes, truncated: true }; }
    if (space) { text += " "; runes++; space = false; }
    text += ch;
    runes++;
  }
  return { ok: true, text, runes, truncated: false };
}`,
	FnDescribe: `function () {
  const el = this;
  if (!(el instanceof Element) || !el.isConnected) { return { ok: false }; }
  const tag = el.tagName.toLowerCase();
  let type = "";
  if (tag === "input") { type = (el.type || "text").toLowerCase(); }
  if (tag === "button") { type = (el.type || "submit").toLowerCase(); }
  const ac = String(el.getAttribute("autocomplete") || "").toLowerCase().slice(0, 64);
  const form = el.form || el.closest("form");
  let pays = false;
  if (form) {
    for (const f of Array.from(form.elements).slice(0, 500)) {
      const a = String((f.getAttribute && f.getAttribute("autocomplete")) || "").toLowerCase();
      if (/(^|\s)cc-/.test(a)) { pays = true; break; }
    }
  }
  const submits = !!form && ((tag === "button" && type === "submit") || (tag === "input" && (type === "submit" || type === "image")));
  const noText = ["button", "submit", "reset", "checkbox", "radio", "file", "image", "hidden", "range", "color"];
  const editable = !el.disabled && !el.readOnly && (tag === "textarea" || (tag === "input" && !noText.includes(type)) || el.isContentEditable);
  const style = getComputedStyle(el);
  const hidden = el.getClientRects().length === 0 || style.visibility === "hidden" || style.display === "none";
  return { ok: true, tag: tag, type: type, autocomplete: ac, submits: submits, pays: pays, editable: editable,
    multiline: tag === "textarea" || el.isContentEditable, disabled: !!el.disabled, hidden: hidden };
}`,
	FnPrepare: `function () {
  this.focus();
  if (typeof this.select === "function") { this.select(); return document.activeElement === this; }
  const r = document.createRange();
  r.selectNodeContents(this);
  const s = getSelection();
  s.removeAllRanges();
  s.addRange(r);
  return document.activeElement === this || this.contains(document.activeElement);
}`,
	FnErase: `function () {
  this.focus();
  return document.execCommand("delete");
}`,
	FnReadValue: `function () {
  if (this.tagName === "SELECT") { const o = this.options[this.selectedIndex]; return o ? String(o.text) : ""; }
  if (this.tagName === "INPUT" || this.tagName === "TEXTAREA") { return String(this.value); }
  return String(this.innerText);
}`,
	FnHit: `function (x, y) {
  if (x < 0 || y < 0 || x >= innerWidth || y >= innerHeight) { return false; }
  const e = document.elementFromPoint(x, y);
  return !!e && (e === this || this.contains(e));
}`,
	FnChoose: `function (label) {
  if (this.tagName !== "SELECT" || this.disabled) { return null; }
  for (let i = 0; i < this.options.length; i++) {
    const o = this.options[i];
    if (o.label === label || o.text === label) {
      if (o.disabled) { return null; }
      this.selectedIndex = i;
      this.dispatchEvent(new Event("input", { bubbles: true }));
      this.dispatchEvent(new Event("change", { bubbles: true }));
      return String(o.text);
    }
  }
  return null;
}`,
	FnScroll: `function (dy) {
  window.scrollBy(0, dy);
  return Math.round(window.scrollY);
}`,
	FnWatch: watchSource,
	FnReady: `function () { return document.readyState !== "loading"; }`,
	FnAwait: `function (token) {
  const el = this;
  if (!(el instanceof Element) || !el.isConnected) { return false; }
  globalThis.__hyakkiArm = token;
  const fire = () => {
    if (globalThis.__hyakkiArm !== token) { return; }
    globalThis.__hyakkiArm = null;
    try { hyakkiSeen("pressed:" + token); } catch (e) {}
  };
  el.addEventListener("click", (e) => { if (e.isTrusted) { fire(); } }, true);
  const submits = (el.tagName === "BUTTON" || el.tagName === "INPUT") && (el.type === "submit" || el.type === "image");
  if (submits && el.form) {
    let keyed = false;
    el.form.addEventListener("keydown", (e) => { if (e.isTrusted && e.key === "Enter") { keyed = true; } }, true);
    el.form.addEventListener("submit", (e) => { const k = keyed; keyed = false; if (e.isTrusted && k) { fire(); } }, true);
  }
  return true;
}`,
}

// WatchScript is the one script added to every new document, in World.
const WatchScript = "(" + watchSource + ")();"

// Source is the function's declaration; ok is false for a name not in the
// table.
func (f Function) Source() (string, bool) {
	s, ok := functions[f]
	return s, ok
}

// Arg is one call argument: a string, a number or a bool, by value.
type Arg struct {
	Value any `json:"value"`
}

// CallParams are Runtime.callFunctionOn's parameters. Build them with
// FunctionCall; Allowed refuses any other declaration.
type CallParams struct {
	FunctionDeclaration string `json:"functionDeclaration"`
	ObjectID            string `json:"objectId,omitempty"`
	ExecutionContextID  int    `json:"executionContextId,omitempty"`
	Arguments           []Arg  `json:"arguments,omitempty"`
	ReturnByValue       bool   `json:"returnByValue"`
}

// FunctionCall builds a call of fn on an object (objectID) or in a world's
// context (contextID): exactly one of the two.
func FunctionCall(fn Function, objectID string, contextID int, args ...any) (CallParams, bool) {
	src, ok := fn.Source()
	if !ok || (objectID == "") == (contextID == 0) {
		return CallParams{}, false
	}
	p := CallParams{FunctionDeclaration: src, ObjectID: objectID, ExecutionContextID: contextID, ReturnByValue: true}
	for _, a := range args {
		p.Arguments = append(p.Arguments, Arg{Value: a})
	}
	return p, true
}

// Result is what callFunctionOn returns by value.
type Result struct {
	Result struct {
		Type  string          `json:"type"`
		Value json.RawMessage `json:"value"`
	} `json:"result"`
	ExceptionDetails *struct {
		Text string `json:"text"`
	} `json:"exceptionDetails"`
}
