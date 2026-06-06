#!/bin/sh
set -u

check_word() {
  word=$1
  printf '%s\n' alpha beta gamma | grep -qx "$word"
}

for word in alpha omega beta; do
  if check_word "$word" >/dev/null 2>&1; then
    printf 'ok:%s\n' "$word"
  else
    code=$?
    printf 'missing:%s:%s\n' "$word" "$code"
  fi
done

for cmd in true false true; do
  "$cmd" >/dev/null 2>&1
  code=$?
  if [ "$code" -eq 0 ]; then
    printf 'run:%s:ok\n' "$cmd"
  else
    printf 'run:%s:%s\n' "$cmd" "$code"
  fi
done

printf '%s\n' done
