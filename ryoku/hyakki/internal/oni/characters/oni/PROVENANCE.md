# Provenance of the shipped Oni character

Every file in this directory was written for Hyakki, in this repository, in
November 2026. License: LICENSE.artwork (GPL-3.0-only).

- `static.svg`: hand-written SVG of plain geometric shapes (a rounded red head,
  two horns, two eyes and a zigzag mouth). It is not traced from, generated
  from or derived from any existing artwork.
- `profile.json`, `persona.md` and `lines.json`: written for Hyakki.
- What is drawn on screen is `static.svg`. The profile's `parametric` kind
  means the program moves that one static artwork by the motion numbers in
  `oni-spec.json` (scale, squash, lean, lift, hover); it does not draw a
  separate figure from those numbers. The strand leaves the artwork at the
  point the spec's `strand.anchor` gives as fractions of its viewBox, and the
  strand itself is drawn as plain strokes, with no image file.

No artwork, text, voice recording or brand element of any other character,
application, product or third party is included. No audio ships with this
character.
