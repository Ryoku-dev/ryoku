#version 440
// light on the figure from where the pointer stands: the edges that face it catch a rim
// of the accent, and the side towards it lifts a little, so the figure reads as a solid
layout(location = 0) in vec2 qt_TexCoord0;
layout(location = 0) out vec4 fragColor;
layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;
    float qt_Opacity;
    vec4 tint;
    vec2 texel;
    vec2 lightDir;
    float strength;
};
layout(binding = 1) uniform sampler2D source;

void main() {
    vec2 uv = qt_TexCoord0;
    vec4 c = texture(source, uv);
    vec2 off = lightDir * texel;
    float edge = c.a * (1.0 - 0.6 * texture(source, uv + off * 2.0).a - 0.4 * texture(source, uv + off * 5.0).a);
    float side = clamp(dot(uv - 0.5, lightDir) * 1.6 + 0.5, 0.0, 1.0);
    vec3 col = c.rgb * (1.0 + 0.16 * strength * side) + tint.rgb * c.a * edge * strength * 0.55;
    fragColor = vec4(min(col, vec3(c.a)), c.a) * qt_Opacity;
}
