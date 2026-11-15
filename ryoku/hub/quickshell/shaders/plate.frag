#version 440
// a plate: a photograph set as a 1-bit ordered dither in ink on a clear ground, as the
// Profile hero and ryoku.dev's sheets are. `develop` (0..1) brings the dots in from the
// top as the plate arrives; `scan` (0..1) is the row a slow scan band crosses, lifting the
// dots it passes and drawing its line in the accent
layout(location = 0) in vec2 qt_TexCoord0;
layout(location = 0) out vec4 fragColor;
layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;
    float qt_Opacity;
    vec4 ink;
    vec4 accent;
    vec2 srcSize;
    float dotScale;
    float develop;
    float scan;
    float band;
    float invert;
};
layout(binding = 1) uniform sampler2D src;

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

void main() {
    vec2 uv = qt_TexCoord0;
    vec4 c = texture(src, uv);
    float lum = dot(c.rgb, vec3(0.299, 0.587, 0.114));
    // a dark subject on a pale ground: the shadows take the dots
    lum = mix(lum, 1.0 - lum, invert);

    // the plate develops from the top down
    float reveal = clamp((develop * 1.35 - uv.y) / 0.35, 0.0, 1.0);
    lum *= reveal;

    // the scan band lifts the dots it crosses
    float d = abs(uv.y - scan);
    float lift = band * (1.0 - smoothstep(0.0, 0.08, d));
    lum = min(1.0, lum + lift * 0.28);

    vec2 px = uv * srcSize / max(dotScale, 1.0);
    float on = step(bayer4(px), lum) * step(0.004, c.a);
    vec3 col = mix(ink.rgb, accent.rgb, lift);

    // the scan's own line, one cell tall
    float cell = max(dotScale, 1.0) / srcSize.y;
    float line = band * step(d, cell * 0.5) * step(0.5, mod(floor(px.x), 2.0));

    vec4 dots = vec4(col, 1.0) * on * ink.a;
    vec4 rule = vec4(accent.rgb, 1.0) * line * 0.9;
    fragColor = max(dots, rule) * qt_Opacity;
}
