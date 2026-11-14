# Results — 2026-11-13

## Versions

- Fabro nightly: `0.415.0-nightly.0`, commit `a60f509`, image digest `sha256:5440f204b70da0a4165b34d8591fb9461317bd98fa138ae61ec9095052d599c3`.
- Comparison release: `0.254.0` (separate storage, no downgrade of nightly data).
- ACP adapter: `@agentclientprotocol/codex-acp@2.6.0`.
- Bundled Codex: `codex-cli 0.166.0` (observed dependency version; the adapter declares a compatible version range).
- Codex probe reported model `gpt-6.1-sol`; authentication was the existing ChatGPT login copied from the Sortie pod.

## Verified

1. Podman runs the Fabro server/UI and the ACP adapter without mounting host auth or a container-engine socket.
2. The control workflow runs three real Python tests, sees their nonzero result, routes to repair, tests again, and waits at a human gate. Corrected run `01M7AHPV43AH4QPCM3NK9R2QF4` has exactly one repair and two test stages, first failed then succeeded. This run is left waiting for user interaction.
3. Earlier control run `01M7AHE3PRZ5VHTF2GQ33ED20P` retained its pending question across a server restart. A test API answer advanced the workflow to success. This was a simulated approval of a disposable fixture, not a production approval or merge.
4. Standalone ACP initialization, session creation, prompt execution and completion work with the real Codex App Server and ChatGPT authentication.
5. In the second standalone test, Codex read `implement`, `tdd`, and `code-review`; it reported separate standards/spec reviews and committed `48d830b` in `/storage/workspaces/acp-skills`. Independent verification confirmed all three tests pass and only `calculator.py` changed. The test file is unchanged. Protocol trace: `.local/data/acp-skills-events.jsonl`.
6. After rebuilding and replacing the container, the final checked-in `scripts/acp-probe.py` also passed: commit `cf303fb`, three passing tests, unchanged test fixture, skills read, and standards/spec reviews reported. Evidence: `.local/data/evidence/acp-1794576856732370049/`. The pending control gate and Codex login survived container replacement. The UI returned HTTP 200; no interactive browser-terminal claim was tested.

## Native ACP blocker

Nightly rejects the ACP-only graph before execution:

```text
fabro.model.no_ready_provider: no default model is available:
no LLM provider is ready, and the workflow has a node that runs a model
```

Providing `--model gpt-5.4 --provider openai` does not help. The explicit selection was an admission experiment; no model call was made.

Release 0.254.0 accepted run `01M7AHJRM1TS7V4CH3HH80CM1V` but immediately failed:

```text
Precondition failed: No LLM providers configured.
Set ANTHROPIC_API_KEY or OPENAI_API_KEY, or pass --dry-run to simulate.
```

We used neither API keys nor dry-run to claim a successful agent execution. Fabro's documentation says ACP owns its auth; actual execution currently requires additional native-provider readiness in the tested configuration. The nightly source checks `!has_ready_provider && admitted.needs_model()` in [petri_check.rs](https://github.com/fabro-sh/fabro/blob/719fa32cf3a759b1bdf323722f6421caae77e988/lib/apps/fabro-server/src/petri_check.rs). This is a source-level lead, not proof that changing this one condition fixes the complete execution path.

## Setup findings and corrections

- A conditional routing node must have an unconditional fallback edge. The validator rejects graphs where all outgoing edges are conditional.
- The image's original passwd home was `/var/fabro`, while our env pointed to `/storage/home`. Worker shells used the passwd home. Aligning these and restarting the server fixed missing Git configuration. A narrowly scoped `safe.directory` for the fixture addresses macOS shared-filesystem ownership differences.
- Skill build directories initially inherited restrictive permissions. The image now grants read/traverse access to bundled definitions. First ACP run fixed the code but could not read skills; only the second run counts as skill-integration evidence.
- Rapid same-size Python edits can leave timestamp-based bytecode stale. Command test nodes now use `python3 -B` from a fresh clone; the final control run needs only one repair.
- The persistent server state is separate from the readonly scripts, fixtures and workflow definitions.

## Assessment

