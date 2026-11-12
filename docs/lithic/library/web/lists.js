import tokens from './tokens.js';
import {register} from './core.js';

const rowLists = new WeakMap();
const boundLists = new WeakSet();
const boundGroups = new WeakSet();
const boundMarks = new WeakSet();
const toneAnimations = new WeakMap();
let rowIdentity = 0;

const SETTING_ROW_INTERACTIVE = 'a[href], button, input, select, textarea, label, [contenteditable="true"], [role="switch"], [role="slider"], [role="spinbutton"], [role="combobox"], [tabindex]';
const TILE_MOVES = new Set(['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End']);

function all(root, selector) {
  const matches = root instanceof Element && root.matches(selector) ? [root] : [];
  return matches.concat(Array.from(root.querySelectorAll(selector)));
}

function documentFor(node) {
  return node.ownerDocument || node;
}

function rootElement(root) {
  if (root instanceof Document) return root.documentElement;
  if (root instanceof ShadowRoot) return root.host;
  return root;
}

function motionReduced(root) {
  const documentRoot = documentFor(root);
  return rootElement(root)?.hasAttribute('data-l-reduce') || documentRoot.defaultView?.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

function visibleRows(list) {
  return Array.from(list.querySelectorAll('.l-tablet[role="option"]')).filter(row => !row.closest('[hidden]'));
}
function writeSelected(row, selected) {
  const value = String(selected);
  if (row.getAttribute('aria-selected') !== value) row.setAttribute('aria-selected', value);
}


function interactiveTarget(target) {
  return target.closest('button, a, input, select, textarea, [role="button"]');
}

function updateThumb(state) {
  const {list, items, thumb} = state;
  const viewport = items.clientHeight;
  const content = items.scrollHeight;
  const scrollable = content > viewport;
  list.dataset.lScrollable = String(scrollable);
  if (!scrollable) return;

  const size = Math.max(tokens.size.scrollThumbMin, viewport * viewport / content);
  const range = Math.max(0, viewport - size);
  const offset = items.scrollTop / Math.max(1, content - viewport) * range;
  thumb.style.setProperty('--l-thumb-size', `${size}px`);
  thumb.style.setProperty('--l-thumb-offset', `${offset}px`);
}

function updateSelection(state, initial = false) {
  const {list, items, slab} = state;
  const rows = visibleRows(list);
  let selected = rows.find(row => row.getAttribute('aria-selected') === 'true');
  if (!selected && rows.length) {
    selected = rows[0];
    writeSelected(selected, true);
  }

  if (!selected) {
    slab.hidden = true;
    list.removeAttribute('aria-activedescendant');
    updateThumb(state);
    return;
  }

  rows.forEach(row => writeSelected(row, row === selected));
  if (!selected.id) selected.id = `l-row-${++rowIdentity}`;
  list.setAttribute('aria-activedescendant', selected.id);

  const itemRect = items.getBoundingClientRect();
  const rowRect = selected.getBoundingClientRect();
  const offset = rowRect.top - itemRect.top + items.scrollTop;
  slab.hidden = false;
  slab.style.setProperty('--l-selection-offset', `${offset}px`);
  slab.style.setProperty('--l-selection-height', `${rowRect.height}px`);
  updateThumb(state);

  if (!initial) list.dataset.lSelectionReady = 'true';
}

function selectionEvent(row, rows) {
  const view = row.ownerDocument.defaultView;
  return new view.CustomEvent('lithic-select', {
    bubbles: true,
    detail: {
      index: rows.indexOf(row),
      row,
      value: row.dataset.value || row.id,
    },
  });
}

export function selectRow(list, row, {focus = false, scroll = true} = {}) {
  const state = rowLists.get(list);
  if (!state || !row || row.closest('[hidden]')) return false;
  const rows = visibleRows(list);
  if (!rows.includes(row)) return false;

  rows.forEach(item => writeSelected(item, item === row));
  if (scroll) row.scrollIntoView({block: 'nearest', inline: 'nearest', behavior: 'auto'});
  updateSelection(state);
  if (focus) list.focus({preventScroll: true});
  row.dispatchEvent(selectionEvent(row, rows));
  return true;
}

function activateRow(row) {
  const action = row.querySelector('.l-tablet__action .l-plaque, .l-tablet__action button, .l-tablet__action a');
  if (!action || action.matches(':disabled, [aria-disabled="true"]')) return false;
  action.click();
  return true;
}

function handleListKey(event, state) {
  if (event.target !== state.list) return;
  const rows = visibleRows(state.list);
  if (!rows.length) return;
  const selected = rows.find(row => row.getAttribute('aria-selected') === 'true') || rows[0];
  let index = rows.indexOf(selected);
  let next = null;

  switch (event.key) {
    case 'ArrowDown':
      next = rows[Math.min(rows.length - 1, index + 1)];
      break;
    case 'ArrowUp':
      next = rows[Math.max(0, index - 1)];
      break;
    case 'Home':
      next = rows[0];
      break;
    case 'End':
      next = rows[rows.length - 1];
      break;
    case 'PageDown': {
      const step = Math.max(1, Math.floor(state.items.clientHeight / Math.max(1, selected.getBoundingClientRect().height)));
      next = rows[Math.min(rows.length - 1, index + step)];
      break;
    }
    case 'PageUp': {
      const step = Math.max(1, Math.floor(state.items.clientHeight / Math.max(1, selected.getBoundingClientRect().height)));
      next = rows[Math.max(0, index - step)];
      break;
    }
    case 'Enter':
      event.preventDefault();
      if (!activateRow(selected)) selectRow(state.list, selected);
      return;
    case ' ':
      event.preventDefault();
      selectRow(state.list, selected);
      return;
    default:
      return;
  }

  event.preventDefault();
  selectRow(state.list, next);
}

function refreshRowList(list, initial = false) {
  const state = rowLists.get(list);
  if (!state) return;
  visibleRows(list).forEach(row => {
    row.setAttribute('role', 'option');
    state.resizeObserver?.observe(row);
  });
  updateSelection(state, initial);
}

function setupRowList(list) {
  if (boundLists.has(list)) {
    refreshRowList(list);
    return;
  }
  boundLists.add(list);

  const items = list.querySelector(':scope > .l-row-list__items') || list;
  const documentRoot = documentFor(list);
  const slab = documentRoot.createElement('span');
  slab.className = 'l-row-list__slab l-place-row';
  slab.setAttribute('aria-hidden', 'true');
  items.prepend(slab);

  const thumb = documentRoot.createElement('span');
  thumb.className = 'l-row-list__thumb';
  thumb.setAttribute('aria-hidden', 'true');
  list.append(thumb);

  list.setAttribute('role', 'listbox');
  if (!list.hasAttribute('tabindex')) list.tabIndex = 0;

  const state = {list, items, slab, thumb, resizeObserver: null};
  const Resize = documentRoot.defaultView?.ResizeObserver;
  if (Resize) {
    state.resizeObserver = new Resize(() => updateSelection(state));
    state.resizeObserver.observe(items);
  }
  rowLists.set(list, state);

  items.addEventListener('scroll', () => updateThumb(state), {passive: true});
  list.addEventListener('keydown', event => handleListKey(event, state));
  list.addEventListener('click', event => {
    const row = event.target.closest('.l-tablet[role="option"]');
    if (!row || !list.contains(row) || interactiveTarget(event.target)) return;
    selectRow(list, row, {focus: true});
  });
  list.addEventListener('dblclick', event => {
    const row = event.target.closest('.l-tablet[role="option"]');
    if (!row || !list.contains(row) || interactiveTarget(event.target)) return;
    selectRow(list, row, {focus: true});
    activateRow(row);
  });

  refreshRowList(list, true);
  list.dataset.lSelectionReady = 'true';
}

function groupTarget(toggle) {
  const id = toggle.getAttribute('aria-controls');
  if (!id) return null;
  const root = toggle.getRootNode();
  return root.getElementById?.(id) || null;
}

function syncGroup(toggle) {
  const target = groupTarget(toggle);
  if (!target) return;
  const group = toggle.closest('.l-group');
  const rows = target.querySelectorAll('.l-tablet[role="option"]');
  const count = toggle.closest('.l-group-header')?.querySelector('.l-group-header__count');
  if (count && count.textContent !== String(rows.length)) count.textContent = String(rows.length);
  if (group) group.hidden = rows.length === 0;
  target.hidden = toggle.getAttribute('aria-expanded') === 'false';
}

function setupGroup(toggle) {
  if (boundGroups.has(toggle)) {
    syncGroup(toggle);
    return;
  }
  boundGroups.add(toggle);
  if (!toggle.hasAttribute('aria-expanded')) toggle.setAttribute('aria-expanded', 'true');
  syncGroup(toggle);
  toggle.addEventListener('click', () => {
    const expanded = toggle.getAttribute('aria-expanded') !== 'false';
    toggle.setAttribute('aria-expanded', String(!expanded));
    syncGroup(toggle);
    const list = toggle.closest('.l-row-list');
    if (list) refreshRowList(list);
    toggle.dispatchEvent(new toggle.ownerDocument.defaultView.CustomEvent('lithic-disclosure', {
      bubbles: true,
      detail: {expanded: !expanded, target: groupTarget(toggle)},
    }));
  });
}

function lifecycleActive(element) {
  return element.getAttribute('aria-busy') === 'true' || element.getAttribute('data-active') === 'true';
}

function syncActivity(element) {
  element.dataset.lActive = String(lifecycleActive(element));
  if (element.classList.contains('l-working') && !element.querySelector('.l-working__flake')) {
    const count = Math.round(tokens.size.workingFlakeCount);
    for (let index = 0; index < count; index += 1) {
      const flake = element.ownerDocument.createElement('span');
      flake.className = 'l-working__flake';
      flake.setAttribute('aria-hidden', 'true');
      element.append(flake);
    }
  }
}

function syncBladeProgress(element) {
  syncActivity(element);
  if (element.getAttribute('data-indeterminate') === 'true') return;
  const min = Number(element.getAttribute('aria-valuemin') ?? 0);
  const max = Number(element.getAttribute('aria-valuemax') ?? 100);
  const now = Number(element.getAttribute('aria-valuenow') ?? min);
  const fraction = Math.min(1, Math.max(0, (now - min) / Math.max(Number.EPSILON, max - min)));
  element.style.setProperty('--l-fraction', String(fraction));
}

function syncBladeMeter(element) {
  const min = Number(element.getAttribute('aria-valuemin') ?? 0);
  const max = Number(element.getAttribute('aria-valuemax') ?? 100);
  const now = Number(element.getAttribute('aria-valuenow') ?? min);
  const fraction = Math.min(1, Math.max(0, (now - min) / Math.max(Number.EPSILON, max - min)));
  const threshold = tokens.size.bladeMeterAlertAt;
  element.style.setProperty('--l-fraction', String(fraction));
  if (fraction >= threshold) {
    if (element.dataset.lAutoAttention !== 'true') element.dataset.lAutoAttention = 'true';
    if (element.dataset.tone !== 'attention') element.dataset.tone = 'attention';
  } else if (element.dataset.lAutoAttention === 'true') {
    delete element.dataset.lAutoAttention;
    delete element.dataset.tone;
  }
}

function fitMarks(element) {
  const marks = Array.from(element.querySelectorAll(':scope > .l-marks__mark'));
  marks.forEach(mark => { mark.hidden = false; });
  for (let index = marks.length - 1; index > 0 && element.scrollWidth > element.clientWidth; index -= 1) {
    marks[index].hidden = true;
  }
}

function setupMarks(element) {
  if (boundMarks.has(element)) {
    fitMarks(element);
    return;
  }
  boundMarks.add(element);
  const Resize = element.ownerDocument.defaultView?.ResizeObserver;
  if (Resize) new Resize(() => fitMarks(element)).observe(element);
  fitMarks(element);
}

export function setWorking(element, active) {
  element.setAttribute('aria-busy', String(Boolean(active)));
  syncActivity(element);
  return Boolean(active);
}

function syncBecomingName(element) {
  const min = Number(element.getAttribute('aria-valuemin') ?? 0);
  const max = Number(element.getAttribute('aria-valuemax') ?? 100);
  const now = Number(element.getAttribute('aria-valuenow') ?? min);
  const span = Math.max(Number.EPSILON, max - min);
  const fraction = Math.min(1, Math.max(0, (now - min) / span));
  element.style.setProperty('--l-fraction', String(fraction));
  const text = element.textContent.trim();
  if (element.dataset.lText !== text) element.dataset.lText = text;
  if (!element.hasAttribute('role')) element.setAttribute('role', 'progressbar');
  if (element.getAttribute('aria-valuemin') !== String(min)) element.setAttribute('aria-valuemin', String(min));
  if (element.getAttribute('aria-valuemax') !== String(max)) element.setAttribute('aria-valuemax', String(max));
}

export function setBecomingName(element, fraction, {active = fraction < 1} = {}) {
  const clamped = Math.min(1, Math.max(0, Number(fraction) || 0));
  element.setAttribute('aria-valuemin', '0');
  element.setAttribute('aria-valuemax', '100');
  element.setAttribute('aria-valuenow', String(clamped * 100));
  element.setAttribute('aria-busy', String(Boolean(active)));
  syncBecomingName(element);
  return clamped;
}

function animateTone(element, oldTone, root) {
  if (!oldTone || motionReduced(root) || typeof element.animate !== 'function') return;
  const documentRoot = documentFor(element);
  const probe = documentRoot.createElement('span');
  probe.dataset.tone = oldTone;
  probe.style.cssText = 'position:fixed;visibility:hidden;pointer-events:none;background:var(--l-tone-field)';
  element.append(probe);
  const from = documentRoot.defaultView.getComputedStyle(probe).backgroundColor;
  probe.remove();
  const to = documentRoot.defaultView.getComputedStyle(element).backgroundColor;
  if (!from || !to || from === to) return;

  toneAnimations.get(element)?.cancel();
  const curve = `cubic-bezier(${tokens.motion.curve.join(',')})`;
  const animation = element.animate([{backgroundColor: from}, {backgroundColor: to}], {
    duration: tokens.motion.field,
    easing: curve,
  });
  toneAnimations.set(element, animation);
  animation.addEventListener('finish', () => toneAnimations.delete(element), {once: true});
  animation.addEventListener('cancel', () => toneAnimations.delete(element), {once: true});
}

export function setTone(element, tone) {
  element.dataset.tone = tone;
  return tone;
}

function setupWithin(root) {
  all(root, '.l-row-list').forEach(setupRowList);
  all(root, '.l-group-header__toggle[aria-controls]').forEach(setupGroup);
  all(root, '.l-working, .l-skeleton').forEach(syncActivity);
  all(root, '.l-blade-progress').forEach(syncBladeProgress);
  all(root, '.l-blade-meter').forEach(syncBladeMeter);
  all(root, '.l-becoming-name').forEach(syncBecomingName);
  all(root, '.l-marks').forEach(setupMarks);
}

export function initLists(root) {
  setupWithin(root);
  initSettingRows(root);
  initDesignGalleries(root);
  const Observer = documentFor(root).defaultView?.MutationObserver;
  if (!Observer) return;

  const observer = new Observer(records => {
    for (const record of records) {
      if (record.type === 'childList') {
        record.addedNodes.forEach(node => {
          if (node instanceof Element) setupWithin(node);
        });
        const target = record.target instanceof Element ? record.target : record.target.parentElement;
        const list = target?.closest('.l-row-list');
        if (list) refreshRowList(list);
        const becoming = target?.closest('.l-becoming-name');
        if (becoming) syncBecomingName(becoming);
        const marks = target?.closest('.l-marks');
        if (marks) fitMarks(marks);
        all(root, '.l-group-header__toggle[aria-controls]').forEach(syncGroup);
        continue;
      }

      const element = record.target;
      if (record.attributeName === 'data-tone' && element.classList.contains('l-mineral-field')) {
        animateTone(element, record.oldValue, root);
      } else if (record.attributeName === 'aria-selected' && element.classList.contains('l-tablet')) {
        const list = element.closest('.l-row-list');
        if (list) refreshRowList(list);
      } else if (record.attributeName === 'aria-expanded' && element.classList.contains('l-group-header__toggle')) {
        syncGroup(element);
      } else if (element.classList.contains('l-blade-progress')) {
        syncBladeProgress(element);
      } else if (element.classList.contains('l-blade-meter')) {
        syncBladeMeter(element);
      } else if (element.matches('.l-working, .l-skeleton')) {
        syncActivity(element);
      } else if (element.classList.contains('l-becoming-name')) {
        syncBecomingName(element);
      }
    }
  });

  observer.observe(root instanceof Document ? root.documentElement : root, {
    attributeFilter: ['aria-busy', 'aria-expanded', 'aria-selected', 'aria-valuemax', 'aria-valuemin', 'aria-valuenow', 'data-active', 'data-indeterminate', 'data-tone'],
    attributeOldValue: true,
    attributes: true,
    childList: true,
    subtree: true,
  });
}

function emit(element, name, detail = {}) {
  element.dispatchEvent(new CustomEvent(name, {bubbles: true, composed: true, detail}));
}

function unavailable(row) {
  return row.getAttribute('aria-disabled') === 'true';
}

export function syncSettingRow(row) {
  const off = unavailable(row);
  const control = row.querySelector(':scope > .l-setting-row__control');
  if (control) control.inert = off;
  if (row.hasAttribute('data-l-activatable')) row.tabIndex = off ? -1 : 0;
  const resetPart = row.querySelector(':scope > .l-setting-row__reset');
  const reset = resetPart?.matches('button') ? resetPart : resetPart?.querySelector('button');
  const title = row.querySelector('.l-setting-row__title')?.textContent.trim();
  if (reset && title && !reset.hasAttribute('aria-label')) reset.setAttribute('aria-label', `Reset ${title}`);
}

function activate(row) {
  if (unavailable(row)) return;
  emit(row, 'lithic:activate', {row});
}

function initSettingRows(root) {
  all(root, '.l-setting-row').forEach(syncSettingRow);

  const observer = new MutationObserver(records => {
    const rows = new Set();
    records.forEach(record => {
      if (record.target.matches?.('.l-setting-row')) rows.add(record.target);
    });
    rows.forEach(syncSettingRow);
  });
  observer.observe(root, {subtree: true, attributes: true, attributeFilter: ['aria-disabled', 'data-l-activatable']});

  root.addEventListener('click', event => {
    const reset = event.target.closest('.l-setting-row__reset');
    if (reset) {
      const row = reset.closest('.l-setting-row');
      if (row && !unavailable(row)) emit(row, 'lithic:reset', {row});
      return;
    }
    const row = event.target.closest('.l-setting-row[data-l-activatable]');
    if (!row) return;
    const inner = event.target.closest(SETTING_ROW_INTERACTIVE);
    if (inner && inner !== row && row.contains(inner)) return;
    activate(row);
  });

  root.addEventListener('keydown', event => {
    if (event.key !== 'Enter' && event.key !== ' ') return;
    const row = event.target;
    if (!row.matches?.('.l-setting-row[data-l-activatable]')) return;
    event.preventDefault();
    if (!event.repeat) activate(row);
  });
}

function tilesOf(gallery) {
  return [...gallery.querySelectorAll(':scope > .l-design-tile')];
}

function detailFor(tile) {
  const gallery = tile.closest('.l-design-gallery');
  return {tile, value: tile.dataset.value, index: gallery ? tilesOf(gallery).indexOf(tile) : 0};
}

function ensureRing(tile) {
  if (tile.querySelector(':scope > .l-design-tile__ring')) return;
  const ring = tile.ownerDocument.createElement('span');
  ring.className = 'l-design-tile__ring l-focus-ring';
  ring.setAttribute('aria-hidden', 'true');
  tile.append(ring);
}

function columns(tiles) {
  if (!tiles.length) return 1;
  const top = tiles[0].getBoundingClientRect().top;
  const count = tiles.findIndex(tile => tile.getBoundingClientRect().top > top + 1);
  return count < 0 ? tiles.length : Math.max(1, count);
}

export function selectTile(tile, {notify = true, focus = false} = {}) {
  const gallery = tile.closest('.l-design-gallery');
  const tiles = gallery ? tilesOf(gallery) : [tile];
  const changed = tile.getAttribute('aria-selected') !== 'true';
  tiles.forEach(candidate => {
    candidate.setAttribute('aria-selected', String(candidate === tile));
    candidate.tabIndex = candidate === tile ? 0 : -1;
  });
  if (focus) tile.focus({preventScroll: false});
  if (notify && changed) emit(tile, 'lithic:select', detailFor(tile));
}

function nextTile(gallery, current, key) {
  const tiles = tilesOf(gallery);
  const index = tiles.indexOf(current);
  if (key === 'Home') return tiles[0];
  if (key === 'End') return tiles.at(-1);
  const step = key === 'ArrowLeft' ? -1 : key === 'ArrowRight' ? 1 : key === 'ArrowUp' ? -columns(tiles) : columns(tiles);
  return tiles[Math.min(tiles.length - 1, Math.max(0, index + step))];
}

function initDesignGalleries(root) {
  all(root, '.l-design-gallery').forEach(gallery => {
    if (!gallery.hasAttribute('role')) gallery.setAttribute('role', 'listbox');
    const tiles = tilesOf(gallery);
    const selected = tiles.find(tile => tile.getAttribute('aria-selected') === 'true');
    tiles.forEach(tile => {
      if (!tile.hasAttribute('role')) tile.setAttribute('role', 'option');
      if (!tile.hasAttribute('aria-selected')) tile.setAttribute('aria-selected', 'false');
      tile.tabIndex = tile === (selected || tiles[0]) ? 0 : -1;
    });
  });
  all(root, '.l-design-tile').forEach(tile => {
    ensureRing(tile);
    if (!tile.closest('.l-design-gallery') && !tile.hasAttribute('tabindex')) tile.tabIndex = 0;
  });

  root.addEventListener('click', event => {
    const tile = event.target.closest('.l-design-tile');
    if (!tile) return;
    const action = event.target.closest('.l-design-tile__action');
    if (action) {
      if (!action.matches(':disabled, [aria-disabled="true"], [aria-busy="true"]')) emit(tile, 'lithic:action', detailFor(tile));
      return;
    }
    if (tile.closest('.l-design-gallery')) selectTile(tile);
    emit(tile, 'lithic:activate', detailFor(tile));
  });

  root.addEventListener('keydown', event => {
    const tile = event.target;
    if (!tile.matches?.('.l-design-tile')) return;
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      if (!event.repeat) emit(tile, 'lithic:activate', detailFor(tile));
      return;
    }
    const gallery = tile.closest('.l-design-gallery');
    if (!gallery || !TILE_MOVES.has(event.key)) return;
    event.preventDefault();
    const next = nextTile(gallery, tile, event.key);
    if (next && next !== tile) selectTile(next, {focus: true});
  });
}

register('lists', initLists);
