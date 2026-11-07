# Project Idea: A Safety Layer That Lives in Your Terminal

**Working title:** *SafeShell* (placeholder, rename later)
**One-line pitch:** A permanent guardian inside the terminal that stops harmful commands before they run, explains why in plain English, suggests a safer command, and lets you talk to the terminal in normal language instead of memorising commands.

Base idea: *Transactional Command Framework with AI-Generated Undo Plans and Simulation-Based Safety Guarantees.*

---

## 1. The Vision

Today the terminal trusts you completely. One typo in `rm -rf`, a copy-pasted command from the internet, or a wrong `chmod -R` can destroy a system, and there is no undo, no warning and no explanation. Beginners also struggle because they must remember hundreds of commands and flags.

We want a package that is installed once and then **is always active whenever a terminal opens**. It does three jobs:

1. **Protect.** Check every command before it runs. If it is dangerous, block or pause it, say *why*, and show a *better* command.
2. **Prove.** Do not just guess the impact. Run the command in a throwaway sandbox first and show the *real* diff (what would be deleted, changed, created).
3. **Translate.** Let the user say "copy my report from Downloads to Documents" and turn it into the correct command, show it, and run it safely.

Plus a safety net: if something still goes wrong, **roll back** using a snapshot.

---

## 2. What the User Experiences

### 2.1 Harmful command is caught

```
$ rm -rf ~/projects /tmp/old

 SafeShell: BLOCKED (irreversible, high impact)

 Why:    This permanently deletes 4,312 files (2.1 GB) in ~/projects.
         Files removed this way cannot be recovered.
 Seen in sandbox: 4,312 deleted, 0 created, 0 modified.

 Your options:
   1) trash-put ~/projects /tmp/old          (safer: moves to Trash, recoverable)
   2) rm -rI ~/projects /tmp/old             (safer: asks before deleting)
   3) Do it anyway, with a safety snapshot   (runs YOUR command; roll back for 24h)
   4) Do it anyway, no snapshot              (runs YOUR command exactly, no net)
   5) Cancel

 Choose [1-5]:
```

**The user always has the final say.** Blocking never means "you are not allowed"; it means "are you sure, here is what will happen". The "do it anyway" options exist for every class except R4, and:

- Option 3 (anyway + snapshot) is the default "anyway" because it costs the user nothing.
- Option 4 needs a typed confirmation for R3 (e.g. type the number of files, `4312`) so it cannot be chosen by muscle memory.
- R4 (catastrophic, e.g. `rm -rf /`) needs `ss --force` plus a typed phrase, and is logged. It is still possible, but never one keypress.
- Non-interactive use (scripts, CI) follows a policy setting: `allow`, `ask`, or `deny`, so automation never hangs on a prompt.
- Users can mark a path or command as trusted ("always allow `rm` inside `~/build`") to avoid repeated prompts.

### 2.2 Plain-language command

```
$ ss copy report.pdf from downloads to documents
 Understood: copy a file
 Command:    cp ~/Downloads/report.pdf ~/Documents/
 Safety:     SAFE (nothing is overwritten, reversible)
 Run it? [Y/n]
```

```
$ ss open the folder where my college project is
 Found: ~/Desktop/study/sem7/prj/minor
 Command: cd ~/Desktop/study/sem7/prj/minor
```

```
$ ss undo
 Last action: moved 12 files to ~/Archive. Roll back? [Y/n]
```

### 2.3 Explain mode

```
$ ss explain "chmod -R 777 /"
 Gives everyone full read/write/execute on every file on the system.
 This breaks system security and can stop programs (sudo, ssh) from working.
```

---

## 3. Core Concepts

### 3.1 Reversibility classification

Every command is placed in a class before it runs.

| Class | Meaning | Examples | Action |
|---|---|---|---|
| **R0: Read-only** | Changes nothing | `ls`, `cat`, `grep`, `pwd` | Run instantly, no delay |
| **R1: Reversible** | Can be undone exactly | `mv`, `cp` (new dest), `mkdir`, `touch` | Run, record an undo plan |
| **R2: Recoverable** | Undo possible with snapshot/trash | `rm` on small sets, `sed -i`, overwrite via `>` | Simulate, snapshot, then run |
| **R3: Irreversible** | Cannot be undone | `rm -rf` on big trees, `dd`, `mkfs`, `shred`, `git push --force`, `DROP TABLE` | Block by default, explain, offer safer option |
| **R4: Catastrophic** | System-wide damage | `rm -rf /`, fork bomb, `chmod -R 777 /`, `curl ... \| sudo bash` | Hard block, needs explicit override |

