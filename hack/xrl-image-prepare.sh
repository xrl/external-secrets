#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
set -eu
binary=$1
image_root=$2
cache="$image_root/var/cache/onepassword-sdk"
"$binary" onepassword-sdk-cache prepare --cache-dir "$cache"
# Cache entries are trusted executable input, never writable by the controller.
chown -R 0:0 "$image_root"
find "$image_root" -type d -exec chmod 0555 {} \;
find "$image_root" -type f -exec chmod 0444 {} \;
