import tokens from './tokens.js';

const ROLE_KEYS = Object.freeze([
  'ground', 'stone1', 'stone2', 'stone3', 'stone4', 'slab', 'field', 'vein', 'edge',
  'fg', 'fg2', 'fg3', 'sel', 'onSel', 'onSel2',
  'gold', 'onGold', 'goldInk', 'jade', 'lapis', 'amber', 'jasper', 'scrim',
]);

const MODES = new Set(['follow', 'dark', 'light']);
const CONTRASTS = Object.freeze(['standard', 'high']);
const TINTS = Object.freeze(['off', 'low', 'medium']);
const TONES = new Set(['idle', 'alive', 'becoming', 'attention', 'paused', 'failed']);
const BLACK = Object.freeze({r: 0, g: 0, b: 0, a: 255});
const WHITE = Object.freeze({r: 255, g: 255, b: 255, a: 255});

function parseColor(value) {
  if (value && typeof value === 'object' && ['r', 'g', 'b'].every(key => Number.isFinite(value[key]))) {
    return {r: value.r, g: value.g, b: value.b, a: Number.isFinite(value.a) ? value.a : 255};
  }
  if (typeof value !== 'string') return null;
  const rgb = value.match(/^rgb\(\s*(\d+(?:\.\d+)?)\s+(\d+(?:\.\d+)?)\s+(\d+(?:\.\d+)?)(?:\s*\/\s*(\d+(?:\.\d+)?))?\s*\)$/i);
  if (rgb) {
    return {
      r: Math.round(Number(rgb[1])),
      g: Math.round(Number(rgb[2])),
      b: Math.round(Number(rgb[3])),
      a: rgb[4] === undefined ? 255 : Math.round(Number(rgb[4]) * 255),
    };
  }
  if (!/^#[\da-f]{6}([\da-f]{2})?$/i.test(value)) return null;
  const digits = value.slice(1);
  return {
    r: Number.parseInt(digits.slice(0, 2), 16),
    g: Number.parseInt(digits.slice(2, 4), 16),
    b: Number.parseInt(digits.slice(4, 6), 16),
    a: digits.length === 8 ? Number.parseInt(digits.slice(6, 8), 16) : 255,
  };
}

function toCSS(color) {
  const alpha = Math.min(Math.max(color.a ?? 255, 0), 255) / 255;
  return alpha === 1
    ? `rgb(${color.r} ${color.g} ${color.b})`
    : `rgb(${color.r} ${color.g} ${color.b} / ${alpha.toFixed(3)})`;
}

function asColor(value, caller) {
  const parsed = parseColor(value);
  if (!parsed) throw new TypeError(`${caller} expects a supported CSS colour`);
  return parsed;
}

function mixColor(from, to, amount) {
  const channel = key => Math.round(from[key] + (to[key] - from[key]) * amount);
  return {r: channel('r'), g: channel('g'), b: channel('b'), a: channel('a')};
}

function relativeLuminance(color) {
  const linear = value => {
    const channel = value / 255;
    return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * linear(color.r) + 0.7152 * linear(color.g) + 0.0722 * linear(color.b);
}

function linearChannel(value) {
  const channel = value / 255;
  return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
}

// The OKLCH chroma and hue of an sRGB colour: what a palette colour dyes with.
function dyeOf(color) {
  const r = linearChannel(color.r);
  const g = linearChannel(color.g);
  const b = linearChannel(color.b);
  const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b);
  const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b);
  const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
  const a = 1.9779984951 * l - 2.4285922050 * m + 0.4505937099 * s;
  const bb = 0.0259040371 * l + 0.7827717662 * m - 0.8086757660 * s;
  let hue = Math.atan2(bb, a) * 180 / Math.PI;
  if (hue < 0) hue += 360;
  return {c: Math.hypot(a, bb), h: hue};
}

function linearFromOKLCH(lightness, chroma, hue) {
  const a = chroma * Math.cos(hue * Math.PI / 180);
  const bb = chroma * Math.sin(hue * Math.PI / 180);
  const lc = lightness + 0.3963377774 * a + 0.2158037573 * bb;
  const mc = lightness - 0.1055613458 * a - 0.0638541728 * bb;
  const sc = lightness - 0.0894841775 * a - 1.2914855480 * bb;
  const l = lc * lc * lc;
  const m = mc * mc * mc;
  const s = sc * sc * sc;
  return [
    4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
    -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
    -0.0041960863 * l - 0.7034186147 * m + 1.7076147010 * s,
  ];
}

