#version 440
// a light that crosses the whole card now and then, as foil catches a lamp: a soft diagonal
// band at `pos` (it travels from -0.4 to 1.4), with a thin bright core
layout(location = 0) in vec2 qt_TexCoord0;
layout(location = 0) out vec4 fragColor;
layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;
    float qt_Opacity;
    vec4 tint;
    vec2 size;
    float pos;
    float cut;
};

void main() {
    vec2 uv = qt_TexCoord0;
    vec2 p = uv * size;
    vec2 q = min(p, size - p);
    float inside = step(cut, q.x + q.y);
    float d = uv.x * 0.8 + uv.y * 0.45 - pos;
    float band = exp(-d * d / 0.006) * 0.35 + exp(-d * d / 0.0004) * 0.4;
    fragColor = vec4(mix(tint.rgb, vec3(1.0), 0.5), 1.0) * band * inside * qt_Opacity;
}