Classification is a **hybrid**: fast deterministic rules first (parser + pattern/flag table), and the AI layer only for ambiguous or unknown commands. This keeps it fast and means the AI can never *lower* a rule-based risk level, only raise it.

### 3.2 Simulation in an OverlayFS sandbox (the key idea)

Instead of *predicting* what a command will do, we **run it for real in a copy-on-write sandbox** and observe.

```
lowerdir  = real filesystem (read-only view)
upperdir  = sandbox layer (all writes land here)
merged    = what the command sees

Run command inside merged  ->  inspect upperdir  ->  that IS the diff
```

- New file in `upperdir` = created.
- Modified file in `upperdir` = changed.
- Whiteout entry (char device 0/0) in `upperdir` = deleted.

So the impact report is an **observed fact**, not an AI guess. After the user approves, the same command is run on the real system (or the upperdir is committed).

OverlayFS alone only sees the filesystem, so it is just **one layer** of a bigger design. Network, processes, services, databases, devices and cloud tools each get their own handling. See section 3.3 (Effect Coverage), which replaces the old "partially simulated" label.

### 3.3 Effect Coverage: handling everything, not just files

**Goal:** no command is ever silently unprotected. Every kind of side effect is handled by one of four *evidence levels*, and the report always tells the user which level it is.

| Level | Label shown | How we know | Trust |
|---|---|---|---|
| **A** | `OBSERVED` | Command really ran in the sandbox; we measured the result | Highest |
| **B** | `PREVIEWED` | The tool's own dry-run (`--dry-run`, `apt -s`, `kubectl diff`, SQL in a rolled-back transaction) | High |
| **C** | `INSPECTED` | We read the *real current state* (read-only) to list what would be hit, e.g. which processes `pkill` would match | Medium |
| **D** | `ESTIMATED` | Static analysis only; we escalate risk conservatively and require confirmation | Lowest, never auto-run |

The sandbox is built from standard unprivileged Linux features, and none of them need root:

```
user namespace   -> fake root inside, real privileges outside
mount namespace  -> OverlayFS for files (observed diff)
PID namespace    -> command can only see/signal sandbox processes, not real ones
net namespace    -> no real network; connection attempts are recorded, not sent
seccomp-notify   -> kernel pauses selected syscalls (connect, kill, mount, reboot,
                    ptrace...) so we can log, fake, or deny them
Landlock         -> extra hard limit on paths the sandbox may touch
cgroups          -> CPU/memory/time limit so a fork bomb dies in the sandbox
```

Per-effect handling:

