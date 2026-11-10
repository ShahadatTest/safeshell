# Packaging

How SafeShell is built and distributed.

| Artifact | Built by | Where it goes |
|---|---|---|
| Tarballs (linux amd64, arm64) | `make dist` | GitHub Release |
| `.deb` (amd64) | `make deb` | GitHub Release |
| `SHA256SUMS` | release workflow | GitHub Release |
| Source package for Launchpad | `debian/` + `debuild -S` | PPA `ppa:samarthsinh/safeshell` |

Releases are made by merging a `release/vX.Y.Z` PR; see [CONTRIBUTING.md](../CONTRIBUTING.md#releases).

## The `.deb` and the tarballs
`make dist VERSION=vX.Y.Z` and `make deb VERSION=vX.Y.Z` build static binaries (CGO off). The version
is stamped in with `-X main.version`. The `.deb` installs `/usr/bin/safeshell` and an AppArmor profile for
exactly that path (Ubuntu 24.04+ otherwise blocks the sandbox).

The command is `safeshell` and not `ss`: `/usr/bin/ss` belongs to `iproute2` on every Debian and Ubuntu machine.

## The `debian/` source package
`debian/` is a normal Debian source package using `dh-golang`. It is what the PPA builds from. It builds
offline and `lintian --pedantic` shows no errors. To check it in a clean Debian unstable container (needs Docker):

```bash
docker run --rm -v "$PWD":/src -w /src debian:sid bash -c '
  apt-get update -qq && apt-get install -y -qq build-essential devscripts equivs lintian fakeroot >/dev/null &&
  mk-build-deps -i -r -t "apt-get -y -qq" debian/control >/dev/null &&
  dpkg-buildpackage -us -uc -b && lintian -i --pedantic ../safeshell_*.changes'
```

Non-Go files the code embeds must be listed in `DH_GOLANG_INSTALL_EXTRA` in `debian/rules`, or the
offline build cannot see them.

## The PPA
Launchpad builds only for series whose Go and library packages are new enough. The PPA targets the newest
Ubuntu series; older releases should use the `.deb` or the tarball. To upload a new version:

1. Set the series and a `~ppaN` version in a copy of `debian/changelog` (for example `1.0.1-1~ppa1`, series `stonking`).
2. Make an orig tarball from the release tag without `debian/`:
   `git archive --format=tar.gz --prefix=safeshell-X.Y.Z/ -o ../safeshell_X.Y.Z.orig.tar.gz vX.Y.Z -- . ':(exclude)debian'`
3. Build the source package (`dpkg-buildpackage -S -us -uc -d`), sign it (`debsign -k<key id> *_source.changes`),
   and upload it (`dput ppa:samarthsinh/safeshell *_source.changes`).

## Debian and Ubuntu archives
An ITP was filed (Debian bug #1153343) and was closed by a Debian Developer, who questioned whether the
project was mature and maintained enough. SafeShell is therefore not in the Debian or Ubuntu archives.
It can be proposed again later, once it has users, a longer history and a maintainer who can answer for every part of it.
