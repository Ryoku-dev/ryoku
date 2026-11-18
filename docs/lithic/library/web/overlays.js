import {register} from './core.js';

const states = new WeakMap();
let anchorSequence = 0;
let sectionPickerSequence = 0;
const menuCanvases = new WeakMap();

function documentFor(root) {
  return root.nodeType === Node.DOCUMENT_NODE ? root : root.ownerDocument;
}

function findById(root, id) {
  if (!id) return null;
  if (root.nodeType === Node.DOCUMENT_NODE) return root.getElementById(id);
  return root.querySelector(`#${CSS.escape(id)}`);
}

function tokenNumber(element, name, fallback = 0) {
  const value = Number.parseFloat(getComputedStyle(element).getPropertyValue(name));
  return Number.isFinite(value) ? value : fallback;
}

function tokenDuration(element, name) {
  const source = getComputedStyle(element).getPropertyValue(name).trim();
  if (source.endsWith('ms')) return Number.parseFloat(source) || 0;
  if (source.endsWith('s')) return (Number.parseFloat(source) || 0) * 1000;
  return Number.parseFloat(source) || 0;
}

function hydrateBrandMark(mark) {
  // a host's own svg replaces the octagon; the library only paints the square around it
  if (mark.querySelector(':scope > svg:not([data-l-brand="library"])')) return;
  const apothem = tokenNumber(mark, '--lithic-size-brand-octagon-apothem-ratio');
  const point = tokenNumber(mark, '--lithic-size-brand-point-ratio');
  const near = tokenNumber(mark, '--lithic-size-brand-pointer-near-ratio');
  const middle = tokenNumber(mark, '--lithic-size-brand-pointer-mid-ratio');
  const far = tokenNumber(mark, '--lithic-size-brand-pointer-far-ratio');
  const center = 0.5;
  const tangent = apothem * (Math.SQRT2 - 1);
  const namespace = 'http://www.w3.org/2000/svg';
  const svg = documentFor(mark).createElementNS(namespace, 'svg');
  const octagon = documentFor(mark).createElementNS(namespace, 'polygon');
  const dot = documentFor(mark).createElementNS(namespace, 'circle');
  const pointer = documentFor(mark).createElementNS(namespace, 'path');

  svg.setAttribute('viewBox', '0 0 1 1');
  svg.setAttribute('aria-hidden', 'true');
  svg.setAttribute('data-l-brand', 'library');
  octagon.setAttribute('class', 'l-brand-mark-octagon');
  octagon.setAttribute('points', [
    [center - tangent, center - apothem],
    [center + tangent, center - apothem],
    [center + apothem, center - tangent],
    [center + apothem, center + tangent],
    [center + tangent, center + apothem],
    [center - tangent, center + apothem],
    [center - apothem, center + tangent],
    [center - apothem, center - tangent],
  ].map(pair => pair.join(',')).join(' '));
  dot.setAttribute('class', 'l-brand-mark-point');
  dot.setAttribute('cx', center);
  dot.setAttribute('cy', center);
  dot.setAttribute('r', point);
  pointer.setAttribute('class', 'l-brand-mark-pointer');
  pointer.setAttribute('d', [
    `M ${center + near} ${center + near}`,
    `L ${center + far} ${center + middle}`,
    `L ${center + middle} ${center + far}`,
    'Z',
  ].join(' '));
  svg.append(octagon, dot, pointer);
  mark.replaceChildren(svg);
}

