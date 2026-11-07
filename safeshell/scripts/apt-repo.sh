#!/usr/bin/env bash
# Add a .deb to an apt repository directory and (re)generate its signed index.
#   scripts/apt-repo.sh REPO_DIR PACKAGE.deb [GPG_KEY_ID]
# Older versions already in REPO_DIR/pool stay available. Without a key the index is unsigned (testing only).
set -euo pipefail
repo=$1 deb=$2 key=${3:-}
mkdir -p "$repo/pool/main" "$repo/dists/stable/main/binary-amd64"
cp "$deb" "$repo/pool/main/"
cd "$repo"
dpkg-scanpackages --multiversion pool/main > dists/stable/main/binary-amd64/Packages 2>/dev/null
gzip -9kf dists/stable/main/binary-amd64/Packages
apt-ftparchive \
  -o APT::FTPArchive::Release::Origin=SafeShell \
  -o APT::FTPArchive::Release::Label=SafeShell \
  -o APT::FTPArchive::Release::Suite=stable \
  -o APT::FTPArchive::Release::Codename=stable \
  -o APT::FTPArchive::Release::Architectures=amd64 \
  -o APT::FTPArchive::Release::Components=main \
  release dists/stable > dists/stable/Release.new
mv dists/stable/Release.new dists/stable/Release
if [ -n "$key" ]; then
  gpg --batch --yes --default-key "$key" -abs -o dists/stable/Release.gpg dists/stable/Release
  gpg --batch --yes --default-key "$key" --clearsign -o dists/stable/InRelease dists/stable/Release
  gpg --batch --yes --export "$key" > safeshell.gpg
fi
