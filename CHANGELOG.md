# Changelog

All notable changes to SafeShell. Versions follow [semantic versioning](https://semver.org/).

## 1.0.0 - 2026-11-07

First stable release.

### Safety layer
- Rates every command R0 to R4 by how reversible it is, with a rule set covering files, disks, git, containers, Kubernetes, databases, packages, services, networking and cloud tools.
- Rootless sandbox (user, mount, PID and network namespaces with OverlayFS) that shows the real file changes, with the network blocked and every connection attempt recorded.
- Real-world checks for processes, disks, packages and services a command would affect.
- Blocks with a reason and a safer alternative; "do it anyway" is always the user's choice.
- Snapshots before approved risky commands, and `safeshell undo` with verification.
- bash and zsh hooks that fail open with a visible warning if SafeShell itself breaks; optional background daemon for faster checks.

### AI assistant (optional)
- Plain English to a command that is checked like any other; the AI can only raise a risk level.
- Built-in local model (default Qwen2.5-Coder 3B) with a managed, checksum-verified llama.cpp engine (CUDA, Vulkan or CPU), or your own Claude, GPT or Gemini key.
- `/models` menu, `safeshell help`, and `safeshell ai bench` (43 labelled requests).

### Packaging
- The command is `safeshell` (`/usr/bin/ss` belongs to `iproute2`).
- Linux amd64 and arm64 tarballs, a `.deb`, and a Debian source package; PPA at `ppa:samarthsinh/safeshell` for the newest Ubuntu.

### Measured
- 92.3% exact on a held-out set of 220 commands; 97% on the English-to-command benchmark (local Qwen 3B); 492 labelled commands run through the real binary with no change outside the sandbox.