function reduced(root) {
  const documentRoot = documentFor(root);
  const stateElement = root.nodeType === Node.DOCUMENT_NODE ? root.documentElement : root.host || root;
  return stateElement.hasAttribute('data-l-reduce') || documentRoot.defaultView.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

function emit(element, name, detail = {}) {
  element.dispatchEvent(new CustomEvent(name, {bubbles: true, composed: true, detail}));
}

function focusableItems(menu) {
  return [...menu.querySelectorAll('[role="menuitem"], [role="menuitemradio"], [role="menuitemcheckbox"], [role="option"]')]
    .filter(item => !item.matches(':disabled, [aria-disabled="true"], [hidden]'));
}

function menuFor(overlay) {
  return overlay.matches('.l-menu') ? overlay : overlay.querySelector('.l-menu');
}

function menuViewport(menu) {
  return menu.querySelector(':scope > .l-menu__viewport');
}

function alignMenuLabels(menu) {
  const documentRoot = documentFor(menu);
  let context = menuCanvases.get(documentRoot);
  if (!context) {
    context = documentRoot.createElement('canvas').getContext('2d');
    if (!context) return;
    menuCanvases.set(documentRoot, context);
  }
  menu.querySelectorAll('.l-menu-item-label').forEach(label => {
    const style = getComputedStyle(label);
    context.font = `${style.fontStyle} ${style.fontWeight} ${style.fontSize} ${style.fontFamily}`;
    const reference = context.measureText('H');
    const text = context.measureText(label.textContent);
    // Fallback scripts must not move the primary face's cap-glyph baseline.
    const shift = (reference.fontBoundingBoxAscent - reference.fontBoundingBoxDescent -
      text.fontBoundingBoxAscent + text.fontBoundingBoxDescent) / 2;
    if (Number.isFinite(shift)) label.style.setProperty('--l-menu-label-shift', `${shift}px`);
  });
}

function keepMenuItemVisible(menu, item) {
  const viewport = menuViewport(menu);
  if (!viewport || !item || !menu.contains(item)) return;
  const bounds = viewport.getBoundingClientRect();
  const style = getComputedStyle(viewport);
  const top = bounds.top + Number.parseFloat(style.paddingTop);
  const bottom = bounds.bottom - Number.parseFloat(style.paddingBottom);
  const row = item.getBoundingClientRect();
  if (row.top < top) viewport.scrollTop += row.top - top;
  else if (row.bottom > bottom) viewport.scrollTop += row.bottom - bottom;
}

function prepareMenu(menu) {
  if (menuViewport(menu)) return;
  const viewport = documentFor(menu).createElement('div');
  viewport.className = 'l-menu__viewport l-scroll';
  while (menu.firstChild) viewport.append(menu.firstChild);
  menu.append(viewport);
  menu.addEventListener('wheel', event => {
    if (event.ctrlKey) return;
    const unit = event.deltaMode === 1
      ? tokenNumber(menu, '--lithic-size-menu-item')
      : event.deltaMode === 2 ? viewport.clientHeight : 1;
    viewport.scrollTop += event.deltaY * unit;
    event.preventDefault();
    event.stopPropagation();
  }, {passive: false});
  let touch = null;
  menu.addEventListener('touchstart', event => {
    if (event.touches.length === 1) {
      const point = event.touches[0];
      touch = {id: point.identifier, y: point.clientY};
    } else touch = null;
  }, {passive: true});
  menu.addEventListener('touchmove', event => {
    if (!touch || event.touches.length !== 1) return;
    const point = [...event.touches].find(candidate => candidate.identifier === touch.id);
    if (!point) return;
    viewport.scrollTop += touch.y - point.clientY;
    touch.y = point.clientY;
    event.preventDefault();
    event.stopPropagation();
  }, {passive: false});
  for (const type of ['touchend', 'touchcancel']) {
    menu.addEventListener(type, () => { touch = null; }, {passive: true});
  }
  menu.addEventListener('focusin', event => keepMenuItemVisible(menu, event.target.closest('.l-menu-item')));
  alignMenuLabels(menu);
}

function canAnchor() {
  return CSS.supports('anchor-name: --lithic-anchor') &&
    CSS.supports('position-anchor: --lithic-anchor') &&
    CSS.supports('position-area: bottom');
}

// edgeProperty keeps the overlay that far inside every window edge, so its own
// edge and a focus ring inside it are never cut off
function anchorOverlay(anchor, overlay, gapProperty, edgeProperty = null) {
  if (overlay.hasAttribute('data-l-static-open')) return;
  const view = documentFor(overlay).defaultView;
  const margin = edgeProperty ? tokenNumber(overlay, edgeProperty) : 0;
  const popover = !overlay.matches('.l-tooltip');
  const menu = menuFor(overlay);
  if (!popover && canAnchor()) {
    const name = `--lithic-anchor-${++anchorSequence}`;
    anchor.style.anchorName = name;
    overlay.style.positionAnchor = name;
    overlay.style.left = '';
    overlay.style.top = '';
    overlay.setAttribute('data-l-native-anchor', '');
    // Tooltips can use native anchors; menu placement always reserves one side.
    const placed = overlay.getBoundingClientRect();
    if (placed.left >= margin && placed.top >= margin &&
        placed.right <= view.innerWidth - margin && placed.bottom <= view.innerHeight - margin) return;
  }

  overlay.removeAttribute('data-l-native-anchor');
  overlay.style.positionAnchor = '';
  overlay.style.maxHeight = '';
  overlay.style.removeProperty('--l-menu-available');
  const anchorRect = anchor.getBoundingClientRect();
  const overlayWidth = overlay.offsetWidth;
  const desired = overlay.offsetHeight;
  const gap = tokenNumber(overlay, gapProperty);
  const below = anchorRect.bottom + gap;
  if (!popover) {
    const above = anchorRect.top - gap - desired;
    const clamp = (value, size, extent) => Math.max(margin, Math.min(value, extent - size - margin));
    const top = below + desired <= view.innerHeight - margin ? below
      : above >= margin ? above : clamp(below, desired, view.innerHeight);
    overlay.style.left = `${clamp(anchorRect.left, overlayWidth, view.innerWidth)}px`;
    overlay.style.top = `${top}px`;
    overlay.dataset.lSide = top < anchorRect.top ? 'above' : 'below';
    return;
  }
  const belowRoom = Math.max(0, view.innerHeight - margin - below);
  const aboveRoom = Math.max(0, anchorRect.top - gap - margin);
  const side = desired <= belowRoom ? 'below'
    : desired <= aboveRoom ? 'above'
      : belowRoom >= aboveRoom ? 'below' : 'above';
  const room = side === 'below' ? belowRoom : aboveRoom;
  const height = Math.min(desired, room);
  if (menu) {
    const chrome = Math.max(0, desired - menu.offsetHeight);
    overlay.style.setProperty('--l-menu-available', `${Math.max(0, height - chrome)}px`);
  }
  overlay.style.maxHeight = `${height}px`;
  const top = side === 'below' ? below : anchorRect.top - gap - height;
  const left = Math.max(margin, Math.min(anchorRect.left, view.innerWidth - overlayWidth - margin));
  overlay.style.left = `${left}px`;
  overlay.style.top = `${top}px`;
  overlay.dataset.lSide = side;
}

function revealPopover(target) {
  if (typeof target.showPopover === 'function') {
    if (!target.hasAttribute('popover')) target.setAttribute('popover', 'manual');
    try {
      target.showPopover();
    } catch {
      target.hidden = false;
    }
  } else {
    target.hidden = false;
  }
  target.setAttribute('data-l-open', '');
}

function concealPopover(target) {
  if (typeof target.hidePopover === 'function' && target.matches(':popover-open')) {
    target.hidePopover();
  }
  target.removeAttribute('data-l-open');
  if (!target.hasAttribute('data-l-static-open')) target.hidden = true;
}

function finishPopoverClose(state, target, returnFocus) {
  const opener = state.openers.get(target);
  concealPopover(target);
  target.style.maxHeight = '';
  target.style.removeProperty('--l-menu-available');
  target.removeAttribute('data-l-closing');
  target.removeAttribute('inert');
  if (state.popover === target) state.popover = null;
  state.blocking = Math.max(0, state.blocking - 1);
  emit(target, 'lithic:close', {opener});
  if (returnFocus && opener?.isConnected) opener.focus({preventScroll: true});
}

function closePopover(state, target = state.popover, {returnFocus = true, immediate = false} = {}) {
  if (!target || target.hasAttribute('data-l-static-open') || target.hasAttribute('data-l-closing')) return;
  const opener = state.openers.get(target);
  if (opener) opener.setAttribute('aria-expanded', 'false');
  target.setAttribute('data-l-closing', '');
  target.setAttribute('inert', '');
  state.blocking += 1;
  const duration = immediate || reduced(state.root)
    ? 0
    : tokenDuration(target, '--lithic-motion-menu-out');
  if (duration === 0) {
    finishPopoverClose(state, target, returnFocus);
  } else {
    setTimeout(() => finishPopoverClose(state, target, returnFocus), duration);
  }
}

function openPopover(state, opener, target) {
  if (!target || target.hasAttribute('data-l-static-open')) return;
  if (state.popover && state.popover !== target) {
    closePopover(state, state.popover, {returnFocus: false, immediate: true});
  }
  state.openers.set(target, opener);
  state.popover = target;
  opener.setAttribute('aria-expanded', 'true');
  target.removeAttribute('data-l-closing');
  const menu = menuFor(target);
  if (menu) {
    prepareMenu(menu);
    alignMenuLabels(menu);
  }
  revealPopover(target);
  anchorOverlay(opener, target, '--lithic-size-popover-anchor-gap', '--lithic-size-popover-anchor-gap');
  emit(target, 'lithic:open', {opener});
  const items = focusableItems(target);
  const current = items.find(item => item.matches('[aria-checked="true"], [aria-selected="true"], [aria-current]:not([aria-current="false"])')) || items[0];
  if (current) {
    current.focus({preventScroll: true});
    if (menu) keepMenuItemVisible(menu, current);
  } else if (target.tabIndex >= 0) target.focus({preventScroll: true});
}

function togglePopover(state, opener, target) {
  if (state.popover === target && target.hasAttribute('data-l-open')) closePopover(state, target);
  else openPopover(state, opener, target);
}

function finishDialogClose(state, dialog, returnValue) {
  const opener = state.openers.get(dialog);
  if (dialog.open) dialog.close(returnValue);
  dialog.removeAttribute('data-l-closing');
  dialog.removeAttribute('inert');
  state.blocking = Math.max(0, state.blocking - 1);
  if (opener) opener.setAttribute('aria-expanded', 'false');
  emit(dialog, 'lithic:close', {opener, returnValue});
  if (opener?.isConnected) opener.focus({preventScroll: true});
}

function closeDialog(state, dialog, returnValue = '') {
  if (!dialog?.open || dialog.hasAttribute('data-l-static-open') || dialog.hasAttribute('data-l-closing')) return;
  dialog.setAttribute('data-l-closing', '');
  dialog.setAttribute('inert', '');
  state.blocking += 1;
  const duration = reduced(state.root)
    ? 0
    : tokenDuration(dialog, '--lithic-motion-dialog-out');
  if (duration === 0) finishDialogClose(state, dialog, returnValue);
  else setTimeout(() => finishDialogClose(state, dialog, returnValue), duration);
}

function openDialog(state, opener, dialog) {
  if (!(dialog instanceof HTMLDialogElement) || dialog.hasAttribute('data-l-static-open')) return;
  state.openers.set(dialog, opener);
  opener.setAttribute('aria-expanded', 'true');
  dialog.removeAttribute('data-l-closing');
  if (!dialog.open) dialog.showModal();
  emit(dialog, 'lithic:open', {opener});
  const focus = dialog.querySelector('[autofocus], input, button, select, textarea, [tabindex]:not([tabindex="-1"])');
  focus?.focus({preventScroll: true});
}

function updateConfirm(dialog) {
  const expected = dialog.dataset.lConfirmName;
  if (expected === undefined) return;
  const input = dialog.querySelector('[data-l-confirm-input]');
  const action = dialog.querySelector('[data-l-confirm-action]');
  if (!input || !action) return;
  const valid = input.value === expected;
  action.disabled = !valid;
  action.setAttribute('aria-disabled', String(!valid));
  emit(dialog, 'lithic:confirm-valid', {valid, value: input.value});
}

function selectMenuItem(state, item) {
  if (item.matches(':disabled, [aria-disabled="true"]')) return;
  const menu = item.closest('.l-menu');
  if (!menu) return;
  const overlay = menu.closest('.l-popover') || menu;
  const opener = state.openers.get(overlay);
  if (opener?.classList.contains('l-picker')) {
    menu.querySelectorAll('[role="option"]').forEach(option => option.setAttribute('aria-selected', String(option === item)));
    const value = item.dataset.value || item.querySelector('.l-menu-item-label')?.textContent.trim() || item.textContent.trim();
    const valueNode = opener.querySelector('.l-picker-value');
    if (valueNode) valueNode.textContent = value;
    opener.dataset.value = item.dataset.value || value;
    emit(opener, 'lithic:change', {value: opener.dataset.value, item});
  }
  if (item.getAttribute('role') === 'menuitemcheckbox') {
    item.setAttribute('aria-checked', String(item.getAttribute('aria-checked') !== 'true'));
  }
  if (item.getAttribute('role') === 'menuitemradio') {
    menu.querySelectorAll('[role="menuitemradio"]').forEach(radio => radio.setAttribute('aria-checked', String(radio === item)));
  }
  emit(menu, 'lithic:select', {item, value: item.dataset.value});
  closePopover(state, overlay);
}

function moveMenuFocus(menu, key) {
  const items = focusableItems(menu);
  if (!items.length) return;
  const current = items.indexOf(menu.getRootNode().activeElement || documentFor(menu).activeElement);
  let next = current;
  if (key === 'ArrowDown') next = current < 0 ? 0 : (current + 1) % items.length;
  if (key === 'ArrowUp') next = current < 0 ? items.length - 1 : (current - 1 + items.length) % items.length;
  if (key === 'Home') next = 0;
  if (key === 'End') next = items.length - 1;
  if (key === 'PageDown' || key === 'PageUp') {
    const viewport = menuViewport(menu);
    const row = tokenNumber(menu, '--lithic-size-menu-item');
    const page = Math.max(1, Math.floor((viewport?.clientHeight || row) / row));
    next = Math.max(0, Math.min(items.length - 1, (current < 0 ? 0 : current) + (key === 'PageDown' ? page : -page)));
  }
  items[next].focus({preventScroll: true});
  keepMenuItemVisible(menu, items[next]);
}

function showTooltip(state, anchor, tooltip) {
  if (!tooltip || tooltip.hasAttribute('data-l-static-open')) return;
  if (state.tooltip && state.tooltip !== tooltip) hideTooltip(state, state.tooltip);
  state.tooltip = tooltip;
  state.openers.set(tooltip, anchor);
  if (typeof tooltip.showPopover === 'function') {
    tooltip.setAttribute('popover', 'manual');
    try {
      tooltip.showPopover();
    } catch {
      tooltip.hidden = false;
    }
  } else {
    tooltip.hidden = false;
  }
  tooltip.setAttribute('data-l-open', '');
  anchorOverlay(anchor, tooltip, '--lithic-size-tooltip-anchor-gap');
}

function hideTooltip(state, tooltip = state.tooltip) {
  if (!tooltip) return;
  clearTimeout(state.tooltipTimer);
  if (typeof tooltip.hidePopover === 'function' && tooltip.matches(':popover-open')) tooltip.hidePopover();
  tooltip.removeAttribute('data-l-open');
  if (!tooltip.hasAttribute('data-l-static-open')) tooltip.hidden = true;
  if (state.tooltip === tooltip) state.tooltip = null;
}

function scheduleTooltip(state, anchor, tooltip) {
  clearTimeout(state.tooltipTimer);
  const delay = reduced(state.root) ? 0 : tokenDuration(tooltip, '--lithic-motion-tooltip-delay');
  state.tooltipTimer = setTimeout(() => showTooltip(state, anchor, tooltip), delay);
}

function showPlacard(state, placard) {
  if (!placard || placard.hasAttribute('data-l-static-open')) return;
  const stack = placard.closest('.l-placard-stack');
  placard.hidden = false;
  placard.setAttribute('data-l-open', '');
  requestAnimationFrame(() => placard.setAttribute('data-l-draining', ''));
  if (stack) {
    stack.append(placard);
    const active = [...stack.querySelectorAll(':scope > .l-placard[data-l-open]')];
    const limit = Number.parseInt(getComputedStyle(stack).getPropertyValue('--lithic-size-placard-stack-count'), 10);
    if (Number.isFinite(limit)) {
      active.slice(0, Math.max(0, active.length - limit)).forEach(extra => dismissPlacard(state, extra, 'stack'));
    }
  }
  stopPlacardLife(state, placard);
  const life = tokenDuration(placard, '--lithic-motion-placard-life');
  if (placard.hasAttribute('data-l-autoclose') && life > 0) {
    state.placardLives.set(placard, {left: life, since: 0, timer: 0});
    syncPlacardLife(state, placard);
  }
  emit(placard, 'lithic:open');
}

function stopPlacardLife(state, placard) {
  clearTimeout(state.placardLives.get(placard)?.timer);
  state.placardLives.delete(placard);
  placard.removeAttribute('data-l-held');
}

// The life stands still while the placard is under the pointer or holds focus
// (its action included), and the drain blade stands still with it
function syncPlacardLife(state, placard, held = placard.matches(':hover') || placard.contains(placard.ownerDocument.activeElement)) {
  const life = state.placardLives.get(placard);
  if (!life) return;
  if (held && life.timer) {
    clearTimeout(life.timer);
    life.timer = 0;
    life.left -= performance.now() - life.since;
  } else if (!held && !life.timer) {
    life.since = performance.now();
    life.timer = setTimeout(() => dismissPlacard(state, placard), Math.max(0, life.left));
  }
  placard.toggleAttribute('data-l-held', held);
}

function dismissPlacard(state, placard, reason = 'timeout') {
  if (!placard || placard.hasAttribute('data-l-static-open')) return;
  stopPlacardLife(state, placard);
  placard.removeAttribute('data-l-open');
  placard.removeAttribute('data-l-draining');
  placard.hidden = true;
  emit(placard, 'lithic:placard-dismiss', {reason});
}

function sizeClass(frame) {
  const style = getComputedStyle(frame);
  const medium = Number.parseFloat(style.getPropertyValue('--lithic-breakpoint-medium'));
  const expanded = Number.parseFloat(style.getPropertyValue('--lithic-breakpoint-expanded'));
  return frame.clientWidth >= expanded ? 'expanded' : frame.clientWidth >= medium ? 'medium' : 'compact';
}

function updateFrame(frame) {
  const next = frame.dataset.lPreviewSize || sizeClass(frame);
  if (frame.dataset.lSizeClass === next) return;
  frame.dataset.lSizeClass = next;
  frame.querySelectorAll('.l-rail').forEach(syncRailSelection);
  emit(frame, 'lithic:size-class', {sizeClass: next});
}

function fitRailTiles(rail) {
  const sizeClass = rail.closest('.l-app-frame')?.dataset.lSizeClass;
  const labels = [...rail.querySelectorAll('.l-rail-destination .l-rail-label')];
  if (sizeClass === 'medium' || sizeClass === 'compact') {
    rail.style.removeProperty('--l-rail-tile-width');
    labels.forEach(label => label.closest('.l-rail-destination').style.removeProperty('--l-rail-extra-lines'));
    return;
  }
  const probe = documentFor(rail).createElement('span');
  probe.className = 'l-rail-label';
  probe.setAttribute('aria-hidden', 'true');
  probe.style.cssText = 'position:absolute;visibility:hidden;white-space:nowrap;inline-size:max-content;max-inline-size:none';
  rail.append(probe);
  let widest = 0;
  labels.forEach(label => {
    probe.textContent = label.textContent.trim();
    widest = Math.max(widest, probe.getBoundingClientRect().width);
  });
  probe.remove();
  const inset = tokenNumber(rail, '--lithic-space-s1');
  const least = tokenNumber(rail, '--lithic-size-rail-destination-expanded');
  const most = tokenNumber(rail, '--lithic-size-rail-expanded') - 2 * inset;
  const next = `${Math.min(most, Math.max(least, Math.ceil(widest) + 2 * inset))}px`;
  if (rail.style.getPropertyValue('--l-rail-tile-width') !== next) rail.style.setProperty('--l-rail-tile-width', next);
  // a wrapped label grows its tile by each extra caption line
  const line = tokenNumber(rail, '--lithic-type-caption-line');
  labels.forEach(label => {
    const extra = String(Math.max(0, Math.round(label.getBoundingClientRect().height / line) - 1));
    const tile = label.closest('.l-rail-destination');
    if (tile.style.getPropertyValue('--l-rail-extra-lines') !== extra) tile.style.setProperty('--l-rail-extra-lines', extra);
  });
}

function syncRailSelection(rail) {
  fitRailTiles(rail);
  const current = rail.querySelector('.l-rail-destination[aria-current="page"]');
  let selection = rail.querySelector(':scope > .l-rail-selection');
  if (!selection) {
    selection = documentFor(rail).createElement('span');
    selection.className = 'l-rail-selection l-cut';
    selection.setAttribute('aria-hidden', 'true');
    rail.prepend(selection);
  }
  const bottomBar = rail.closest('.l-app-frame')?.dataset.lSizeClass === 'compact';
  selection.classList.toggle('l-place-row', !bottomBar);
  selection.classList.toggle('l-place-chip', bottomBar);
  if (!current) {
    selection.hidden = true;
    rail.removeAttribute('data-l-selection-ready');
    return;
  }
  const railRect = rail.getBoundingClientRect();
  const currentRect = current.getBoundingClientRect();
  selection.hidden = false;
  selection.style.width = `${currentRect.width}px`;
  selection.style.height = `${currentRect.height}px`;
  selection.style.transform = `translate(${currentRect.left - railRect.left}px, ${currentRect.top - railRect.top}px)`;
  rail.setAttribute('data-l-selection-ready', '');
}

function buildSectionPicker(index) {
  if (index.querySelector(':scope > .l-section-index-picker')) return;
  const links = [...index.querySelectorAll(':scope > a[href^="#"]')];
  if (!links.length) return;
  const documentRoot = documentFor(index);
  const id = `l-section-picker-${++sectionPickerSequence}`;
  const wrapper = documentRoot.createElement('div');
  const picker = documentRoot.createElement('button');
  const label = documentRoot.createElement('span');
  const value = documentRoot.createElement('span');
  const arrow = documentRoot.createElement('span');
  const menu = documentRoot.createElement('div');
  wrapper.className = 'l-section-index-picker';
  picker.type = 'button';
  picker.className = 'l-picker l-cut';
  picker.dataset.lPopover = id;
  picker.dataset.lSectionPicker = '';
  label.className = 'l-picker-label';
  label.textContent = index.getAttribute('aria-label') || '';
  value.className = 'l-picker-value';
  arrow.className = 'l-glyph';
  arrow.setAttribute('aria-hidden', 'true');
  arrow.textContent = 'expand_more';
  picker.append(label, value, arrow);
  menu.id = id;
  menu.className = 'l-menu l-popover l-cut';
  menu.setAttribute('role', 'listbox');
  links.forEach(link => {
    const option = documentRoot.createElement('button');
    const optionLabel = documentRoot.createElement('span');
    option.type = 'button';
    option.className = 'l-menu-item';
    option.setAttribute('role', 'option');
    option.dataset.value = link.hash;
    optionLabel.className = 'l-menu-item-label';
    optionLabel.textContent = link.textContent.trim();
    option.append(optionLabel);
    menu.append(option);
  });
  wrapper.append(picker, menu);
  index.prepend(wrapper);
}

function markSection(index, current) {
  index.querySelectorAll(':scope > a[href^="#"]').forEach(link => {
    const selected = link === current;
    link.setAttribute('aria-current', String(selected));
    link.classList.toggle('l-place-row', selected);
  });
  const picker = index.querySelector(':scope > .l-section-index-picker > .l-picker');
  if (!picker || !current) return;
  picker.dataset.value = current.hash;
  picker.querySelector('.l-picker-value').textContent = current.textContent.trim();
  findById(index.getRootNode(), picker.dataset.lPopover)?.querySelectorAll('[role="option"]').forEach(option => {
    option.setAttribute('aria-selected', String(option.dataset.value === current.hash));
  });
}

function currentSection(index) {
  const links = [...index.querySelectorAll(':scope > a[href^="#"]')];
  const sections = links.map(link => findById(index.getRootNode(), link.hash.slice(1)));
  const viewportLine = documentFor(index).defaultView.innerHeight / 3;
  let current = 0;
  sections.forEach((section, position) => {
    if (section && section.getBoundingClientRect().top <= viewportLine) current = position;
  });
  if (documentFor(index).documentElement.scrollHeight - documentFor(index).defaultView.scrollY <= documentFor(index).defaultView.innerHeight + 1) {
    current = Math.max(0, sections.length - 1);
  }
  markSection(index, links[current]);
  return links[current];
}

function initOverlays(root) {
  if (states.has(root)) return;
  const documentRoot = documentFor(root);
  const state = {
    root,
    popover: null,
    tooltip: null,
    tooltipTimer: 0,
    blocking: 0,
    openers: new WeakMap(),
    placardLives: new WeakMap(),
  };
  states.set(root, state);
  root.querySelectorAll('.l-section-index').forEach(buildSectionPicker);

  root.querySelectorAll('.l-popover:not([data-l-static-open])').forEach(popover => {
    if (!popover.hasAttribute('popover')) popover.setAttribute('popover', 'manual');
    if (!popover.matches(':popover-open')) popover.hidden = true;
  });

  root.querySelectorAll('.l-tooltip:not([data-l-static-open])').forEach(tooltip => {
    tooltip.setAttribute('role', 'tooltip');
    tooltip.setAttribute('popover', 'manual');
    tooltip.hidden = true;
  });

  root.querySelectorAll('.l-placard:not([data-l-static-open]):not([data-l-open])').forEach(placard => {
    placard.hidden = true;
  });

  root.querySelectorAll('.l-menu').forEach(menu => {
    if (!menu.hasAttribute('role')) menu.setAttribute('role', 'menu');
    menu.querySelectorAll('.l-menu-item:not([role])').forEach(item => {
      item.setAttribute('role', menu.getAttribute('role') === 'listbox' ? 'option' : 'menuitem');
    });
    focusableItems(menu).forEach(item => item.tabIndex = -1);
    prepareMenu(menu);
  });

  root.querySelectorAll('[data-l-popover]').forEach(opener => {
    const target = findById(root, opener.dataset.lPopover);
    if (!target) return;
    opener.setAttribute('aria-controls', target.id);
    if (!opener.hasAttribute('aria-expanded')) opener.setAttribute('aria-expanded', 'false');
    const menu = menuFor(target);
    if (!opener.hasAttribute('aria-haspopup') && menu) {
      opener.setAttribute('aria-haspopup', menu.getAttribute('role') === 'listbox' ? 'listbox' : 'menu');
    }
  });

  root.querySelectorAll('.l-picker').forEach(picker => {
    picker.setAttribute('aria-haspopup', 'listbox');
    if (!picker.hasAttribute('aria-expanded')) picker.setAttribute('aria-expanded', 'false');
  });

  root.querySelectorAll('[data-l-dialog]').forEach(opener => {
    opener.setAttribute('aria-haspopup', 'dialog');
    opener.setAttribute('aria-controls', opener.dataset.lDialog);
    if (!opener.hasAttribute('aria-expanded')) opener.setAttribute('aria-expanded', 'false');
  });

  root.querySelectorAll('.l-dialog').forEach(dialog => {
    dialog.addEventListener('cancel', event => {
      event.preventDefault();
      closeDialog(state, dialog, 'cancel');
    });
    dialog.addEventListener('click', event => {
      if (event.target !== dialog || dialog.hasAttribute('data-l-static-open')) return;
      const bounds = dialog.getBoundingClientRect();
      const outside = event.clientX < bounds.left || event.clientX > bounds.right ||
        event.clientY < bounds.top || event.clientY > bounds.bottom;
      if (outside) closeDialog(state, dialog, 'backdrop');
    });
    updateConfirm(dialog);
  });

  root.querySelectorAll('[data-l-tooltip]').forEach(anchor => {
    const tooltip = findById(root, anchor.dataset.lTooltip);
    if (!tooltip) return;
    const describedBy = new Set((anchor.getAttribute('aria-describedby') || '').split(/\s+/).filter(Boolean));
    describedBy.add(tooltip.id);
    anchor.setAttribute('aria-describedby', [...describedBy].join(' '));
  });

  root.querySelectorAll('.l-brand-mark').forEach(hydrateBrandMark);

  root.querySelectorAll('.l-rail').forEach(rail => {
    syncRailSelection(rail);
    const observer = new ResizeObserver(() => syncRailSelection(rail));
    observer.observe(rail);
    state.railObservers ||= [];
    state.railObservers.push(observer);
  });
  // label widths are only true once the faces have loaded
  documentRoot.fonts?.ready.then(() => root.querySelectorAll('.l-rail').forEach(syncRailSelection));

  root.querySelectorAll('.l-app-frame').forEach(frame => {
    updateFrame(frame);
    const observer = new ResizeObserver(() => updateFrame(frame));
    observer.observe(frame);
    state.frameObservers ||= [];
    state.frameObservers.push(observer);
  });

  let sectionFrame = 0;
  const updateIndexes = () => {
    sectionFrame = 0;
    root.querySelectorAll('.l-section-index').forEach(index => {
      if (index.dataset.lHold === 'true') return;
      const link = currentSection(index);
      if (link) emit(index, 'lithic:section-change', {target: link.hash});
    });
  };
  documentRoot.defaultView.addEventListener('scroll', () => {
    root.querySelectorAll('.l-section-index').forEach(index => index.dataset.lHold = 'false');
    if (!sectionFrame) sectionFrame = requestAnimationFrame(updateIndexes);
  }, {passive: true});
  updateIndexes();

  const repositionPopover = () => {
    const target = state.popover;
    const opener = target && state.openers.get(target);
    if (!opener || target.hasAttribute('data-l-closing')) return;
    anchorOverlay(opener, target, '--lithic-size-popover-anchor-gap', '--lithic-size-popover-anchor-gap');
    const menu = menuFor(target);
    if (menu) keepMenuItemVisible(menu, menu.getRootNode().activeElement || documentRoot.activeElement);
  };
  documentRoot.defaultView.addEventListener('resize', repositionPopover);
  documentRoot.addEventListener('scroll', event => {
    if (state.popover?.contains(event.target)) return;
    repositionPopover();
  }, {capture: true, passive: true});
  documentRoot.fonts?.ready.then(() => {
    root.querySelectorAll('.l-menu').forEach(alignMenuLabels);
    repositionPopover();
  });

  root.addEventListener('pointerdown', event => {
    if (state.blocking > 0) {
      event.preventDefault();
      event.stopImmediatePropagation();
      return;
    }
    if (!state.popover) return;
    const opener = state.openers.get(state.popover);
    if (state.popover.contains(event.target) || opener?.contains(event.target)) return;
    event.preventDefault();
    event.stopImmediatePropagation();
    closePopover(state, state.popover);
  }, true);

  root.addEventListener('click', event => {
    const dialogClose = event.target.closest('[data-l-dialog-close]');
    if (dialogClose) {
      const dialog = dialogClose.closest('.l-dialog');
      closeDialog(state, dialog, dialogClose.value || dialogClose.dataset.lDialogClose || 'close');
      return;
    }

    const dialogOpener = event.target.closest('[data-l-dialog]');
    if (dialogOpener) {
      openDialog(state, dialogOpener, findById(root, dialogOpener.dataset.lDialog));
      return;
    }

    const popoverOpener = event.target.closest('[data-l-popover]');
    if (popoverOpener) {
      togglePopover(state, popoverOpener, findById(root, popoverOpener.dataset.lPopover));
      return;
    }

    const menuItem = event.target.closest('.l-menu-item:is([role="menuitem"], [role="menuitemradio"], [role="menuitemcheckbox"], [role="option"])');
    if (menuItem) {
      selectMenuItem(state, menuItem);
      return;
    }

    const placardClose = event.target.closest('[data-l-placard-close]');
    if (placardClose) {
      dismissPlacard(state, placardClose.closest('.l-placard'), 'action');
      return;
    }

    const railDestination = event.target.closest('.l-rail-destination, .l-rail-home');
    if (railDestination) {
      const rail = railDestination.closest('.l-rail');
      rail?.querySelectorAll('[aria-current="page"]').forEach(item => item.removeAttribute('aria-current'));
      railDestination.setAttribute('aria-current', 'page');
      emit(railDestination, 'lithic:navigate', {destination: railDestination.dataset.destination});
      syncRailSelection(rail);
    }

    const indexLink = event.target.closest('.l-section-index a[href^="#"]');
    if (indexLink) {
      const index = indexLink.closest('.l-section-index');
      index.dataset.lHold = 'true';
      markSection(index, indexLink);
    }
  });

  root.addEventListener('lithic:change', event => {
    if (!event.target.matches('[data-l-section-picker]')) return;
    const index = event.target.closest('.l-section-index');
    const link = [...index.querySelectorAll(':scope > a[href^="#"]')].find(candidate => candidate.hash === event.detail.value);
    link?.click();
  });

  root.addEventListener('input', event => {
    const dialog = event.target.closest('.l-confirm');
    if (dialog) updateConfirm(dialog);
  });

  root.addEventListener('keydown', event => {
    const menu = event.target.closest('.l-menu');
    if (menu && ['ArrowDown', 'ArrowUp', 'Home', 'End', 'PageUp', 'PageDown'].includes(event.key)) {
      event.preventDefault();
      event.stopPropagation();
      moveMenuFocus(menu, event.key);
      return;
    }
    if (menu && ['Enter', ' '].includes(event.key)) {
      const item = event.target.closest('.l-menu-item');
      if (item) {
        event.preventDefault();
        selectMenuItem(state, item);
      }
      return;
    }
    // a control that used Escape itself (an edit it dropped) marks it handled
    if (event.key !== 'Escape' || event.defaultPrevented) return;
    if (state.popover) {
      event.preventDefault();
      event.stopPropagation();
      closePopover(state, state.popover);
      return;
    }
    const dialogs = [...root.querySelectorAll('.l-dialog[open]:not([data-l-static-open])')];
    if (dialogs.length) {
      event.preventDefault();
      event.stopPropagation();
      closeDialog(state, dialogs.at(-1), 'cancel');
      return;
    }
    const frame = event.target.closest('.l-app-frame') || root.querySelector('.l-app-frame');
    if (frame) emit(frame, 'lithic:escape');
  });

  // the pointer or focus entering a placard holds its life; leaving both resumes it
  for (const type of ['pointerover', 'pointerout', 'focusin', 'focusout']) {
    root.addEventListener(type, event => {
      const placard = event.target.closest?.('.l-placard[data-l-open]');
      if (!placard || placard.contains(event.relatedTarget)) return;
      const entering = type === 'pointerover' || type === 'focusin';
      const other = type.startsWith('pointer')
        ? placard.contains(placard.ownerDocument.activeElement)
        : placard.matches(':hover');
      syncPlacardLife(state, placard, entering || other);
    });
  }

  root.addEventListener('pointerover', event => {
    const anchor = event.target.closest('[data-l-tooltip]');
    if (!anchor || anchor.contains(event.relatedTarget)) return;
    scheduleTooltip(state, anchor, findById(root, anchor.dataset.lTooltip));
  });

  root.addEventListener('pointerout', event => {
    const anchor = event.target.closest('[data-l-tooltip]');
    if (!anchor || anchor.contains(event.relatedTarget)) return;
    clearTimeout(state.tooltipTimer);
    hideTooltip(state);
  });

  root.addEventListener('focusin', event => {
    const anchor = event.target.closest('[data-l-tooltip]');
    if (anchor) scheduleTooltip(state, anchor, findById(root, anchor.dataset.lTooltip));
  });

  root.addEventListener('focusout', event => {
    const anchor = event.target.closest('[data-l-tooltip]');
    if (anchor && !anchor.contains(event.relatedTarget)) hideTooltip(state);
  });

  root.addEventListener('lithic:placard-show', event => {
    const target = typeof event.detail?.target === 'string'
      ? findById(root, event.detail.target)
      : event.detail?.target;
    showPlacard(state, target);
  });
}

register('overlays', initOverlays);

export {initOverlays};
