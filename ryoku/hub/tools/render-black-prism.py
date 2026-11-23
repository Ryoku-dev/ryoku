#!/usr/bin/env python3
"""Render Ryoku's original seamless Black Prism animation with ImageMagick."""
from __future__ import annotations

import math
import random
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1] / "quickshell"
DEST = ROOT / "assets" / "rewards"
FRAMES = 28
WIDTH = 1024
HEIGHT = 640

def wave(x: float, index: int, phase: float) -> float:
    return (
        35 * math.sin(x * 0.006 + index * 0.51 + phase)
        + 28 * math.sin(x * 0.015 - index * 1.1 - phase)
        + 11 * math.sin(x * 0.039 + index * 0.2 + phase * 2)
    )

def render_svg(index: int) -> str:
    phase = math.tau * index / FRAMES
    random.seed(914)
    clouds = []
    strands = []
    sparks = []
    for layer in range(17):
        x = random.randrange(-100, WIDTH + 100)
        y = random.randrange(-100, HEIGHT + 100)
        w = random.randrange(150, 480)
        h = random.randrange(80, 260)
        turn = random.randrange(-75, 75)
        drift_x = math.sin(phase + layer * 0.38) * 32
        drift_y = math.cos(phase + layer * 0.21) * 24
        tint = ["#724ba0", "#262b95", "#a149a9", "#2b6e8b", "#642b83", "#b64d86"][layer % 6]
        clouds.append(
            f'<ellipse cx="{x+drift_x:.1f}" cy="{y+drift_y:.1f}" rx="{w}" ry="{h}" '
            f'transform="rotate({turn} {x+drift_x:.1f} {y+drift_y:.1f})" '
            f'fill="{tint}" opacity="{0.13+(layer%4)*0.04:.2f}" filter="url(#blur)"/>'
        )
    for strand in range(34):
        y0 = 22 + strand * 21 + math.sin(strand * 0.4) * 35
        points = []
        for column in range(49):
            x = -80 + column * 26
            y = y0 + wave(x, strand, phase) + 14 * math.sin(phase + strand)
            points.append(f"{x:.1f},{y:.1f}")
        color = ["#ad76d8", "#66a9c5", "#c361b0", "#5a4aaf", "#e3a2ba"][strand % 5]
        path = "M" + " L".join(points)
        thick = 8 + (strand % 5) * 5
        strands.append(
            f'<path d="{path}" fill="none" stroke="{color}" stroke-width="{thick}" '
            f'stroke-opacity="{0.09+(strand%4)*0.035:.3f}" filter="url(#blur)"/>'
            f'<path d="{path}" fill="none" stroke="{color}" '
            f'stroke-width="{0.8+(strand%4)*0.55:.2f}" stroke-opacity="{0.27+(strand%3)*0.12:.2f}"/>'
        )
    for particle in range(75):
        x = random.randrange(0, WIDTH)
        y = random.randrange(0, HEIGHT)
        size = random.choice((0.5, 0.7, 1.1, 1.4, 2.0))
        fade = 0.1 + (1+math.sin(phase + particle * 0.63)) * 0.21
        sparks.append(
            f'<circle cx="{x}" cy="{y}" r="{size}" fill="#dbc6ef" opacity="{fade:.2f}"/>'
        )
    return f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {WIDTH} {HEIGHT}" width="{WIDTH}" height="{HEIGHT}">
<defs>
<radialGradient id="nebula" cx=".6" cy=".45" r=".8">
<stop stop-color="#251344"/><stop offset=".42" stop-color="#10132a"/><stop offset=".8" stop-color="#080b18"/><stop offset="1" stop-color="#050710"/>
</radialGradient>
<radialGradient id="vignette"><stop stop-color="#080615" stop-opacity="0"/><stop offset=".78" stop-color="#02030d" stop-opacity=".32"/><stop offset="1" stop-color="#02030d" stop-opacity=".85"/></radialGradient>
<filter id="blur" x="-.5" y="-.5" width="2" height="2"><feGaussianBlur stdDeviation="22"/></filter>
</defs>
<rect width="{WIDTH}" height="{HEIGHT}" fill="url(#nebula)"/>
{''.join(clouds)}
<g>{''.join(strands)}</g>
{''.join(sparks)}
<rect width="{WIDTH}" height="{HEIGHT}" fill="url(#vignette)"/>
</svg>"""

def main() -> None:
    DEST.mkdir(parents=True, exist_ok=True)
    work = Path("/tmp/ryoku-black-prism-frames")
    work.mkdir(parents=True, exist_ok=True)
    for index in range(FRAMES):
        source = work / f"{index:03d}.svg"
        image = work / f"{index:03d}.png"
        source.write_text(render_svg(index))
        subprocess.run(["magick", "-background", "none", str(source), str(image)],
                       check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    frames = [str(work / f"{index:03d}.png") for index in range(FRAMES)]
    subprocess.run(
        ["magick", "-delay", "9", *frames, "-loop", "0", "-layers", "Optimize",
         str(DEST / "black-prism.gif")],
        check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL
    )
    print("Rendered original Black Prism GIF:", DEST / "black-prism.gif",
          f"({(DEST / 'black-prism.gif').stat().st_size / 1048576:.1f} MiB)")

if __name__ == "__main__":
    main()