| Effect | What goes wrong today | How we cover it | Level |
|---|---|---|---|
| **Files** (rm, mv, sed -i, >) | Data loss | OverlayFS diff: created / modified / deleted | A |
| **Network** (curl, scp, rsync, `git push`, `wget \| sh`) | Data leaves the machine, remote changes cannot be undone | Net namespace with no route. seccomp-notify records every `connect()`/`sendto()` so the report says "would POST to 1.2.3.4:443". Remote writes are marked **irreversible** (R3); downloads piped into a shell are flagged by pre-fetching the script (read-only) and scanning it | A (attempts) + D (remote result) |
| **Processes** (kill, pkill, killall, `kill -9 -1`) | Wrong process dies, unsaved work lost | PID namespace means the sandboxed `kill` cannot touch real processes. For the report we **resolve the targets against the real `/proc` (read-only)** and list "would kill: firefox (pid 4121, 2h 10m, owned by you)". Critical targets (init, sshd, the display server, your own shell) escalate to R4 | C |
| **Services** (`systemctl stop/disable`, `service`, reboot, shutdown) | System/service outage | systemd calls go over D-Bus; the sandbox has no access to the real bus. We inspect the unit (`systemctl show`) and its dependents (`list-dependencies --reverse`) and report impact. Undo = the inverse action (start/enable) | C |
| **Databases, file-based** (SQLite, `.db` files) | Data loss | Lives on disk, so the OverlayFS covers it fully | A |
| **Databases, server** (psql, mysql, redis-cli, mongosh) | `DELETE` without `WHERE`, `DROP TABLE` | Parse the SQL. For Postgres, run it in `BEGIN ... ROLLBACK` to get real row counts. For MySQL/MariaDB (DDL auto-commits, so a transaction is unsafe) run against a **throwaway clone** or only count rows with a matching `SELECT COUNT(*)`. Redis: `DBSIZE`/`SCAN` for the key pattern. Before a real run, auto-backup the affected rows/tables so undo is possible | B / C |
| **Git** (`reset --hard`, `clean -fdx`, `push --force`, `rebase`) | Lost work, rewritten shared history | Local effects live in the worktree + `.git`, so the overlay captures them (A). `push` goes through the network layer. Undo via `git reflog` stored before running. Force-push is R3 with "do it anyway" | A / B |
| **Package managers** (apt, dnf, pip, npm) | Removing needed packages, breaking the system | Native dry run (`apt -s`, `dnf --assumeno`, `npm --dry-run`) shows which packages would be removed. Overlay on `/usr`, `/var/lib` for the rest. Warn when removing `essential` packages | B + A |
| **Containers / cloud** (docker, kubectl, terraform, aws, gcloud) | Production outage, cost | Use native dry-run: `kubectl diff` / `--dry-run=server`, `terraform plan`, `aws --dry-run` where supported. Where none exists, level D: ask for confirmation and flag the target context (prod vs dev) | B / D |
| **Disks and devices** (`dd`, `mkfs`, `fdisk`, `wipefs`, `> /dev/sda`) | Whole disk wiped | Never run for real in a sandbox (devices are denied). Inspect the target (`lsblk`, mount status, is it the system disk?) and report "this will erase /dev/sdb (32 GB, contains 2 partitions, mounted at /media/usb)" | C |
| **Permissions / ownership** (`chmod -R`, `chown -R`) | Broken system or security hole | Overlay shows exactly which files change mode/owner; recursion on system paths like `/`, `/etc`, `/usr` is R4 | A |
| **Privilege** (`sudo`, `su`) | Everything above, but with root | Simulated with fake-root in the user namespace; the report states the real run needs root. Raise one risk level | A + rule |
| **Environment** (`export`, `alias`, `source`, `cd`) | Changes the shell itself, not the disk | Handled by the shell hook, not the sandbox (the sandbox is a child process and cannot change the parent shell). Rule-based only | Rule |
| **Time / random / non-deterministic** | Real run differs from the simulation | Detected via syscalls (`clock_gettime`, `getrandom`) and the report says the real result may differ | Flag |
| **Scripts and subshells** (`./clean.sh`, `bash -c`, `make clean`) | Hidden dangerous commands | Run the script inside the sandbox and observe real effects (A); also statically scan it before running | A |

**Rule that makes this a guarantee instead of a hope:** every command's report lists each effect category it touches with its evidence level. If any category is level D (we could not observe or preview it), the command can never auto-run; it needs confirmation, and the report says so plainly. So the promise is *"you are always told what we could prove and what we could only estimate"*, not *"we can predict everything"*.

**Honest limits that remain** (to be stated in the final report):
1. Effects on **remote systems** (an API call that deletes a cloud resource) cannot be observed locally. We detect the call and treat it as irreversible, but we cannot show the remote result.
2. A command that **behaves differently depending on remote responses or time** can differ between simulation and real run. We flag it.
3. **Hardware and kernel effects** (`echo b > /proc/sysrq-trigger`, loading kernel modules) are denied in the sandbox and handled by rules (R4).
4. Sandbox escape bugs in the kernel are out of scope; we rely on the standard kernel isolation features.
5. Simulation costs time and disk space on very large trees, so there is a size threshold (see risks).

Existing open-source work (e.g. Sandlock, which stages writes copy-on-write with a `--dry-run` that shows what a command would change, and Syd, which intercepts syscalls using seccomp-notify and Landlock) shows these pieces are practical on unprivileged Linux, so this is an engineering task, not a research gamble. We should study their designs but build our own, since our goal (explain + alternatives + undo + AI) differs.

