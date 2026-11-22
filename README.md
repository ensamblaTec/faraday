# Faraday

Faraday runs a program inside a **Faraday cage**: a network namespace with no
network interface other than (optionally) `lo`. Whatever you launch under
`faraday run` cannot reach the internet, your LAN, or any other host — your
data stays in, and nothing goes out. Everything else about the host (the
filesystem, IPC, the process tree) is left alone, so GUI apps with a Wayland
window and PipeWire audio keep working normally.

It's a lightweight, no-root alternative to reaching for `firejail` or a full
container just to take network access away from one program.

## Install

```sh
make install
```

This builds `faraday` (`CGO_ENABLED=0`) and installs it to `~/.local/bin`.
Make sure that directory is on your `PATH`. `make build` builds the binary
into the repo root without installing it, and `make test` runs the test
suite.

## Usage

```
faraday run [--loopback] -- <cmd> [args...]   run a command with no network
faraday run [--loopback] <profile>            run a saved profile
faraday list                                  list saved profiles
faraday save <name> [--loopback] (--flatpak <appid> | -- <cmd>...)
faraday version | help
```

### Run an ad-hoc command

```sh
faraday run -- curl -sS https://example.com   # fails: no network
faraday run --loopback -- ip -o link show     # only lo, and it's UP
```

### Save and run a profile

Profiles are YAML files under `~/.config/faraday/profiles/<name>.yaml`
(`$XDG_CONFIG_HOME/faraday/profiles` if `XDG_CONFIG_HOME` is set). You can
create one with `faraday save`, or drop a file in by hand.

```sh
faraday save mytool --loopback -- /usr/bin/mytool --some-flag
faraday list
faraday run mytool
```

### Spotify example (flatpak backend)

A ready-made profile ships in [`profiles/spotify.yaml`](profiles/spotify.yaml):

```yaml
description: Spotify sin red (solo música descargada)
flatpak: com.spotify.Client
loopback: false
```

Install it and run Spotify with no network access (only previously-downloaded
tracks will play):

```sh
faraday save spotify --flatpak com.spotify.Client   # or: cp profiles/spotify.yaml ~/.config/faraday/profiles/
faraday run spotify                                  # plays downloaded music, no network
```

