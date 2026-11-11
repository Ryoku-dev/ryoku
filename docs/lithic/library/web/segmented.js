import {register} from './core.js';

const KEYS = new Set(['ArrowLeft', 'ArrowUp', 'ArrowRight', 'ArrowDown', 'Home', 'End']);

function all(root, selector) {
  const matches = [...root.querySelectorAll(selector)];
  if (root.nodeType === Node.ELEMENT_NODE && root.matches(selector)) matches.unshift(root);
  return matches;
}

function emit(element, type) {
  element.dispatchEvent(new Event(type, {bubbles: true}));
}

function segmentsOf(group) {
  return [...group.querySelectorAll(':scope > .l-segment')];
}

function enabled(segment) {
  return !segment.matches(':disabled, [aria-disabled="true"]');
}

function blocked(group) {
  return group.matches('[aria-disabled="true"], [data-disabled="true"]');
}

function ensureThumb(group) {
  if (group.querySelector(':scope > .l-segmented-thumb')) return;
  const thumb = group.ownerDocument.createElement('span');
  thumb.className = 'l-segmented-thumb l-cut l-place-chip';
  thumb.setAttribute('aria-hidden', 'true');
  group.prepend(thumb);
}

export function selectSegment(group, segment, {notify = true, focus = false} = {}) {
  const segments = segmentsOf(group);
  const index = segments.indexOf(segment);
  if (index < 0) return;
  segments.forEach((candidate, position) => {
    candidate.setAttribute('aria-checked', String(position === index));
    candidate.tabIndex = position === index ? 0 : -1;
  });
  group.style.setProperty('--l-segment-count', String(segments.length));
  group.style.setProperty('--l-segment-index', String(index));
  group.dataset.value = segment.dataset.value || segment.textContent.trim();
  if (focus) segment.focus();
  if (!notify) return;
  group.dataset.lSelectionReady = 'true';
  emit(group, 'input');
  emit(group, 'change');
}

function nextSegment(group, current, key) {
  const segments = segmentsOf(group).filter(enabled);
  if (!segments.length) return null;
  if (key === 'Home') return segments[0];
  if (key === 'End') return segments.at(-1);
  const step = key === 'ArrowLeft' || key === 'ArrowUp' ? -1 : 1;
  const index = segments.indexOf(current);
  return segments[(index + step + segments.length) % segments.length];
}

register('segmented', root => {
  all(root, '.l-segmented').forEach(group => {
    if (!group.hasAttribute('role')) group.setAttribute('role', 'radiogroup');
    segmentsOf(group).forEach(segment => {
      if (!segment.hasAttribute('role')) segment.setAttribute('role', 'radio');
    });
    ensureThumb(group);
    const segments = segmentsOf(group);
    const initial = segments.find(segment => segment.getAttribute('aria-checked') === 'true') || segments[0];
    if (initial) selectSegment(group, initial, {notify: false});
  });

  root.addEventListener('click', event => {
    const segment = event.target.closest('.l-segment');
    const group = segment?.closest('.l-segmented');
    if (!group || blocked(group) || !enabled(segment) || segment.getAttribute('aria-checked') === 'true') return;
    selectSegment(group, segment);
  });

  root.addEventListener('keydown', event => {
    if (!KEYS.has(event.key)) return;
    const segment = event.target.closest('.l-segment');
    const group = segment?.closest('.l-segmented');
    if (!group || blocked(group)) return;
    const next = nextSegment(group, segment, event.key);
    event.preventDefault();
    if (next && next !== segment) selectSegment(group, next, {focus: true});
  });
});
