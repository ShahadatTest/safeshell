#!/bin/sh
# One-time: let apt know where SafeShell lives, then  sudo apt install safeshell
set -eu
[ "$(id -u)" = 0 ] || { echo "Run with sudo:  curl -fsSL https://samarthsinh2660.github.io/safeshell/setup.sh | sudo sh" >&2; exit 1; }
base=https://samarthsinh2660.github.io/safeshell
curl -fsSL "$base/safeshell.gpg" -o /usr/share/keyrings/safeshell.gpg
echo "deb [signed-by=/usr/share/keyrings/safeshell.gpg] $base stable main" > /etc/apt/sources.list.d/safeshell.list
apt-get update -o Dir::Etc::sourcelist=/etc/apt/sources.list.d/safeshell.list -o Dir::Etc::sourceparts=- -o APT::Get::List-Cleanup=0
apt-get install -y safeshell
echo "Done. Now run:  ss install"
