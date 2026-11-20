#version 440
layout(location = 0) in vec2 qt_TexCoord0;
layout(location = 0) out vec4 fragColor;

layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;
    float qt_Opacity;
    vec2 split;
    float strength;
};

layout(binding = 1) uniform sampler2D source;

void main() {
    vec2 p = qt_TexCoord0;
    vec4 a = texture(source, p);
    vec4 red = texture(source, p + split);
    vec4 blue = texture(source, p - split);
    vec3 rgb = vec3(red.r, a.g, blue.b);
    vec3 vermilion = vec3(0.96, 0.20, 0.06);
    rgb = mix(a.rgb, rgb, strength);
    rgb = mix(rgb, rgb * vermilion, strength * 0.12);
    fragColor = vec4(rgb, max(a.a, max(red.a, blue.a))) * qt_Opacity;
}
