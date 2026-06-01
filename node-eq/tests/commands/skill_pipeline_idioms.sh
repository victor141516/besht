#!/bin/sh
set -eu

ROOT=${1:-.}

printf '%s\n' '--- top go files ---'
find "$ROOT" -maxdepth 2 -type f -name '*.go' 2>/dev/null \
  | sed 's#^\./##' \
  | grep -v '/vendor/' \
  | sort \
  | head -n 5

printf '%s\n' '--- markdown mentions ---'
grep -RIn 'Besht' "$ROOT" --include='*.md' 2>/dev/null \
  | cut -d: -f1 \
  | sed 's#^\./##' \
  | sort -u \
  | head -n 4

printf '%s\n' '--- workdir root ---'
(cd "$ROOT" && pwd)

printf '%s\n' '--- env value ---'
BESHT_SKILL_MODE=scan printenv BESHT_SKILL_MODE

printf '%s\n' '--- silent probe ---'
find "$ROOT" -maxdepth 1 -type f >/dev/null 2>/dev/null && printf '%s\n' ok

printf '%s\n' '--- silent count ---'
find "$ROOT" -type f -name '*.bsh' 2>/dev/null | wc -l | tr -d ' '