function inGamut(lightness, chroma, hue) {
  return linearFromOKLCH(lightness, chroma, hue).every(value => value >= -1e-7 && value <= 1 + 1e-7);
}

// Paints a role at a ladder lightness with a dye, keeping as much of the dye's
// chroma as sRGB can show at that lightness.
function toned(lightness, dye) {
  let chroma = dye.c;
  if (!inGamut(lightness, chroma, dye.h)) {
    let low = 0;
    let high = chroma;
    for (let step = 0; step < 24; step += 1) {
      const mid = (low + high) / 2;
      if (inGamut(lightness, mid, dye.h)) low = mid;
      else high = mid;
    }
    chroma = low;
  }
  const encode = value => {
    const x = Math.min(Math.max(value, 0), 1);
    const gamma = x <= 0.0031308 ? x * 12.92 : 1.055 * x ** (1 / 2.4) - 0.055;
    return Math.floor(gamma * 255 + 0.5);
  };
  const [r, g, b] = linearFromOKLCH(lightness, chroma, dye.h).map(encode);
  return {r, g, b, a: 255};
}

function accentDye(color) {
  const dye = dyeOf(color);
  return {c: Math.min(tokens.dye.chroma.gold, dye.c * tokens.dye.boost.gold), h: dye.h};
}

function parsedRoles(object) {
  const result = {};
  if (!object || typeof object !== 'object') return result;
  for (const [key, value] of Object.entries(object)) {
    const parsed = parseColor(value);
    if (parsed) result[key] = parsed;
  }
  return result;
}

function descriptor(input) {
  if (!input || typeof input !== 'object') return {roles: {}, source: 'signature', edition: null};
  if (input.roles && typeof input.roles === 'object') {
    return {roles: parsedRoles(input.roles), source: input.source ?? 'scheme', edition: input.edition ?? null};
  }
  if (input.themePalette && typeof input.themePalette === 'object') {
    return {roles: parsedRoles(input.themePalette), source: 'scheme', edition: input.edition ?? null};
  }
  const roles = parsedRoles(input);
  const source = Object.keys(roles).length === 0 ? 'signature' : Object.hasOwn(roles, 'surface') ? 'wallpaper' : 'scheme';
  return {roles, source, edition: input.edition ?? null};
}

function modeObject(group, mode) {
  const value = group?.[mode];
  if (!value || typeof value !== 'object') throw new TypeError(`tokens.js is missing ${mode} palette data`);
  return value;
}

export function contrastRatio(left, right) {
  const values = [relativeLuminance(asColor(left, 'contrastRatio')), relativeLuminance(asColor(right, 'contrastRatio'))]
    .sort((a, b) => b - a);
  return (values[0] + 0.05) / (values[1] + 0.05);
}

export function inkOn(fill) {
  const color = asColor(fill, 'inkOn');
  return toCSS(contrastRatio(BLACK, color) >= contrastRatio(WHITE, color) ? BLACK : WHITE);
}

export function mix(from, to, amount) {
  if (!Number.isFinite(amount) || amount < 0 || amount > 1) throw new RangeError('mix amount must be between 0 and 1');
  return toCSS(mixColor(asColor(from, 'mix'), asColor(to, 'mix'), amount));
}

export function withAlpha(color, alpha) {
  if (!Number.isFinite(alpha) || alpha < 0 || alpha > 1) throw new RangeError('withAlpha alpha must be between 0 and 1');
  const parsed = asColor(color, 'withAlpha');
  return toCSS({...parsed, a: Math.round(parsed.a * alpha)});
}

export function stone(roles, tone) {
  if (!TONES.has(tone)) throw new TypeError(`Unknown Lithic tone: ${tone}`);
  if (tone === 'alive') return roles.jade;
  if (tone === 'becoming') return roles.lapis;
  if (tone === 'attention' || tone === 'paused') return roles.amber;
  if (tone === 'failed') return roles.jasper;
  return roles.vein;
}

// The mineral field of a tone, from roles returned by rolesFromPalette.
export function mineralField(roles, tone) {
  if (!TONES.has(tone)) throw new TypeError(`Unknown Lithic tone: ${tone}`);
  if (tone === 'idle') return roles.stone3;
  return roles.mineralFields[tone === 'paused' ? 'attention' : tone];
}

