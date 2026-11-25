// Package fonts holds Lithic's two faces, embedded so every application sets the same type on
// every system: Geologica for everything a person reads and Martian Mono for literal data.
// theme.New loads them; the QML and web libraries load the same files from this directory.
//
// They are static instances of the Google Fonts variable builds at the commits build.py pins:
// Geologica at full sharpness in Regular, Medium and Bold, and Martian Mono at its normal width
// in Regular and Bold. Rebuild them with python3 build.py, which needs fontTools.
//
// Both are released under the SIL Open Font License 1.1; Geologica-OFL.txt and
// MartianMono-OFL.txt in this directory carry it.
package fonts
