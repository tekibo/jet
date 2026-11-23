# Agents

jet is built so coding agents (Claude Code, Codex, agents in T3 Code and similar) can use it from
an app's checkout: scaffold, build, test, run and sign.

## Install

```sh
jet agents install
```

| What | Where |
|---|---|
| `jet` | link in `~/.local/bin` |
| `android-jet` skill | links in `~/.claude/skills` and `~/.agents/skills` |
| The Jet instructions | between the `<!-- jet:agents begin -->` / `<!-- jet:agents end -->` markers in `~/.claude/CLAUDE.md` and `~/.codex/AGENTS.md` (each backed up as `.bak-jet` before it changes) |

`jet agents uninstall` removes the links (it leaves the instructions).

## What an agent does with it

From an app's checkout (a git root):

```sh
jet doctor        # the environment: JDK, SDK, tools, licenses. Prints the next step.
jet init MyApp    # scaffold
jet build --json  # compile; diagnostics as JSON (file, line, column, severity, message)
jet test --json   # unit tests; failure locations as JSON
jet run           # build, install, launch, stream logcat
jet devices       # connected devices and emulators
```

The [android-jet skill](../skills/android-jet/SKILL.md) is the rulebook for agents: diagnose
build errors with `jet build --json` (never raw `./gradlew` output), verify with
`jet test --json`, and run `jet doctor` first in a new checkout.