### 3.4 Rollback with snapshots

For R2 and risky R1 commands, take a snapshot of the affected paths before running.

- Preferred: Btrfs/ZFS/LVM snapshots when available (instant).
- Fallback: copy-on-write copy of just the affected paths (known from the simulation diff), stored in a snapshot store like `~/.safeshell/snapshots/<id>/`.
- `ss undo` / `ss rollback <id>` restores them.

Because simulation already told us exactly which files will change, we only back up those, which keeps snapshots small.

### 3.5 AI layer (three roles, all verified)

| Role | What it does | Safety rule |
|---|---|---|
| **Intent classifier** | Decides what a command or sentence is trying to do | Can only raise risk, never lower the rule-based level |
| **Explainer** | Plain-English "what and why" | Based on the *observed diff*, so it cannot invent effects |
| **Undo/alternative proposer** | Suggests an undo plan and a safer command | **Verified in the sandbox before being trusted** |

Important principle: **the AI proposes, the sandbox verifies.** An undo plan is shown to the user only if simulating "command, then undo plan" returns the filesystem to its original state. If the check fails, we fall back to snapshot rollback. This is what gives the "simulation-based safety guarantee".

### 3.6 AI backends: local, cloud, or both (user's choice)

The AI is **optional and swappable**. The safety core (parser, rules, sandbox, snapshots) works with no AI at all, so the tool is useful on an offline laptop and the AI only improves explanations, alternatives and natural language.

```
[ai]
backend = "local"      # "local" | "cloud" | "hybrid" | "off"
```

| Mode | Behaviour | Best for |
|---|---|---|
| `off` | Rules + templates only. Explanations come from fixed text templates filled with the observed diff | Servers, strict-privacy, very low-end machines |
| `local` | Everything runs on the user's machine; nothing leaves it | Privacy, offline, free |
| `cloud` | Calls a hosted model through an API key the user provides | Weak hardware, best quality |
| `hybrid` (default suggestion) | Local model for the fast/simple work (classify, common file operations); cloud only for hard requests, and only if the user opted in | Balance of speed, quality, privacy |

Code design: one `AiBackend` interface (a Rust trait) with `LocalBackend` and `CloudBackend` implementations, so the rest of the system does not care which is used and new providers can be added without touching the core.

#### Local model: which one and how

Rule of thumb: pick by the user's RAM/GPU; the tool detects hardware at setup (`ss setup`) and recommends one. The tasks here (turn a sentence into a short command, classify a command, explain a diff) are small and structured, so a **small quantized model is enough**; we do not need a huge one.

| Hardware | Suggested local model (quantized GGUF, ~Q4) | Approx. memory |
|---|---|---|
| Low (4 GB RAM, no GPU) | A ~3B model, e.g. Llama 3.2 3B, Phi-4 Mini, or Gemma small variants | ~2-3 GB |
| Normal (8 GB RAM / laptop) | **Qwen3 8B** (or its 4B size if tight) | ~5 GB |
| Strong (16 GB+ RAM or a GPU) | A larger Qwen3 coder-class model (14B and up) | 10 GB+ |

Qwen3 is the recommended default family: it has permissive (Apache 2.0) licensing, decent code and shell ability at small sizes, and is available in all sizes above, so one setup path covers everyone. Local-model rankings change every few months, so **treat these as a starting list and benchmark 3-4 candidates on our own command dataset** (the evaluation in section 8) before fixing the default. That benchmark is itself a good project result.

How we run it (two supported options):

1. **Embedded (default):** link llama.cpp into our Rust binary through a Rust binding crate and load a GGUF file directly. Nothing extra to install, no separate server, lowest latency, and the model stays warm inside the daemon.
2. **Ollama / OpenAI-compatible local server (option for people who already have one):** the daemon talks to `http://localhost:11434`. Easy to switch models.

