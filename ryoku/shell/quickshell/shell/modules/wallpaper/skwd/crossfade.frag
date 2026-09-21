#version 440

// skwd transition: crossfade. Ports skwd-wall v2's transition.wgsl default
// branch (the kind-0 path, shaders/transition.wgsl): a plain dissolve between
// the two frames on a smoothstep-eased progress. Math verbatim, only Qt
// plumbing changed. This is the one clean cross-dissolve the GLSL catalog
// lacked: static-fade hard-cuts at the midpoint through a noise wash, so a soft
// blend from old to new had no preset until now.

layout(location = 0) in vec2 qt_TexCoord0;
layout(location = 0) out vec4 fragColor;

layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;
    float qt_Opacity;
    float progress;
    float seed;
    vec2 res;
};

layout(binding = 1) uniform sampler2D oldTex;
layout(binding = 2) uniform sampler2D newTex;

void main() {
    vec2 v_uv = qt_TexCoord0;
    float p = clamp(progress, 0.0, 1.0);
    float t = p * p * (3.0 - 2.0 * p);
    fragColor = mix(texture(oldTex, v_uv), texture(newTex, v_uv), t) * qt_Opacity;
}
