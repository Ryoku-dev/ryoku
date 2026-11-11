import {
  applyRoles as writeRoles,
  CONTRASTS,
  loadPalette,
  rolesFromPalette,
  TINTS,
} from './palette.js';

const MODES = new Set(['follow', 'dark', 'light']);
const registry = new Map();
const roots = new Map();
const initialized = new WeakMap();
const keyboardTracked = new WeakSet();
const motionTracked = new WeakSet();
let mode = 'follow';
let look = {contrast: 'standard', tint: 'off'};
let reduceOverride = null;

function contextFor(root) {
  const documentRoot = root?.nodeType === 9 ? root : root?.ownerDocument;
  if (!documentRoot) throw new TypeError('Lithic.init expects a Document, ShadowRoot, or Element');
  const element = root.nodeType === 9 ? root.documentElement : root.host ?? root;
  if (!element?.style) throw new TypeError('Lithic.init root must expose a styled element');
  return {documentRoot, element};
}

function keyboardFocus(documentRoot, element) {
  if (keyboardTracked.has(element)) return;
  const focusKeys = new Set(['Tab', 'ArrowUp', 'ArrowRight', 'ArrowDown', 'ArrowLeft', 'Home', 'End', 'PageUp', 'PageDown']);
  documentRoot.addEventListener('keydown', event => {
    if (focusKeys.has(event.key)) element.toggleAttribute('data-l-keyboard', true);
  }, true);
  const pointer = () => element.removeAttribute('data-l-keyboard');
  documentRoot.addEventListener('pointerdown', pointer, true);
  documentRoot.addEventListener('mousedown', pointer, true);
  documentRoot.addEventListener('touchstart', pointer, {capture: true, passive: true});
  keyboardTracked.add(element);
}

function effectiveReduce(documentRoot) {
  const query = documentRoot.defaultView?.matchMedia?.('(prefers-reduced-motion: reduce)');
  return Boolean(reduceOverride) || Boolean(query?.matches);
}

function writeReduce(state) {
  state.element.toggleAttribute('data-l-reduce', effectiveReduce(state.documentRoot));
}

function trackMotion(documentRoot) {
  if (motionTracked.has(documentRoot) || !documentRoot.defaultView?.matchMedia) return;
  const query = documentRoot.defaultView.matchMedia('(prefers-reduced-motion: reduce)');
  query.addEventListener('change', () => {
    for (const state of roots.values()) writeReduce(state);
  });
  motionTracked.add(documentRoot);
}

const snapTokens = new WeakMap();

function markRoleSnap(element) {
  const token = {};
  snapTokens.set(element, token);
  element.toggleAttribute('data-l-snap', true);
  const frame = element.ownerDocument?.defaultView?.requestAnimationFrame?.bind(element.ownerDocument.defaultView);
  const clear = () => {
    if (snapTokens.get(element) !== token) return;
    snapTokens.delete(element);
    element.removeAttribute('data-l-snap');
  };
  if (frame) {
    frame(() => frame(clear));
  } else {
    queueMicrotask(() => queueMicrotask(clear));
  }
}

function writeSnappedRoles(roles, element) {
  markRoleSnap(element);
  return writeRoles(roles, element);
}

function runModule(name, initFn, root) {
  let names = initialized.get(root);
  if (!names) {
    names = new Set();
    initialized.set(root, names);
  }
  if (names.has(name)) return;
  initFn(root, Lithic);
  names.add(name);
}

function renderState(state) {
  const roles = rolesFromPalette(state.palette, {mode, ...look});
  writeSnappedRoles(roles, state.element);
  state.element.dataset.lPalette = state.source;
  state.element.dataset.lRequestedMode = mode;
  return roles;
}

export function register(name, initFn) {
  if (!name || typeof name !== 'string') throw new TypeError('Lithic.register expects a module name');
  if (typeof initFn !== 'function') throw new TypeError('Lithic.register expects an initializer function');
  if (registry.has(name)) throw new Error(`Lithic module already registered: ${name}`);
  registry.set(name, initFn);
  for (const root of roots.keys()) runModule(name, initFn, root);
  return Lithic;
}

export async function init(root = document) {
  const existing = roots.get(root);
  if (existing) return existing.roles;
  const {documentRoot, element} = contextFor(root);
  const url = new URL(documentRoot.defaultView?.location?.href ?? 'http://localhost/');
  const requestedMode = url.searchParams.get('mode');
  if (MODES.has(requestedMode)) mode = requestedMode;
  const requestedContrast = url.searchParams.get('contrast');
  const requestedTint = url.searchParams.get('tint');
  if (CONTRASTS.includes(requestedContrast)) look = {...look, contrast: requestedContrast};
  if (TINTS.includes(requestedTint)) look = {...look, tint: requestedTint};
  const paletteURL = url.searchParams.get('palette');
  const state = {
    root,
    documentRoot,
    element,
    palette: {},
    source: 'signature',
    roles: null,
  };
  roots.set(root, state);
  keyboardFocus(documentRoot, element);
  trackMotion(documentRoot);
  writeReduce(state);
  state.roles = renderState(state);
  if (paletteURL) {
    state.palette = await loadPalette(paletteURL);
    state.source = 'url';
    state.roles = renderState(state);
  }
  for (const [name, initFn] of registry) runModule(name, initFn, root);
  return state.roles;
}

export function setMode(nextMode) {
  if (!MODES.has(nextMode)) throw new TypeError(`Unknown Lithic mode: ${nextMode}`);
  mode = nextMode;
  for (const state of roots.values()) state.roles = renderState(state);
  return mode;
}

export function setLook({contrast = look.contrast, tint = look.tint} = {}) {
  if (!CONTRASTS.includes(contrast)) throw new TypeError(`Unknown Lithic contrast: ${contrast}`);
  if (!TINTS.includes(tint)) throw new TypeError(`Unknown Lithic tint: ${tint}`);
  look = {contrast, tint};
  for (const state of roots.values()) state.roles = renderState(state);
  return {...look};
}

export function setReduce(on = null) {
  if (on !== null && typeof on !== 'boolean') throw new TypeError('Lithic.setReduce expects true, false, or null');
  reduceOverride = on;
  for (const state of roots.values()) writeReduce(state);
  return reduceOverride;
}

export function applyRoles(roles, element = document.documentElement) {
  return writeSnappedRoles(roles, element);
}

const Lithic = Object.freeze({
  init,
  register,
  setMode,
  setLook,
  setReduce,
  applyRoles,
  rolesFromPalette,
});

export {rolesFromPalette};
export default Lithic;
