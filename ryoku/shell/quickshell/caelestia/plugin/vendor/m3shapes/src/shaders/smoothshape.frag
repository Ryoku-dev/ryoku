#version 440

// The feather is encoded entirely in the interpolated (premultiplied) vertex
// colour: solid vertices carry full colour, the outer feather vertices carry
// (0,0,0,0), so the rasteriser produces a 1-device-pixel alpha ramp at edges.

layout(location = 0) in vec4 vColor;
layout(location = 0) out vec4 fragColor;

void main()
{
    fragColor = vColor;
}
