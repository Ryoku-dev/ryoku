import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("benchmark", Path(__file__).with_name("run.py"))
benchmark = importlib.util.module_from_spec(spec)
spec.loader.exec_module(benchmark)


class WordErrorTests(unittest.TestCase):
    def test_existing_normalization(self):
        self.assertEqual(benchmark.normalize("Mr. Jones, Mrs. Jones; Ruggedo's 25!"),
                         ["mister", "jones", "missus", "jones", "ruggedo's", "25"])

    def test_insert_delete_and_substitute(self):
        self.assertEqual(benchmark.score("one two three", "one four five three")["word_errors"], 2)
        self.assertEqual(benchmark.score("one two three", "one three")["word_errors"], 1)

    def test_punctuation_does_not_change_score(self):
        self.assertEqual(benchmark.score("Go to full globe view, and then turn on the radio.",
                                         "Go to full globe view and then turn on the radio")["wer"], 0)

    def test_empty_reference_is_rejected(self):
        with self.assertRaises(ValueError):
            benchmark.score("...", "text")


if __name__ == "__main__":
    unittest.main()
