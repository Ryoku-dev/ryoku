#version 440
// the light Home's plate stands in: a soft glow set as 1-bit dots around the pointer (or,
// at rest, behind the figure's head), and a few motes of dust drifting up through it. The
// motes only move while `time` advances; the glow is still unless the pointer moves
layout(location = 0) in vec2 qt_TexCoord0;
layout(location = 0) out vec4 fragColor;
layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;
    float qt_Opacity;
    vec4 tint;
    vec2 size;
    vec2 center;
    float radius;
    float strength;
    float time;
    float motes;
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
float hash(float n) {
    return fract(sin(n) * 43758.5453);
}

void main() {
    vec2 p = qt_TexCoord0 * size;
    float d = length(p - center * size) / max(radius, 1.0);
    float glow = exp(-d * d * 2.0) * strength;

    // the glow's body as dots on a 2 px cell, its heart soft under them
    float dots = step(bayer4(floor(p / 2.0)), glow * 1.25) * 0.5;
    float soft = glow * 0.22;

    float m = 0.0;
    for (int i = 0; i < 18; i++) {
        float fi = float(i);
        float sx = hash(fi * 12.9898 + 1.0);
        float sy = hash(fi * 78.233 + 2.0);
        float sp = 0.008 + 0.018 * hash(fi * 3.17 + 3.0);
        vec2 mp = vec2(sx * size.x + sin(time * 0.5 + fi * 1.7) * 14.0,
                       fract(sy - time * sp) * size.y);
        float md = length(p - mp);
        float r = 1.0 + 1.4 * hash(fi * 5.31 + 4.0);
        m += smoothstep(r + 0.8, r - 0.4, md) * (0.4 + 0.6 * hash(fi * 9.1 + 5.0));
    }
    float near = exp(-pow(length(p - center * size) / max(radius * 1.6, 1.0), 2.0));
    m *= motes * (0.15 + 0.85 * near);

    float a = clamp(soft + dots * glow + m * 0.8, 0.0, 1.0);
    fragColor = vec4(tint.rgb, 1.0) * a * tint.a * qt_Opacity;
}
