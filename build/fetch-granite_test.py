import hashlib
import io
import tempfile
import unittest
from pathlib import Path
from unittest import mock
from urllib.error import URLError

import fetch_granite

MODEL = b"verified granite model"


class Response(io.BytesIO):
    def __enter__(self):
        return self

    def __exit__(self, *args):
        return None


class FetchGraniteTests(unittest.TestCase):
    def test_downloads_verified_model_atomically(self):
        with tempfile.TemporaryDirectory() as directory, mock.patch(
            "fetch_granite.urllib.request.urlopen", return_value=Response(MODEL)
        ):
            output = Path(directory) / "model.gguf"
            fetch_granite.fetch("https://example.invalid/model", hashlib.sha256(MODEL).hexdigest(), output)
            self.assertEqual(output.read_bytes(), MODEL)
            self.assertFalse((Path(directory) / "model.gguf.part").exists())

    def test_rejects_wrong_hash_without_output(self):
        with tempfile.TemporaryDirectory() as directory, mock.patch(
            "fetch_granite.urllib.request.urlopen", return_value=Response(MODEL)
        ):
            output = Path(directory) / "model.gguf"
            with self.assertRaisesRegex(ValueError, "GGUF SHA-256"):
                fetch_granite.fetch("https://example.invalid/model", "0" * 64, output)
            self.assertFalse(output.exists())

    def test_rejects_existing_output(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "model.gguf"
            output.write_bytes(b"existing")
            with self.assertRaisesRegex(ValueError, "already exists"):
                fetch_granite.fetch("https://example.invalid/model", "0" * 64, output)

    def test_retries_transient_download_failures(self):
        with tempfile.TemporaryDirectory() as directory, mock.patch(
            "fetch_granite.urllib.request.urlopen",
            side_effect=[URLError("dns"), URLError("dns"), Response(MODEL)],
        ) as opener, mock.patch("fetch_granite.time.sleep") as sleep:
            output = Path(directory) / "model.gguf"
            fetch_granite.fetch("https://example.invalid/model", hashlib.sha256(MODEL).hexdigest(), output)
            self.assertEqual(opener.call_count, 3)
            self.assertEqual(sleep.call_count, 2)

    def test_removes_partial_file_when_download_fails(self):
        with tempfile.TemporaryDirectory() as directory, mock.patch(
            "fetch_granite.urllib.request.urlopen", side_effect=URLError("dns")
        ), mock.patch("fetch_granite.time.sleep"):
            output = Path(directory) / "model.gguf"
            with self.assertRaises(URLError):
                fetch_granite.fetch("https://example.invalid/model", "0" * 64, output)
            self.assertEqual(list(Path(directory).iterdir()), [])


if __name__ == "__main__":
    unittest.main()
