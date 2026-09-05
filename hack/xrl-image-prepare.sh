#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
set -eu
binary=$1
image_root=$2
mkdir -p "$image_root"
echo 'Native executable smoke test only: SDK cache preparation is not integrated.'
"$binary" --help
# Files written beneath image_root are copied at the same absolute paths in the
# final image. The SDK integration owns cache population and read permissions.
