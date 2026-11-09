import tokens from './tokens.js';

const ROLE_KEYS = Object.freeze([
  'ground', 'stone1', 'stone2', 'stone3', 'stone4', 'vein',
  'fg', 'fg2', 'fg3', 'sel', 'onSel', 'onSel2',
  'gold', 'onGold', 'goldInk', 'jade', 'lapis', 'amber', 'jasper', 'scrim',
]);

const MODES = new Set(['follow', 'dark', 'light']);
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

function rgbToHSL(color) {
  const [red, green, blue] = [color.r, color.g, color.b].map(value => value / 255);
  const high = Math.max(red, green, blue);
  const low = Math.min(red, green, blue);
  const lightness = (high + low) / 2;
  const delta = high - low;
  if (delta === 0) return [0, 0, lightness];
  const saturation = delta / (1 - Math.abs(2 * lightness - 1));
  let hue;
  if (high === red) hue = ((green - blue) / delta) % 6;
  else if (high === green) hue = (blue - red) / delta + 2;
  else hue = (red - green) / delta + 4;
  hue /= 6;
  return [hue < 0 ? hue + 1 : hue, saturation, lightness];
}

function hslToRGB(hue, saturation, lightness, alpha) {
  const chroma = (1 - Math.abs(2 * lightness - 1)) * saturation;
  const x = chroma * (1 - Math.abs((hue * 6) % 2 - 1));
  const offset = lightness - chroma / 2;
  let channels;
  switch (Math.floor(hue * 6)) {
    case 0: channels = [chroma, x, 0]; break;
    case 1: channels = [x, chroma, 0]; break;
    case 2: channels = [0, chroma, x]; break;
    case 3: channels = [0, x, chroma]; break;
    case 4: channels = [x, 0, chroma]; break;
    default: channels = [chroma, 0, x];
  }
  const channel = value => Math.round(Math.min(Math.max(value + offset, 0), 1) * 255);
  return {r: channel(channels[0]), g: channel(channels[1]), b: channel(channels[2]), a: alpha};
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

function mappedRole(map, name, paletteRoles, fallback) {
  const paletteName = map?.[name];
  return paletteName ? paletteRoles[paletteName] ?? fallback : fallback;
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

export function contrastTone(ink, background, lighter) {
  const foreground = asColor(ink, 'contrastTone');
  const backdrop = asColor(background, 'contrastTone');
  if (contrastRatio(foreground, backdrop) >= tokens.contrast.minimum) return toCSS(foreground);
  const [hue, saturation, lightness] = rgbToHSL(foreground);
  for (let step = 1; step <= 255; step += 1) {
    const amount = step / 255;
    const next = lighter ? lightness + (1 - lightness) * amount : lightness * (1 - amount);
    const candidate = hslToRGB(hue, saturation, next, foreground.a);
    if (contrastRatio(candidate, backdrop) >= tokens.contrast.minimum) return toCSS(candidate);
  }
  return toCSS(lighter ? WHITE : BLACK);
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

export function field(roles, tone) {
  if (!TONES.has(tone)) throw new TypeError(`Unknown Lithic tone: ${tone}`);
  if (tone === 'idle') return roles.stone3;
  return mix(roles.stone2, stone(roles, tone), tone === 'alive' ? tokens.alpha.mineralJade : tokens.alpha.mineral);
}

export function rolesFromPalette(palette = {}, {mode = 'follow'} = {}) {
  if (!MODES.has(mode)) throw new TypeError(`Unknown Lithic mode: ${mode}`);
  const {roles: paletteRoles, source, edition} = descriptor(palette);
  const forced = mode !== 'follow';
  const surface = paletteRoles.surface;
  const detectedDark = !surface || (0.299 * surface.r + 0.587 * surface.g + 0.114 * surface.b) / 255 <= 0.5;
  const dark = forced ? mode === 'dark' : detectedDark;
  const resolvedMode = dark ? 'dark' : 'light';
  const signature = modeObject(tokens.signature, resolvedMode);
  const minerals = modeObject(tokens.minerals, resolvedMode);
  const mapping = modeObject(tokens.palette, resolvedMode);
  const result = parsedRoles(signature);
  Object.assign(result, parsedRoles(minerals));

  const editionKey = typeof edition === 'string' ? edition.toLowerCase() : null;
  const hasEditionAccent = source === 'signature' && editionKey && tokens.edition?.[editionKey];
  if (hasEditionAccent) {
    result.gold = asColor(tokens.edition[editionKey], 'rolesFromPalette');
    result.onGold = asColor(inkOn(result.gold), 'rolesFromPalette');
  }

  if ((source === 'wallpaper' || source === 'scheme') && !forced) {
    for (const name of ['ground', 'stone1', 'stone2', 'stone3', 'stone4', 'vein', 'fg', 'fg2', 'fg3']) {
      result[name] = mappedRole(mapping, name, paletteRoles, result[name]);
    }
  }

  if (source === 'wallpaper' || source === 'scheme') {
    const accents = tokens.palette.accents;
    const toned = tokens.palette.toned;
    result.gold = mappedRole(accents, 'gold', paletteRoles, result.gold);
    result.onGold = mappedRole(accents, 'onGold', paletteRoles, result.onGold);
    result.sel = mappedRole(accents, 'sel', paletteRoles, result.sel);
    result.onSel = mappedRole(accents, 'onSel', paletteRoles, result.onSel);
    result.jasper = mappedRole(accents, 'jasper', paletteRoles, result.jasper);
    const scrim = mappedRole(accents, 'scrim', paletteRoles, result.scrim);
    result.scrim = asColor(withAlpha(scrim, dark ? tokens.alpha.scrimDark : tokens.alpha.scrimLight), 'rolesFromPalette');
    result.onSel2 = mixColor(result.onSel, result.sel, tokens.alpha.onSel2);
    result.goldInk = asColor(contrastTone(result[toned.goldInk], result.stone2, dark), 'rolesFromPalette');
    result.jasper = asColor(contrastTone(result[toned.jasper], result.stone2, dark), 'rolesFromPalette');
  } else if (hasEditionAccent) {
    result.goldInk = asColor(contrastTone(result.gold, result.stone2, dark), 'rolesFromPalette');
  }

  return Object.fromEntries(ROLE_KEYS.map(key => [key, toCSS(result[key])]).concat([['dark', dark], ['jasperInk', inkOn(result.jasper)]]));
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
  element.dataset.lMode = roles.dark === false ? 'light' : 'dark';
  element.style.colorScheme = roles.dark === false ? 'light' : 'dark';
  return element;
}

export {ROLE_KEYS};
