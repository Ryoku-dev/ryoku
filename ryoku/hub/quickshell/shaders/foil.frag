#version 440
// a holo foil as a trading card has it: a soft wash of the palette's four state colours in
// broad diagonal bands, and one sheen of light across it, both sliding as the card tilts
// (`shift`). Smooth, never dotted, and held to a low opacity so what lies on it stays read
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
    float cut;
};

vec3 band(float t) {
    float f = fract(t) * 4.0;
    vec3 a = f < 1.0 ? c1.rgb : f < 2.0 ? c2.rgb : f < 3.0 ? c3.rgb : c4.rgb;
    vec3 b = f < 1.0 ? c2.rgb : f < 2.0 ? c3.rgb : f < 3.0 ? c4.rgb : c1.rgb;
    return mix(a, b, smoothstep(0.0, 1.0, fract(f)));
}

void main() {
    vec2 uv = qt_TexCoord0;
    // a cut corner stays clear, so the foil keeps the card's shape
    vec2 p = uv * size;
    vec2 q = min(p, size - p);
    float inside = step(cut, q.x + q.y);

    float t = uv.x * 0.7 + uv.y * 0.4 + shift.x * 0.35 + shift.y * 0.2;
    vec3 wash = band(t * 0.9);
    float swell = 0.6 + 0.4 * sin(t * 7.0);

    // the sheen: one soft line of light that crosses as the card turns
    float along = uv.x * 0.75 + uv.y * 0.35 - (0.55 + shift.x * 0.6 + shift.y * 0.25);
    float sheen = exp(-along * along / 0.012);

    float a = strength * (0.22 * swell + 0.55 * sheen);
    vec3 col = mix(wash, vec3(1.0), sheen * 0.45);
    fragColor = vec4(col, 1.0) * clamp(a, 0.0, 0.6) * inside * qt_Opacity;
}
