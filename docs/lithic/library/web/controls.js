import {register} from './core.js';
import tokens from './tokens.js';

// key tokens follow ryoku/hub Combos.qtKeyName so a chord reads the way the store binds it
const NAMED = {
  Enter: 'Return',
  ' ': 'Space',
  Spacebar: 'Space',
  Tab: 'Tab',
  ArrowLeft: 'Left',
  ArrowRight: 'Right',
  ArrowUp: 'Up',
  ArrowDown: 'Down',
  Backspace: 'BackSpace',
  Delete: 'Delete',
  Home: 'Home',
  End: 'End',
  PageUp: 'Prior',
  PageDown: 'Next',
  Insert: 'Insert',
  PrintScreen: 'Print',
  AudioVolumeUp: 'XF86AudioRaiseVolume',
  AudioVolumeDown: 'XF86AudioLowerVolume',
  AudioVolumeMute: 'XF86AudioMute',
  MicrophoneVolumeMute: 'XF86AudioMicMute',
  MediaPlayPause: 'XF86AudioPlay',
  MediaPlay: 'XF86AudioPlay',
  MediaTrackNext: 'XF86AudioNext',
  MediaTrackPrevious: 'XF86AudioPrev',
  BrightnessUp: 'XF86MonBrightnessUp',
  BrightnessDown: 'XF86MonBrightnessDown',
};

const PUNCTUATION = {
  Minus: 'minus',
  Equal: 'equal',
  Comma: 'comma',
  Period: 'period',
  Slash: 'slash',
  Backslash: 'backslash',
  Semicolon: 'semicolon',
  Quote: 'apostrophe',
  BracketLeft: 'bracketleft',
  BracketRight: 'bracketright',
  Backquote: 'grave',
};

const NUMPAD = {
  NumpadEnter: 'KP_Enter',
  NumpadAdd: 'KP_Add',
  NumpadSubtract: 'KP_Subtract',
  NumpadMultiply: 'KP_Multiply',
  NumpadDivide: 'KP_Divide',
  NumpadDecimal: 'KP_Decimal',
  NumpadComma: 'KP_Decimal',
};

// the faces wm.DisplayKeys shows for the same tokens
const FACES = {
  Return: 'Enter',
  Escape: 'Esc',
  space: 'Space',
  Prior: 'Page Up',
  Next: 'Page Down',
  grave: '`',
  comma: ',',
  bracketleft: '[',
  bracketright: ']',
  XF86AudioRaiseVolume: 'Vol +',
  XF86AudioLowerVolume: 'Vol -',
  XF86AudioMute: 'Mute',
  XF86AudioMicMute: 'Mic mute',
  XF86AudioPlay: 'Play',
  XF86AudioNext: 'Next',
  XF86AudioPrev: 'Prev',
  XF86MonBrightnessUp: 'Brightness +',
  XF86MonBrightnessDown: 'Brightness -',
};

const PLACEHOLDER = 'Not set';
const PROMPT = 'Press a shortcut';
let conflictSequence = 0;

const HINT = 'Use a colour like #ce5e32';
let helpSequence = 0;

function all(root, selector) {
  const matches = [...root.querySelectorAll(selector)];
  if (root.nodeType === Node.ELEMENT_NODE && root.matches(selector)) matches.unshift(root);
  return matches;
}

function emit(element, type = 'change') {
  element.dispatchEvent(new Event(type, {bubbles: true}));
}

function emitDetail(element, name, detail = {}) {
  element.dispatchEvent(new CustomEvent(name, {bubbles: true, composed: true, detail}));
}

function number(value, fallback = 0) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function bounded(value, min, max) {
  return Math.min(max, Math.max(min, value));
}

function setGlyph(button, name) {
  const glyph = button.querySelector('.l-glyph');
  if (glyph) glyph.textContent = name;
}

function syncSearch(search) {
  const input = search.querySelector('input');
  const clear = search.querySelector('[data-l-search-clear]');
  if (!input) return;
  const hasValue = input.value.length > 0;
  search.toggleAttribute('data-l-has-value', hasValue);
  if (clear) clear.hidden = !hasValue;
}

