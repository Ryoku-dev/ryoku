#version 440

layout(location = 0) in vec2 qt_TexCoord0;
layout(location = 0) out vec4 fragColor;

layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;
    float qt_Opacity;
    float progress;
    float intensity;
    float radius;
    float itemWidth;
    float itemHeight;
    int side;
};

void main() {
    float distanceFromEdge = side == 0 ? 1.0 - qt_TexCoord0.x : qt_TexCoord0.x;
    float falloff = 1.0 - smoothstep(0.0, 1.0, distanceFromEdge);
    float edgePixels = distanceFromEdge * itemWidth;
    float strength = clamp(progress * intensity, 0.0, 1.5);

    float corner = 1.0;
    if (radius > 0.0) {
        float nearestEnd = min(qt_TexCoord0.y * itemHeight,
                               (1.0 - qt_TexCoord0.y) * itemHeight);
        corner = smoothstep(0.0, min(radius, itemHeight * 0.5), nearestEnd);
    }

    float veil = 0.08 * pow(falloff, 0.65);
    float castShadow = 0.48 * pow(falloff, 2.2);
    float shadowAlpha = clamp((veil + castShadow) * strength * mix(0.35, 1.0, corner), 0.0, 0.86);

    float hairline = 1.0 - smoothstep(0.35, 1.35, edgePixels);
    float highlightAlpha = clamp(0.18 * strength * hairline * corner, 0.0, 0.30);

    float alpha = highlightAlpha + shadowAlpha * (1.0 - highlightAlpha);
    fragColor = vec4(vec3(highlightAlpha * qt_Opacity), alpha * qt_Opacity);
}
