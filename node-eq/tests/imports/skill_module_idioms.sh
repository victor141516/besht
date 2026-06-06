#!/bin/sh
set -eu

dir=$(dirname "$0")
. "$dir/skill_module_idioms_report.sh"
. "$dir/skill_module_idioms_legacy_text.sh"

role=${ROLE-admin}
for name in ada grace linus; do
  upper=$(legacy_upper "$name")
  label=$(format_label "$role" "$upper")
  print_info "$label"
done

run_done