function syncSlider(slider) {
  const field = slider.closest('.l-slider-field');
  if (!field) return;
  const min = number(slider.min, 0);
  const max = number(slider.max, 100);
  if (!slider.hasAttribute('step')) {
    const keyboardStep = (max - min) * tokens.size.sliderKeyboardStep;
    if (keyboardStep > 0) slider.step = String(keyboardStep);
  }
  const value = bounded(number(slider.value, min), min, max);
  const fraction = max === min ? 0 : (value - min) / (max - min);
  field.style.setProperty('--l-value', String(fraction));
  slider.setAttribute('aria-valuetext', slider.dataset.lValueText || String(value));
  const output = field.querySelector('[data-l-slider-value]');
  if (output) output.textContent = slider.dataset.lValueText || String(value);
}

function stepperParts(stepper) {
  return {
    value: stepper.querySelector('.l-stepper-value'),
    decrement: stepper.querySelector('[data-l-stepper="decrement"]'),
    increment: stepper.querySelector('[data-l-stepper="increment"]'),
  };
}

function syncStepper(stepper, nextValue = number(stepper.dataset.value, 0), notify = false) {
  const min = number(stepper.dataset.min, Number.NEGATIVE_INFINITY);
  const max = number(stepper.dataset.max, Number.POSITIVE_INFINITY);
  const value = bounded(nextValue, min, max);
  const parts = stepperParts(stepper);
  stepper.dataset.value = String(value);
  if (parts.value) {
    parts.value.value = String(value);
    parts.value.textContent = String(value);
    parts.value.setAttribute('aria-valuenow', String(value));
    if (Number.isFinite(min)) parts.value.setAttribute('aria-valuemin', String(min));
    if (Number.isFinite(max)) parts.value.setAttribute('aria-valuemax', String(max));
  }
  if (parts.decrement) {
    parts.decrement.disabled = value <= min;
    parts.decrement.setAttribute('aria-disabled', String(value <= min));
  }
  if (parts.increment) {
    parts.increment.disabled = value >= max;
    parts.increment.setAttribute('aria-disabled', String(value >= max));
  }
  if (notify) {
    emit(stepper, 'input');
    emit(stepper, 'change');
  }
}

function moveStepper(stepper, direction) {
  const step = number(stepper.dataset.step, 1);
  syncStepper(stepper, number(stepper.dataset.value, 0) + direction * step, true);
}

function selectedButton(group, selector, attribute) {
  return group.querySelector(`${selector}[${attribute}="true"]`) || group.querySelector(selector);
}

function syncGlider(group, selector, attribute) {
  const selected = selectedButton(group, selector, attribute);
  if (!selected) return;
  const groupRect = group.getBoundingClientRect();
  const selectedRect = selected.getBoundingClientRect();
  const x = selectedRect.left - groupRect.left + group.scrollLeft;
  const oldX = group.style.getPropertyValue('--l-active-x');
  const oldWidth = group.style.getPropertyValue('--l-active-width');
  const nextX = `${x}px`;
  const nextWidth = `${selectedRect.width}px`;
  if (oldX !== nextX) group.style.setProperty('--l-active-x', nextX);
  if (oldWidth !== nextWidth) group.style.setProperty('--l-active-width', nextWidth);
}

function selectFilter(group, button, notify = true) {
  const buttons = [...group.querySelectorAll('.l-filter')];
  buttons.forEach(candidate => {
    const selected = candidate === button;
    candidate.setAttribute('aria-pressed', String(selected));
    candidate.tabIndex = selected ? 0 : -1;
  });
  group.dataset.value = button.dataset.value || button.textContent.trim();
  syncGlider(group, '.l-filter', 'aria-pressed');
  if (notify) {
    emit(group, 'input');
    emit(group, 'change');
  }
}

function panelFor(root, tab) {
  const id = tab.getAttribute('aria-controls');
  if (!id) return null;
  const escaped = CSS.escape(id);
  return root.querySelector(`#${escaped}`);
}

