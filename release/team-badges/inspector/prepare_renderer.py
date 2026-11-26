#!/usr/bin/env python3
"""Build-time only: verify private dependency, add the owned data-only hero seam."""
import json
from pathlib import Path
from policy import sha, require
from worker import renderer_manifest

root=Path('/opt/ryoku-inspector/renderer')
manifest=renderer_manifest(root)
original=sha((root/'manifest.json').read_bytes())
for source,name in (('ProfileStore.qml','qml/Ryoku/Ui/Singletons/ProfileStore.qml'),('Quickshell.qml','qml/Quickshell/Quickshell.qml'),('Ambient.qml','hub/Singletons/Ambient.qml')):
    overlay=(Path('/opt/ryoku-inspector')/source).read_bytes()
    (root/name).write_bytes(overlay)
    manifest['files'][name]=sha(overlay)
manifest['baseManifestSha']=original
manifest['version']+='-inspector-hero2'
(root/'manifest.json').write_text(json.dumps(manifest,sort_keys=True,indent=2)+'\n')
