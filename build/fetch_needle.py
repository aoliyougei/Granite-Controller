#!/usr/bin/env python3
"""Download and extract one cryptographically pinned Needle native library."""

import argparse
import hashlib
import os
import shutil
import tempfile
import urllib.request
import zipfile
from pathlib import Path, PurePosixPath

LIBRARY_MEMBER = "needle/libneedle.so"
NOTICE_NAMES = {"LICENSE", "NOTICE", "COPYING"}


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def safe_member(name: str) -> bool:
    path = PurePosixPath(name)
    return not path.is_absolute() and ".." not in path.parts and "\\" not in name


def fetch_and_extract(source: str, wheel_sha256: str, lib_sha256: str, output: Path) -> None:
    output = Path(output)
    if output.exists():
        raise ValueError("output directory already exists")

    with tempfile.TemporaryDirectory() as directory:
        wheel = Path(directory) / "needle.whl"
        with urllib.request.urlopen(source, timeout=120) as response, wheel.open("wb") as handle:
            shutil.copyfileobj(response, handle, 1024 * 1024)
            handle.flush()
            os.fsync(handle.fileno())

        if sha256_file(wheel) != wheel_sha256.lower():
            raise ValueError("wheel SHA-256 mismatch")

        with zipfile.ZipFile(wheel) as archive:
            infos = archive.infolist()
            for info in infos:
                if not safe_member(info.filename):
                    raise ValueError(f"unsafe ZIP member: {info.filename}")
            libraries = [info for info in infos if info.filename == LIBRARY_MEMBER]
            if len(libraries) != 1:
                raise ValueError("wheel must contain exactly one needle/libneedle.so")
            library = archive.read(libraries[0])
            if hashlib.sha256(library).hexdigest() != lib_sha256.lower():
                raise ValueError("library SHA-256 mismatch")

            staging = Path(directory) / "output"
            staging.mkdir()
            (staging / "libneedle.so").write_bytes(library)
            notices = staging / "notices"
            for info in infos:
                filename = PurePosixPath(info.filename).name.upper()
                if any(filename == base or filename.startswith(base + ".") for base in NOTICE_NAMES):
                    notices.mkdir(exist_ok=True)
                    target = notices / PurePosixPath(info.filename).name
                    if target.exists():
                        raise ValueError(f"duplicate notice filename: {target.name}")
                    target.write_bytes(archive.read(info))

        os.replace(staging, output)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", required=True)
    parser.add_argument("--wheel-sha256", required=True)
    parser.add_argument("--lib-sha256", required=True)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    fetch_and_extract(args.source, args.wheel_sha256, args.lib_sha256, args.out)


if __name__ == "__main__":
    main()
