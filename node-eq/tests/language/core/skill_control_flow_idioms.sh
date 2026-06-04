#!/bin/sh
set -eu

scan() {
  label=$1
  limit=$2
  query=$3

  printf '%s\n' "$label"
  if [ -z "$query" ]; then
    printf '%s\n' 'usage: scan QUERY'
    return 0
  fi

  count=0
  for item in alpha beta gamma delta alphabet betamax; do
    case "$item" in
      *"$query"*) ;;
      *) continue ;;
    esac

    count=$((count + 1))
    if [ "$count" -gt "$limit" ]; then
      printf '%s\n' 'too many matches'
      break
    fi

    printf '%s:%s\n' "$count" "$item"
  done

  if [ "$count" -eq 0 ]; then
    printf '%s\n' 'no matches'
  fi
}

scan '--- match a limit 5 ---' 5 a
scan '--- limit one ---' 1 a
scan '--- none ---' 5 z
scan '--- empty ---' 5 ''
