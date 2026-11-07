//@ pragma UseQApplication
// Threaded render loop: the frame's blob melt is a per-frame spring plus live
// scene-graph MultiEffects (menu glass blur, blobs); threaded is vsync-locked and
// frees the GUI thread so those effects get regular frame deltas and never ghost
// or stutter. `basic` idled a touch cheaper on NVIDIA but smeared the live effects
// when switching menu pages, so this matches the pill's proven render setup.
//@ pragma DefaultEnv QSG_RENDER_LOOP=threaded
//@ pragma DefaultEnv QS_DROP_EXPENSIVE_FONTS=1
//@ pragma DefaultEnv QT_QUICK_FLICKABLE_WHEEL_DECELERATION=10000

import Quickshell
import shell as Shell

// Quickshell reads the pragmas above from the root file only, so they stay here
// (UseQApplication once for the whole shell: the tray needs Qt Widgets). The
// root itself loads through Quickshell's qs: scheme, and Qt keeps no disk cache
// for a non-file URL, nor for anything the root reaches by relative imports.
// The body comes in through the `shell` module instead: it resolves on the
// import path to a file URL, so the body and every file it imports get a
// qmlcache entry, while a broken body still fails the root load as before.
ShellRoot {
    Shell.ShellBody {}
}
