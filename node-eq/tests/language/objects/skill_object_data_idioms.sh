#!/bin/sh
set -eu

TEAM='ada:admin:yes
grace:member:yes
linus:member:no
ken:guest:no'

printf '%s\n' '--- active staff ---'
printf '%s\n' "$TEAM" | awk -F: '$3 == "yes" && $2 != "guest" { print $1 "=" $2 }'

printf '%s\n' '--- probes ---'
for probe in ada ken alan; do
  if printf '%s\n' "$TEAM" | awk -F: -v p="$probe" '$1 == p { found = 1 } END { exit found ? 0 : 1 }'; then
    printf '%s=true\n' "$probe"
  else
    printf '%s=false\n' "$probe"
  fi
done

printf '%s\n' '--- names ---'
printf '%s\n' "$TEAM" | cut -d: -f1 | paste -sd, -

printf '%s\n' '--- role entries ---'
printf '%s\n' "$TEAM" | awk -F: '{ print $1 ":" $2 }'

printf '%s\n' '--- summary json ---'
printf '%s\n' '{"primary":"ada","primaryRole":"admin","hasAlan":false,"activeCount":2}'