function selectTab(root, list, tab, notify = true) {
  const tabs = [...list.querySelectorAll('[role="tab"]')];
  tabs.forEach(candidate => {
    const selected = candidate === tab;
    candidate.setAttribute('aria-selected', String(selected));
    candidate.tabIndex = selected ? 0 : -1;
    const panel = panelFor(root, candidate);
    if (panel) panel.hidden = !selected;
  });
  list.dataset.value = tab.dataset.value || tab.textContent.trim();
  syncGlider(list, '.l-tab', 'aria-selected');
  if (notify) {
    emit(list, 'input');
    emit(list, 'change');
  }
}

function setDisclosure(disclosure, open, notify = true) {
  const trigger = disclosure.querySelector('[data-l-disclosure-trigger]');
  const content = disclosure.querySelector('.l-disclosure-content');
  if (!trigger || !content) return;
  disclosure.toggleAttribute('data-l-open', open);
  trigger.setAttribute('aria-expanded', String(open));
  content.setAttribute('aria-hidden', String(!open));
  content.inert = !open;
  disclosure.style.setProperty('--l-disclosure-height', open ? `${content.scrollHeight}px` : '0px');
  if (notify) emit(disclosure, 'change');
}

function adjacent(buttons, current, key) {
  let index = buttons.indexOf(current);
  if (key === 'Home') index = 0;
  else if (key === 'End') index = buttons.length - 1;
  else if (key === 'ArrowLeft') index = (index - 1 + buttons.length) % buttons.length;
  else if (key === 'ArrowRight') index = (index + 1) % buttons.length;
  return buttons[index];
}

export function keyToken(event) {
  const {code = '', key = ''} = event;
  if (NUMPAD[code]) return NUMPAD[code];
  if (/^Numpad[0-9]$/.test(code)) return `KP_${code.slice(6)}`;
  if (/^[a-z]$/i.test(key)) return key.toUpperCase();
  if (/^Key[A-Z]$/.test(code)) return code.slice(3);
  if (/^[0-9]$/.test(key)) return key;
  if (/^Digit[0-9]$/.test(code)) return code.slice(5);
  if (/^F([1-9]|1[0-9]|2[0-4])$/.test(key)) return key;
  if (/^F([1-9]|1[0-9]|2[0-4])$/.test(code)) return code;
  if (NAMED[key]) return NAMED[key];
  if (PUNCTUATION[code]) return PUNCTUATION[code];
  return '';
}

function standsAlone(token) {
  return /^F\d+$/.test(token) || token.startsWith('XF86');
}

export function chordFrom(event) {
  const token = keyToken(event);
  if (!token) return '';
  const mods = [];
  if (event.metaKey) mods.push('Super');
  if (event.ctrlKey) mods.push('Ctrl');
  if (event.altKey) mods.push('Alt');
  if (event.shiftKey) mods.push('Shift');
  if (!mods.length && !standsAlone(token)) return '';
  return [...mods, token].join('+');
}

export function displayKeys(chord) {
  return String(chord || '').split('+').map(part => part.trim()).filter(Boolean).map(token => {
    if (FACES[token]) return FACES[token];
    const digit = /^KP_([0-9])$/.exec(token);
    return digit ? `Num ${digit[1]}` : token;
  });
}

function partsOf(field) {
  let control = field.querySelector(':scope > .l-chord-field__control');
  if (!control) {
    control = field.ownerDocument.createElement('button');
    control.type = 'button';
    control.className = 'l-chord-field__control l-cut';
    field.prepend(control);
  }
  let conflict = field.querySelector(':scope > .l-chord-field__conflict');
  if (!conflict) {
    conflict = field.ownerDocument.createElement('span');
    conflict.className = 'l-chord-field__conflict';
    conflict.id = `lithic-chord-conflict-${++conflictSequence}`;
    field.append(conflict);
  }
  return {control, conflict};
}

