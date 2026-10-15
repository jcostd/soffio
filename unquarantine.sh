#!/bin/sh
# Copyright (C) 2026 Jacopo Costantini
# SPDX-License-Identifier: GPL-3.0-or-later
#
# unquarantine.sh: let macOS run soffio and preview. What comes out of a
# downloaded archive is quarantined, and Gatekeeper will not run a
# program that Apple has not notarized. This lifts the quarantine from
# this folder alone, gives back the exec bits a download may have
# dropped, and checks that soffio runs. Nothing may be executable yet,
# so run it from the Terminal, in this folder:
#
#   sh unquarantine.sh

cd "$(dirname "$0")" || exit 1

if [ "$(uname -s)" != Darwin ]; then
  echo "unquarantine.sh: for macOS only" >&2
  exit 1
fi

chmod 755 soffio preview
xattr -dr com.apple.quarantine . 2> /dev/null

if ./soffio -v > /dev/null 2>&1; then
  echo "unquarantine.sh: soffio and preview can run"
else
  echo "unquarantine.sh: macOS still blocks soffio. In System Settings," >&2
  echo "Privacy & Security, press Open Anyway next to it, then run this again." >&2
  exit 1
fi
