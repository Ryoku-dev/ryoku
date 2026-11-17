#version 440
// the Stand card's face while time is played with: THE WORLD's sphere opens from the pointer,
// everything inside it in negative with a magenta and a cyan skin; and the yellowed freeze
// frame of TO BE CONTINUED. The card code is drawn above this layer, so it never changes
layout(location = 0) in vec2 qt_TexCoord0;
layout(location = 0) out vec4 fragColor;
layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;
    float qt_Opacity;
    vec2 size;
    vec2 centre;
    float radius;
    float sepia;
    float res;
};
layout(binding = 1) uniform sampler2D source;

void main() {
    vec2 p = qt_TexCoord0 * size;
    vec4 c = texture(source, qt_TexCoord0);
    vec3 rgb = c.a > 0.0 ? c.rgb / c.a : vec3(0.0);
    if (radius > 0.0) {
        float d = length(p - centre);
        float aa = 0.8 / max(res, 0.25);
        float inside = smoothstep(radius + aa, radius - aa, d);
        vec3 negative = vec3(1.0) - rgb;
        negative = mix(negative, negative * vec3(0.78, 1.0, 0.92), 0.5);
        rgb = mix(rgb, negative, inside);
        rgb = mix(rgb, vec3(0.93, 0.16, 0.56), smoothstep(7.0, 0.0, abs(d - radius + 4.0)) * 0.85);
        rgb = mix(rgb, vec3(0.12, 0.80, 0.86), smoothstep(5.0, 0.0, abs(d - radius - 6.0)) * 0.8);
    }
    if (sepia > 0.0) {
        float l = dot(rgb, vec3(0.299, 0.587, 0.114));
        vec3 aged = mix(vec3(0.20, 0.13, 0.05), vec3(0.97, 0.89, 0.62), l);
        vec2 v = qt_TexCoord0 - 0.5;
        aged *= 1.0 - 0.35 * (v.x * v.x + v.y * v.y);
        rgb = mix(rgb, aged, sepia);
    }
    fragColor = vec4(rgb * c.a, c.a) * qt_Opacity;
}
