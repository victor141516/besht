#!/bin/sh
set -eu

WORDS='  alpha
Beta
apricot
banana
  avocado  '
NUMBERS='3
7
10
2'

printf '%s\n' '--- a words uppercase ---'
printf '%s\n' "$WORDS" \
  | sed 's/^ *//; s/ *$//' \
  | grep '^a' \
  | tr '[:lower:]' '[:upper:]'

printf '%s\n' '--- joined labels ---'
printf '%s\n' "$WORDS" \
  | sed 's/^ *//; s/ *$//' \
  | awk 'BEGIN{sep=""} /^a/ {printf "%sitem:%s", sep, $0; sep=", "} END{printf "\n"}'

printf '%s\n' '--- totals ---'
printf '%s\n' "$NUMBERS" \
  | awk '{sum += $1; if ($1 >= 5) big += 1} END {printf "sum=%d\nbig=%d\n", sum, big}'

printf '%s\n' '--- indexed ---'
printf '%s\n' "$WORDS" \
  | sed 's/^ *//; s/ *$//' \
  | awk '{printf "%d:%s\n", NR - 1, $0}'
