#version 440
layout(location = 0) in vec2 qt_TexCoord0;
layout(location = 0) out vec4 fragColor;
layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;
    float qt_Opacity;
    vec4 tint;
    vec2 size;
    vec2 shift;
    float time;
    float mode;
};
layout(binding = 1) uniform sampler2D emission;
float hash(float n) { return fract(sin(n * 127.1) * 43758.5453); }
float hex(vec2 p) {
    p = abs(p);
    return max(dot(p, vec2(0.8660254, 0.5)), p.y);
}
void main() {
    vec2 uv = qt_TexCoord0;
    float light = 0.0;
    vec3 color = tint.rgb;
    if (mode < 0.5) {
        vec2 p = (uv * size + shift * 6.0) / 28.0;
        vec2 lattice = vec2(1.7320508, 1.0);
        vec2 a = mod(p, lattice) - lattice * 0.5;
        vec2 b = mod(p - lattice * 0.5, lattice) - lattice * 0.5;
        vec2 cell = dot(a, a) < dot(b, b) ? a : b;
        float line = 1.0 - smoothstep(0.009, 0.035, abs(hex(cell) - 0.47));
        float breath = 0.65 + 0.35 * sin(uv.x * 4.0 - uv.y * 3.0 + time * 0.8);
        light = line * breath * 0.16;
    } else if (mode < 1.5) {
        vec4 mask = texture(emission, uv);
        float vein = dot(mask.rgb, vec3(0.299, 0.587, 0.114));
        float travel = uv.x * 0.65 + uv.y * 0.45 - fract(time * 0.16);
        float wave = exp(-travel * travel / 0.008);
        light = vein * (0.14 + wave * 0.8);
        color = mix(tint.rgb, vec3(1.0), wave * 0.45);
    } else if (mode < 2.5) {
        float line = exp(-pow(uv.y - fract(time * 0.12 + 0.2), 2.0) / 0.00015);
        float trailing = exp(-pow(uv.y - fract(time * 0.12 + 0.17), 2.0) / 0.002);
        light = line * 0.22 + trailing * 0.04;
    } else if (mode < 3.5) {
        // Fixed seeds keep a saved card reproducible; the live field rises without a reset flash.
        for (int i = 0; i < 22; ++i) {
            float seed = float(i) + 1.0;
            float age = fract(hash(seed * 3.0) + time * (0.025 + hash(seed) * 0.035));
            vec2 point = vec2(hash(seed * 9.0) + sin(time * 0.5 + seed) * 0.015, 1.05 - age * 1.15);
            vec2 d = (uv - point) * size;
            float spark = exp(-dot(d, d) / (1.0 + hash(seed * 2.0) * 3.0));
            vec2 tail = d / vec2(0.7, 5.0);
            float streak = exp(-dot(tail, tail) / 3.0) * 0.2;
            light += (spark + streak) * sin(age * 3.14159) * 0.8;
        }
        color = mix(tint.rgb, vec3(1.0, 0.95, 0.8), 0.3);
    } else {
        float x = uv.x + shift.x * 0.03;
        float ribbon = 0.48 + sin(x * 4.3 + time * 0.18) * 0.16 + sin(x * 9.0 - time * 0.23) * 0.055;
        float curtain = exp(-pow(uv.y - ribbon, 2.0) / 0.016);
        float folds = 0.45 + 0.55 * pow(0.5 + 0.5 * sin(x * 61.0 + time * 0.4), 2.0);
        light = curtain * folds * 0.27;
        color = mix(tint.rgb, tint.brg, smoothstep(0.1, 0.9, uv.x) * 0.65);
    }
    float alpha = clamp(light, 0.0, 0.8);
    fragColor = vec4(color * alpha, alpha) * qt_Opacity;
}
