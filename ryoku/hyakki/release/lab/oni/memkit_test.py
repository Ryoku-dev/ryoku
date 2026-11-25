# SPDX-License-Identifier: GPL-3.0-only
import unittest

import memkit


class PrimaryCopyBoundary(unittest.TestCase):
    def child(self):
        return {"daemon": 100, "comm": "cat", "argv": ["cat"], "exe": "/usr/bin/cat",
                "parent": {"pid": 101, "comm": "wl-paste", "exe": "/usr/bin/wl-paste",
                           "argv": ["wl-paste", "--primary", "--no-newline", "--type",
                                    "text/plain;charset=utf-8"]}}

    def allowed(self, child):
        return memkit.primary_copy_child(child, "/usr/bin/cat", "/usr/bin/wl-paste")

    def test_primary_data_copier_requires_its_exact_observed_parent(self):
        self.assertTrue(self.allowed(self.child()))
        mutations = ({"daemon": None}, {"comm": "sh"}, {"exe": None}, {"exe": "/usr/bin/wtype"},
                     {"argv": ["cat", "/etc/passwd"]}, {"argv": ["cat", "--help"]}, {"parent": None})
        for change in mutations:
            with self.subTest(change=change):
                self.assertFalse(self.allowed(dict(self.child(), **change)))

    def test_other_parent_images_and_clipboard_commands_remain_forbidden(self):
        mutations = ({"pid": None}, {"pid": 100}, {"comm": "sh"}, {"exe": None},
                     {"exe": "/usr/bin/wtype"}, {"argv": ["wl-paste"]},
                     {"argv": ["wl-paste", "--watch", "cat"]},
                     {"argv": ["wl-paste", "--no-newline", "--type", "text/plain;charset=utf-8"]})
        for change in mutations:
            with self.subTest(change=change):
                child = self.child()
                child["parent"].update(change)
                self.assertFalse(self.allowed(child))
        self.assertFalse(memkit.primary_copy_child(self.child(), None, "/usr/bin/wl-paste"))
        self.assertFalse(memkit.primary_copy_child(self.child(), "/usr/bin/cat", None))


if __name__ == "__main__":
    unittest.main()