The executable workflow, deterministic failure routing and durable human checkpoint are substantiated. Codex ACP with subscription auth and the selected skills is substantiated, including native Fabro execution after the user configured OpenRouter (see follow-up below). ACP-only launch without native-provider readiness remains blocked. No real GitHub tasks or PRs were created, and no merge automation was enabled. Kubernetes/OpenShell/per-step Pods remain untested.

## Follow-up: OpenRouter configured by the user

Run `01M7AJGHBRDW5NW6ZBT004P66D` reached the human gate on 2026-11-13 using the unchanged `workflows/codex.fabro`:

- `implement@1`: succeeded via native Fabro ACP handling. Process inspection confirmed `codex-acp` and `codex app-server`; the run event stream contains ACP tool and message events. Codex reported reading the three skills, reviewing and committing the one-operator fix as `880f76c`.
- `tests@1`: Fabro's separate command node ran three unittest cases; all passed. Independent comparison confirmed that `test_calculator.py` is byte-for-byte unchanged from the fixture base.
- `approve@1`: waits for Approve/Revise, status `blocked / human_input_required`. Left pending for the user; no merge or publication occurs.

Evidence snapshots: `.local/evidence/openrouter-acp-events.jsonl` and `.local/evidence/openrouter-acp-state.json`. The OpenRouter credential was not displayed or copied. The implementation demonstrably used ACP, not a native OpenRouter agent. This test did not independently inspect OpenRouter billing or test Fabro's automatic PR text generation.

## Upstream issue investigation

Checked on 2026-11-13: [issue #949 — ACP-only workflows should launch without native-provider credentials](https://github.com/fabro-sh/fabro/issues/949) is open, created by contributor Scott Werner (`swerner`) on October 9, with no comments at inspection time. Its reproduction matches ours: a minimal ACP workflow is blocked by `fabro.model.no_ready_provider`. It reports that an unreachable native-provider endpoint was enough for that earlier minimal reproduction to proceed without making a native call. We did not reproduce or adopt that workaround.

The issue explicitly expects workflows needing only ACP to launch without native credentials. It requests product agreement on native-dependent operations, particularly automatic PR title/body generation. This supports classifying our blocker as a known overly broad prerequisite, not an inherent ACP authentication requirement. It does not establish that a one-line fix is sufficient or approved.

Current inspected Fabro code computes `has_ready_provider` from the native ready-provider list in `run_manifest.rs` and combines it with `admitted.needs_model()` in `petri_check.rs`. The guard contains no explicit ACP exception. Separately, `build_pr_content` in `lib/components/fabro-workflow/src/pull_request.rs` builds a native LLM client and invokes `complete_object` for the PR text; that is a real native-provider dependency, unlike deterministic GitHub PR creation with precomputed title/body. The upstream issue also distinguishes native/API agent stages and standalone prompt stages from ACP stages.

No fixing PR was found by searching ACP PRs, issue #949 references, and its cross-reference timeline. This is a bounded search result, not proof that no unlinked work exists. No issue/comment was posted and no upstream code was modified.

## Real-repository checkout fix (2026-11-13)

Issue run `01M7AS0E0ZDWGXJPZZ2AM4Y2AR` failed before any agent step: `checkout: could not extract the checkout: tar said tar: unknown typeflag: 0x53`. The image supplied BusyBox tar; 0x53 is a GNU sparse entry. Added GNU tar to the image and recreated the idle container with the existing persistent data and bot PAT mount.

A command-only Fabro run against the same saved pi5-eeprom-probe source checkout (`01M7AS4ER4ZSFM886KVS6YNBN3`) succeeded through start, verification and exit. It checked for Makefile and ran `git diff --exit-code HEAD`, confirming checkout extraction with no tracked-content changes. No agent, GitHub push, PR creation or real issue retry was started.

The rebuild also encountered visualize skills in two cached marketplaces. The image bundler now adds the marketplace prefix on a skill-name collision, preserving both definitions. The rebuilt image contains 172 bundled skills.

## End-to-end issue #8 (2026-11-13)

Started `geoberle/pi5-eeprom-probe#8` at the user's request. First run `01M7AT8K7CXKDHSDBG7BH09AG1` implemented a bounded contribution, but the fixed verification step failed because the container lacked `lz4` (the repository's CI installs it explicitly). The repair stage correctly reported `needs_human`, and publication was blocked. Added lz4 to the Dockerfile and running container. Independent verification of that attempt then passed. Also fixed our clean-worktree check to tolerate only untracked `.cache/` build artifacts while still rejecting tracked changes and unrelated untracked files; covered by a regression test (16 tests pass).

