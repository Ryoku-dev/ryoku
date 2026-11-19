
const WORK_LIMITS = {
    input: 32768,
    output: 262144,
    jsonDepth: 64,
    diffLines: 512,
    diffCells: 65536,
    regexInput: 8192,
    regexPattern: 1024,
    regexFlags: 16,
    regexMatches: 1000,
    regexRequestBytes: 61440,
    regexMilliseconds: 1500
};

function workLimits() {
    return WORK_LIMITS;
}

function inputLimit(toolId) {
    return toolId === "regex_tester" ? WORK_LIMITS.regexInput : WORK_LIMITS.input;
}

function inputError(toolId, input, options = {}) {
    if (String(input).length > inputLimit(toolId))
        return "Input exceeds " + inputLimit(toolId) + " characters";
    if (toolId === "regex_tester") {
        if (String(options.pattern || "").length > WORK_LIMITS.regexPattern)
            return "Regular expression pattern exceeds 1024 characters";
        if (String(options.flags ?? "g").length > WORK_LIMITS.regexFlags)
            return "Regular expression flags exceed 16 characters";
    }
    return "";
}

function regexRequest(input, options = {}) {
    const error = inputError("regex_tester", input, options);
    if (error)
        return { error };
    const request = JSON.stringify({
        input: String(input),
        options: { pattern: String(options.pattern || ""), flags: String(options.flags ?? "g") }
    });
    if (utf8Encode(request).length > WORK_LIMITS.regexRequestBytes)
        return { error: "Regular expression request exceeds 61440 bytes" };
    return { request };
}

function parseBoundedJson(text) {
    let depth = 0;
    let quoted = false;
    let escaped = false;
    for (let index = 0; index < text.length; index++) {
        const character = text[index];
        if (quoted) {
            if (escaped)
                escaped = false;
            else if (character === "\\")
                escaped = true;
            else if (character === '"')
                quoted = false;
        } else if (character === '"') {
            quoted = true;
        } else if (character === "[" || character === "{") {
            if (++depth > WORK_LIMITS.jsonDepth)
                throw new Error("JSON nesting exceeds 64 levels");
        } else if (character === "]" || character === "}") {
            depth--;
        }
    }
    return JSON.parse(text);
}

function utf8Encode(str) {
    if (typeof TextEncoder !== "undefined")
        return new TextEncoder().encode(str);
    const utf8 = [];
    for (let i = 0; i < str.length; i++) {
        let charcode = str.charCodeAt(i);
        if (charcode < 0x80) {
            utf8.push(charcode);
        } else if (charcode < 0x800) {
            utf8.push(0xc0 | (charcode >> 6), 0x80 | (charcode & 0x3f));
        } else if (charcode < 0xd800 || charcode >= 0xe000) {
            utf8.push(0xe0 | (charcode >> 12),
                0x80 | ((charcode >> 6) & 0x3f),
                0x80 | (charcode & 0x3f));
        } else {
            i++;
            charcode = 0x10000 + (((charcode & 0x3ff) << 10) | (str.charCodeAt(i) & 0x3ff));
            utf8.push(0xf0 | (charcode >> 18),
                0x80 | ((charcode >> 12) & 0x3f),
                0x80 | ((charcode >> 6) & 0x3f),
                0x80 | (charcode & 0x3f));
        }
    }
    return new Uint8Array(utf8);
}

