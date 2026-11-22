#!/usr/bin/env python3
"""Generate flat, two-tone Hellenic achievement insignias for Hyperion."""

from __future__ import annotations

import argparse
import hashlib
import html
import math
import re
from pathlib import Path
from xml.etree import ElementTree as ET

ROOT = Path(__file__).resolve().parents[1]
REGISTRY = ROOT / "quickshell/registry/progression.js"
OUT = ROOT / "quickshell/assets/badges"

# Main and secondary inks, with an inset background ink. No gradients.
INKS = {
    "common":      ("#848b99", "#b5b9c3", "#141d2c"),
    "uncommon":    ("#849a65", "#b8c993", "#18231d"),
    "rare":        ("#4487ec", "#8ab3ff", "#101e39"),
    "epic":        ("#9860db", "#c79aef", "#241336"),
    "legendary":   ("#edb84f", "#f7d484", "#34220f"),
    "mythic":      ("#dd5c59", "#f39991", "#37171f"),
    "holographic": ("#79cfed", "#d3a9e8", "#172640"),
}

MOTIFS = {
    "sandal": '<path d="M-35 12Q-20 28 2 24L31 16L34 25Q8 37-17 35L-35 27Z" fill="P"/>'
              '<path d="M-17 10L-4-25L6-17L1 8L21-20L31-12L14 11Z" fill="P"/>'
              '<path d="M-30 21H25" stroke="N" stroke-width="4"/>',
    "temple": '<path d="M-39-25L0-43L39-25V-19H-39Z" fill="P"/>'
              '<path d="M-36-13H36V-7H-36Z M-36 27H36V34H-36Z" fill="P"/>'
              '<path d="M-29-7H-17V27H-29Z M-6-7H6V27H-6Z M17-7H29V27H17Z" fill="P"/>'
              '<path d="M0-35L-13-27H13Z" fill="N"/>',
    "owl": '<path d="M-37-36L-16-26Q0-33 16-26L37-36L31 14Q21 37 0 41Q-21 37-31 14Z" fill="P"/>'
           '<path d="M-28-8Q-18-22-1-8Q-1 10-18 12Q-32 6-28-8Z M1-8Q18-22 28-8Q32 6 18 12Q1 10 1-8Z" fill="N"/>'
           '<circle cx="-15" cy="-3" r="5" fill="P"/><circle cx="15" cy="-3" r="5" fill="P"/>'
           '<path d="M-8 16L0 30L8 16Z" fill="N"/>',
    "torch": '<path d="M-7 0H7V34H-7Z M-18 32H18V39H-18Z M-14-3H14V5H-14Z" fill="P"/>'
             '<path d="M2-45Q-28-20-15-4Q-11 5 1 4Q28 0 20-18Q10-28 14-36Q2-30 2-45Z" fill="P"/>'
             '<path d="M-4-21Q-12-8-1-6Q8-5 7-16Q0-14-4-21Z" fill="N"/>',
    "helmet": '<path d="M-34-9Q-32-41 1-41Q34-39 36-9L24-5L21 26L9 35L0 13L-10 37L-27 23L-17-6Z" fill="P"/>'
              '<path d="M-19-6L18-12L15 1L-14 6Z M-4 12L10 11L0 27Z" fill="N"/>',
    "lyre": '<path d="M-29-34L-35-10Q-31 10-15 14L-18 30H-25V35H25V30H18L15 14Q31 10 35-10L29-34L19-25Q31 1 10 6H-10Q-31 1-19-25Z" fill="P"/>'
            '<path d="M-13-23V13 M0-25V13 M13-23V13" stroke="P" stroke-width="4"/>'
            '<path d="M-24 18H24" stroke="P" stroke-width="5"/>',
    "amphora": '<path d="M-15-39H15V-32H-15Z M-20-29Q-28-10-22 13L-13 34H13L22 13Q28-10 20-29Z" fill="P"/>'
               '<path d="M-20-23Q-44-25-36-6Q-31 7-23 9 M20-23Q44-25 36-6Q31 7 23 9" fill="none" stroke="P" stroke-width="6"/>'
               '<path d="M-12-17H12V-10H-12Z M-12 16H12V23H-12Z M0-7L10 2L0 11L-10 2Z" fill="N"/>',
    "trident": '<path d="M-37-24V-10Q-37 13-8 16V39H8V16Q37 13 37-10V-24H29V-11Q29 1 8 4V-26H17L0-46L-17-26H-8V4Q-29 1-29-11V-24Z" fill="P"/>',
    "sun": '<circle r="19" fill="P"/>' +
           ''.join(f'<path d="M-5-28L0-45L5-28Z" fill="P" transform="rotate({k*45})"/>' for k in range(8)),
    "olive": '<path d="M-35 34Q-14 1 34-32" stroke="P" stroke-width="6" fill="none"/>' +
             ''.join(f'<ellipse cx="{x}" cy="{y}" rx="15" ry="6.5" fill="P" transform="rotate({angle} {x} {y})"/>'
                     for x,y,angle in [(-25,18,-44),(-15,3,28),(0,-7,-40),(12,-18,27),(23,-29,-39),(-7,21,31)]),
    "labyrinth": '<path d="M-37 0V-22L-20-39H20L37-22V22L19 39H-19L-37 22V0'
                  ' M-24 5V-17L-13-26H13L25-15V15L12 26H-12V12L0-1L11 9V-8H-11V9" '
                  'fill="none" stroke="P" stroke-width="8" stroke-linejoin="miter"/>',
    "scroll": '<path d="M-31-37H27V28H-23Q-37 28-37 15Q-37 5-27 5H-22V-29H-31Z" fill="P"/>'
              '<path d="M-18-25H15 M-18-14H15 M-18-3H9 M-17 9H13" stroke="N" stroke-width="5"/>'
              '<path d="M-27 16H14" stroke="N" stroke-width="4"/>',
    "sail": '<path d="M-5-42V20L30 20Z M-11-32L-34 14H-11Z" fill="P"/>'
            '<path d="M-38 22H37L23 37H-23Z" fill="P"/><path d="M-17 29H20" stroke="N" stroke-width="4"/>',
    "hourglass": '<path d="M-28-40H28V-33Q28-12 9 0Q28 12 28 33V40H-28V33Q-28 12-9 0Q-28-12-28-33Z" fill="P"/>'
                 '<path d="M-18-30H18Q16-13 0-8Q-16-13-18-30Z M-19 31L0 8L19 31Z" fill="N"/>',
    "moon": '<path d="M20-41Q-31-28-30 10Q-25 42 11 42Q30 42 39 23Q9 23 1 5Q-6-18 20-41Z" fill="P"/>',
    "bolt": '<path d="M9-43L-30 7H-5L-17 43L31-13H5Z" fill="P"/>',
    "column": '<path d="M-33-37H33V-28H-33Z M-39-27H39V-20H-39Z M-23-17H23V24H-23Z M-37 27H37V36H-37Z" fill="P"/>'
              '<path d="M-13-17V23 M0-17V23 M13-17V23" stroke="N" stroke-width="4"/>',
    "mask": '<path d="M-34-29Q0-44 34-29L31 19Q9 43 0 43Q-9 43-31 19Z" fill="P"/>'
            '<path d="M-23-10L-5-6L-13 5Z M23-10L5-6L13 5Z" fill="N"/>'
            '<path d="M-18 17Q0 30 18 17" fill="none" stroke="N" stroke-width="6"/>',
    "compass": '<circle r="31" fill="none" stroke="P" stroke-width="7"/>'
               '<path d="M0-44L9-9L43 0L9 9L0 44L-9 9L-43 0L-9-9Z" fill="P"/>'
               '<circle r="6" fill="N"/>',
    "eye": '<path d="M-43 0Q0-38 43 0Q0 38-43 0Z" fill="P"/>'
           '<circle r="16" fill="N"/><circle r="7" fill="P"/>',
    "shield": '<path d="M0-43Q16-28 35-26L33 8Q29 31 0 44Q-29 31-33 8L-35-26Q-16-28 0-43Z" fill="P"/>'
              '<path d="M0-30Q12-20 22-19L20 8Q18 21 0 31Q-18 21-20 8L-22-19Q-12-20 0-30Z" fill="N"/>'
              '<path d="M-13 0L-2 13L17-12" stroke="P" stroke-width="8" fill="none"/>',
    "wing": '<path d="M-42-8Q-9-24 21-42L15-24L37-26L20-8L41-6L19 10L28 18L-10 22L-33 38L-26 14Z" fill="P"/>'
            '<path d="M-30 15L15-12 M-22 23L26 1" stroke="N" stroke-width="5"/>',
    "flame": '<path d="M5-44Q-14-25-9-14Q-30-19-34 9Q-35 39 0 42Q35 40 34 10Q29-12 19-19Q19-3 7 0Q17-26 5-44Z" fill="P"/>'
             '<path d="M0-10Q-16 7-11 23Q0 35 13 22Q16 9 0-10Z" fill="N"/>',
    "star": '<path d="M0-44L11-13L43-13L17 7L26 39L0 21L-26 39L-17 7L-43-13L-11-13Z" fill="P"/>',
    "mirror": '<path d="M-29-34H29V25H-29Z M-15 26H15V33H-15Z M-26 33H26V39H-26Z" fill="P"/>'
              '<path d="M-21-24H21V17H-21Z" fill="N"/>'
              '<path d="M-13 7L9-15 M1 10L14-3" stroke="P" stroke-width="5"/>',
    "feather": '<path d="M-35 39Q-28-6 28-40Q47 3-3 29Z" fill="P"/>'
               '<path d="M-35 39L29-26 M-16 17L-15-13 M0 3L27 3" stroke="N" stroke-width="4"/>',
    "mountain": '<path d="M-43 32L-11-26L4-4L21-38L44 32Z" fill="P"/>'
                '<path d="M-18-12L-11-26L-4-13Z M14-23L21-38L29-21Z" fill="N"/>',
    "heart": '<path d="M0 39L-34 5Q-49-18-28-34Q-10-43 0-20Q16-44 32-32Q53-13 34 10Z" fill="P"/>',
    "key": '<circle cx="-17" cy="-15" r="19" fill="none" stroke="P" stroke-width="12"/>'
           '<path d="M-4-2L32 35H42V23H31V14H22V5L4-12Z" fill="P"/>',
    "coin": '<circle r="38" fill="P"/><circle r="28" fill="N"/>'
            '<path d="M0-24L13-8L8 20H-8L-13-8Z" fill="P"/>',
    "rose": '<path d="M0-43L10-17L33-31L25-5L43 0L25 8L31 33L6 23L0 43L-9 19L-33 31L-25 5L-43 0L-25-9L-31-33L-5-23Z" fill="P"/>'
            '<circle r="10" fill="N"/>',
    "hare": '<path d="M-9-16L-16-45L-6-47L6-17L8-43L20-43L19-12Q34-3 29 19Q19 37-6 35Q-31 34-35 14Q-36-7-9-16Z" fill="P"/>'
            '<circle cx="-4" cy="4" r="4" fill="N"/><path d="M21 19L37 29L24 33Z" fill="P"/>',
    "tablet": '<path d="M-35-39H31L36-31V40H-35Z" fill="P"/>'
              '<path d="M-25-24H22V-16H-25Z M-25-7H14V1H-25Z M-25 10H22V18H-25Z" fill="N"/>',
}

