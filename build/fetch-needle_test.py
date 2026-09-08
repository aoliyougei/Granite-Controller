import hashlib
import io
import tempfile
import unittest
import warnings
import zipfile
from pathlib import Path
from unittest import mock
from urllib.error import URLError

import fetch_needle


LIB = b"verified-native-library"
LICENSE = b"Apache License fixture"


def make_wheel(members):
    stream = io.BytesIO()
    with warnings.catch_warnings():
        warnings.simplefilter("ignore", UserWarning)
        with zipfile.ZipFile(stream, "w") as archive:
            for name, data in members:
                archive.writestr(name, data)
    return stream.getvalue()


class FetchNeedleTests(unittest.TestCase):
    def extract(self, wheel, lib_hash=None):
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "needle.whl"
            output = Path(directory) / "out"
            source.write_bytes(wheel)
            fetch_needle.fetch_and_extract(
                source.as_uri(),
                hashlib.sha256(wheel).hexdigest(),
                lib_hash or hashlib.sha256(LIB).hexdigest(),
                output,
            )
            return {p.relative_to(output).as_posix(): p.read_bytes() for p in output.rglob("*") if p.is_file()}

    def test_extracts_only_verified_library_and_notices(self):
        wheel = make_wheel([
            ("needle/libneedle.so", LIB),
            ("pkg.dist-info/LICENSE", LICENSE),
            ("pkg.dist-info/METADATA", b"ignored"),
        ])
        self.assertEqual(
            self.extract(wheel),
            {"libneedle.so": LIB, "notices/LICENSE": LICENSE},
        )

    def test_rejects_wrong_wheel_hash_before_extraction(self):
        wheel = make_wheel([("needle/libneedle.so", LIB)])
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "needle.whl"
            source.write_bytes(wheel)
            with self.assertRaisesRegex(ValueError, "wheel SHA-256"):
                fetch_needle.fetch_and_extract(source.as_uri(), "0" * 64, hashlib.sha256(LIB).hexdigest(), Path(directory) / "out")
            self.assertFalse((Path(directory) / "out").exists())

    def test_rejects_wrong_library_hash(self):
        wheel = make_wheel([("needle/libneedle.so", LIB)])
        with self.assertRaisesRegex(ValueError, "library SHA-256"):
            self.extract(wheel, "0" * 64)

    def test_rejects_missing_or_duplicate_library_member(self):
        for members in ([], [("needle/libneedle.so", LIB), ("needle/libneedle.so", LIB)]):
            with self.subTest(count=len(members)):
                wheel = make_wheel(members)
                with self.assertRaisesRegex(ValueError, "exactly one needle/libneedle.so"):
                    self.extract(wheel)

    def test_retries_transient_download_failures(self):
        wheel = make_wheel([("needle/libneedle.so", LIB)])
        response = io.BytesIO(wheel)
        response.__enter__ = lambda value: value
        response.__exit__ = lambda *args: None
        with tempfile.TemporaryDirectory() as directory, \
             mock.patch("fetch_needle.urllib.request.urlopen", side_effect=[URLError("dns"), URLError("dns"), response]) as opener, \
             mock.patch("fetch_needle.time.sleep") as sleep:
            output = Path(directory) / "out"
            fetch_needle.fetch_and_extract("https://example.invalid/needle.whl", hashlib.sha256(wheel).hexdigest(), hashlib.sha256(LIB).hexdigest(), output)
            self.assertEqual(opener.call_count, 3)
            self.assertEqual(sleep.call_count, 2)
            self.assertEqual((output / "libneedle.so").read_bytes(), LIB)

    def test_rejects_zip_path_traversal(self):
        wheel = make_wheel([
            ("needle/libneedle.so", LIB),
            ("../LICENSE", LICENSE),
        ])
        with self.assertRaisesRegex(ValueError, "unsafe ZIP member"):
            self.extract(wheel)


if __name__ == "__main__":
    unittest.main()
