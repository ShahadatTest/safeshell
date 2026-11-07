# SafeShell

A safety layer for your terminal. Every command you press Enter on is rated by how
reversible it is, tried in a sandbox first, and blocked, confirmed or undoable
before it can hurt anything.

| Level | Meaning | What happens |
|---|---|---|
| R0 | Read-only | runs |
| R1 | Easily reversed | runs |
| R2 | Reversible with a snapshot | snapshot taken, runs, `ss undo` puts it back |
| R3 | Irreversible | blocked, with the reason and a safer alternative; you can still say "do it anyway" |
| R4 | Catastrophic | hard block |

Evidence matters: a rating is `ESTIMATED` (from the command text), `INSPECTED`
(real disks/processes/packages looked at), `PREVIEWED` or `OBSERVED` (it actually
ran in the sandbox). Observed effects and the AI can only raise a rating, never lower it.

## Install

Linux, amd64 or arm64 (bash or zsh).

```bash
# from a release
tar -xzf safeshell-vX.Y.Z-linux-amd64.tar.gz && cd safeshell-vX.Y.Z-linux-amd64
bash scripts/install.sh            # add --ai to also set up the built-in AI

# or the Debian package (also installs the AppArmor rule)
sudo apt install ./safeshell_X.Y.Z_amd64.deb && ss install

# or from source (needs Go)
cd safeshell && make build && bash scripts/install.sh
```

On Ubuntu 24.04+ the sandbox needs a one-time AppArmor rule
(`scripts/install-apparmor.sh`, needs sudo). `ss doctor` tells you what your machine supports.

## Use

Open a new terminal; just work normally. Inside the prompt:

- `/models` choose the AI: download a local model or connect Claude, GPT or Gemini with an API key
- `/help` or `ss help` guided help
- `ss undo` restore what the last approved command changed (verified)
- Type English, e.g. `show the 5 largest files here`; the AI proposes a command and it is checked like any other

Other commands: `ss check|analyze|simulate <cmd>`, `ss explain`, `ss ask`, `ss snapshots`,
`ss daemon`, `ss ai setup|status|stop|bench`, `ss doctor`, `ss version`, `ss uninstall`.

## AI (optional, off until you choose)

- **Local (default Qwen2.5-Coder 3B):** SafeShell downloads and manages a pinned,
  checksum-verified `llama.cpp` server (CUDA, Vulkan or CPU, picked for your machine) and the model.
  Nothing else to install; it starts on demand and stops when idle.
- **Cloud:** Claude, GPT or Gemini with your own key (stored 0600 or read from
  `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` / `GEMINI_API_KEY`). Your home path is redacted before anything is sent.

`ss ai bench` measures how accurate the active model is on 43 labelled requests.

## Develop

```bash
cd safeshell
make check        # vet, gofmt, race tests, arm64/riscv64 vet
make integration  # real binary on the real kernel (needs the AppArmor rule)
SS_SWEEP=1 go test ./test/integration -run Sweep   # every labelled command through the real binary
make fuzz
```

Layout: pure `domain/classify/policy/report`, an `engine` with ports, adapters
(sandbox, snapshot, AI, runtime), and `cmd/ss` as the composition root. See
[doc/architecture.md](doc/architecture.md).

## Publishing a release

```bash
git tag v0.1.0 && git push origin v0.1.0
```

`.github/workflows/release.yml` runs the checks, builds both tarballs, the `.deb` and
`SHA256SUMS`, and attaches them to a GitHub Release. Locally: `make dist VERSION=v0.1.0 && make deb`.

## Team

Samarth Vala (23BCP422) and Tavish Gupta (23BCP064), Sem 7 minor project.