> **Heads-up:** Spotify is single-instance. If a normal (networked) Spotify is
> already open, a second launch would hand off to that instance instead of a
> caged one, so `faraday run spotify` **refuses** when it detects a running
> instance — close it first with `flatpak kill com.spotify.Client`, or pass
> `--force` to launch anyway. See [Limitations](#limitations).

## How it works

`faraday run` picks one of three backends, in this order, depending on what's
available on the host and what the spec asks for:

1. **`netns` (default, pure Go, no root, no external binaries).** Faraday
   re-execs itself into a new, unprivileged user namespace + network
   namespace (`CLONE_NEWUSER | CLONE_NEWNET`) with an identity uid/gid
   mapping, so the process keeps its normal user identity and X11/
   Wayland/PipeWire socket credentials still work. Inside that namespace
   there are no network devices except `lo`, which starts `DOWN`. The child
   then execs the target command, replacing the faraday process. This is the
   backend used for `faraday run -- <cmd>` and for profiles with a `cmd:`
   list.
2. **`bwrap` (fallback).** If unprivileged user namespaces aren't usable on
   the host (see Limitations), Faraday shells out to `bwrap --unshare-net`
   instead. Same effect, different mechanism, requires `bubblewrap` to be
   installed.
3. **`flatpak` (for flatpak apps).** When a profile sets `flatpak: <app-id>`,
   Faraday runs `flatpak run --unshare=network <app-id>`, which asks the
   Flatpak sandbox itself to drop network access for that app. That isolation
   belongs to the Flatpak instance, not to Faraday, so it holds even after
   `faraday run` returns. Faraday deliberately does **not** pass
   `--die-with-parent` here: single-instance apps (e.g. Spotify) hand a launch
   off and let `flatpak run` return immediately, so `--die-with-parent` would
   reap the app the moment Faraday exits. The app therefore runs detached;
   close it the normal way (or `flatpak kill <app-id>`). Used for apps like
   Spotify that are installed and normally launched as flatpaks.

`faraday list` and `faraday save` only read/write profile YAML files; they
don't touch any namespace.

### `--loopback`

By default `lo` is present but `DOWN` inside the sandbox, so even
loopback connections fail. `--loopback` (or `loopback: true` in a profile)
brings the namespace-internal `lo` up before exec'ing your command.

**This is not the host's localhost.** It's a brand-new loopback interface
that exists only inside the new network namespace created for this run. A
server listening on `127.0.0.1` on your real desktop is not reachable from
inside the sandbox, and a server started inside the sandbox on `127.0.0.1`
is not reachable from outside it — `--loopback` only lets a sandboxed
process talk to *itself* (or to another process started in the same
sandbox invocation) over loopback.

Bringing `lo` up briefly requires `CAP_NET_ADMIN`, which the netns backend
acquires only for that one syscall via an ambient capability granted at
re-exec; it is dropped (effective/permitted/inheritable/ambient sets all
cleared) before the sandboxed target is exec'd, so the program you actually
launch never sees that capability — or any other.

## Limitations

- **Filesystem, IPC, and the PID namespace are intentionally not isolated.**
  Faraday only isolates the network. The sandboxed process sees the same
  filesystem, the same `/tmp`, the same X11/Wayland and PipeWire/D-Bus unix
  sockets, and the same process tree as everything else on your desktop.
  This is by design: it's what lets a GUI app open a window and play audio
  without any extra wiring. If you need filesystem or IPC isolation too,
  reach for `firejail`, `bwrap` directly, or a container instead.
- **Unprivileged user namespaces may be restricted.** Some distributions
  (notably Ubuntu, via an AppArmor profile) restrict unprivileged use of
  `CLONE_NEWUSER` for ordinary binaries. When that's the case, the `netns`
  backend is unavailable and Faraday automatically falls back to `bwrap`
  (which must be installed separately, e.g. `apt install bubblewrap`). If
  neither is usable, `faraday run` fails with an explicit error rather than
  silently running unsandboxed.
- **Flatpak network isolation is enforced by Flatpak, not Faraday.** For
  `flatpak:` profiles, Faraday simply asks `flatpak run` to unshare the
  network namespace for that app; it trusts the Flatpak sandbox to honor
  `--unshare=network`.
- **Single-instance flatpak apps can hand off to an already-running copy.**
  Many flatpak GUI apps (Spotify and other Chromium/Electron-based apps) allow
  only one instance: launching the app again hands the request to the copy
  that's already running and exits. Because Faraday launches the app through
  `flatpak run`, `faraday run <app>` inherits that rule. If a normal,
  **networked** instance is already open, the caged launch would be absorbed by
  that online instance — a success exit would then *not* prove the app is
  caged. To avoid that false sense of isolation, the `flatpak` backend checks
  `flatpak ps` first and **refuses to launch when an instance is already
  running**, telling you to close it (`flatpak kill <appid>`) or re-run with
  `--force`. This guard affects only the `flatpak` backend; the `netns`/`bwrap`
  backends always start a fresh process. After launch you can still confirm
  isolation as shown under Verification.

## Verification

These were run against this repository to confirm the no-network promise
holds in practice (see `internal/sandbox/ns_linux_test.go` for the
automated versions):

```sh
$ ./faraday run -- ip -o link show
1: lo: <LOOPBACK> mtu 65536 qdisc noop state DOWN mode DEFAULT group default qlen 1000

$ ./faraday run -- curl -sS --max-time 5 https://example.com; echo exit=$?
curl: (7) Failed to connect to example.com port 443 after 1 ms: Couldn't connect to server
exit=7

$ ./faraday run --loopback -- ip -o link show
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 ... state UNKNOWN ...

$ ./faraday run --loopback -- sh -c 'grep -E "^Cap(Eff|Prm|Amb)" /proc/self/status'
CapEff:	0000000000000000
CapPrm:	0000000000000000
CapAmb:	0000000000000000
```

Only `lo` ever exists inside the sandbox, it's down unless `--loopback` is
passed, network syscalls fail, and the sandboxed program never inherits the
`CAP_NET_ADMIN` capability that the netns backend itself uses internally to
bring `lo` up.

For a **flatpak** app, confirm the running instance is actually caged by
comparing its network namespace and interfaces against the host's (no root
needed — this reads the app's own `/proc/<pid>/net/*`):

```sh
$ pid=$(pgrep -x spotify | head -1)
$ readlink /proc/$pid/ns/net            # differs from the host → isolated netns
net:[4026534516]
$ readlink /proc/self/ns/net
net:[4026531833]
$ awk 'NR>2{print $1}' /proc/$pid/net/dev   # only lo, no eth/wlan
lo:
$ ss -tnp state established | grep -c spotify   # 0 outbound connections
0
```

A normal (un-caged) Spotify shows a dozen-plus established connections to
Spotify/CDN hosts here; under Faraday it shows none.

## Contributors

- **ensamblaTec** — Author & Maintainer
- **Google Antigravity / Gemini** — AI-assisted development

See [CONTRIBUTORS.md](CONTRIBUTORS.md) for full contributor details.
