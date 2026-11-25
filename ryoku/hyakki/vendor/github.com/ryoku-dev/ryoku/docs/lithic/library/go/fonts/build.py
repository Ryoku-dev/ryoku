#!/usr/bin/env python3
"""Rebuilds the embedded faces from the Google Fonts builds at pinned commits.

    python3 build.py    (needs fontTools)

Each family is cut into static instances with one width, because Gio cannot
ask for a width and draws a condensed style when the family holds one. The
source timestamps are kept, so a rebuild is byte-identical.
"""

import hashlib
import io
import pathlib
import urllib.request

from fontTools.ttLib import TTFont
from fontTools.varLib import instancer

GOOGLE_FONTS = "https://raw.githubusercontent.com/google/fonts"
FACES = [
    {
        "family": "Geologica",
        "tree": f"{GOOGLE_FONTS}/a60a77e14f28abd4ef243a1b5dfc48df0cec5205/ofl/geologica",
        "file": "Geologica%5BCRSV%2CSHRP%2Cslnt%2Cwght%5D.ttf",
        "sha256": "9124d9e88ac6c11d761f35241713a51d68e2c4ebedce0edaca834717a00959ec",
        "license": "778186245840aea0e60bec6a46e7fb1442e0cd78e41afeadffcd3e8824b379e0",
        "styles": {"Regular": 400, "Medium": 500, "Bold": 700},
        # full sharpness cuts the curve ends at an angle, like the plaques
        "axes": {"SHRP": 100, "CRSV": 0, "slnt": 0},
    },
    {
        "family": "Martian Mono",
        "tree": f"{GOOGLE_FONTS}/c8bba5c4a69195e4fabc69d75136814c65fe0cf5/ofl/martianmono",
        "file": "MartianMono%5Bwdth%2Cwght%5D.ttf",
        "sha256": "c3467843ec1c2574b05fbcfd7147c7bfbcf63ddca8fc2bcb9d117f1bfb1b22e7",
        "license": "ddafd2c3f37ef1d83ef284ca63e159befd3820850c5e9eded14ca52eee92c256",
        "styles": {"Regular": 400, "Bold": 700},
        "axes": {"wdth": 100},
    },
]
HERE = pathlib.Path(__file__).resolve().parent


def fetch(url, sha256):
    data = urllib.request.urlopen(url, timeout=60).read()
    if hashlib.sha256(data).hexdigest() != sha256:
        raise SystemExit(f"build.py: {url} does not match its pinned hash")
    return data


def rename(face, family, style):
    ribbi = style in ("Regular", "Bold")
    stem = family.replace(" ", "")
    table = face["name"]
    for name_id in (1, 2, 3, 4, 6, 16, 17, 21, 22, 25):
        table.removeNames(nameID=name_id)
    names = {
        1: family if ribbi else f"{family} {style}",
        2: style if ribbi else "Regular",
        3: f"{stem}-{style};lithic",
        4: f"{family} {style}",
        6: f"{stem}-{style}",
    }
    if not ribbi:
        names[16] = family
        names[17] = style
    for name_id, value in names.items():
        table.setName(value, name_id, 3, 1, 0x409)
    selection = face["OS/2"].fsSelection & ~0b1100001
    if style == "Bold":
        selection |= 1 << 5
    elif style == "Regular":
        selection |= 1 << 6
    face["OS/2"].fsSelection = selection
    face["head"].macStyle = 1 if style == "Bold" else 0


def main():
    for spec in FACES:
        stem = spec["family"].replace(" ", "")
        source = fetch(f"{spec['tree']}/{spec['file']}", spec["sha256"])
        (HERE / f"{stem}-OFL.txt").write_bytes(fetch(f"{spec['tree']}/OFL.txt", spec["license"]))
        for style, weight in spec["styles"].items():
            face = instancer.instantiateVariableFont(
                TTFont(io.BytesIO(source), recalcTimestamp=False), {"wght": weight, **spec["axes"]})
            rename(face, spec["family"], style)
            face.save(HERE / f"{stem}-{style}.ttf")


if __name__ == "__main__":
    main()