Practical details:
- **Model download:** `ss setup` downloads the chosen GGUF once to `~/.safeshell/models/`, verifies its checksum, and shows the size first.
- **Structured output:** use llama.cpp grammar-constrained decoding (a GBNF/JSON schema) so the model can only emit valid fields like `{intent, command, risk, explanation}`. This removes most parsing failures and prompt-injection weirdness.
- **Short prompts and small context:** send only the command, current directory listing summary and the sandbox diff, not whole files. This keeps local inference fast.
- **Speed budget:** R0 commands never wait for the AI. The AI is only on the slow path (blocked/risky commands and NL requests), where 1-3 seconds is acceptable. Cache results for repeated commands.
- **Lazy loading:** load the model on first need and unload after idle time to save RAM.
- **Fine-tuning (stretch goal):** a small model fine-tuned on our labeled command dataset can beat a bigger general model for this one job.

#### Cloud model

- User supplies their own API key (stored in the OS keyring, not in plain config).
- **Privacy controls:** by default, send only the command text and the summarized diff; replace home paths and usernames with placeholders (`~/Documents/...` becomes `<HOME>/...`); never send file contents unless the user explicitly allows it; a `--show-prompt` flag prints exactly what would be sent.
- Timeouts and automatic fallback to local/rules if the network is down.
- Same output schema as local, so results are comparable.

#### Safety rules that hold for both

1. The AI can only **raise** the risk class, never lower it.
2. Explanations are generated from the **observed diff**, so the model cannot invent effects.
3. Any command the AI proposes (safer alternative, undo, NL translation) goes through the **same classifier and sandbox** as a user-typed command.
4. File names and command output are **untrusted text** (a file named `ignore previous instructions` must not steer the model). They go into the prompt as quoted data, and the output is schema-constrained.
5. If the AI is slow, wrong, or unavailable, the system degrades to rules + templates; it never falls open.

---

## 4. How It Stays Permanently Active

The tool must hook into the shell so the user does not have to think about it.

- **Install:** one command (`pip install` / `apt` package / install script) adds a line to `~/.bashrc` and `~/.zshrc` that loads the SafeShell hook.
- **Interception:**
  - Zsh: `preexec` hook plus a ZLE widget bound to Enter (can cancel the command).
  - Bash: `bind -x` on Enter, or the `DEBUG` trap with `extdebug` (returning non-zero cancels the command), plus `PROMPT_COMMAND`.
  - Fish (later): `fish_preexec` event / key binding.
- **Flow on every Enter:**
  1. Hook grabs the command line.
  2. Sends it to a small local **daemon** over a Unix socket (fast, keeps models/state warm).
  3. Daemon parses, classifies, simulates if needed, and returns: allow / warn / block + report.
  4. Hook prints the report and either runs the command, runs the safer one, or cancels.
- **Fast path:** R0 commands return in a few milliseconds, so the terminal never feels slow.
- **Always-on safety:** a systemd user service starts the daemon on login; if the daemon is down the hook fails *open with a warning* (configurable to fail *closed* for strict mode).
- **Escape hatch:** a bypass like `ss --force <cmd>` or prefixing a command with a space is available but logged, so an expert is never blocked forever.

Known limitation to document: a user who deliberately runs `/bin/sh` or a script can bypass a shell hook. Scripts and subshells should be analysed too (we parse the script file before it runs), and a stronger enforcement option (seccomp / an `LD_PRELOAD` wrapper / a restricted shell) is future work.

---

## 5. Natural-Language Mode

Goal: *"People should not need to remember commands."*

Pipeline:

```
user sentence
   -> intent + entity extraction (action, source, destination, filter)
   -> path resolution (fuzzy match: "downloads" -> ~/Downloads, "my project" -> best match)
   -> command generation (cp / mv / mkdir / find / tar ...)
   -> SAME safety pipeline (classify -> simulate -> explain)
   -> show command + confirmation -> run
```

Key points:

- **Transparency:** the generated command is always shown, so users slowly *learn* the real commands (a teaching feature, not just a shortcut).
- **Ambiguity handling:** if "the report" matches 3 files, ask which one instead of guessing.
- **Same safety rules apply:** "delete everything in this folder" is still classified R3 and blocked or simulated. Natural language gives no extra power.
- **Context memory:** remembers the current directory and the last few actions ("move *those* to Archive").
- **Offline-friendly:** a small local model (or rule templates) for common file operations, and an optional cloud model for complex requests. Privacy matters because commands and file names are sensitive.

Starting scope (file operations only): open/go to folder, copy, move, rename, create folder/file, delete (safely), find a file, compress/extract, show size or contents, undo.

