#!/usr/bin/env python3
import hashlib
import importlib.machinery
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

helper = Path(__file__).resolve().parents[2] / 'fedora/system/extras/ryoku-install-extra'
extra = importlib.machinery.SourceFileLoader('extra', str(helper)).load_module()


class Extras(unittest.TestCase):
    def test_failed_update_preserves_file_and_retry_works(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            target = root / 'share/fonts/MaterialSymbolsRounded.ttf'
            target.parent.mkdir(parents=True)
            target.write_bytes(b'\x00\x01\x00\x00old font')
            with patch.object(extra, 'download', side_effect=ValueError('truncated')):
                with self.assertRaises(ValueError):
                    extra.install('material-symbols', root)
            self.assertEqual(target.read_bytes(), b'\x00\x01\x00\x00old font')
            receipt = root / 'state/ryoku/extras/material-symbols.json'
            self.assertFalse(receipt.exists())
            updated = b'\x00\x01\x00\x00new font'
            with patch.object(extra, 'download', return_value=updated):
                extra.install('material-symbols', root)
            self.assertEqual(target.read_bytes(), updated)
            with patch.object(extra, 'download', side_effect=AssertionError('should use receipt')):
                extra.install('material-symbols', root)

    def test_bad_content_does_not_become_skip_marker(self):
        with tempfile.TemporaryDirectory() as directory:
            with patch.object(extra, 'download', return_value=b'<html>failure</html>'):
                with self.assertRaises(ValueError):
                    extra.install('material-symbols', Path(directory))
            self.assertFalse((Path(directory) / 'share/fonts/MaterialSymbolsRounded.ttf').exists())

    def test_cursor_aliases_remain_links(self):
        import io
        import tarfile
        blob = io.BytesIO()
        with tarfile.open(fileobj=blob, mode='w:xz') as archive:
            regular = tarfile.TarInfo('Bibata-Modern-Ice/cursors/left_ptr')
            regular.size = 6
            archive.addfile(regular, io.BytesIO(b'cursor'))
            alias = tarfile.TarInfo('Bibata-Modern-Ice/cursors/arrow')
            alias.type = tarfile.SYMTYPE
            alias.linkname = 'left_ptr'
            archive.addfile(alias)
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            with patch.object(extra, 'download', return_value=blob.getvalue()):
                extra.install('bibata', root)
            alias_path = root / 'share/icons/Bibata-Modern-Ice/cursors/arrow'
            self.assertTrue(alias_path.is_symlink())
            self.assertEqual(alias_path.read_bytes(), b'cursor')


    def test_jetbrains_mono_nerd_fonts_extraction(self):
        import io, tarfile
        blob = io.BytesIO()
        with tarfile.open(fileobj=blob, mode='w:xz') as archive:
            font_data = b'\x00\x01\x00\x00mock font data'
            info = tarfile.TarInfo('JetBrainsMonoNerdFont-Regular.ttf')
            info.size = len(font_data)
            archive.addfile(info, io.BytesIO(font_data))
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            with patch.object(extra, 'download', return_value=blob.getvalue()):
                extra.install('jetbrains-mono-nerd-fonts', root)
            target = root / 'share/fonts/JetBrainsMonoNerdFont/JetBrainsMonoNerdFont-Regular.ttf'
            self.assertTrue(target.is_file())
            self.assertEqual(target.read_bytes(), font_data)

    def test_space_mono_nerd_fonts_extraction(self):
        import io, tarfile
        blob = io.BytesIO()
        with tarfile.open(fileobj=blob, mode='w:xz') as archive:
            font_data = b'\x00\x01\x00\x00mock font data'
            info = tarfile.TarInfo('SpaceMonoNerdFont-Regular.ttf')
            info.size = len(font_data)
            archive.addfile(info, io.BytesIO(font_data))
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            with patch.object(extra, 'download', return_value=blob.getvalue()):
                extra.install('space-mono-nerd-fonts', root)
            target = root / 'share/fonts/SpaceMonoNerdFont/SpaceMonoNerdFont-Regular.ttf'
            self.assertEqual(target.read_bytes(), font_data)

    def test_truncated_download_fails_checksum(self):
        import io
        with patch.object(extra.urllib.request, 'urlopen', return_value=io.BytesIO(b'partial')):
            with self.assertRaises(ValueError):
                extra.download('https://example.invalid', hashlib.sha256(b'complete').hexdigest())


if __name__ == '__main__':
    unittest.main()
