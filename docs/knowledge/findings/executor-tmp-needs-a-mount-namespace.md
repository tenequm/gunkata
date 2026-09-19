---
type: Finding
title: TMPDIR does not keep an executor out of /tmp - only a mount namespace does
description: Agents hardcode /tmp regardless of TMPDIR, so an executor's scratch leaks into the shared host /tmp; an unprivileged user+mount namespace that bind-mounts the job's HOME/tmp over /tmp confines it with stdlib Go, keeps the pid and process group, and needs the host to allow unprivileged user namespaces.
tags: [executor, isolation, linux, namespaces, tmp]
status: stable
stale_after: "2027-03-31T00:00:00Z"
generated: { by: claude-code/opus-5, at: "2026-09-19T00:20:00Z" }
sources:
  - id: leak
    resource: "gunkata review run on ws-pond-01, 2026-09-18: a claude executor ran gh pr checkout into /tmp/glim-sh-cuttle-pr-73 with TMPDIR=<job home>/tmp set"
    title: The observed leak
  - id: live
    resource: "Live gunkata run on ws-pond-01 (NixOS, kernel 6.18, uid 10003), 2026-09-19: claude-haiku-4-5 via acpx 0.17.0, prompt writing /tmp/gk-probe-$RANDOM.txt and calling gh api"
    title: Private /tmp live check
  - id: userns
    resource: https://man7.org/linux/man-pages/man7/user_namespaces.7.html
    title: user_namespaces(7) - capabilities across execve, setgroups, unmapped ids
  - id: ubuntu
    resource: https://ubuntu.com/blog/ubuntu-23-10-restricted-unprivileged-user-namespaces
    title: Ubuntu restricted unprivileged user namespaces (default on in 24.04)
  - id: code
    resource: /packages/gunkata/internal/engine/privtmp_linux.go
    title: The engine's private /tmp helper
---

# Finding

Setting `TMPDIR` to the job's `HOME/tmp` is not enough: an agent writes literal `/tmp`
paths, as a Claude executor did with `gh pr checkout` into `/tmp/glim-sh-cuttle-pr-73` -
outside the run's evidence and shared between runs.[^leak] Claude Code itself also puts
`claude-<uid>` and `cc-socks-<uid>` under `/tmp`.[^live]

What works, with Go's stdlib only: start the executor through the engine's own binary
(`gunkata __private-tmp <home>/tmp <acpx> <args...>`) with `CLONE_NEWUSER|CLONE_NEWNS`,
the caller's uid and gid mapped to themselves, and `CAP_SYS_ADMIN` as an ambient
capability. The helper bind-mounts `<home>/tmp` onto `/tmp`, clears its ambient
capabilities on the exec thread, and `execve`s acpx in place.[^code] Ambient is the key:
a non-root uid loses every capability across `execve` otherwise, so the helper could not
mount; mapping root instead would run the agent as uid 0 inside.[^userns]

Live, on this host, the probe file and Claude's own temp dirs landed in the job's
`HOME/tmp`, nothing reached the host `/tmp`, `gh api` through the OneCLI gateway still
worked, and the run left no process behind.[^live]

## What the namespace does and does not change

- **Unchanged:** every path but `/tmp` - the real home's credential links, the shared npm
  cache, `/nix/store`, PATH, `~/.onecli` - and the network (no net namespace). The pid is
  the one the engine started and there is no PID namespace, so killing the process group
  from the host still takes the whole tree; the helper's uid equals the host uid, so the
  signal is permitted. Files are created with the real uid and gid.
- **Changed:** supplementary groups show as `nogroup` (access checks still use them);
  host files owned by other users show as `nobody`; setuid binaries such as `sudo` do not
  elevate; host `/tmp` sockets (X11, tmux, a `/tmp` ssh-agent) are unreachable. `/var/tmp`
  and `/dev/shm` stay shared.
- **Constraint:** anything under `/tmp` the executor must reach disappears - the run dir
  most of all, so the engine refuses a runs root under `/tmp` at preflight; an acpx
  binary under `/tmp` would fail to exec. Tests move `TMPDIR` off `/tmp` for this reason.

## Hosts that refuse

The engine probes at preflight, once per run with any executor job, and refuses the run
rather than fall back to a shared `/tmp`. Ubuntu 24.04 - including GitHub's
`ubuntu-latest` runners - restricts unprivileged user namespaces through AppArmor
(`kernel.apparmor_restrict_unprivileged_userns=1`), which is reported to break exactly
this unprivileged mount (not verified here); the
remedy is that sysctl set to 0 or a per-binary AppArmor profile.[^ubuntu] On this NixOS
devbox the namespace is allowed (`max_user_namespaces` 257090, no AppArmor restriction);
`bwrap` is not installed, and `unshare(1)` alone cannot bind-mount without a shell and
`mount(8)` inside. Non-Linux hosts refuse executor jobs outright.
