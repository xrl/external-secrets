#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail
image=$1
run() {
  "${DOCKER:-docker}" run --rm --network=none --read-only --user 65534:65534 \
    --cap-drop=ALL --security-opt=no-new-privileges "$image" "$@"
}
set -x
run onepassword-sdk-cache check --cache-dir /var/cache/onepassword-sdk
# An existing directory without entries must fail, not compile or fall back.
if output=$(run onepassword-sdk-cache check --cache-dir /var/cache 2>&1); then
  echo 'ERROR: strict cache miss unexpectedly succeeded' >&2
  exit 1
fi
printf '%s\n' "$output"
grep -F '1Password SDK cache check failed:' <<< "$output"
# Normal startup also rejects a miss before Kubernetes configuration discovery.
if output=$(run --onepassword-sdk-cache-dir /var/cache 2>&1); then
  echo 'ERROR: controller accepted an unprepared cache' >&2
  exit 1
fi
printf '%s\n' "$output"
grep -F 'unable to prepare provider runtime' <<< "$output"
