#!/bin/sh
set -u

mode=${1:-scan}
limit=${2:-4}

case "$mode" in
  scan|dry-run)
    printf 'mode:%s\n' "$mode"
    ;;
  *)
    printf 'unknown mode:%s\n' "$mode"
    exit 2
    ;;
esac

count=0
for task in alpha '' skip fail omega extra; do
  if [ -z "$task" ]; then
    printf '%s\n' 'skip:empty'
    continue
  fi

  case "$task" in
    skip)
      printf 'skip:%s\n' "$task"
      continue
      ;;
    fail)
      false
      code=$?
      if [ "$code" -ne 0 ]; then
        printf 'failed:%s\n' "$code"
        continue
      fi
      ;;
  esac

  printf 'task:%s\n' "$task"
  count=$((count + 1))
  if [ "$count" -ge "$limit" ]; then
    printf '%s\n' 'limit reached'
    break
  fi
done

probe_input=${PROBE_INPUT:-scan}
if printf '%s\n' "$probe_input" | grep -qx "$mode" >/dev/null 2>&1; then
  printf '%s\n' 'probe:match'
else
  code=$?
  printf 'probe:miss:%s\n' "$code"
fi

exit 0