Resume of the already terminal run did not re-execute its failed result gate. Rewind was rejected because this Local run has no GitHub-backed checkpoint pushes. Preserved the first attempt and its original needs_human report; started a fresh regular issue run after the setup fixes.

Second run `01M7ATV7XKXRZAXHWF2F104HQE` succeeded through context, implement, result, verify, pr, publish and exit without a repair pass. Published [PR #43](https://github.com/geoberle/pi5-eeprom-probe/pull/43), authored by `goberlec-ghbot`, HEAD `a2420ed71f4c2e16e699eccc7f9465a82f36eb24`. The fixed `make verify` passed formatting, module checks, lint, race-enabled tests, Go/C builds and 381,612 differential cases. The PR validates network-fault parameters and adds public-command tests plus documentation. It explicitly uses `Part of #8`; the issue remains open. Real firmware/board/QEMU scenarios are not claimed as tested.

At the final GitHub check, the PR was OPEN, autoMergeRequest was null, and GitHub CI was IN_PROGRESS. No merge was performed. The end-to-end result is successful after operator fixes to the experiment, not a claim that the original setup worked unattended on the first try. New image `412694bd84b9` contains lz4; the running container received the same package without interrupting the run.

## Markdown context and UI artifacts (2026-11-13)

Issue intake and the context stage now write context.md; all consuming ACP prompts read Markdown. The context stage prints the full document and writes a workspace copy .fabro-context.md for native run.artifacts capture. Git's local info/exclude keeps that copy out of commits. Combining an exact checkpoint exclude glob with an ignored file caused Fabro's git add to fail in the first smoke test; using Git ignore alone passed.

17 tests pass, including a Markdown golden fixture and a real Git test that confirms stdout, the artifact and the agent's document match and remain untracked after git add. Command-only server runs verified stage output and artifact capture. Run 01M7AXATS6RCT4VQMZ6MTYEZSN succeeded using the saved real issue #8 snapshot: 1,077 bytes in both context output and the captured artifact. No agent, GitHub write or new implementation ran. The current Artifacts UI provides downloads, not inline Markdown rendering; read the text directly in the context stage's Output. Existing runs are not rewritten.

## Go orchestration (2026-11-13)

Implemented cmd/orchestrator with Cobra raw → validated → completed options, initialized GitHub/Fabro clients, logr logging, a pure selection function and a polling executor. Workflow packaging remains Fabro CLI's responsibility. The Python starter was removed; its deterministic workflow steps remain. Orchestration state is reconstructed from GitHub and Fabro, not local claims. A temporary process lock prevents competing orchestrators. Workspace input files still exist for workflow execution.

Selection uses both readiness labels, the wayfinder exclusion, native blockers, associated PRs and existing runs. One active run globally; oldest eligible issue first. Submitted runs are adopted. Failed latest runs receive a fixed bot comment and require a rocket reaction by geoberle's resolved GitHub user ID. Comments from other authors, reactions by other users and reactions to old runs cannot authorize the next attempt. The accepted create/list visibility race remains deliberately unprotected. HTTP errors and incomplete run lists do not authorize work.

Validation: make verify passed (Go race tests, golangci-lint with zero findings, Go/Python/shell format checks, Ruff, ShellCheck, YAML and 16 remaining Python tests). Go HTTP tests exercise preview, create, recovery after an accepted create with lost response, submitted resume, failure comments, retry authorization, withdrawn readiness, incomplete pagination and safe errors. Failure comment text is checked against a golden fixture. Image built with Go 1.25.5; the running container was updated with its persistent data preserved.

Read-only live preview against geoberle/pi5-eeprom-probe succeeded: issue #8 selected first; #15 requires retry, several others are dependency-blocked or lack readiness. No new GitHub comment, PR or implementation run was created. The new controller's mutation paths were tested with local HTTP servers, not live GitHub writes. The simplified ACP PR step still needs a fresh end-to-end issue run. Issues write permission is now required for failure comments. Polling is not auto-started by the server startup script.