function utf8Decode(bytes) {
    if (typeof TextDecoder !== "undefined")
        return new TextDecoder("utf-8").decode(bytes);
    let output = "";
    let index = 0;
    while (index < bytes.length) {
        const first = bytes[index++];
        if (first < 0x80) {
            output += String.fromCharCode(first);
        } else if (first > 0xbf && first < 0xe0) {
            const second = bytes[index++];
            output += String.fromCharCode(((first & 0x1f) << 6) | (second & 0x3f));
        } else if (first > 0xdf && first < 0xf0) {
            const second = bytes[index++];
            const third = bytes[index++];
            output += String.fromCharCode(((first & 0x0f) << 12) | ((second & 0x3f) << 6) | (third & 0x3f));
        } else {
            const second = bytes[index++];
            const third = bytes[index++];
            const fourth = bytes[index++];
            const point = (((first & 0x07) << 18) | ((second & 0x3f) << 12)
                | ((third & 0x3f) << 6) | (fourth & 0x3f)) - 0x10000;
            output += String.fromCharCode(0xd800 + (point >> 10), 0xdc00 + (point & 0x3ff));
        }
    }
    return output;
}

const B64_CHARS = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
const B64_LOOKUP = new Int16Array(256);
B64_LOOKUP.fill(-1);
for (let index = 0; index < B64_CHARS.length; index++)
    B64_LOOKUP[B64_CHARS.charCodeAt(index)] = index;

function bytesToBase64(bytes, urlSafe = false) {
    let output = "";
    for (let index = 0; index < bytes.length; index += 3) {
        const first = bytes[index];
        const second = index + 1 < bytes.length ? bytes[index + 1] : 0;
        const third = index + 2 < bytes.length ? bytes[index + 2] : 0;
        const triple = (first << 16) | (second << 8) | third;
        output += B64_CHARS.charAt((triple >> 18) & 63);
        output += B64_CHARS.charAt((triple >> 12) & 63);
        output += index + 1 < bytes.length ? B64_CHARS.charAt((triple >> 6) & 63) : (urlSafe ? "" : "=");
        output += index + 2 < bytes.length ? B64_CHARS.charAt(triple & 63) : (urlSafe ? "" : "=");
    }
    if (urlSafe)
        output = output.replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
    return output;
}

function base64ToBytes(value, urlSafe = false) {
    let clean = String(value || "").trim().replace(/\s+/g, "");
    if (urlSafe || clean.includes("-") || clean.includes("_")) {
        clean = clean.replace(/-/g, "+").replace(/_/g, "/");
        while (clean.length % 4 !== 0)
            clean += "=";
    }
    if (clean.length % 4 !== 0)
        throw new Error("Invalid Base64 string length");

    let padding = 0;
    if (clean.charAt(clean.length - 1) === "=")
        padding++;
    if (clean.charAt(clean.length - 2) === "=")
        padding++;
    const bytes = new Uint8Array((clean.length * 3) / 4 - padding);
    let cursor = 0;
    for (let index = 0; index < clean.length; index += 4) {
        const firstChar = clean.charCodeAt(index);
        const secondChar = clean.charCodeAt(index + 1);
        const thirdChar = clean.charCodeAt(index + 2);
        const fourthChar = clean.charCodeAt(index + 3);
        const first = B64_LOOKUP[firstChar];
        const second = B64_LOOKUP[secondChar];
        const third = thirdChar === 61 ? 0 : B64_LOOKUP[thirdChar];
        const fourth = fourthChar === 61 ? 0 : B64_LOOKUP[fourthChar];
        if (first < 0 || second < 0 || (thirdChar !== 61 && third < 0) || (fourthChar !== 61 && fourth < 0))
            throw new Error("Invalid Base64 character");
        const triple = (first << 18) | (second << 12) | (third << 6) | fourth;
        bytes[cursor++] = (triple >> 16) & 255;
        if (thirdChar !== 61)
            bytes[cursor++] = (triple >> 8) & 255;
        if (fourthChar !== 61)
            bytes[cursor++] = triple & 255;
    }
    return bytes;
}

function secureRandom(options) {
    const state = options && options._randomState;
    if (!state || !state.bytes || !Number.isFinite(Number(state.index)))
        throw new Error("Secure random source unavailable");
    const byte = () => {
        if (state.index >= state.bytes.length)
            throw new Error("Secure random pool depleted");
        const value = Number(state.bytes[state.index++]);
        if (!Number.isInteger(value) || value < 0 || value > 255)
            throw new Error("Secure random source returned invalid data");
        return value;
    };
    const below = maximum => {
        const max = Number(maximum);
        if (!Number.isInteger(max) || max < 1 || max > 256)
            throw new Error("Invalid secure random range");
        const limit = 256 - (256 % max);
        let value;
        do {
            value = byte();
        } while (value >= limit);
        return value % max;
    };
    return { byte, below };
}