MOTIF_BY_ID = {
    "ryoku-first-steps": "sandal", "ryoku-pathfinder": "compass",
    "ryoku-voyager": "sail", "ryoku-versatile": "rose",
    "ryoku-tenfold": "olive", "ryoku-cartographer": "labyrinth",
    "ryoku-omniscope": "eye", "ryoku-steadfast": "sun",
    "ryoku-two-weeks": "hourglass", "ryoku-thirty": "sun",
    "ryoku-mapmaker": "compass",
    "profile-curator": "owl", "profile-sticker": "feather",
    "profile-background": "mountain", "profile-type": "scroll",
    "profile-mosaic": "rose", "profile-gallery": "temple",
    "profile-sanctuary": "temple", "profile-self": "mirror",
    "profile-identity": "tablet", "profile-custom": "shield",
    "profile-variety": "mask", "profile-curation": "column",
    "profile-week": "olive", "profile-wardrobe": "amphora",
    "welcome": "torch", "explorer": "temple", "identity": "feather",
    "portrait": "mask", "layout": "column", "showcase": "lyre",
    "card-spectrum": "rose", "card-complete": "shield",
    "card-tapestry": "scroll", "card-collector": "olive",
    "card-exhibit": "owl",
    "collection-firstclaim": "laurel", "collection-fifth": "star",
    "collection-tenth": "coin", "collection-twentieth": "amphora",
    "collection-firstbuy": "amphora", "collection-trio": "lyre",
    "collection-stickers": "rose", "collection-backgrounds": "mountain",
    "collection-fonts": "tablet", "collection-everything": "sun",
    "collection-thirty": "scroll", "collection-forty": "coin",
    "collection-tenth-buy": "key", "collection-twentieth-buy": "amphora",
    "collection-patron": "olive",
    "master-archive": "temple", "master-ryoku": "wing",
    "master-profile": "owl", "master-card": "shield",
    "master-collection": "laurel", "master-hyperion": "sun",
    "secret-signal": "eye", "secret-sevenfold": "bolt",
    "secret-monolith": "column", "secret-totality": "labyrinth",
    "secret-keepsake": "heart", "secret-empty": "tablet",
    "secret-twelve": "rose", "secret-atelier": "lyre",
    "secret-moon": "moon", "secret-scribe": "feather",
    "secret-timekeeper": "hourglass", "secret-many-faces": "mask",
    "secret-nightshift": "owl", "secret-rabbit": "hare",
    "secret-absolute": "star",
}