---

## 6. Architecture

```
+-------------------- Terminal (bash / zsh / fish) --------------------+
|  Shell hook (preexec / bind Enter)                                    |
+----------------------------------+------------------------------------+
                                   | Unix socket
                     +-------------v--------------+
                     |     SafeShell Daemon        |
                     |                            |
   NL input -------> | 1. NL Translator           |
                     | 2. Parser (shlex / bashlex)|
                     | 3. Risk Classifier         |<---- rules + AI
                     | 4. Sandbox Simulator       |<---- OverlayFS + namespaces
                     | 5. Impact Report Builder   |
                     | 6. Explainer + Alternatives|<---- AI (verified)
                     | 7. Snapshot / Undo Manager |
                     | 8. Policy + Audit Log      |
                     +-------------+--------------+
                                   |
                  +----------------+------------------+
                  |                                   |
          Real filesystem                    ~/.safeshell/ (snapshots, logs, config)
```

### Module responsibilities

| Module | Job |
|---|---|
| Shell hook | Capture command, show result, allow/cancel |
| Parser | Break command into program, flags, paths, pipes, redirects, `sudo`, subshells |
| Classifier | Assign R0-R4 using rule table, then AI for unknowns |
| Simulator | Run in OverlayFS + user/mount/network namespaces, collect diff |
| Report builder | Turn the diff into counts, sizes, key paths |
| Explainer | Plain-English reason + safer alternative |
| Undo manager | Snapshots, undo plans, verified rollback |
| Policy | User/team rules (e.g. "never allow delete in `/etc`"), protected paths |
| Audit log | History of what was blocked/run, helps debugging and evaluation |

---

## 7. Suggested Tech Stack

| Area | Choice | Why |
|---|---|---|
| Language | **Rust (recommended for the core)**; Go is an acceptable alternative | No Python. Rust gives a single static binary, no garbage-collector pauses on the keypress path, safe low-level code for namespaces/seccomp, and good llama.cpp bindings. Go is faster to learn but its runtime is awkward with `unshare`/namespaces (threads and `setns`), so if the team knows Go better we can keep it for the CLI/daemon and use a tiny Rust or C helper for the sandbox |
| Shell hook | Small shell scripts (bash/zsh/fish) + a tiny native client binary | The hook must start in a few ms, so it is a thin client that talks to the daemon |
| Shell parsing | A Rust shell-grammar parser (e.g. `tree-sitter-bash`, or our own) | Proper handling of pipes, quotes, redirects, `$(...)` |
| Sandbox | `nix` / `libc` bindings for `unshare`, `mount` OverlayFS, user/PID/net namespaces; `libseccomp` (seccomp-notify); `landlock` crate; cgroups v2 | Standard unprivileged Linux features, no root |
| Snapshots | Btrfs/ZFS if present, else path-level copy-on-write store | Works everywhere |
| AI (local) | llama.cpp embedded through a Rust binding crate, GGUF models; optional Ollama-compatible server | Section 3.6 |
| AI (cloud) | HTTPS client (`reqwest`) behind the `AiBackend` trait | Section 3.6 |
| Config | TOML (`~/.config/safeshell/config.toml`) | Simple, human-editable |
| IPC | Unix domain socket | Low latency |
| Packaging | `cargo install` first, then `.deb` and an install script | Easy install for demo |
| Testing | `cargo test` + a scripted corpus of safe/dangerous commands run in a VM | Needed for a "testable core" |

---

## 8. How We Will Prove It Works (Evaluation)

The project has a *clear, testable core*, so we should measure it:

1. **Classification accuracy:** a labeled dataset of a few hundred commands (safe / risky / catastrophic). Report precision and recall. Priority is **zero false negatives on R3/R4**.
2. **Simulation fidelity:** run commands both in sandbox and for real in a VM, and compare diffs. Target: identical for filesystem effects.
3. **Undo correctness:** for each command, run it, then the undo plan, then check the tree hash equals the original.
4. **Latency:** R0 under ~20 ms, simulated commands within a few seconds for normal directories.
5. **NL translation:** percentage of natural-language requests converted into the correct command, and how often ambiguity was correctly flagged.
6. **Usability:** a small user test (classmates) on tasks with and without the tool.

