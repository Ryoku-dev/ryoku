#version 440

layout(location = 0) in vec2 qt_TexCoord0;
layout(location = 0) out vec4 fragColor;

layout(binding = 1) uniform sampler2D source;
layout(binding = 2) uniform sampler2D mask;

layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;
    float qt_Opacity;
    float videoRatio;
    float screenRatio;
};

void main() {
    vec4 src = texture(source, qt_TexCoord0);
    vec2 uv = qt_TexCoord0;

    // Calculate scale to fit video inside screen without cropping
    vec2 scale = vec2(1.0);
    if (screenRatio > videoRatio) {
        // Screen is wider than video (pillarbox)
        scale.x = screenRatio / videoRatio;
    } else {
        // Screen is taller than video (letterbox)
        scale.y = videoRatio / screenRatio;
    }

    // Center the video
    uv = (uv - 0.5) * scale + 0.5;

    vec4 m;
    if (uv.x < 0.0 || uv.x > 1.0 || uv.y < 0.0 || uv.y > 1.0) {
        // Out of bounds (padding area). Sample top-left corner of video to guess background color
        m = texture(mask, vec2(0.01, 0.01));
    } else {
        m = texture(mask, uv);
    }

    float maskAlpha = 1.0 - m.r;
    fragColor = src * maskAlpha * qt_Opacity;
}