# Construct larger emblem silhouettes from a handful of Greek heraldic families.
def star_points(n, ro, ri, cx=160, cy=153, offset=-90):
    pts = []
    for i in range(n * 2):
        t = math.radians(offset + i * 180 / n)
        r = ro if i % 2 == 0 else ri
        pts.append(f"{cx+math.cos(t)*r:.1f},{cy+math.sin(t)*r:.1f}")
    return " ".join(pts)


def silhouette(kind):
    if kind == 0:  # hoplite shield
        return ('<path d="M160 43L235 72V169Q236 225 160 271Q84 225 85 169V72Z" fill="P"/>'
                '<path d="M160 62L217 84V164Q217 207 160 244Q103 207 103 164V84Z" fill="N"/>')
    if kind == 1:  # ceremonial hexagon
        return ('<path d="M160 40L235 82V220L160 274L85 220V82Z" fill="P"/>'
                '<path d="M160 60L216 94V207L160 251L104 207V94Z" fill="N"/>')
    if kind == 2:  # round seal
        return ('<circle cx="160" cy="155" r="112" fill="P"/>'
                '<circle cx="160" cy="155" r="96" fill="N"/>')
    if kind == 3:  # octagonal bronze-age tablet
        return (f'<polygon points="{star_points(8,112,107)}" fill="P"/>'
                f'<polygon points="{star_points(8,94,90)}" fill="N"/>')
    if kind == 4:  # shield with Greek temple pediment
        return ('<path d="M160 40L239 87V204L160 274L81 204V87Z" fill="P"/>'
                '<path d="M160 64L219 99V196L160 251L101 196V99Z" fill="N"/>'
                '<path d="M99 95L160 52L221 95" stroke="P" stroke-width="8" fill="none"/>')
    return ('<path d="M92 65H228V217L160 272L92 217Z" fill="P"/>'
            '<path d="M107 83H213V206L160 250L107 206Z" fill="N"/>')


