#!/usr/bin/env python3
"""Download one cryptographically pinned Granite GGUF."""

import argparse
import hashlib
import os
import shutil
import time
import urllib.request
from pathlib import Path


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def fetch(source: str, expected_sha256: str, output: Path) -> None:
    output = Path(output)
    if output.exists():
        raise ValueError(f"output already exists: {output}")
    output.parent.mkdir(parents=True, exist_ok=True)
    partial = output.with_name(output.name + ".part")
    try:
        response = None
        for attempt in range(3):
            try:
                response = urllib.request.urlopen(source, timeout=120)
                break
            except OSError:
                if attempt == 2:
                    raise
                time.sleep(attempt + 1)
        with response, partial.open("wb") as handle:
            shutil.copyfileobj(response, handle, 1024 * 1024)
            handle.flush()
            os.fsync(handle.fileno())
        if sha256_file(partial) != expected_sha256.lower():
            raise ValueError("GGUF SHA-256 mismatch")
        os.replace(partial, output)
    finally:
        partial.unlink(missing_ok=True)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", required=True)
    parser.add_argument("--sha256", required=True)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    fetch(args.source, args.sha256, args.out)


if __name__ == "__main__":
    main()