function ladder(dark, contrast) {
  const key = `${dark ? 'dark' : 'light'}${contrast === 'high' ? 'High' : ''}`;
  const steps = tokens.tone?.[key];
  if (!steps) throw new TypeError(`tokens.js is missing the ${key} tone ladder`);
  return steps;
}

function signatureTable(dark, contrast) {
  const key = `${dark ? 'dark' : 'light'}${contrast === 'high' ? 'High' : ''}`;
  const table = tokens.signature?.[key];
  if (!table) throw new TypeError(`tokens.js is missing the ${key} signature`);
  return Object.fromEntries(Object.entries(table).map(([role, value]) => [role, asColor(value, 'rolesFromPalette')]));
}

const NEUTRAL_ROLES = ['ground', 'stone1', 'stone2', 'stone3', 'stone4', 'slab', 'field', 'vein', 'edge', 'fg', 'fg2', 'fg3', 'scrim'];

// Without a palette Lithic paints its signature: the ryoku.dev stock (black
// paper, bone ink, the sun as the one action), with an edition's signal in
// place of the sun where one is named. A palette only dyes: Lithic owns every
// lightness (the tone ladder for the mode and contrast), and the palette lends
// hue and chroma, neutrals from its surface, ink from its on-surface, the
// action from its primary, the selection from its containers and jasper from
// its error. A forced mode keeps the signature's neutrals.
export function rolesFromPalette(palette = {}, {mode = 'follow', contrast = 'standard', tint = 'off'} = {}) {
  if (!MODES.has(mode)) throw new TypeError(`Unknown Lithic mode: ${mode}`);
  if (!CONTRASTS.includes(contrast)) throw new TypeError(`Unknown Lithic contrast: ${contrast}`);
  if (!TINTS.includes(tint)) throw new TypeError(`Unknown Lithic tint: ${tint}`);
  const {roles: paletteRoles, source, edition} = descriptor(palette);
  const forced = mode !== 'follow';
  const surface = paletteRoles.surface;
  const detectedDark = !surface || (0.299 * surface.r + 0.587 * surface.g + 0.114 * surface.b) / 255 <= 0.5;
  const dark = forced ? mode === 'dark' : detectedDark;
  const steps = ladder(dark, contrast);
  const table = signatureTable(dark, contrast);
  const dye = tokens.dye;
  const fallback = dye.fallback;
  const result = Object.fromEntries(NEUTRAL_ROLES.map(role => [role, table[role]]));

  if (source !== 'wallpaper' && source !== 'scheme') {
    for (const role of ['sel', 'onSel', 'onSel2', 'gold', 'onGold', 'goldInk', 'jasper']) result[role] = table[role];
    const editionKey = typeof edition === 'string' ? edition.toLowerCase() : null;
    if (editionKey && tokens.edition?.[editionKey]) {
      const signal = asColor(tokens.edition[editionKey], 'rolesFromPalette');
      result.gold = signal;
      result.goldInk = signal;
      result.onGold = asColor(inkOn(signal), 'rolesFromPalette');
    }
    return finish(result, steps, dark);
  }

  const accents = tokens.palette.accents;
  if (!forced) {
    let neutral = {c: fallback.stoneChroma, h: fallback.neutralHue};
    let groundChroma = fallback.groundChroma;
    let ink = {c: fallback.inkChroma, h: fallback.neutralHue};
    const surfaceRole = paletteRoles[tokens.palette.dye.neutral];
    if (surfaceRole) {
      const surfaceDye = dyeOf(surfaceRole);
      const factor = tint === 'low' ? dye.tint.low : tint === 'medium' ? dye.tint.medium : 1;
      neutral = {c: Math.min(dye.chroma.stone, surfaceDye.c) * factor, h: surfaceDye.h};
      groundChroma = Math.min(dye.chroma.ground, surfaceDye.c * dye.boost.ground) * factor;
    }
    const inkRole = paletteRoles[tokens.palette.dye.ink];
    if (inkRole) {
      const inkDye = dyeOf(inkRole);
      ink = {c: Math.min(dye.chroma.ink, inkDye.c), h: inkDye.h};
    }
    const vein = {c: neutral.c * dye.boost.vein, h: neutral.h};
    result.ground = toned(steps.ground, {c: groundChroma, h: neutral.h});
    for (const role of ['stone1', 'stone2', 'stone3', 'stone4', 'slab', 'field']) result[role] = toned(steps[role], neutral);
    result.vein = toned(steps.vein, vein);
    result.edge = toned(steps.edge, vein);
    for (const role of ['fg', 'fg2', 'fg3']) result[role] = toned(steps[role], ink);
  }

  let gold = {c: fallback.goldChroma, h: fallback.goldHue};
  const jasper = {c: dye.chroma.jasper, h: fallback.jasperHue};
  let sel = {c: fallback.selChroma, h: fallback.neutralHue};
  const scrimRole = paletteRoles[accents.scrim];
  if (scrimRole) result.scrim = asColor(withAlpha(scrimRole, dark ? tokens.alpha.scrimDark : tokens.alpha.scrimLight), 'rolesFromPalette');
  if (paletteRoles[accents.gold]) gold = accentDye(paletteRoles[accents.gold]);
  const container = paletteRoles[accents.sel];
  if (container) {
    const toward = paletteRoles[accents.selMix];
    const containerDye = dyeOf(toward ? mixColor(container, toward, tokens.alpha.selMix) : container);
    sel = {c: Math.min(dye.chroma.sel, containerDye.c), h: containerDye.h};
  }
  if (paletteRoles[accents.jasper]) jasper.h = dyeOf(paletteRoles[accents.jasper]).h;
  result.gold = toned(steps.gold, gold);
  result.goldInk = toned(steps.goldInk, gold);
  result.onGold = toned(steps.onGold, {c: dye.chroma.onGold, h: gold.h});
  result.jasper = toned(steps.jasper, jasper);
  result.sel = toned(steps.sel, sel);
  result.onSel = toned(steps.onSel, {c: dye.chroma.onSel, h: sel.h});
  result.onSel2 = mixColor(result.onSel, result.sel, tokens.alpha.onSel2);
  return finish(result, steps, dark);
}