def wings(tier, second):
    p = []
    rows = 2 + (tier >= 3) + (tier >= 5)
    for side in (-1, 1):
        for j in range(rows):
            x = 160 + side * 63
            tip = 160 + side * (132 + j * 7)
            y = 121 + j * 19
            p.append(f'<path d="M{x} {y+8}L{tip} {y-13}L{tip-side*17} {y+10}L{x} {y+22}Z" fill="P"/>')
            if second and j < rows - 1:
                p.append(f'<path d="M{x+side*10} {y+10}L{tip-side*19} {y-2}" fill="none" stroke="S" stroke-width="3"/>')
    return "".join(p)


def laurels(tier, with_stems=True):
    parts = []
    count = 3 if tier < 2 else 5 if tier < 5 else 7
    for side in (-1, 1):
        if with_stems:
            parts.append(f'<path d="M{160+side*65} 238Q{160+side*120} 189 {160+side*88} 103" '
                         'stroke="P" stroke-width="5.5" fill="none"/>')
        for j in range(count):
            x = 160 + side * (77 + 9 * math.sin(math.pi * j / count))
            y = 216 - j * (119 / max(1,count-1))
            rotation = (-47 + j * 13) * side
            parts.append(f'<ellipse cx="{x:.1f}" cy="{y:.1f}" rx="{12 if tier>=4 else 10}" ry="5.8" '
                         f'fill="P" transform="rotate({rotation} {x:.1f} {y:.1f})"/>')
    return "".join(parts)


