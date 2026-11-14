#!/usr/bin/env python3
"""Offscreen test for Settings > Voice typing. Needs qs; set RYOKU_TEST_HUB_BIN.

The page owns its backend (`ryoku-hub dictation`), so this loads the real page
against a fake one and checks the two things a user would notice if they broke:
the page renders what the backend reports, and changing a control sends the
right key to the backend instead of something else.
"""
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
BINARY = os.environ.get('RYOKU_TEST_HUB_BIN')
PAGE = (ROOT / 'ryoku/hub/quickshell/pages/VoiceTyping.qml').as_uri()

SNAPSHOT = {
    'enabled': True, 'source': '', 'model': '', 'device': 'auto', 'language': '',
    'mode': 'paste', 'maxDurationSecs': 120, 'silenceMs': 1500, 'silenceDb': -45,
    'typeDelayMs': 2, 'engine': True, 'gpu': 'vulkan', 'backends': 'cpu,vulkan',
    'activeModel': '', 'running': False, 'state': 'stopped',
    'sources': [{'name': 'alsa_input.usb-mic', 'description': 'USB Microphone', 'default': True}],
    'models': [
        {'id': 'parakeet-110m', 'label': 'Parakeet 110M', 'detail': 'small English model', 'language': 'English',
         'wer': '2.43%', 'size': '135 MB', 'sizeBytes': 135373280, 'licence': 'CC-BY-4.0',
         'removable': False, 'installed': False, 'path': ''},
        {'id': 'parakeet-v2', 'label': 'Parakeet 0.6B', 'detail': 'accurate', 'language': 'English',
         'wer': '1.69%', 'size': '730 MB', 'sizeBytes': 729574912, 'licence': 'CC-BY-4.0',
         'removable': False, 'installed': False, 'path': ''},
        {'id': 'parakeet-ultra', 'label': 'Parakeet Ultra', 'detail': '25 languages',
         'language': '25 European', 'wer': '1.80%', 'size': '740 MB', 'sizeBytes': 740363168,
         'licence': 'CC-BY-4.0', 'removable': False, 'installed': False, 'path': ''},
        {'id': 'whisper-turbo', 'label': 'Whisper Large v3 Turbo', 'detail': '100 languages',
         'language': 'about 100', 'wer': '2.01%', 'size': '886 MB', 'sizeBytes': 886381728,
         'licence': 'MIT', 'removable': False, 'installed': False, 'path': ''},
    ],
}

HARNESS = '''import QtQuick
import Quickshell
ShellRoot {
  id: root
  property var page
  property int ticks: 0
  property int stage: 0
  Timer {
    interval: 200; running: true; repeat: true
    onTriggered: {
      root.ticks++;
      if (root.ticks > 60) { console.log("FAIL timeout"); Qt.quit(); return; }
      if (!root.page) {
        var c = Qt.createComponent(__PAGE__);
        if (c.status === Component.Error) { console.log("FAIL " + c.errorString()); Qt.quit(); return; }
        // The page is written against the Hub shell: it asks the hub for
        // availability, the layout class and where overlays mount. Give it the
        // smallest thing that answers, so this exercises the page and its
        // backend rather than the navigation chrome.
        var hub = {
          wide: true, sizeClass: "expanded", noticeInset: 0, overlayItem: null,
          gate: function () { return true; },
          rowAvailable: function () { return true; },
          pageAvailable: function () { return true; },
          providerRowsFor: function () { return []; },
          unhonoredReason: function () { return ""; },
          navigate: function () {},
          confirm: function () {}
        };
        root.page = c.createObject(root, {width: 1100, height: 900, hub: hub});
        return;
      }
      // A page that never registers its backend stays unbound: the switch would
      // show a default instead of the truth, and no control would write.
      if (root.stage === 0 && root.page.loaded) {
        var store = root.page.store;
        if (root.page.usable || store.activeId() !== "") { console.log("FAIL missing model was treated as ready"); Qt.quit(); return; }
        root.stage = 1;
        store.write("mode", "type", function () {});
        return;
      }
      if (root.stage === 1 && root.page.store && root.page.store.mode === "type") {
        console.log("PASS voice typing page");
        Qt.quit();
      }
    }
  }
}
'''


class VoiceTypingPage(unittest.TestCase):
    def setUp(self):
        if not (BINARY and shutil.which('qs')):
            self.skipTest('needs qs and RYOKU_TEST_HUB_BIN')

    def test_page_renders_the_backend_and_writes_back(self):
        with tempfile.TemporaryDirectory(prefix='ryoku-dictation-page-') as directory:
            root = Path(directory)
            bin_dir = root / 'bin'
            bin_dir.mkdir()
            calls = root / 'calls.txt'
            # The real backend writes the settings file and reads it back, so a
            # fake that answers `get` with a frozen snapshot would hide a store
            # that reverts its own optimistic change. This one keeps the state.
            state = root / 'state.json'
            state.write_text(json.dumps(SNAPSHOT))
            backend = bin_dir / 'ryoku-hub'
            backend.write_text(
                '#!' + sys.executable + '\nimport sys,os,json\n'
                'snap = json.load(open(' + repr(str(state)) + '))\n'
                'if sys.argv[2] == "get":\n'
                '    print(json.dumps(snap))\n'
                'elif sys.argv[2] == "set":\n'
                '    open(' + repr(str(calls)) + ',"a").write(sys.argv[3]+chr(10))\n'
                '    snap.update(json.loads(sys.argv[3]))\n'
                '    json.dump(snap, open(' + repr(str(state)) + ',"w"))\n'
                '    print(json.dumps(snap))\n'
                'else: os.execv(' + repr(BINARY) + ',[' + repr(BINARY) + ']+sys.argv[1:])\n')
            backend.chmod(0o755)

            qml = root / 'harness.qml'
            qml.write_text(HARNESS.replace('__PAGE__', json.dumps(PAGE)))

            env = dict(os.environ, QT_QPA_PLATFORM='offscreen',
                       PATH=str(bin_dir) + ':' + os.environ['PATH'],
                       HOME=str(root), XDG_CONFIG_HOME=str(root / 'config'),
                       XDG_DATA_HOME=str(root / 'data'), XDG_CACHE_HOME=str(root / 'cache'))
            for key in ['__QUICKSHELL_CRASH_INFO_FD', '__QUICKSHELL_CRASH_DUMP_FD']:
                env.pop(key, None)

            result = subprocess.run(['qs', '-p', str(qml)], env=env, capture_output=True,
                                    text=True, timeout=60)
            output = result.stdout + result.stderr
            self.assertIn('PASS voice typing page', output, output[-3000:])
            self.assertNotIn('FAIL ', output, output[-3000:])

            written = calls.read_text() if calls.exists() else ''
            self.assertIn('"mode":"type"', written.replace(' ', ''), written)


if __name__ == '__main__':
    unittest.main()
