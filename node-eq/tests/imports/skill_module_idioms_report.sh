#!/bin/sh

format_label() {
  printf '%s:%s\n' "$1" "$2"
}

print_info() {
  printf '[INFO] %s\n' "$1"
}

run_done() {
  printf '%s\n' 'done'
}