def ornaments(kind, tier, variant):
    parts = []
    if tier >= 2:
        if kind in (0, 1, 4):
            parts.append('<path d="M109 93L160 64L211 93" fill="none" stroke="S" stroke-width="3.5"/>')
        elif kind == 2:
            parts.append('<circle cx="160" cy="155" r="85" fill="none" stroke="S" stroke-width="3"/>')
    if tier >= 4:
        count = 5 if tier < 6 else 7
        for j in range(count):
            x = 160 + (j-(count-1)/2) * 18
            parts.append(f'<path d="M{x-5:.1f} 51L{x:.1f} {34-(j%2)*5}L{x+5:.1f} 51Z" fill="P"/>')
    if kind in (0, 4, 5):
        parts.append('<path d="M121 247H199" stroke="P" stroke-width="5"/>')
    if variant % 3 == 1:
        parts.append('<path d="M123 69L160 51L197 69" stroke="P" stroke-width="4" fill="none"/>')
    if variant % 3 == 2:
        parts.append('<circle cx="160" cy="59" r="5.5" fill="S"/>')
    return "".join(parts)


def meander(tier):
    if tier < 3:
        return ""
    parts = []
    for k in range(3 if tier < 5 else 4):
        x = 127 + 20*k if tier < 5 else 117 + 19*k
        parts.append(f'<path d="M{x} 252v-8h9v7h-5" fill="none" stroke="S" stroke-width="2.7"/>')
    return "".join(parts)


