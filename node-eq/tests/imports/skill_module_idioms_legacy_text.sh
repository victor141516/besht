#!/bin/sh

legacy_upper() {
  printf '%s\n' "$1" | tr '[:lower:]' '[:upper:]'
}
