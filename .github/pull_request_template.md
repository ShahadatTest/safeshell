## What and why
<!-- One or two sentences. Link the issue if there is one. -->

## How I checked it
- [ ] `make check` passes (from `safeshell/`)
- [ ] `make integration` passes (if sandbox, hook, daemon or snapshots changed)
- [ ] Classifier change? Dev corpus updated, held-out corpus untouched, accuracy floors still pass
- [ ] AI change? `safeshell ai bench` numbers noted below
- [ ] No secrets, keys or personal paths in the diff
- [ ] Docs updated (README / doc/architecture.md / `safeshell help`) if behaviour changed

## Notes for the reviewer
<!-- Risky parts, things you chose not to do. -->

<!-- Release PR? Branch must be named release/vX.Y.Z and target main. Merging it publishes the release. -->
