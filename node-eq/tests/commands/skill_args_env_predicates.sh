#!/bin/sh
set -eu

root=.
name=guest
verbose=false

while [ "$#" -gt 0 ]; do
  case "$1" in
    --root=*) root=${1#*=} ;;
    --root|-r) shift; root=${1-.} ;;
    --name=*) name=${1#*=} ;;
    --name|-n) shift; name=${1-guest} ;;
    --verbose|-v) verbose=true ;;
    --) shift; break ;;
    *) break ;;
  esac
  shift
done

first=${1-}
second=${2-}
mode=${BESHT_SKILL_MODE-default}
empty=${BESHT_SKILL_EMPTY-fallback}
label=${BESHT_SKILL_LABEL-}

printf '%s\n' '--- args ---'
printf 'root=%s\n' "$root"
printf 'name=%s\n' "$name"
printf 'verbose=%s\n' "$verbose"
if [ -n "$first" ]; then
  printf 'first=%s\n' "$first"
else
  printf '%s\n' 'first=<missing-or-empty>'
fi
if [ -n "$second" ]; then
  printf 'second=%s\n' "$second"
else
  printf '%s\n' 'second=<missing-or-empty>'
fi

printf '%s\n' '--- env ---'
printf 'mode=%s\n' "$mode"
printf 'empty=%s\n' "$empty"
if [ -z "$label" ]; then
  printf '%s\n' 'label=<empty>'
else
  printf 'label=%s\n' "$label"
fi

printf '%s\n' '--- path ---'
if [ -d "$root" ]; then printf '%s\n' 'root-is-dir=true'; else printf '%s\n' 'root-is-dir=false'; fi
if [ -f "$root/README.md" ]; then printf '%s\n' 'readme-is-file=true'; else printf '%s\n' 'readme-is-file=false'; fi
if [ -r "$root/README.md" ]; then printf '%s\n' 'readme-readable=true'; else printf '%s\n' 'readme-readable=false'; fi
if [ -x "$root/dist/besht" ]; then printf '%s\n' 'compiler-executable=true'; else printf '%s\n' 'compiler-executable=false'; fi