def badge_svg(badge):
    name, rarity, category = badge["id"], badge["rarity"], badge["category"]
    main, secondary, negative = INKS[rarity]
    digest = hashlib.sha256(name.encode("utf8")).digest()
    variant = digest[0] % 6
    tier = list(INKS).index(rarity)
    groups = {
        "Ryoku": [1, 4, 0, 3],
        "Profile": [0, 2, 5, 4],
        "Card studio": [4, 1, 5, 0],
        "Collection": [2, 3, 1, 5],
        "Mastery": [4, 0, 3, 2],
        "Classified": [3, 1, 2, 0],
    }
    shape = groups[category][digest[1] % 4]
    motif = MOTIF_BY_ID[name]
    if motif == "laurel":
        art = '<path d="M0 34Q-37 12-34-23" fill="none" stroke="P" stroke-width="6"/>' + ''.join(
            f'<ellipse cx="{-28+i*7}" cy="{18-i*10}" rx="12" ry="6" fill="P" '
            f'transform="rotate({-22+i*12} {-28+i*7} {18-i*10})"/>'
            for i in range(5)) + '<g transform="scale(-1 1)">' + ''.join(
            f'<ellipse cx="{-28+i*7}" cy="{18-i*10}" rx="12" ry="6" fill="P" '
            f'transform="rotate({-22+i*12} {-28+i*7} {18-i*10})"/>'
            for i in range(5)) + '</g>'
    else:
        art = MOTIFS[motif]

    # The negative field is genuinely opaque only within the insignia.
    winged = category in ("Ryoku", "Card studio") or (tier >= 4 and variant % 2 == 0)
    wreath = category in ("Profile", "Collection", "Mastery") or (tier >= 5 and variant % 2 == 1)
    rays = ""
    if tier >= 4 and variant % 3 != 0:
        rays = '<g>' + ''.join(
            f'<path d="M155 48L160 {17 if tier >= 6 else 27}L165 48Z" '
            f'fill="P" transform="rotate({i*360/(12 if tier>=6 else 9):.1f} 160 155)"/>'
            for i in range(12 if tier >= 6 else 9)) + '</g>'

    extra = ""
    if tier >= 5 and category in ("Mastery", "Classified"):
        halo = variant % 4
        if halo == 0:
            extra = f'<polygon points="{star_points(12, 138, 124)}" fill="P"/>'
            extra += f'<polygon points="{star_points(12, 125, 112)}" fill="N"/>'
        elif halo == 1:
            extra = f'<polygon points="{star_points(8, 137, 117)}" fill="P"/>'
            extra += f'<polygon points="{star_points(8, 123, 108)}" fill="N"/>'
        elif halo == 2:
            extra = '<circle cx="160" cy="155" r="127" fill="none" stroke="P" stroke-width="12" stroke-dasharray="31 13"/>'
        else:
            extra = f'<polygon points="{star_points(16, 137, 120)}" fill="P"/>'
            extra += f'<polygon points="{star_points(16, 123, 109)}" fill="N"/>'

    # Some glyphs take a contrasting secondary ink, without gradients or gloss.
    accent_icon = rarity == "holographic" or (tier >= 4 and variant % 3 == 1)
    mark_ink = secondary if accent_icon else main
    icon = (art.replace('fill="P"', f'fill="{mark_ink}"')
               .replace('stroke="P"', f'stroke="{mark_ink}"')
               .replace('fill="N"', f'fill="{negative}"')
               .replace('stroke="N"', f'stroke="{negative}"'))
    ring = '<circle cx="160" cy="155" r="66" fill="N" stroke="P" stroke-width="7"/>'
    if shape == 3 or (tier >= 4 and variant % 2):
        ring = '<circle cx="160" cy="155" r="67" fill="N" stroke="P" stroke-width="5"/>' \
               '<circle cx="160" cy="155" r="59" fill="none" stroke="S" stroke-width="3"/>'
    banner = ''
    if variant % 3 == 0:
        banner = ('<path d="M113 244Q160 255 207 244L215 260Q160 278 105 260Z" fill="P"/>'
                  '<path d="M122 254H198" stroke="N" stroke-width="3"/>')
    if variant % 3 == 1 and tier >= 3:
        banner = '<path d="M127 250L160 267L193 250" fill="none" stroke="S" stroke-width="5"/>'

    body = silhouette(shape) + ornaments(shape, tier, variant) + meander(tier)
    svg = f'''<svg xmlns="http://www.w3.org/2000/svg" width="320" height="320" viewBox="0 0 320 320">
<title>{html.escape(badge["title"])}</title>
<desc>Flat Hellenic achievement insignia. {rarity.title()} rarity. {html.escape(category)}.</desc>
<g stroke-linejoin="round" stroke-linecap="round">
{rays}
{wings(tier, tier>=4) if winged else ""}
{extra}
{body}
{laurels(tier) if wreath else ""}
{ring}
<g transform="translate(160 155) scale({1.09 if motif in ("sun","rose","compass","owl") else 1.16})">{icon}</g>
{banner}
</g>
</svg>'''
    # No filters, gradients, embedded images, or scripts.
    svg = (svg.replace('fill="P"', f'fill="{main}"')
              .replace('stroke="P"', f'stroke="{main}"')
              .replace('fill="S"', f'fill="{secondary}"')
              .replace('stroke="S"', f'stroke="{secondary}"')
              .replace('fill="N"', f'fill="{negative}"')
              .replace('stroke="N"', f'stroke="{negative}"'))
    ET.fromstring(svg)
    return svg


def entries():
    source = REGISTRY.read_text()
    fragment = source.split("var achievements = [", 1)[1].split("\n];", 1)[0]
    result = []
    for line in fragment.splitlines():
        if not line.lstrip().startswith("{ id:"):
            continue
        item = {}
        for key in ("id", "title", "category", "rarity"):
            match = re.search(r'\b' + key + r':\s*"([^"]+)"', line)
            if not match:
                raise ValueError(f"Missing {key}: {line}")
            item[key] = match.group(1)
        result.append(item)
    return result


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    badges = entries()
    actual = {p.stem for p in OUT.glob("*.svg")}
    ids = {b["id"] for b in badges}
    if len(badges) != 72 or ids != actual or ids != set(MOTIF_BY_ID):
        raise RuntimeError("Achievement registry and art identifiers do not match")
    hashes = set()
    for badge in badges:
        art = badge_svg(badge)
        assert "<linearGradient" not in art and "<filter" not in art
        assert art not in hashes, badge["id"]
        hashes.add(art)
        target = OUT / (badge["id"] + ".svg")
        if args.check:
            if target.read_text() != art + "\n":
                raise RuntimeError(f"Out-of-date badge asset: {target}")
        else:
            target.write_text(art + "\n")
    print(f"Validated {len(badges)} flat Greek insignias")


if __name__ == "__main__":
    main()