// Adds the fixed state minerals and their fields, and serialises the roles.
function finish(result, steps, dark) {
  const minerals = modeObject(tokens.minerals, dark ? 'dark' : 'light');
  for (const mineral of ['jade', 'lapis', 'amber']) result[mineral] = asColor(minerals[mineral], 'rolesFromPalette');
  const mineralFields = Object.fromEntries([['alive', 'jade'], ['becoming', 'lapis'], ['attention', 'amber'], ['failed', 'jasper']]
    .map(([tone, mineral]) => [tone, toCSS(toned(steps.mineralField, {c: tokens.dye.mineral[mineral], h: dyeOf(result[mineral]).h}))]));
  return Object.fromEntries(ROLE_KEYS.map(key => [key, toCSS(result[key])]).concat([
    ['dark', dark], ['jasperInk', inkOn(result.jasper)], ['mineralFields', mineralFields],
  ]));
}

export async function loadPalette(url) {
  if (!url) return {};
  const response = await fetch(url, {headers: {accept: 'application/json'}});
  if (!response.ok) throw new Error(`Palette request failed with ${response.status}`);
  const payload = await response.json();
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) throw new TypeError('Palette response must be a JSON object');
  if (payload.themePalette && typeof payload.themePalette === 'object') {
    return {roles: payload.themePalette, source: 'scheme', edition: payload.edition ?? null};
  }
  return payload;
}

export function applyRoles(roles, element = document.documentElement) {
  if (!element?.style) throw new TypeError('applyRoles expects an element with a style property');
  for (const key of ROLE_KEYS) {
    if (roles[key]) element.style.setProperty(`--lithic-${key.replace(/[A-Z]/g, letter => `-${letter.toLowerCase()}`)}`, roles[key]);
  }
  if (roles.jasperInk) element.style.setProperty('--lithic-jasper-ink', roles.jasperInk);
  for (const [tone, value] of Object.entries(roles.mineralFields ?? {})) {
    element.style.setProperty(`--lithic-mineral-field-${tone}`, value);
  }
  element.dataset.lMode = roles.dark === false ? 'light' : 'dark';
  element.style.colorScheme = roles.dark === false ? 'light' : 'dark';
  return element;
}

export {CONTRASTS, ROLE_KEYS, TINTS};
