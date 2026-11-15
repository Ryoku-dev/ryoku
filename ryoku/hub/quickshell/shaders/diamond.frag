#version 440
// a Secret rare's border, cut like a gem: the band is faceted into small triangles, each
// catching its own colour from the palette's minerals as the card tilts (`shift`), with
// darker seams between facets and a few that flash white now and then (`time`)
layout(location = 0) in vec2 qt_TexCoord0;
layout(location = 0) out vec4 fragColor;
layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;
    float qt_Opacity;
    vec4 c1;
    vec4 c2;
    vec4 c3;
    vec4 c4;
    vec4 c5;
    vec2 size;
    vec2 shift;
    float time;
    float cut;
    float facet;
};

float hash(vec2 p) {
    return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453);
}

vec3 mineral(float t) {
    float f = fract(t) * 5.0;
    vec3 a = f < 1.0 ? c1.rgb : f < 2.0 ? c2.rgb : f < 3.0 ? c3.rgb : f < 4.0 ? c4.rgb : c5.rgb;
    vec3 b = f < 1.0 ? c2.rgb : f < 2.0 ? c3.rgb : f < 3.0 ? c4.rgb : f < 4.0 ? c5.rgb : c1.rgb;
    return mix(a, b, smoothstep(0.0, 1.0, fract(f)));
}

void main() {
    vec2 p = qt_TexCoord0 * size;
    vec2 q = min(p, size - p);
    float inside = step(cut, q.x + q.y);

    // facets: a grid turned 45 degrees, each square split into two triangles
    vec2 r = vec2(p.x + p.y, p.x - p.y) * 0.7071 / facet;
    vec2 cell = floor(r);
    vec2 f = fract(r);
    float upper = step(f.y, f.x);
    vec2 id = cell * 2.0 + vec2(upper, 0.0);
    float h = hash(id);

    // each facet's own tilt, so it lights as the card turns past it
    vec2 n = normalize(vec2(h - 0.5, hash(id + 7.0) - 0.5));
    float catchLight = 0.5 + 0.5 * cos(dot(n, shift) * 5.0 + h * 6.2832 + time * 0.6);
    vec3 hue = mineral(h * 0.6 + shift.x * 0.35 + shift.y * 0.2 + time * 0.03);
    // a facet facing the light goes nearly white, one turned away shows its colour deep
    vec3 col = mix(hue * (0.45 + 0.55 * catchLight), vec3(1.0), pow(catchLight, 6.0) * 0.75);

    // the seams between facets
    float edge = min(min(f.x, f.y), min(1.0 - f.x, 1.0 - f.y));
    float diag = abs(f.x - f.y) * 0.7071;
    float seam = smoothstep(0.0, 0.06, min(edge, diag));
    col *= 0.55 + 0.45 * seam;

    // a few facets flash
    float flash = pow(max(0.0, sin(time * 1.7 + h * 60.0)), 40.0) * step(0.86, hash(id + 3.0));
    col = mix(col, vec3(1.0), flash);

    fragColor = vec4(col, 1.0) * inside * qt_Opacity;
}