---

## 9. Roadmap

| Phase | Goal | Deliverable |
|---|---|---|
| **1: Foundation** | Parser, rule-based R0-R4 classifier, CLI `ss check "<cmd>"` | Works standalone, test corpus |
| **2: Sandbox** | OverlayFS + namespaces simulator, diff report; then effect adapters (network, process, database, git, package manager) with evidence levels | `ss simulate "<cmd>"` shows real impact |
| **3: Shell integration** | Bash/zsh hook + daemon, block/allow flow | Always-on protection demo |
| **4: AI layer** | `AiBackend` trait, local (embedded llama.cpp) + cloud backends, model benchmark, explain, safer alternatives, verified undo | Plain-English reports, works offline |
| **5: Rollback** | Snapshots, `ss undo` | End-to-end transactional behaviour |
| **6: Natural language** | NL to command for file operations | `ss <sentence>` |
| **7: Polish and evaluate** | Metrics, installer, docs, demo video | Final report |
| **Future** | Voice control, more shells, networking/package/db commands, team policies, GUI | See below |

Minimum viable demo (for the minor-project review): phases 1 to 3 plus a basic version of 4 and 6.

---

## 10. Future Scope

- **Voice control:** speech-to-text (Whisper or similar) in front of the NL translator. Because NL already goes through the safety pipeline, voice adds convenience without adding risk. For spoken commands we should require an extra confirmation for R2 and above ("Say *confirm* to delete 20 files").
- **Wider coverage:** `git` (force-push, reset --hard), package managers, `docker`, `kubectl`, SQL, cloud CLIs.
- **Learning mode:** "you ran `cp -r` 5 times, here is a shortcut" and tutorials for beginners.
- **Team policies:** shared rules for labs, servers and company machines.
- **Remote/SSH awareness:** extra caution on production hosts.
- **GUI/notification companion** and editor-terminal (VS Code) integration.
- **Learning from the audit log** to improve classification.

---

## 11. Risks and Open Questions

| Risk | Mitigation |
|---|---|
| Simulation cannot capture network/process/system/database effects | Effect-coverage design in section 3.3: each effect is OBSERVED, PREVIEWED, INSPECTED or ESTIMATED; anything only ESTIMATED never auto-runs |
| Slowdown on every command | R0 fast path, daemon stays warm, cache results |
| Huge directories make simulation slow/expensive | Dry-run sizing first, sample or refuse simulation above a threshold and fall back to classification + snapshot |
| Non-deterministic commands (time, random, `curl`) give different results in the real run | Mark as non-deterministic, simulate with caution |
| AI hallucination | AI can only raise risk; explanations built from observed diff; undo verified in sandbox |
| User bypasses the hook | Document limits, log bypasses, strict mode later |
| Privacy of file names sent to a cloud model | Local-first, redact paths, opt-in cloud |
| Alert fatigue (too many prompts) | Only prompt at R2+, allow per-path trust and learn from choices |
| `sudo`/root commands | Treated as higher risk; sandbox with elevated rules carefully |

Open questions for the team:
1. ~~Python or Go/Rust?~~ **Decided: no Python. Rust recommended for the core, Go as fallback (see stack table).**
2. ~~API vs local model?~~ **Decided: support both** (`off | local | cloud | hybrid`). Open: which cloud provider for the demo and which local default model after benchmarking.
3. Bash only first, or bash + zsh together?
4. Do we target Ubuntu only for the prototype? (Recommended: yes.)

---

## 12. Suggested Team Split

| Area | Owner (to assign) |
|---|---|
| Shell hook + daemon + packaging | Lead |
| Parser + classifier + test corpus | Teammate A |
| OverlayFS simulator + snapshots/undo | Teammate B |
| AI layer + natural-language translator | Teammate A/B or shared |
| Evaluation, docs, demo | Everyone |

---

## 13. Why This Project Stands Out

- Mixes **operating systems** (namespaces, OverlayFS, snapshots, syscalls) with **applied AI** in a way that has real, measurable results.
- Does not just *warn*; it **shows observed proof** and **provides a verified way back**.
- Solves a real problem for beginners and experienced users alike, and the natural-language and voice direction makes the terminal accessible to everyone.