export function renderChordField(field) {
  const {control, conflict} = partsOf(field);
  const documentRoot = field.ownerDocument;
  const recording = field.hasAttribute('data-l-recording');
  const keys = displayKeys(field.dataset.value);
  const content = [];
  if (recording) {
    const prompt = documentRoot.createElement('span');
    prompt.className = 'l-chord-field__prompt';
    prompt.textContent = PROMPT;
    content.push(prompt);
  } else if (keys.length) {
    keys.forEach(name => {
      const key = documentRoot.createElement('kbd');
      key.className = 'l-chord-field__key l-cut';
      key.textContent = name;
      content.push(key);
    });
  } else {
    const placeholder = documentRoot.createElement('span');
    placeholder.className = 'l-chord-field__placeholder';
    placeholder.textContent = field.dataset.lPlaceholder || PLACEHOLDER;
    content.push(placeholder);
  }
  control.replaceChildren(...content);
  control.setAttribute('aria-pressed', String(recording));
  const conflictText = field.dataset.lConflict || '';
  conflict.textContent = conflictText;
  if (conflictText) control.setAttribute('aria-describedby', conflict.id);
  else control.removeAttribute('aria-describedby');
}

function setRecording(field, on) {
  if (field.hasAttribute('data-l-recording') === on) return;
  field.toggleAttribute('data-l-recording', on);
  renderChordField(field);
  emitDetail(field, 'lithic:recording', {recording: on});
}

function initChordFields(root) {
  all(root, '.l-chord-field').forEach(renderChordField);

  const observer = new MutationObserver(records => {
    const fields = new Set();
    records.forEach(record => {
      if (record.target.matches?.('.l-chord-field')) fields.add(record.target);
    });
    fields.forEach(renderChordField);
  });
  observer.observe(root, {subtree: true, attributes: true, attributeFilter: ['data-value', 'data-l-conflict', 'data-l-placeholder']});

  root.addEventListener('click', event => {
    const control = event.target.closest('.l-chord-field__control');
    if (!control || control.disabled) return;
    const field = control.closest('.l-chord-field');
    setRecording(field, !field.hasAttribute('data-l-recording'));
  });

  root.addEventListener('keydown', event => {
    const control = event.target.closest?.('.l-chord-field__control');
    const field = control?.closest('.l-chord-field');
    if (!field?.hasAttribute('data-l-recording')) return;
    const plain = !event.metaKey && !event.ctrlKey && !event.altKey && !event.shiftKey;
    if (event.key === 'Tab' && plain) return;
    event.preventDefault();
    event.stopPropagation();
    if (event.repeat) return;
    if (event.key === 'Escape') {
      setRecording(field, false);
      return;
    }
    if (plain && (event.key === 'Backspace' || event.key === 'Delete')) {
      field.dataset.value = '';
      setRecording(field, false);
      emitDetail(field, 'lithic:clear', {});
      return;
    }
    const chord = chordFrom(event);
    if (!chord) return;
    field.dataset.value = chord;
    setRecording(field, false);
    emitDetail(field, 'lithic:record', {chord});
  }, true);

  root.addEventListener('focusout', event => {
    const field = event.target.closest?.('.l-chord-field');
    if (field?.hasAttribute('data-l-recording') && !field.contains(event.relatedTarget)) setRecording(field, false);
  });
}

function byte(value) {
  return Math.round(Math.min(255, Math.max(0, value)));
}

// #rgb and #rrggbb always; #rgba and #rrggbbaa (CSS order) only when alpha is allowed
export function parseHex(text, allowAlpha = false) {
  const match = /^#([0-9a-f]{3,8})$/i.exec(String(text || '').trim());
  if (!match) return null;
  let digits = match[1].toLowerCase();
  const lengths = allowAlpha ? [3, 4, 6, 8] : [3, 6];
  if (!lengths.includes(digits.length)) return null;
  if (digits.length <= 4) digits = [...digits].map(digit => digit + digit).join('');
  const channel = index => Number.parseInt(digits.slice(index, index + 2), 16);
  return {r: channel(0), g: channel(2), b: channel(4), a: digits.length === 8 ? channel(6) / 255 : 1};
}

export function formatHex({r, g, b, a = 1}) {
  const pair = value => byte(value).toString(16).padStart(2, '0');
  const base = `#${pair(r)}${pair(g)}${pair(b)}`;
  return a < 1 ? `${base}${pair(a * 255)}` : base;
}

