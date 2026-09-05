#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail
image=$1
echo 'Fresh-process smoke test only: this does not assert an SDK cache hit.'
# Replace --help with the real offline require-hit command when the SDK lands.
# Keep these sandbox flags: no writable mounts, tmpfs, network or capabilities.
set -x
"${DOCKER:-docker}" run --rm --network=none --read-only --user 65534:65534 \
  --cap-drop=ALL --security-opt=no-new-privileges "$image" --help
