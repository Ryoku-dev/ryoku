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
    def test_failed_update_preserves_binary_and_retry_works(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            target = root / 'bin/prowl-agent'
            target.parent.mkdir()
            target.write_bytes(b'old working executable')
            with patch.object(extra, 'download', side_effect=ValueError('truncated')):
                with self.assertRaises(ValueError):
                    extra.install('prowl-agent', root)
            self.assertEqual(target.read_bytes(), b'old working executable')
            self.assertFalse((root / 'state/ryoku/extras/prowl-agent.json').exists())
            with patch.object(extra, 'download', return_value=b'\x7fELFnew executable'):
                extra.install('prowl-agent', root)
            self.assertEqual(target.read_bytes(), b'\x7fELFnew executable')
            with patch.object(extra, 'download', side_effect=AssertionError('should use receipt')):
                extra.install('prowl-agent', root)

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

    def test_exact_font_url(self):
        url = extra.RELEASES['material-symbols'][1]
        self.assertIn('Rounded%5BFILL%2CGRAD%2Copsz%2Cwght%5D.ttf', url)
        self.assertNotIn('%%', url)

    def test_jetbrains_mono_nerd_fonts_release(self):
        self.assertIn('jetbrains-mono-nerd-fonts', extra.RELEASES)
        version, url, digest, relative = extra.RELEASES['jetbrains-mono-nerd-fonts']
        self.assertEqual(version, '3.3.0')
        self.assertTrue(url.endswith('.tar.xz'))
        self.assertTrue(relative.startswith('share/fonts/'))

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
