#version 440
layout(location = 0) in vec2 qt_TexCoord0;
layout(location = 0) out vec4 fragColor;
layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;
    float qt_Opacity;
    vec4 aspects;
    vec4 depths;
    vec4 blends;
    vec2 shift;
    float aspect;
    float count;
    float time;
    float glitch;
};
layout(binding = 1) uniform sampler2D source0;
layout(binding = 2) uniform sampler2D source1;
layout(binding = 3) uniform sampler2D source2;
layout(binding = 4) uniform sampler2D source3;
layout(binding = 5) uniform sampler2D lettering;
vec2 coords(float imageAspect, float depth) {
    vec2 cover = imageAspect > aspect ? vec2(aspect / imageAspect, 1.0) : vec2(1.0, imageAspect / aspect);
    // Overscan exceeds the largest tilt offset even for the deepest layer.
    return (qt_TexCoord0 - 0.5) * cover / (1.025 + depth * 0.12) + 0.5 + shift * depth * 0.035;
}
vec4 over(vec4 base, vec4 layer, float mode) {
    vec3 b = base.a > 0.001 ? base.rgb / base.a : vec3(0.0);
    vec3 s = layer.a > 0.001 ? layer.rgb / layer.a : vec3(0.0);
    vec3 mixed = mode < 0.5 ? s : mode < 1.5 ? 1.0 - (1.0 - b) * (1.0 - s) : min(b + s, vec3(1.0));
    return vec4(mix(s, mixed, base.a) * layer.a + base.rgb * (1.0 - layer.a), layer.a + base.a * (1.0 - layer.a));
}
vec4 art(vec2 offset) {
    vec4 a = texture(source0, coords(aspects.x, depths.x) + offset);
    // The word sits in the scene, behind the foreground figure, not across its face.
    a = over(a, texture(lettering, qt_TexCoord0 + offset), 0.0);
    if (count > 1.5) a = over(a, texture(source1, coords(aspects.y, depths.y) + offset), blends.y);
    if (count > 2.5) a = over(a, texture(source2, coords(aspects.z, depths.z) + offset), blends.z);
    if (count > 3.5) a = over(a, texture(source3, coords(aspects.w, depths.w) + offset), blends.w);
    return a;
}
void main() {
    vec4 a = art(vec2(0.0));
    // The split is a brief, narrow fault on hover, never a full-card strobe.
    float event = smoothstep(0.985, 1.0, sin(time * 2.1));
    float band = exp(-pow(qt_TexCoord0.y - fract(time * 0.17), 2.0) / 0.001);
    float split = event * band * glitch * 0.004;
    if (split > 0.00001) {
        a.r = art(vec2(split, 0.0)).r;
        a.b = art(vec2(-split, 0.0)).b;
    }
    fragColor = a * qt_Opacity;
}
