#version 440

// skwd transition: circle-crop. Ports skwd-daemon's CIRCLE_CROP_FRAG
// (crates/paper/src/transition_paper.rs). Qt plumbing, the progress-dependent
// `s` moved out of global scope (non-const global init is illegal in 440), and
// the black bgcolor canvas replaced by an old->new crossfade so the wallpaper
// never blanks to black through the middle of the switch (#238).

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

const float ratio = 1.7777;

vec2 ratio2 = vec2(1.0, 1.0 / ratio);

vec4 transition(vec2 p) {
  float s = pow(2.0 * abs(progress - 0.5), 3.0);
  float dist = length((vec2(p) - 0.5) * ratio2);
  // The uncovered ring must never be a black void: a wallpaper switch that
  // flashes the desktop to bgcolor at the midpoint (s -> 0 at progress 0.5,
  // so step(s, dist) covers the whole surface) reads as a glitch. Cross the
  // two images across that ring instead, so the shrinking then growing crop
  // rides over a clean old->new dissolve and the wallpaper stays painted.
  vec4 canvas = mix(texture(oldTex, p), texture(newTex, p), progress);
  return mix(
    progress < 0.5 ? texture(oldTex, p) : texture(newTex, p), // static branch on the u_progress uniform (uniform across pixels)
    canvas,
    step(s, dist)
  );
}
void main() {
    vec2 v_uv = qt_TexCoord0;
    fragColor = transition(v_uv) * qt_Opacity;
}