// computed colours arrive as rgb(), rgba() or color(srgb ...)
function parseComputed(text) {
  const source = String(text || '').trim();
  let match = /^rgba?\(([^)]+)\)$/i.exec(source);
  if (match) {
    const parts = match[1].split(/[\s,/]+/).filter(Boolean).map(Number.parseFloat);
    if (parts.length < 3 || parts.some(Number.isNaN)) return null;
    return {r: parts[0], g: parts[1], b: parts[2], a: parts.length > 3 ? parts[3] : 1};
  }
  match = /^color\(srgb\s+([^)]+)\)$/i.exec(source);
  if (match) {
    const parts = match[1].split(/[\s/]+/).filter(Boolean).map(Number.parseFloat);
    if (parts.length < 3 || parts.some(Number.isNaN)) return null;
    return {r: parts[0] * 255, g: parts[1] * 255, b: parts[2] * 255, a: parts.length > 3 ? parts[3] : 1};
  }
  return parseHex(source, true);
}

function allowsAlpha(field) {
  return field.hasAttribute('data-l-allow-alpha');
}

function panelOf(field) {
  return field.querySelector('.l-color-field__panel');
}

function optionsOf(field) {
  return [...(panelOf(field)?.querySelectorAll('.l-color-field__swatches [role="option"]') ?? [])];
}

function hexInputs(field) {
  return [...field.querySelectorAll('.l-field .l-text-field')];
}

function alphaSlider(field) {
  return panelOf(field)?.querySelector('.l-color-field__alpha .l-slider') ?? null;
}

function optionColor(option) {
  if (option.dataset.value) return formatHex(parseHex(option.dataset.value, true) ?? {r: 0, g: 0, b: 0});
  const chip = option.querySelector('.l-color-field__chip');
  const color = chip ? parseComputed(getComputedStyle(chip, '::after').backgroundColor) : null;
  return color ? formatHex(color) : '';
}

function prepareOption(option) {
  if (!option.querySelector('.l-color-field__chip')) {
    const chip = option.ownerDocument.createElement('span');
    chip.className = 'l-color-field__chip';
    chip.setAttribute('aria-hidden', 'true');
    option.prepend(chip);
  }
  if (option.dataset.lRole) option.style.setProperty('--l-color', `var(--lithic-${option.dataset.lRole})`);
  else if (option.dataset.value) option.style.setProperty('--l-color', option.dataset.value);
  option.tabIndex = -1;
}

function helpFor(input) {
  const wrapper = input.closest('.l-field');
  let help = wrapper?.querySelector('.l-field-help');
  if (wrapper && !help) {
    help = input.ownerDocument.createElement('span');
    help.className = 'l-field-help';
    help.hidden = true;
    wrapper.append(help);
  }
  return help;
}

function setError(input, on) {
  const wrapper = input.closest('.l-field');
  const help = helpFor(input);
  wrapper?.toggleAttribute('data-error', on);
  input.setAttribute('aria-invalid', String(on));
  if (help) {
    help.textContent = on ? HINT : '';
    help.hidden = !on;
    if (!help.id) help.id = `lithic-colour-help-${++helpSequence}`;
    if (on) input.setAttribute('aria-describedby', help.id);
    else input.removeAttribute('aria-describedby');
  }
}

export function syncColorField(field) {
  const color = parseHex(field.dataset.value, true) ?? {r: 0, g: 0, b: 0, a: 1};
  if (!allowsAlpha(field)) color.a = 1;
  const value = formatHex(color);
  field.dataset.value = value;
  field.style.setProperty('--l-color', value);
  // a field in error keeps the text the user typed until they fix it
  hexInputs(field).forEach(input => {
    if (input.getAttribute('aria-invalid') !== 'true') input.value = value;
  });
  optionsOf(field).forEach(option => option.setAttribute('aria-selected', String(optionColor(option) === value)));
  const slider = alphaSlider(field);
  if (slider) {
    const percent = Math.round(color.a * 100);
    slider.value = String(percent);
    slider.dataset.lValueText = `${percent}%`;
    slider.setAttribute('aria-valuetext', `${percent}%`);
    const output = slider.closest('.l-color-field__alpha')?.querySelector('[data-l-slider-value]');
    if (output) output.textContent = `${percent}%`;
    slider.closest('.l-slider-field')?.style.setProperty('--l-value', String(percent / 100));
  }
  return value;
}

