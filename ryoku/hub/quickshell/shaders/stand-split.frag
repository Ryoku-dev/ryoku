#version 440
// the Stand card's picture pulled apart into red and cyan while the card is pointed at
layout(location = 0) in vec2 qt_TexCoord0;
layout(location = 0) out vec4 fragColor;
layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;
    float qt_Opacity;
    vec2 split;
};
layout(binding = 1) uniform sampler2D source;

void main() {
    vec4 r = texture(source, qt_TexCoord0 + split);
    vec4 g = texture(source, qt_TexCoord0);
    vec4 b = texture(source, qt_TexCoord0 - split);
    fragColor = vec4(r.r, g.g, b.b, max(g.a, max(r.a, b.a))) * qt_Opacity;
}
