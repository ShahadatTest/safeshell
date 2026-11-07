#!/usr/bin/env bash
# Install (or remove) the AppArmor profile that lets the `ss` binary use user
# namespaces. Needs sudo. It shows the profile and asks before changing anything.
#
#   scripts/install-apparmor.sh [path/to/ss]      install (default: bin/ss)
#   scripts/install-apparmor.sh --uninstall       remove
#   add --yes to skip the confirmation question
set -euo pipefail

PROFILE=/etc/apparmor.d/safeshell-ss
here=$(cd "$(dirname "$0")/.." && pwd)
yes=0 uninstall=0 bin=""
for a in "$@"; do
  case "$a" in
    --yes) yes=1 ;;
    --uninstall) uninstall=1 ;;
    -*) echo "unknown option $a" >&2; exit 2 ;;
    *) bin="$a" ;;
  esac
done

confirm() { [ "$yes" = 1 ] && return 0; read -r -p "$1 [y/N] " r; [ "$r" = y ] || [ "$r" = Y ]; }

if [ "$uninstall" = 1 ]; then
  confirm "Remove $PROFILE?" || exit 1
  sudo apparmor_parser -R "$PROFILE" 2>/dev/null || true
  sudo rm -f "$PROFILE"
  echo "removed"
  exit 0
fi

bin=$(realpath "${bin:-$here/bin/ss}")
[ -x "$bin" ] || { echo "no executable at $bin (run: make build)" >&2; exit 1; }
case "$bin" in *[!A-Za-z0-9/_.-]*) echo "path has characters not allowed in the profile: $bin" >&2; exit 1 ;; esac

profile=$(sed "s#@BIN@#$bin#" "$here/packaging/apparmor/safeshell-ss.tmpl")
echo "This profile will be written to $PROFILE:"
echo
echo "$profile"
echo
echo "It applies only to: $bin"
echo "Anyone who can replace that file gets the same permission, so keep it in a folder only you can write to."
confirm "Install it?" || exit 1

echo "$profile" | sudo tee "$PROFILE" >/dev/null
sudo apparmor_parser -r "$PROFILE"
echo "installed. Check with: $bin doctor"
