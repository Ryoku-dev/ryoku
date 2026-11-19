#version 440

// The 64-bucket piece map as one quad: a grid of cells, each lit by how much of
// its bucket has landed. The fills travel as four matrices (16 buckets each),
// twice: where the map was and where it is going, crossed by `blend`.

layout(location = 0) in vec2 qt_TexCoord0;
layout(location = 0) out vec4 fragColor;

layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;
    float qt_Opacity;
    mat4 to0;
    mat4 to1;
    mat4 to2;
    mat4 to3;
    mat4 from0;
    mat4 from1;
    mat4 from2;
    mat4 from3;
    vec4 ink;
    vec4 track;
    vec2 extent;
    vec2 grid;
    vec2 cell;
    vec2 gap;
    float blend;
    float radius;
    float sweep;
};

vec4 pick(float i) {
    return vec4(1.0) - step(vec4(0.5), abs(vec4(0.0, 1.0, 2.0, 3.0) - vec4(i)));
}

float element(mat4 m, float k) {
    float row = floor(k / 4.0);
    return dot(m * pick(k - row * 4.0), pick(row));
}

float fill(mat4 m0, mat4 m1, mat4 m2, mat4 m3, float index) {
    float block = floor(index / 16.0);
    float k = index - block * 16.0;
    vec4 w = pick(block);
    return w.x * element(m0, k) + w.y * element(m1, k) + w.z * element(m2, k) + w.w * element(m3, k);
}

void main() {
    vec2 px = qt_TexCoord0 * extent;
    vec2 pitch = cell + gap;
    vec2 at = floor(px / pitch);
    vec2 local = px - at * pitch;

    if (at.x >= grid.x || at.y >= grid.y || local.x > cell.x || local.y > cell.y) {
        fragColor = vec4(0.0);
        return;
    }

    vec2 half_ = cell * 0.5;
    vec2 q = abs(local - half_) - (half_ - vec2(radius));
    float d = length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - radius;
    float cover = radius > 0.0 ? clamp(0.5 - d, 0.0, 1.0) : 1.0;

    float index = at.y * grid.x + at.x;
    float level = mix(fill(from0, from1, from2, from3, index), fill(to0, to1, to2, to3, index), blend);
    level = clamp(level, 0.0, 1.0);

    float lit = sweep > 0.5 ? clamp(level * cell.x - local.x + 0.5, 0.0, 1.0) : level;
    fragColor = mix(track, ink, lit) * cover * qt_Opacity;
}
