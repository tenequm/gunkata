# gunkata kata spec (v1)

Normative. A workflow file is a **kata**: a declared choreography the engine executes
faithfully - nothing is built in, no roles, no prompts, no implicit steps. A run does
what the kata says. Files are named `<name>.kata.yml`.

## File

```yaml
name: <kata>

params:                        # everything invocation-specific; nothing else enters a run
  <key>: {}                    #   required: gunkata run w.kata.yml -p key=value
  <key>: {default: <value>}    #   optional

agents:                        # a profile fully describes one executor shape
  <profile>:
    harness: <id>              # required
    model: <id>                # required
    acp_adapter: <npm spec>    # optional: pins the harness's ACP adapter package
    append_system_prompt: |    # optional: text appended to the agent's system prompt
      ...
    timeout_seconds: <int>     # optional
    options: {}                # optional harness-specific settings
    skills: []                 # skill dirs: local path or GitHub tree URL
    mcps: []                   # MCP servers: <url> | {url: <url>, required: <bool>}

workflow:                      # map of jobs = the DAG
  <job>:
    needs: [<job>, ...]        # DAG edges; absent = root
    pre-steps:                 # deterministic setup: engine-run argv, no shell, no model
      - <step>
    agent: <profile> | {profile: <profile>, <amendments>}   # required iff prompt present
    prompt: |                  # the judgment; the only place a model acts
      ...
    outputs: [<name>, ...]     # files or directories owed, land in artifacts/<job>/;
                               #   message.md is the agent's final message
    post-steps:                # the done-bit; all exit 0 = verified
      - <step>
```

Four top-level keys. Seven job keys. Unknown fields are rejected.

## Agents

A profile is the executor contract factored out: every executor starts bare - engine-owned
HOME, nothing inherited but subscription auth - and every addition is declared.

- `harness:` - the acpx agent that runs the job, e.g. `claude`. `agy` is Google
  Antigravity: it runs the host's `agy-acp-server` on PATH, not acpx's built-in agent.
  `codex` starts in its adapter's full-access mode, as other harnesses run unsandboxed;
  `options: {mode: <mode>}` picks another.
- `acp_adapter:` - pins the ACP adapter package acpx runs for the harness, as one npm
  package spec, e.g. `@agentclientprotocol/claude-agent-acp@0.81.0`. Absent, `claude`
  runs the engine's default, `@agentclientprotocol/claude-agent-acp@0.81.0` - acpx's own
  range stops at 0.76.x, which predates the newest Claude models - and any other
  harness runs acpx's built-in adapter at the version range it pins. Set, the engine
  hands acpx `--agent "npx -y <spec>"` in place of the harness name; the harness's own
  bare setup, credentials and skills location still apply, and npx fetches through the
  engine's shared npm cache. The value is a package name with an optional `@<version>`
  (a version, tag or semver range) - no whitespace, quotes, paths or shell syntax - or
  it is a load error. Only harnesses acpx has a built-in adapter for take one (`claude`,
  `codex`); on a harness that runs its own agent command, such as `agy`, the engine
  refuses the run before it starts.
- `append_system_prompt:` - text appended to the agent's own system prompt, inline in
  the file. Placeholders expand in it as in `prompt`. acpx hands it to the adapter as
  claude-agent-acp's ACP `_meta.systemPrompt.append`, which only the `claude` harness
  honours - with or without a pinned `acp_adapter`; on any other harness the engine
  refuses the run before it starts.
- `skills:` - each entry is a skill directory: a local path, or a GitHub tree URL
  (`https://github.com/<o>/<r>/tree/<ref>/<path>`). The engine fetches it engine-side with
  ambient credentials, resolves the ref to a SHA at run start, snapshots it into the run
  dir (recorded with the SHA), and materializes it where the harness loads skills from.
- `mcps:` - each entry is an MCP server URL, or an object `{url: <url>, required: <bool>}`
  (no other keys). `${VAR}` placeholders are allowed and expand at executor spawn from the
  engine's environment - never written expanded anywhere. A server is optional by default:
  if a referenced variable is unset, the job runs without it, the engine prints a warning,
  and the job's record lists it under `skipped_mcps`. A `required: true` server with an
  unset variable makes the engine refuse to start the run. The engine fails any job whose
  transcript shows a server failed to load.
- `options:` - harness-specific key=value settings, passed through the adapter.

`agent:` on a job takes the profile name, or an object amending it: `profile:` names the
base; scalar fields (`model`, `timeout_seconds`, `acp_adapter`) replace, list fields
(`skills`, `mcps`) append, `options` keys win. `append_system_prompt` appends too: the
job's text follows the profile's, after a blank line. `harness` may not be amended - a
different harness is a different profile. The merge happens once at load, resolved onto
the job.

## Jobs

A job is setup, judgment, evidence:

1. `pre-steps` run first, in order. Any non-zero exit fails the job. Never retried:
   their failure is deterministic.
2. `prompt` runs the resolved profile's executor once, cwd = the job's own working
   directory.
3. `outputs` are the files or directories the job owes. Each gets an implicit check:
   exists and non-empty. `post-steps` add the semantic checks. All checks passing is the
   only thing that makes a job done; dependents release on nothing else. A job must
   declare at least one output or post-step.

The agent's final message - its text after its last tool call - is an output the engine
writes: when the executor ends, whatever its exit, it lands atomically in
`artifacts/<job>/message.md`, before the output checks, replacing anything the agent put
there. Every agent job gets one, empty or not. Declaring `outputs: [message.md]` is how a
job requires a non-empty final message, and how it and its dependents reference it
(`{{output:message.md}}`, `{{artifact:<job>/message.md}}`) - a reference to an undeclared
output is a load error, as for any other. A job without `prompt` declaring `message.md`
is a load error.

A job without `prompt` is a deterministic job: no agent, no model, just steps and
evidence.

Failure parks the job, and nothing that needs it ever runs. There is no retry in v1;
retry arrives only with transcript-based failure classification (principle 2).

## Steps

A step is an argv array, or a string parsed into one at load with quote-aware word
splitting (spaces separate, quotes group). The string form is sugar for the same argv,
never a shell: `|`, `>`, `<`, `&&`, `||`, `;`, `$`, backticks are a load error. Splitting
happens before placeholder expansion and expansion is per-element, so an expanded path
with spaces stays one argument. Anything needing a pipe is a script in the repo, invoked
as one argv.

## Placeholders

Three kinds, expanded in one pass, to absolute paths only:

- `{{param:key}}` - the bound value of a param
- `{{output:name}}` - the job's own output, under `artifacts/<job>/`
- `{{artifact:job/name}}` - an upstream job's output

No content splicing, no expressions, no second pass. Referencing an artifact does not
create an edge; only `needs:` does. An agent job's final message is the output
`message.md` (see Jobs), referenced like any other.

To hand a repository between jobs, make the repo itself the artifact: clone into
`{{output:repo}}`, and let the dependent clone from `{{artifact:<job>/repo}}` - a local,
credential-free operation.

## Run

`gunkata run w.kata.yml -p key=value ...`

1. Bind: every required param bound exactly once, no undeclared bindings. A param whose
   value is an existing file is snapshotted into the run dir, and the placeholder expands
   to the snapshot - the run dir is a complete, self-contained evidence record.
2. Validate: unresolved profile/need/param/artifact references, malformed steps, and
   cycles are rejected before anything runs.
3. Execute: roots start; each verified job releases its dependents.
4. Every job gets its own directories and its executor its own HOME; teardown kills the
   whole process tree.

The run dir holds `record.json` (the verdict), `gunkata.log` (the engine's log, JSON
lines; the same events go to stderr as text), and per job under `jobs/<job>/`:
`executor.jsonl` (acpx's ACP event stream, verbatim), `executor.log` (acpx's stderr) and
`steps.log` (pre- and post-step output). A job's working directory is `home/work`, inside
its executor's HOME: Claude Code searches the cwd's ancestors for skills up to HOME, so a
work dir outside it would reach the operator's real home. The log names MCP servers and measures the
prompt and the appended system prompt; it never holds expanded MCP URLs, environment
values or the text of either.

An executor sees its `home/tmp` as `/tmp`, so scratch an agent writes to a literal `/tmp`
path stays in the run dir. This is best effort: it needs Linux with unprivileged user
namespaces, and a run dir outside `/tmp`. Where either is missing, the run's executors
share the host's `/tmp`, `gunkata.log` warns once, and `record.json` states
`private_tmp: false` with a `private_tmp_reason`; otherwise it states `private_tmp: true`.
A run with no executor job states neither.

## Principles

Held here in quarantine until each proves its keep:

1. **Completion is verified, never claimed.** A finished executor is not a finished job -
   an agent that did nothing terminates exactly like one that did the work. Only the
   declared checks passing completes a job; "the step ran" is not an outcome.
2. **No retry without classification.** Quota, timeout, refusal and real defect want
   opposite responses, so blind retry turns one failure into an expensive one. An
   unclassified failure parks: visible, terminal, never released past.
3. **A run ends when its process tree is dead.** Teardown is part of the run contract;
   the engine owns the full tree via process groups.

## Deliberately absent

No templating, includes, extends, or expression language - repetitive katas are generated
by a real program; the file stays dumb. No shell in any declared command. No secrets in
the file - executor auth inherits from the environment, MCP keys ride `${VAR}`. Matrix,
workspace provisioning, retry, allow_failure, and concurrency knobs stay out until a real
kata demands them.
