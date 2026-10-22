# Isolated PyYAML for Python agent utility validation

Status: current
Last verified: 2026-10-21
Verified at project commit: `f2e19d950703697e1aaa5c47fb6a80343908ae22` (working tree also contains ongoing work).

## Finding and when it matters

On this machine, `python3` resolves through `/opt/homebrew/bin/python3` to Homebrew Python 3.14, outside a virtual environment. Running Codex's generic skill validator with plain `python3` failed with `ModuleNotFoundError: No module named 'yaml'`. This is a utility dependency issue, not an intercom runtime requirement or a skill-validation finding.

`uv` is available at `/Users/booty/.local/bin/uv`. It can supply PyYAML in an isolated environment without changing Homebrew Python. Use `uv run --no-project --with pyyaml python SCRIPT [ARGS...]`; `--no-project` avoids loading an unrelated Python project's dependencies. Plain `python3` remains unchanged.

## Verified evidence

`command -v python3`, `python3 -m pip --version`, and inspection of `sys.executable`/`sys.prefix` confirmed the interpreter and absence of an active virtual environment. This command installed one isolated dependency and completed with `Skill is valid!` and exit status 0:

```sh
uv run --no-project --with pyyaml python \
  ~/.codex/skills/.system/skill-creator/scripts/quick_validate.py \
  ~/code/jbootz-llm-skills/skills/jbootz-claude-from-codex-review
```

For a persistent environment, the alternative setup commands are in [README: Environment Setup](../../README.md#environment-setup). That alternative was documented but not executed here. The skills repository's own Ruby validator also passed and does not need PyYAML.

Reference: [uv script dependencies](https://docs.astral.sh/uv/guides/scripts/#running-a-script-with-dependencies). Recheck executable paths if the machine's Python/tool setup changes.
