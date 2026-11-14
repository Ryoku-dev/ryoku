#!/usr/bin/env python3
"""Offscreen test for the Voice typing store. Needs qs; set RYOKU_TEST_HUB_BIN.

The page draws what the store reports, and the store is the only thing that
talks to `ryoku-hub dictation`. This loads the store against a fake backend and
checks the two things a user would notice if they broke: the page's data is what
the backend said, and a change goes out as the key the backend expects.
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
STORE = (ROOT / 'ryoku/hub/quickshell/components/voice-typing/DictationStore.qml')

SNAPSHOT = {
    'enabled': True, 'source': 'alsa_input.usb-mic', 'model': '/usr/share/ryoku/dictation/models/x.gguf',
    'device': 'auto', 'language': 'de', 'mode': 'paste', 'maxDurationSecs': 120, 'silenceMs': 1500,
    'silenceDb': -45, 'typeDelayMs': 2, 'engine': True, 'packageAvailable': True, 'packageReason': '',
    'gpu': 'vulkan', 'backends': 'cpu,vulkan', 'activeModel': '', 'activeBackend': 'CPU',
    'running': True, 'state': 'idle',
    'sources': [{'name': 'alsa_input.usb-mic', 'description': 'USB Microphone', 'default': True}],
    'models': [
        {'id': 'parakeet-110m', 'label': 'Parakeet 110M', 'detail': 'small English model', 'language': 'English',
         'wer': '2.43%', 'size': '135 MB', 'sizeBytes': 135373280, 'licence': 'CC-BY-4.0',
         'removable': False, 'installed': True, 'path': '/usr/share/ryoku/dictation/models/x.gguf'},
        {'id': 'parakeet-v2', 'label': 'Parakeet 0.6B', 'detail': 'accurate', 'language': 'English',
         'wer': '1.69%', 'size': '730 MB', 'sizeBytes': 729574912, 'licence': 'CC-BY-4.0',
         'removable': False, 'installed': False, 'path': ''},
        {'id': 'parakeet-ultra', 'label': 'Parakeet Ultra', 'detail': '25 languages',
         'language': '25 European', 'wer': '1.80%', 'size': '740 MB', 'sizeBytes': 740363168,
         'licence': 'CC-BY-4.0', 'removable': True, 'installed': True,
         'path': '/home/user/.local/share/ryoku/dictation/models/parakeet-ultra-Q8_0.gguf'},
    ],
}

HARNESS = '''import QtQuick
import Quickshell
import "./Singletons"

ShellRoot {
  id: root
  property var store
  property int ticks: 0
  property int stage: 0
  Timer {
    interval: 250; running: true; repeat: true
    onTriggered: {
      root.ticks++;
      if (root.ticks > 60) { console.log("FAIL timeout"); Qt.quit(); return; }
      if (!root.store) {
        var c = Qt.createComponent(__STORE__);
        if (c.status === Component.Error) { console.log("FAIL " + c.errorString()); Qt.quit(); return; }
        root.store = c.createObject(root);
        return;
      }
      if (root.stage === 0 && root.store.loaded) {
        // (registration is proved on the live hub: a probe that creates the
        // store by absolute URL gets its own copy of the Singletons module, so
        // comparing Changes.stores here would test the harness, not the hub)
        if (root.store.activeId() !== "parakeet-110m") { console.log("FAIL active " + root.store.activeId()); Qt.quit(); return; }
        if (root.store.sourceNameAt(1) !== "alsa_input.usb-mic") { console.log("FAIL source name"); Qt.quit(); return; }
        root.store.write("mode", "type", function (ok, why) {
          console.log(ok ? "PASS write" : "FAIL write " + why);
          root.stage = 1;
        });
      }
      if (root.stage === 1 && root.store.mode === "type") {
        root.store.write("enabled", false, function (ok, why) {
          if (!ok) { console.log("FAIL disable " + why); Qt.quit(); return; }
          root.stage = 2;
        });
      }
      if (root.stage === 2) {
        if (root.store.read("enabled") !== false) { console.log("FAIL disable did not persist"); Qt.quit(); return; }
        root.store.write("enabled", true, function (ok, why) {
          if (!ok) { console.log("FAIL enable " + why); Qt.quit(); return; }
          root.stage = 3;
        });
      }
      if (root.stage === 3) {
        if (root.store.read("enabled") !== true) { console.log("FAIL enable did not persist"); Qt.quit(); return; }
        root.store.apply(JSON.stringify({enabled: true, engine: true, model: "/old/model.gguf", models: [
          {id: "parakeet-110m", installed: true, removable: true, path: "/downloaded/model.gguf"}
        ]}));
        if (root.store.activeId() !== "") {
          console.log("FAIL a different model path was treated as selected"); Qt.quit(); return;
        }
        root.store.apply(JSON.stringify({enabled: true, engine: true, model: "", models: [
          {id: "parakeet-110m", installed: false, removable: false, path: ""}
        ]}));
        if (root.store.activeId() !== "" || root.store.usableWith("parakeet-110m")) {
          console.log("FAIL missing model was treated as ready"); Qt.quit(); return;
        }
        console.log("PASS dictation store");
        Qt.quit();
      }
    }
  }
}
'''


class DictationStore(unittest.TestCase):
    def setUp(self):
        if not (BINARY and shutil.which('qs')):
            self.skipTest('needs qs and RYOKU_TEST_HUB_BIN')

    def test_store_preserves_typed_settings_across_reload(self):
        with tempfile.TemporaryDirectory(prefix='ryoku-dictation-store-') as directory:
            root = Path(directory)
            bin_dir = root / 'bin'
            bin_dir.mkdir()
            calls = root / 'calls.txt'

            state = root / 'state.json'
            state.write_text(json.dumps(SNAPSHOT))

            # The real backend writes the settings file and reads it back, so a
            # fake that answers `get` with a frozen snapshot would hide a store
            # that reverts its own optimistic change. This one keeps the state.
            backend = bin_dir / 'ryoku-hub'
            backend.write_text(
                '#!' + sys.executable + '\nimport sys,os,json\n'
                'state = ' + repr(str(state)) + '\n'
                'snap = json.load(open(state))\n'
                'if sys.argv[2] == "get":\n'
                '    print(json.dumps(snap))\n'
                'elif sys.argv[2] == "set":\n'
                '    open(' + repr(str(calls)) + ',"a").write(sys.argv[3]+chr(10))\n'
                '    snap.update(json.loads(sys.argv[4] if len(sys.argv) > 4 else sys.argv[3]))\n'
                '    json.dump(snap, open(state, "w"))\n'
                '    print(json.dumps(snap))\n'
                'else:\n'
                '    os.execv(' + repr(BINARY) + ',[' + repr(BINARY) + ']+sys.argv[1:])\n')
            backend.chmod(0o755)

            # the harness lives in the hub tree for the duration of the run: it
            # has to import the same Singletons directory the store registers
            # itself with, and relative imports cannot cross out of a temp dir
            qml = ROOT / 'ryoku' / 'hub' / 'quickshell' / '.dictation-store-probe.qml'
            qml.write_text(HARNESS.replace('__STORE__', json.dumps(STORE.as_uri())))
            self.addCleanup(qml.unlink)

            env = dict(os.environ, QT_QPA_PLATFORM='offscreen',
                       PATH=str(bin_dir) + ':' + os.environ['PATH'],
                       HOME=str(root), XDG_CONFIG_HOME=str(root / 'config'),
                       XDG_DATA_HOME=str(root / 'data'), XDG_CACHE_HOME=str(root / 'cache'))
            for key in ['__QUICKSHELL_CRASH_INFO_FD', '__QUICKSHELL_CRASH_DUMP_FD']:
                env.pop(key, None)

            result = subprocess.run(['qs', '-p', str(qml)], env=env, capture_output=True,
                                    text=True, timeout=90)
            logs = result.stdout + result.stderr
            self.assertIn('PASS dictation store', logs, logs[-3000:])
            self.assertNotIn('FAIL ', logs, logs[-3000:])



if __name__ == '__main__':
    unittest.main()
