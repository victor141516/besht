#!/bin/sh
set -eu

make_account() {
  printf '%s|%s|%s\n' "$1" "$2" "$3"
}

account_name() { printf '%s\n' "$1" | cut -d'|' -f1; }
account_role() { printf '%s\n' "$1" | cut -d'|' -f2; }
account_status() { printf '%s\n' "$1" | cut -d'|' -f3; }

account_label() {
  name=$(account_name "$1")
  role=$(account_role "$1")
  printf '%s:%s\n' "$name" "$role"
}

account_set_role() {
  name=$(account_name "$1")
  status=$(account_status "$1")
  make_account "$name" "$2" "$status"
}

account_is_active() {
  status=$(account_status "$1")
  [ "$status" = active ]
}

show_account() {
  label=$(account_label "$1")
  if account_is_active "$1"; then
    printf '%s active\n' "$label"
  else
    printf '%s inactive\n' "$label"
  fi
}

decorate_label() {
  printf '[%s]\n' "$1"
}

acct=$(make_account ada guest active)
show_account "$acct"
acct=$(account_set_role "$acct" admin)
show_account "$acct"
decorate_label "$(account_label "$acct")"
