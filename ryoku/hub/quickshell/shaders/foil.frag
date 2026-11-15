#version 440
// a holo foil as a trading card has: diagonal bands of the palette's four state colours
// set as 1-bit dots, and one bright streak, all sliding as the card tilts (`shift`)
layout(location = 0) in vec2 qt_TexCoord0;
layout(location = 0) out vec4 fragColor;
layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;
    float qt_Opacity;
    vec4 c1;
    vec4 c2;
    vec4 c3;
    vec4 c4;
    vec2 size;
    vec2 shift;
    float strength;
    float cell;
    float cut;
};

// the 4x4 Bayer threshold, as a branch chain: GLSL 1.20 drivers refuse constant arrays
float bayer4(vec2 pos) {
    vec2 f = floor(mod(pos, 4.0));
    float i = f.y * 4.0 + f.x;
    float t = 5.5;
    if (i < 0.5) t = 0.5; else if (i < 1.5) t = 8.5; else if (i < 2.5) t = 2.5;
    else if (i < 3.5) t = 10.5; else if (i < 4.5) t = 12.5; else if (i < 5.5) t = 4.5;
    else if (i < 6.5) t = 14.5; else if (i < 7.5) t = 6.5; else if (i < 8.5) t = 3.5;
    else if (i < 9.5) t = 11.5; else if (i < 10.5) t = 1.5; else if (i < 11.5) t = 9.5;
    else if (i < 12.5) t = 15.5; else if (i < 13.5) t = 7.5; else if (i < 14.5) t = 13.5;
    return t / 16.0;
}
vec3 band(float t) {
    float f = fract(t) * 4.0;
    if (f < 1.0) return mix(c1.rgb, c2.rgb, f);
    if (f < 2.0) return mix(c2.rgb, c3.rgb, f - 1.0);
    if (f < 3.0) return mix(c3.rgb, c4.rgb, f - 2.0);
    return mix(c4.rgb, c1.rgb, f - 3.0);
}

void main() {
    vec2 uv = qt_TexCoord0;
    // a cut corner stays clear, so the foil keeps the card's shape
    vec2 p = uv * size;
    vec2 q = min(p, size - p);
    float inside = step(cut, q.x + q.y);
    float t = uv.x * 0.9 + uv.y * 0.55 + shift.x * 0.45 + shift.y * 0.3;
    float wave = 0.5 + 0.5 * sin(t * 18.0);
    float streak = exp(-pow((uv.x * 0.7 + uv.y * 0.3) - (0.5 + shift.x * 0.55 + shift.y * 0.2), 2.0) / 0.004);
    // a faint foil keeps a faint streak, so text over it stays readable
    streak *= min(1.0, strength * 2.0);
    float level = clamp(strength * (0.35 + 0.65 * wave) + streak * 0.6, 0.0, 1.0);
    vec2 px = floor(uv * size / max(cell, 1.0));
    float on = step(bayer4(px), level);
    vec3 col = mix(band(t * 1.6), vec3(1.0), streak * 0.5);
    fragColor = vec4(col, 1.0) * on * inside * (0.55 + 0.45 * streak) * qt_Opacity;
}
