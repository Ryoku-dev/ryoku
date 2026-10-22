#version 440

// Smooth (analytically antialiased) shape vertex shader.
//
// Each vertex carries an outward feather DIRECTION in item-local coordinates
// (aAaDir; the zero vector marks an interior/solid vertex). Instead of baking a
// fixed local-pixel offset into the mesh on the CPU, the offset is applied here
// AFTER the combined matrix, expanded to a constant number of DEVICE pixels.
//
// Because the renderer hands us the up-to-date combined matrix every frame, the
// feather is exactly aaWidth device pixels wide at ANY on-screen scale -- the
// item's own scale, any accumulated parent scale, and the window device pixel
// ratio are all already folded into qt_Matrix and the viewport. No CPU-side
// re-tessellation or scale tracking is required.

layout(location = 0) in vec2 aPos;
layout(location = 1) in vec4 aColor;   // premultiplied; normalized from ubyte4
layout(location = 2) in vec2 aAaDir;   // local-space outward dir, (0,0)=interior

layout(location = 0) out vec4 vColor;

layout(std140, binding = 0) uniform buf {
    mat4 qt_Matrix;     // combined local -> clip (NDC) matrix
    vec2 pixelToNdc;    // (2/vpW, 2/vpH): NDC units per device pixel
    float aaWidth;      // feather width in device pixels
    float qt_Opacity;   // inherited scene-graph opacity
} ubuf;

out gl_PerVertex { vec4 gl_Position; };

void main()
{
    vec4 clip = ubuf.qt_Matrix * vec4(aPos, 0.0, 1.0);

    if (aAaDir.x != 0.0 || aAaDir.y != 0.0) {
        // Clip-space image of the (direction) vector. w-row kept for generality
        // (perspective); for the affine 2D transforms here it is simply zero.
        vec4 clipDir = ubuf.qt_Matrix * vec4(aAaDir, 0.0, 0.0);

        // First-order rate of change of NDC position as we travel one unit along
        // aAaDir, then convert NDC -> device pixels.
        vec2 dNdc = (clipDir.xy * clip.w - clip.xy * clipDir.w) / (clip.w * clip.w);
        vec2 dScreen = dNdc / ubuf.pixelToNdc;

        float len = length(dScreen);
        if (len > 1e-6)
            clip += clipDir * (ubuf.aaWidth / len); // move exactly aaWidth px
    }

    gl_Position = clip;
    vColor = aColor * ubuf.qt_Opacity;
}
