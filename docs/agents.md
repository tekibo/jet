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
jet init MyApp    # scaffold (:app + :core-ui theme & safe sheet/input primitives)
jet build --json  # compile (debug by default); full structured diagnostics + failed_tasks + raw_output_tail
jet test --json   # multi-module unit tests; failure locations & build diagnostics as JSON
jet install       # install APK onto device (auto-picks or lists multiple; generates v4 .idsig for debug APKs)
jet run           # build, install, detect launcher activity, launch, stream logcat (or --json)
jet devices       # connected devices and emulators
```

In `--json` mode, failed commands exit non-zero (`1`) after printing valid JSON to `stdout`. Missing `gradlew` wrappers and standard local `debug.keystore` files configured by debug builds are restored automatically (reusing `~/.android/debug.keystore` when available so re-clones keep the same debug signing identity).

The [android-jet skill](../skills/android-jet/SKILL.md) is the rulebook for agents: diagnose
build errors with `jet build --json` (never raw `./gradlew` output), verify with
`jet test --json`, and run `jet doctor` first in a new checkout.
