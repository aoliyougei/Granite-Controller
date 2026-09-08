# Third-Party Notices

## Cactus Compute Needle 2

`needle-controller` v0.2.0 is designed to link at runtime to the Needle 2 native engine distributed by Cactus Compute.

- Upstream source: https://github.com/cactus-compute/needle
- Model/artifact repository: https://huggingface.co/Cactus-Compute/needle2
- Verified repository revision: `32e9e3a93b205f786929697446ae669cf0a84579`
- Artifact: `python/cactus_needle-2.0.3-py3-none-manylinux2014_x86_64.whl`
- Package metadata name/version: `cactus-needle` / `2.0.3`
- Wheel SHA-256: `d23df1d0babeb7323dcaf860dfaf833bbd7d2229b205f691c05c9cbc6d3d3653`
- Extracted `needle/libneedle.so` SHA-256: `0d2e125f36269067407ca4460f2d01b9371887366e5949243de9f03d0d93bc78`
- Extracted library size: 14,314,080 bytes
- Declared Hugging Face model-card license: Apache-2.0
- Upstream source `pyproject.toml` license: Apache-2.0
- Upstream source license: Apache License 2.0; reproduced at `/licenses/needle/Apache-2.0.txt` in the image.

The verified wheel contains exactly four members:

- `needle/libneedle.so`
- `cactus_needle-2.0.3.dist-info/METADATA`
- `cactus_needle-2.0.3.dist-info/WHEEL`
- `cactus_needle-2.0.3.dist-info/RECORD`

The wheel's `METADATA` identifies `cactus-needle` version `2.0.3` and describes it as the Needle engine binary container, but it does not contain `License`, `License-Expression`, `License-File`, author, project URL, classifiers, a LICENSE file, or a NOTICE file. No wheel-specific additional restriction was found in the inspected artifact.

### Redistribution decision

The upstream source and Hugging Face model repository both explicitly declare Apache-2.0, and the repository supplies the standard Apache-2.0 license. On that inspected evidence, redistribution of this pinned object artifact with the Apache-2.0 license and attribution is treated as permitted for this project's intended private registry. This is an engineering compliance record, not legal advice. Re-check the artifact and terms whenever the repository revision, wheel, engine version, hash, or distribution destination changes.