function generatePassword(options = {}) {
    const length = Math.max(4, Math.min(128, Number(options.length) || 20));
    const uppercase = options.uppercase !== false;
    const lowercase = options.lowercase !== false;
    const numbers = options.numbers !== false;
    const symbols = options.symbols !== false;
    const avoidAmbiguous = options.avoidAmbiguous !== false;
    const quantity = Math.max(1, Math.min(50, Number(options.quantity) || 1));
    const random = secureRandom(options);

    let upperChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ";
    let lowerChars = "abcdefghijklmnopqrstuvwxyz";
    let numberChars = "0123456789";
    let symbolChars = "!@#$%^&*()_+-=[]{}|;:,.<>?";
    if (avoidAmbiguous) {
        upperChars = upperChars.replace(/[IO]/g, "");
        lowerChars = lowerChars.replace(/[lo]/g, "");
        numberChars = numberChars.replace(/[01]/g, "");
        symbolChars = symbolChars.replace(/[|`'\"]/g, "");
    }

    let pool = "";
    const alphabets = [];
    if (uppercase) {
        pool += upperChars;
        alphabets.push(upperChars);
    }
    if (lowercase) {
        pool += lowerChars;
        alphabets.push(lowerChars);
    }
    if (numbers) {
        pool += numberChars;
        alphabets.push(numberChars);
    }
    if (symbols) {
        pool += symbolChars;
        alphabets.push(symbolChars);
    }
    if (pool.length === 0) {
        pool = lowerChars;
        alphabets.push(lowerChars);
    }

    const createOne = () => {
        const characters = alphabets.map(alphabet => alphabet[random.below(alphabet.length)]);
        while (characters.length < length)
            characters.push(pool[random.below(pool.length)]);
        for (let index = characters.length - 1; index > 0; index--) {
            const swap = random.below(index + 1);
            const value = characters[index];
            characters[index] = characters[swap];
            characters[swap] = value;
        }
        return characters.join("");
    };
    const passwords = [];
    for (let index = 0; index < quantity; index++)
        passwords.push(createOne());
    return { output: passwords.join("\n") };
}

function generateUuid(options = {}) {
    const uppercase = Boolean(options.uppercase);
    const hyphens = options.hyphens !== false;
    const version = String(options.version ?? "4") === "7" ? 7 : 4;
    const quantity = Math.max(1, Math.min(50, Number(options.quantity) || 1));
    const random = secureRandom(options);
    const createOne = () => {
        const bytes = [];
        for (let index = 0; index < 16; index++)
            bytes.push(random.byte());
        if (version === 7) {
            const timestamp = Date.now();
            bytes[0] = Math.floor(timestamp / 0x10000000000) & 0xff;
            bytes[1] = Math.floor(timestamp / 0x100000000) & 0xff;
            bytes[2] = Math.floor(timestamp / 0x1000000) & 0xff;
            bytes[3] = Math.floor(timestamp / 0x10000) & 0xff;
            bytes[4] = Math.floor(timestamp / 0x100) & 0xff;
            bytes[5] = timestamp & 0xff;
            bytes[6] = (bytes[6] & 0x0f) | 0x70;
        } else {
            bytes[6] = (bytes[6] & 0x0f) | 0x40;
        }
        bytes[8] = (bytes[8] & 0x3f) | 0x80;
        let output = bytes.map(value => value.toString(16).padStart(2, "0")).join("");
        if (hyphens)
            output = `${output.slice(0, 8)}-${output.slice(8, 12)}-${output.slice(12, 16)}-${output.slice(16, 20)}-${output.slice(20)}`;
        return uppercase ? output.toUpperCase() : output;
    };
    const uuids = [];
    for (let index = 0; index < quantity; index++)
        uuids.push(createOne());
    return { output: uuids.join("\n") };
}

function toolJsonFormatter(input = "", options = {}) {
    const text = String(input).trim();
    if (text.length === 0)
        return { output: "" };
    const indentMode = options.indent || "2";
    const sortKeys = Boolean(options.sortKeys);
    let space = 2;
    if (indentMode === "4")
        space = 4;
    else if (indentMode === "tab")
        space = "\t";
    else if (indentMode === "minified")
        space = 0;
    try {
        let parsed = parseBoundedJson(text);
        if (sortKeys && typeof parsed === "object" && parsed !== null) {
            const sortObject = object => {
                if (Array.isArray(object))
                    return object.map(sortObject);
                if (object !== null && typeof object === "object") {
                    return Object.keys(object).sort().reduce((result, key) => {
                        result[key] = sortObject(object[key]);
                        return result;
                    }, {});
                }
                return object;
            };
            parsed = sortObject(parsed);
        }
        return { output: JSON.stringify(parsed, null, space) };
    } catch (error) {
        let line = 1;
        let column = 1;
        const message = error.message || "Invalid JSON";
        const match = message.match(/position (\d+)/i);
        if (match) {
            const before = text.slice(0, parseInt(match[1], 10)).split("\n");
            line = before.length;
            column = before[before.length - 1].length + 1;
        }
        return { output: "", error: `JSON syntax error (line ${line}, column ${column}): ${message}` };
    }
}

function toolBase64(input = "", options = {}) {
    const text = String(input);
    if (text.length === 0)
        return { output: "" };
    try {
        if ((options.mode || "encode") === "decode")
            return { output: utf8Decode(base64ToBytes(text, Boolean(options.urlSafe))) };
        return { output: bytesToBase64(utf8Encode(text), Boolean(options.urlSafe)) };
    } catch (error) {
        return { output: "", error: error.message || "Could not process Base64 data" };
    }
}

function toolUrlEncode(input = "", options = {}) {
    const text = String(input);
    if (text.length === 0)
        return { output: "" };
    const component = options.component !== false;
    try {
        if ((options.mode || "encode") === "decode")
            return { output: component ? decodeURIComponent(text) : decodeURI(text) };
        return { output: component ? encodeURIComponent(text) : encodeURI(text) };
    } catch (error) {
        return { output: "", error: error.message || "Invalid URL encoding sequence" };
    }
}

function toolJwtDecode(input = "") {
    const text = String(input).trim();
    if (text.length === 0)
        return { output: "" };
    const parts = text.split(".");
    if (parts.length !== 2 && parts.length !== 3)
        return { output: "", error: "Invalid JWT format. Expected header.payload.signature" };
    try {
        const decodeSegment = segment => parseBoundedJson(utf8Decode(base64ToBytes(segment, true)));
        const header = decodeSegment(parts[0]);
        const payload = decodeSegment(parts[1]);
        const claims = [];
        if (payload.iss)
            claims.push(`Issuer (iss): ${payload.iss}`);
        if (payload.sub)
            claims.push(`Subject (sub): ${payload.sub}`);
        if (payload.aud)
            claims.push(`Audience (aud): ${Array.isArray(payload.aud) ? payload.aud.join(", ") : payload.aud}`);
        const now = Math.floor(Date.now() / 1000);
        if (payload.exp) {
            const date = new Date(payload.exp * 1000).toISOString().replace("T", " ").replace(/\.\d+Z$/, " UTC");
            const minutes = Math.round(Math.abs(payload.exp - now) / 60);
            claims.push(`Expires (exp): ${date} (${payload.exp < now ? "expired " + minutes + " min ago" : "valid for " + minutes + " min"})`);
        }
        if (payload.iat) {
            const date = new Date(payload.iat * 1000).toISOString().replace("T", " ").replace(/\.\d+Z$/, " UTC");
            claims.push(`Issued at (iat): ${date}`);
        }
        if (payload.nbf) {
            const date = new Date(payload.nbf * 1000).toISOString().replace("T", " ").replace(/\.\d+Z$/, " UTC");
            claims.push(`Not before (nbf): ${date}`);
        }
        const sections = ["Signature is not verified."];
        if (claims.length > 0)
            sections.push("Key claims\n" + claims.join("\n"));
        sections.push("Header\n" + JSON.stringify(header, null, 2));
        sections.push("Payload\n" + JSON.stringify(payload, null, 2));
        return { output: sections.join("\n\n") };
    } catch (error) {
        return { output: "", error: "Could not decode JWT: " + (error.message || "Invalid payload") };
    }
}

function toolUnixTimestamp(input = "") {
    const text = String(input).trim();
    let date;
    if (text.length === 0 || text.toLocaleLowerCase() === "now") {
        date = new Date();
    } else if (/^-?\d+$/.test(text)) {
        const value = Number(text);
        date = new Date(text.length <= 11 ? value * 1000 : value);
    } else {
        date = new Date(text);
    }
    if (isNaN(date.getTime()))
        return { output: "", error: "Invalid date or timestamp: " + text };

    const seconds = Math.floor(date.getTime() / 1000);
    const milliseconds = date.getTime();
    const pad = value => String(value).padStart(2, "0");
    const local = `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
    const difference = Math.round((milliseconds - Date.now()) / 1000);
    let relative;
    if (Math.abs(difference) < 60) {
        relative = difference >= 0 ? "in a few seconds" : "a few seconds ago";
    } else if (Math.abs(difference) < 3600) {
        const minutes = Math.round(Math.abs(difference) / 60);
        relative = difference >= 0 ? `in ${minutes} min` : `${minutes} min ago`;
    } else if (Math.abs(difference) < 86400) {
        const hours = Math.round(Math.abs(difference) / 3600);
        relative = difference >= 0 ? `in ${hours} hours` : `${hours} hours ago`;
    } else {
        const days = Math.round(Math.abs(difference) / 86400);
        relative = difference >= 0 ? `in ${days} days` : `${days} days ago`;
    }
    return {
        output: [
            `Unix (seconds):  ${seconds}`,
            `Unix (millis):   ${milliseconds}`,
            `ISO 8601 (UTC):  ${date.toISOString()}`,
            `Local time:      ${local}`,
            `Relative:        ${relative}`,
            `UTC format:      ${date.toUTCString()}`
        ].join("\n")
    };
}

function parseColor(value) {
    const text = String(value || "").trim().toLocaleLowerCase();
    if (!text)
        return null;
    const hexMatch = text.match(/^#?([0-9a-f]{3,8})$/);
    if (hexMatch) {
        const hex = hexMatch[1];
        let red = 0;
        let green = 0;
        let blue = 0;
        let alpha = 1;
        if (hex.length === 3 || hex.length === 4) {
            red = parseInt(hex[0] + hex[0], 16);
            green = parseInt(hex[1] + hex[1], 16);
            blue = parseInt(hex[2] + hex[2], 16);
            if (hex.length === 4)
                alpha = parseInt(hex[3] + hex[3], 16) / 255;
        } else if (hex.length === 6 || hex.length === 8) {
            red = parseInt(hex.slice(0, 2), 16);
            green = parseInt(hex.slice(2, 4), 16);
            blue = parseInt(hex.slice(4, 6), 16);
            if (hex.length === 8)
                alpha = parseInt(hex.slice(6, 8), 16) / 255;
        } else {
            return null;
        }
        return { red, green, blue, alpha };
    }
    const rgbMatch = text.match(/^rgba?\s*\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*([\d.]+)\s*)?\)$/);
    if (rgbMatch) {
        return {
            red: Math.max(0, Math.min(255, parseInt(rgbMatch[1], 10))),
            green: Math.max(0, Math.min(255, parseInt(rgbMatch[2], 10))),
            blue: Math.max(0, Math.min(255, parseInt(rgbMatch[3], 10))),
            alpha: rgbMatch[4] !== undefined ? Math.max(0, Math.min(1, parseFloat(rgbMatch[4]))) : 1
        };
    }
    const hslMatch = text.match(/^hsla?\s*\(\s*(\d+)\s*,\s*([\d.]+)%\s*,\s*([\d.]+)%\s*(?:,\s*([\d.]+)\s*)?\)$/);
    if (!hslMatch)
        return null;
    const hue = parseInt(hslMatch[1], 10) % 360;
    const saturation = parseFloat(hslMatch[2]) / 100;
    const lightness = parseFloat(hslMatch[3]) / 100;
    const alpha = hslMatch[4] !== undefined ? parseFloat(hslMatch[4]) : 1;
    const chroma = (1 - Math.abs(2 * lightness - 1)) * saturation;
    const secondary = chroma * (1 - Math.abs((hue / 60) % 2 - 1));
    const offset = lightness - chroma / 2;
    let red = 0;
    let green = 0;
    let blue = 0;
    if (hue < 60) {
        red = chroma;
        green = secondary;
    } else if (hue < 120) {
        red = secondary;
        green = chroma;
    } else if (hue < 180) {
        green = chroma;
        blue = secondary;
    } else if (hue < 240) {
        green = secondary;
        blue = chroma;
    } else if (hue < 300) {
        red = secondary;
        blue = chroma;
    } else {
        red = chroma;
        blue = secondary;
    }
    return {
        red: Math.round((red + offset) * 255),
        green: Math.round((green + offset) * 255),
        blue: Math.round((blue + offset) * 255),
        alpha: Math.max(0, Math.min(1, alpha))
    };
}

function toolColorConverter(input = "") {
    const text = String(input).trim();
    if (text.length === 0)
        return { output: "" };
    const color = parseColor(text);
    if (!color)
        return { output: "", error: "Could not parse colour: " + text };
    const { red, green, blue, alpha } = color;
    const hexPair = value => value.toString(16).padStart(2, "0").toUpperCase();
    const hex = `#${hexPair(red)}${hexPair(green)}${hexPair(blue)}`;
    const hexAlpha = alpha < 1 ? `${hex}${hexPair(Math.round(alpha * 255))}` : hex;
    const rgb = `rgb(${red}, ${green}, ${blue})`;
    const rgba = `rgba(${red}, ${green}, ${blue}, ${Math.round(alpha * 100) / 100})`;

    const normalRed = red / 255;
    const normalGreen = green / 255;
    const normalBlue = blue / 255;
    const maximum = Math.max(normalRed, normalGreen, normalBlue);
    const minimum = Math.min(normalRed, normalGreen, normalBlue);
    const delta = maximum - minimum;
    const lightness = (maximum + minimum) / 2;
    let hue = 0;
    let saturation = 0;
    if (delta !== 0) {
        saturation = lightness > 0.5 ? delta / (2 - maximum - minimum) : delta / (maximum + minimum);
        if (maximum === normalRed)
            hue = ((normalGreen - normalBlue) / delta + (normalGreen < normalBlue ? 6 : 0)) * 60;
        else if (maximum === normalGreen)
            hue = ((normalBlue - normalRed) / delta + 2) * 60;
        else
            hue = ((normalRed - normalGreen) / delta + 4) * 60;
    }
    const hueDegrees = Math.round(hue);
    const saturationPercent = Math.round(saturation * 100);
    const lightnessPercent = Math.round(lightness * 100);
    const hsl = `hsl(${hueDegrees}, ${saturationPercent}%, ${lightnessPercent}%)`;
    const hsla = `hsla(${hueDegrees}, ${saturationPercent}%, ${lightnessPercent}%, ${Math.round(alpha * 100) / 100})`;
    return {
        output: [
            `Hex:   ${hexAlpha}`,
            `RGB:   ${alpha < 1 ? rgba : rgb}`,
            `HSL:   ${alpha < 1 ? hsla : hsl}`,
            `Alpha: ${Math.round(alpha * 100)}%`
        ].join("\n")
    };
}

function toolRegexTester(input = "", options = {}) {
    const text = String(input);
    const pattern = String(options.pattern || "");
    const flags = String(options.flags !== undefined ? options.flags : "g");
    if (pattern.length === 0)
        return { output: "Enter a regular expression pattern to test." };
    try {
        const regex = new RegExp(pattern, flags.includes("g") ? flags : flags + "g");
        const matches = [];
        let match;
        let characters = 0;
        while ((match = regex.exec(text)) !== null) {
            if (matches.length === WORK_LIMITS.regexMatches)
                return { output: "", error: "Regular expression exceeds 1000 matches" };
            characters += match.reduce((total, value) => total + String(value ?? "").length, 0);
            if (characters > WORK_LIMITS.output)
                return { output: "", error: "Output exceeds 262144 characters" };
            matches.push({ index: match.index, length: match[0].length, value: match[0], groups: match.slice(1) });
            if (match[0].length === 0) {
                const point = text.codePointAt(regex.lastIndex);
                regex.lastIndex += (flags.includes("u") || flags.includes("v")) && point > 0xffff ? 2 : 1;
            }
        }
        if (matches.length === 0)
            return { output: "No matches found." };
        const lines = [`Found ${matches.length} match${matches.length === 1 ? "" : "es"}:`, ""];
        matches.forEach((entry, index) => {
            lines.push(`[${index + 1}] at index ${entry.index} (length ${entry.length}): "${entry.value}"`);
            entry.groups.forEach((group, groupIndex) => {
                lines.push(`    Group ${groupIndex + 1}: "${group !== undefined ? group : ""}"`);
            });
        });
        return { output: lines.join("\n") };
    } catch (error) {
        return { output: "", error: "Regular expression error: " + error.message };
    }
}

function toolTextDiff(input = "") {
    const parts = String(input).split(/\n===DIFF_SPLIT===\n/);
    if (parts.length !== 2)
        return { output: "Provide original and modified text." };
    const originalLines = parts[0].split(/\r?\n/);
    const modifiedLines = parts[1].split(/\r?\n/);
    const originalCount = originalLines.length;
    const modifiedCount = modifiedLines.length;
    if (originalCount > WORK_LIMITS.diffLines || modifiedCount > WORK_LIMITS.diffLines)
        return { output: "", error: "Text diff exceeds 512 lines per side" };
    if (originalCount * modifiedCount > WORK_LIMITS.diffCells)
        return { output: "", error: "Text diff exceeds 65536 line comparisons" };
    const matrix = Array.from({ length: originalCount + 1 }, () => new Int32Array(modifiedCount + 1));
    for (let original = 0; original < originalCount; original++) {
        for (let modified = 0; modified < modifiedCount; modified++) {
            if (originalLines[original] === modifiedLines[modified])
                matrix[original + 1][modified + 1] = matrix[original][modified] + 1;
            else
                matrix[original + 1][modified + 1] = Math.max(matrix[original + 1][modified], matrix[original][modified + 1]);
        }
    }

    let original = originalCount;
    let modified = modifiedCount;
    let additions = 0;
    let deletions = 0;
    const diff = [];
    while (original > 0 || modified > 0) {
        if (original > 0 && modified > 0 && originalLines[original - 1] === modifiedLines[modified - 1]) {
            diff.push("  " + originalLines[original - 1]);
            original--;
            modified--;
        } else if (modified > 0 && (original === 0 || matrix[original][modified - 1] >= matrix[original - 1][modified])) {
            diff.push("+ " + modifiedLines[modified - 1]);
            additions++;
            modified--;
        } else {
            diff.push("- " + originalLines[original - 1]);
            deletions++;
            original--;
        }
    }
    diff.reverse();
    return {
        output: [
            `--- Original (${originalCount} lines)`,
            `+++ Modified (${modifiedCount} lines)`,
            `@@ -${deletions} +${additions} @@`,
            ...diff
        ].join("\n")
    };
}

function toolChmod(input = "") {
    const text = String(input).trim();
    if (text.length === 0)
        return { output: "" };
    const rwx = digit => (digit & 4 ? "r" : "-") + (digit & 2 ? "w" : "-") + (digit & 1 ? "x" : "-");
    let digits;
    let special = 0;
    if (/^[0-7]{3,4}$/.test(text)) {
        const all = text.split("").map(Number);
        if (all.length === 4)
            special = all.shift();
        digits = all;
    } else {
        const symbolic = text.length === 10 ? text.slice(1) : text;
        if (!/^[r-][w-][xsS-][r-][w-][xsS-][r-][w-][xtT-]$/.test(symbolic))
            return { output: "", error: "Enter octal (755, 0644) or symbolic (rwxr-xr-x, -rw-r--r--) permissions." };
        digits = [0, 1, 2].map(group => {
            const part = symbolic.slice(group * 3, group * 3 + 3);
            if (/[sStT]/.test(part[2]))
                special |= [4, 2, 1][group];
            return (part[0] === "r" ? 4 : 0) + (part[1] === "w" ? 2 : 0) + (/[xst]/.test(part[2]) ? 1 : 0);
        });
    }
    const characters = digits.map(rwx).join("").split("");
    if (special & 4)
        characters[2] = characters[2] === "x" ? "s" : "S";
    if (special & 2)
        characters[5] = characters[5] === "x" ? "s" : "S";
    if (special & 1)
        characters[8] = characters[8] === "x" ? "t" : "T";
    const symbolic = characters.join("");
    const octal = (special > 0 ? String(special) : "") + digits.join("");
    const names = ["Owner", "Group", "Others"];
    const lines = [`Octal     ${octal}`, `Symbolic  ${symbolic}`, `Command   chmod ${octal} <file>`, ""];
    digits.forEach((digit, index) => {
        const rights = [digit & 4 ? "read" : "", digit & 2 ? "write" : "", digit & 1 ? "execute" : ""].filter(Boolean);
        lines.push(`${names[index].padEnd(7)}  ${rwx(digit)}  ${rights.length > 0 ? rights.join(", ") : "no access"}`);
    });
    if (special > 0) {
        const flags = [special & 4 ? "setuid" : "", special & 2 ? "setgid" : "", special & 1 ? "sticky bit" : ""].filter(Boolean);
        lines.push("", `Special   ${flags.join(", ")}`);
    }
    return { output: lines.join("\n") };
}

function executeTool(toolId, input = "", options = {}) {
    switch (toolId) {
    case "password":
        return generatePassword(options);
    case "uuid":
        return generateUuid(options);
    case "json_formatter":
        return toolJsonFormatter(input, options);
    case "base64":
        return toolBase64(input, options);
    case "url_encode":
        return toolUrlEncode(input, options);
    case "jwt_decoder":
        return toolJwtDecode(input);
    case "unix_timestamp":
        return toolUnixTimestamp(input);
    case "color_converter":
        return toolColorConverter(input);
    case "regex_tester":
        return toolRegexTester(input, options);
    case "text_diff":
        return toolTextDiff(input);
    case "chmod_calculator":
        return toolChmod(input);
    default:
        return { output: "", error: "Unknown tool: " + toolId };
    }
}

function runTool(toolId, input = "", options = {}) {
    const error = inputError(toolId, input, options);
    if (error)
        return { output: "", error };
    const result = executeTool(toolId, input, options);
    if (String(result.output ?? "").length > WORK_LIMITS.output)
        return { output: "", error: "Output exceeds 262144 characters" };
    return result;
}