function pick(field, value) {
  const previous = field.dataset.value;
  field.dataset.value = value;
  const next = syncColorField(field);
  if (next !== previous) emitDetail(field, 'lithic:pick', {color: next});
}

function commitInput(input) {
  const field = input.closest('.l-color-field');
  if (!field) return;
  const color = parseHex(input.value, allowsAlpha(field));
  if (!color) {
    setError(input, true);
    return;
  }
  setError(input, false);
  pick(field, formatHex(color));
}

function closePanel(field) {
  const panel = panelOf(field);
  if (!panel?.hasAttribute('data-l-open') || panel.hasAttribute('data-l-static-open')) return;
  field.querySelector(`[data-l-popover="${CSS.escape(panel.id)}"]`)?.click();
}

function initColorFields(root) {
  all(root, '.l-color-field').forEach(field => {
    optionsOf(field).forEach(prepareOption);
    hexInputs(field).forEach(input => {
      input.spellcheck = false;
      input.autocomplete = 'off';
    });
    syncColorField(field);
  });

  root.addEventListener('click', event => {
    const option = event.target.closest('.l-color-field__swatches [role="option"]');
    const field = option?.closest('.l-color-field');
    if (!field || option.matches(':disabled, [aria-disabled="true"]')) return;
    const value = optionColor(option);
    if (!value) return;
    hexInputs(field).forEach(input => setError(input, false));
    pick(field, value);
    closePanel(field);
  });

  root.addEventListener('keydown', event => {
    const input = event.target.closest?.('.l-color-field .l-field .l-text-field');
    if (input && event.key === 'Enter') {
      event.preventDefault();
      commitInput(input);
      return;
    }
    const option = event.target.closest?.('.l-color-field__swatches [role="option"]');
    if (!option || !['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return;
    const options = optionsOf(option.closest('.l-color-field')).filter(candidate => !candidate.matches(':disabled, [aria-disabled="true"]'));
    const index = options.indexOf(option);
    const next = event.key === 'Home' ? options[0]
      : event.key === 'End' ? options.at(-1)
        : options[(index + (event.key === 'ArrowDown' ? 1 : -1) + options.length) % options.length];
    event.preventDefault();
    next?.focus({preventScroll: true});
  });

  root.addEventListener('change', event => {
    if (event.target.matches('.l-color-field .l-field .l-text-field')) commitInput(event.target);
  });

  root.addEventListener('input', event => {
    const target = event.target;
    if (target.matches('.l-color-field .l-field .l-text-field')) {
      if (target.getAttribute('aria-invalid') === 'true') setError(target, false);
      return;
    }
    if (!target.matches('.l-color-field__alpha .l-slider')) return;
    const field = target.closest('.l-color-field');
    const color = parseHex(field.dataset.value, true);
    if (!color) return;
    color.a = Number(target.value) / 100;
    field.dataset.value = formatHex(color);
    syncColorField(field);
  }, true);

  root.addEventListener('change', event => {
    if (!event.target.matches('.l-color-field__alpha .l-slider')) return;
    const field = event.target.closest('.l-color-field');
    emitDetail(field, 'lithic:pick', {color: syncColorField(field)});
  });

  const glyph = (event, name) => {
    const panel = event.target;
    if (!panel.matches?.('.l-color-field__panel')) return;
    const opener = event.detail?.opener;
    const icon = opener?.querySelector('.l-glyph');
    if (icon) icon.textContent = name;
    if (name === 'expand_less') panel.querySelector('[role="option"][aria-selected="true"]')?.focus({preventScroll: true});
  };
  root.addEventListener('lithic:open', event => glyph(event, 'expand_less'));
  root.addEventListener('lithic:close', event => glyph(event, 'expand_more'));
}

const PATH_PLACEHOLDERS = {folder: 'Choose a folder', file: 'Choose a file'};

export function syncPathField(field) {
  const input = field.querySelector('.l-text-field');
  const chooseSlot = field.querySelector('.l-path-field__choose');
  const choose = chooseSlot?.matches('button') ? chooseSlot : chooseSlot?.querySelector('button');
  const kind = field.dataset.lKind === 'file' ? 'file' : 'folder';
  if (input) {
    input.spellcheck = false;
    input.autocomplete = 'off';
    // the kind words the placeholder only while the host has not written its own
    if (!input.hasAttribute('placeholder') || input.hasAttribute('data-l-default-placeholder')) {
      input.placeholder = PATH_PLACEHOLDERS[kind];
      input.setAttribute('data-l-default-placeholder', '');
    }
  }
  const label = field.querySelector('.l-field-label')?.textContent.trim() || input?.getAttribute('aria-label') || '';
  // a host-written name wins; otherwise the library names the plaque after the field
  if (choose && (!choose.hasAttribute('aria-label') || choose.hasAttribute('data-l-managed-label'))) {
    choose.setAttribute('aria-label', label ? `Choose ${label}` : PATH_PLACEHOLDERS[kind]);
    choose.setAttribute('data-l-managed-label', '');
  }
}

function initPathFields(root) {
  all(root, '.l-path-field').forEach(syncPathField);
  const observer = new MutationObserver(records => {
    const fields = new Set();
    records.forEach(record => {
      if (record.target.matches?.('.l-path-field')) fields.add(record.target);
    });
    fields.forEach(syncPathField);
  });
  observer.observe(root, {subtree: true, attributes: true, attributeFilter: ['data-l-kind']});

  root.addEventListener('click', event => {
    const choose = event.target.closest('.l-path-field__choose');
    const field = choose?.closest('.l-path-field');
    if (!field || choose.matches(':disabled, [aria-disabled="true"]')) return;
    emitDetail(field, 'lithic:choose', {kind: field.dataset.lKind === 'file' ? 'file' : 'folder', input: field.querySelector('.l-text-field')});
  });
}

register('controls', root => {
  all(root, '.l-search').forEach(syncSearch);
  all(root, '.l-slider').forEach(syncSlider);
  all(root, '.l-stepper').forEach(stepper => syncStepper(stepper));
  all(root, '.l-filters').forEach(group => {
    const initial = selectedButton(group, '.l-filter', 'aria-pressed');
    if (initial) selectFilter(group, initial, false);
  });
  all(root, '.l-tabs[role="tablist"]').forEach(list => {
    const initial = selectedButton(list, '[role="tab"]', 'aria-selected');
    if (initial) selectTab(root, list, initial, false);
  });
  all(root, '.l-disclosure').forEach(disclosure => {
    const trigger = disclosure.querySelector('[data-l-disclosure-trigger]');
    setDisclosure(disclosure, trigger?.getAttribute('aria-expanded') === 'true', false);
  });

  root.addEventListener('click', event => {
    const blocked = event.target.closest(':is(.l-plaque, .l-icon-plaque):is([aria-busy="true"], [aria-disabled="true"])');
    if (!blocked) return;
    event.preventDefault();
    event.stopImmediatePropagation();
  }, true);

  root.addEventListener('click', event => {
    const reveal = event.target.closest('[data-l-reveal]');
    if (reveal) {
      const field = reveal.closest('.l-field-control')?.querySelector('.l-text-field');
      if (!field) return;
      const showing = field.type === 'text';
      field.type = showing ? 'password' : 'text';
      reveal.setAttribute('aria-pressed', String(!showing));
      reveal.setAttribute('aria-label', showing ? 'Show value' : 'Hide value');
      setGlyph(reveal, showing ? 'visibility' : 'visibility_off');
      field.focus();
      emit(field, 'change');
      return;
    }

    const clear = event.target.closest('[data-l-search-clear]');
    if (clear) {
      const search = clear.closest('.l-search');
      const input = search?.querySelector('input');
      if (!input) return;
      input.value = '';
      syncSearch(search);
      input.focus();
      emit(input, 'input');
      emit(input, 'change');
      return;
    }

    const toggle = event.target.closest('.l-toggle[role="switch"]');
    if (toggle) {
      if (toggle.matches(':disabled, [aria-disabled="true"]')) return;
      toggle.setAttribute('aria-checked', String(toggle.getAttribute('aria-checked') !== 'true'));
      emit(toggle, 'input');
      emit(toggle, 'change');
      return;
    }

    const stepButton = event.target.closest('[data-l-stepper]');
    if (stepButton) {
      const stepper = stepButton.closest('.l-stepper');
      if (!stepper || stepButton.disabled) return;
      moveStepper(stepper, stepButton.dataset.lStepper === 'increment' ? 1 : -1);
      return;
    }

    const filter = event.target.closest('.l-filter');
    if (filter) {
      const group = filter.closest('.l-filters');
      if (group) selectFilter(group, filter);
      return;
    }

    const tab = event.target.closest('[role="tab"].l-tab');
    if (tab) {
      const list = tab.closest('.l-tabs[role="tablist"]');
      if (list) selectTab(root, list, tab);
      return;
    }

    const more = event.target.closest('[data-l-split-more]');
    if (more) {
      const split = more.closest('.l-split-plaque');
      const open = more.getAttribute('aria-expanded') !== 'true';
      more.setAttribute('aria-expanded', String(open));
      setGlyph(more, open ? 'expand_less' : 'expand_more');
      if (split) emit(split, 'change');
      return;
    }

    const disclosureTrigger = event.target.closest('[data-l-disclosure-trigger]');
    if (disclosureTrigger) {
      const disclosure = disclosureTrigger.closest('.l-disclosure');
      if (disclosure) setDisclosure(disclosure, disclosureTrigger.getAttribute('aria-expanded') !== 'true');
    }
  });

  root.addEventListener('input', event => {
    if (event.target.matches('.l-slider')) syncSlider(event.target);
    if (event.target.matches('.l-search input')) syncSearch(event.target.closest('.l-search'));
  });

  root.addEventListener('keydown', event => {
    if (event.key === 'Escape' && event.target.matches('.l-search input')) {
      const search = event.target.closest('.l-search');
      if (!event.target.value) return;
      event.target.value = '';
      syncSearch(search);
      emit(event.target, 'input');
      emit(event.target, 'change');
      return;
    }

    const stepperValue = event.target.closest('.l-stepper-value');
    if (stepperValue && ['ArrowLeft', 'ArrowDown', 'ArrowRight', 'ArrowUp', 'Home', 'End'].includes(event.key)) {
      const stepper = stepperValue.closest('.l-stepper');
      event.preventDefault();
      if (event.key === 'Home') syncStepper(stepper, number(stepper.dataset.min, 0), true);
      else if (event.key === 'End') syncStepper(stepper, number(stepper.dataset.max, 0), true);
      else moveStepper(stepper, ['ArrowRight', 'ArrowUp'].includes(event.key) ? 1 : -1);
      return;
    }

    const filter = event.target.closest('.l-filter');
    if (filter && ['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) {
      const group = filter.closest('.l-filters');
      const buttons = [...group.querySelectorAll('.l-filter:not(:disabled)')];
      const next = adjacent(buttons, filter, event.key);
      event.preventDefault();
      next.focus();
      selectFilter(group, next);
      return;
    }

    const tab = event.target.closest('[role="tab"].l-tab');
    if (tab && ['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) {
      const list = tab.closest('.l-tabs[role="tablist"]');
      const tabs = [...list.querySelectorAll('[role="tab"]:not(:disabled)')];
      const next = adjacent(tabs, tab, event.key);
      event.preventDefault();
      next.focus();
      selectTab(root, list, next);
    }
  });

  const documentRoot = root.nodeType === Node.DOCUMENT_NODE ? root : root.ownerDocument;
  documentRoot.defaultView?.addEventListener('resize', () => {
    all(root, '.l-filters').forEach(group => syncGlider(group, '.l-filter', 'aria-pressed'));
    all(root, '.l-tabs[role="tablist"]').forEach(list => syncGlider(list, '.l-tab', 'aria-selected'));
    all(root, '.l-disclosure[data-l-open]').forEach(disclosure => setDisclosure(disclosure, true, false));
  });

  initChordFields(root);
  initColorFields(root);
  initPathFields(root);
});
