#version 440
// the Stand card's screentone: a rotated dot screen whose dots grow along a linear or radial
// ramp, in one ink; a manga page's greys are dots, never a grey
layout(location = 0) in vec2 qt_TexCoord0;
layout(location = 0) out vec4 fragColor;
layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;
    float qt_Opacity;
    vec4 ink;
    vec2 size;
    // linear: from xy to zw; radial: centre xy, radius z
    vec4 ramp;
    // dot coverage at the ramp's start and end; below zero is no dot
    vec2 cover;
    float pitch;
    float angle;
    float radial;
    // device pixels per unit, for a one-pixel edge
    float res;
};

void main() {
    vec2 p = qt_TexCoord0 * size;
    float t;
    if (radial > 0.5) {
        t = clamp(length(p - ramp.xy) / ramp.z, 0.0, 1.0);
    } else {
        vec2 d = ramp.zw - ramp.xy;
        t = clamp(dot(p - ramp.xy, d) / max(dot(d, d), 0.0001), 0.0, 1.0);
    }
    float c = mix(cover.x, cover.y, t);
    float s = sin(angle);
    float co = cos(angle);
    vec2 r = vec2(co * p.x - s * p.y, s * p.x + co * p.y) / pitch;
    float dist = length(fract(r) - 0.5) * pitch;
    float rad = sqrt(max(c, 0.0) / 3.14159) * pitch;
    float aa = 0.6 / max(res, 0.25);
    float a = smoothstep(rad + aa, rad - aa, dist) * step(0.004, c);
    fragColor = ink * a * qt_Opacity;
}
