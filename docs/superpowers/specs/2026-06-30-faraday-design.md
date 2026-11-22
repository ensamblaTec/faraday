# Faraday — Design Spec

> *"Run apps inside a Faraday cage — zero network, your data stays in."*

**Fecha:** 2026-06-30 · **Autor:** ensamblaTec · **Estado:** aprobado (brainstorming)

## Problema

Ejecutar una aplicación **sin acceso a red, bloqueada totalmente**, para que no pueda
enviar datos al exterior — al estilo *firejail* pero hecho en Go. Caso de uso guía:
Spotify reproduciendo solo música ya descargada, sin telemetría ni conexiones salientes.

## Contexto / hechos verificados (Linux host, kernel 6.18, Go 1.25+)

- **User + network namespaces sin privilegios FUNCIONAN** (`unshare --user --net` pasa) →
  se puede bloquear toda la red **sin root**.
- `firejail` no instalado; sí `bwrap` (bubblewrap 0.11.0), `unshare`, `ip`, `flatpak`.
- **Spotify es Flatpak** (`com.spotify.Client`, user install).
- GUI/audio usan *sockets unix* (Wayland `wayland-1`, PipeWire `pipewire-0`, D-Bus `bus`
  en `$XDG_RUNTIME_DIR`), no red → siguen funcionando dentro de un netns aislado.

## Objetivo y no-objetivos

**Objetivo:** aislar la **red** de un proceso (bloqueo total, o `lo` interno opcional),
sin root, con perfiles por app.

**No-objetivos (MVP):** aislamiento de filesystem / IPC / PID (rompería audio/GUI),
red filtrada por allow-list (slirp4netns no instalado), TUI/GUI propia.

## Diseño

### Núcleo: interfaz `Runner` + selector de backend

```
sandbox.Runner: Run(ctx, Spec) error ; Available() bool
  ├─ nsRunner     (Go puro: clone CLONE_NEWUSER|CLONE_NEWNET + re-exec child)
  ├─ bwrapRunner  (bwrap --unshare-net ...)
  └─ flatpakRunner(flatpak run --unshare=network <appid>)

Select(spec):
  if spec.Flatpak           → flatpakRunner
  else if nsRunner.Available()    → nsRunner       // probe userns una vez, cacheado
  else if bwrapRunner.Available() → bwrapRunner
  else → error con guía (habilitar userns / instalar bwrap)
```

`Spec` (merge de CLI + perfil): `Cmd []string`, `Loopback bool`, `Flatpak bool`,
`FlatpakAppID string`, `Env []string` (default `os.Environ()`).

### Backend Go puro (nsRunner) — mecanismo central

Padre re-ejecuta `/proc/self/exe __child` con:

```go
SysProcAttr{
  Cloneflags: CLONE_NEWUSER | CLONE_NEWNET,
  UidMappings: {{ContainerID: getuid, HostID: getuid, Size: 1}},   // identidad
  GidMappings: {{ContainerID: getgid, HostID: getgid, Size: 1}},
  GidMappingsEnableSetgroups: false,
}
```

- Mapeo **identidad** uid→mismo uid: cookies X11/Wayland y permisos de `XDG_RUNTIME_DIR`
  (modo 700, dueño 1000) siguen válidos. Go escribe `uid_map`/`gid_map` solo.
- **Child** (detecta env `FARADAY_CHILD`): ya en el netns nuevo (solo `lo`, DOWN). Si
  `Loopback`, sube `lo` con netlink; luego `unix.Exec(path, argv, env)` → se reemplaza por
  la app. Resultado: ve solo `lo` y cero rutas → sin internet, sin root.

### Backend bwrap (fallback)

`bwrap --unshare-net --die-with-parent --dev-bind / / --proc /proc --tmpfs /tmp -- <cmd>`
(refleja el FS del host, solo aísla red). Flags finos en implementación.

### Backend flatpak

Detección: primer token `flatpak`, o perfil con `flatpak: <appid>`, o `flatpak info <appid>`
existe. Ejecuta `flatpak run --unshare=network [--die-with-parent] <appid> [args]`. Es lo
más robusto para flatpaks (delegamos a su sandbox en vez de anidar el nuestro).

### Semántica de `--loopback`

Sube el `lo` **interno** del namespace (apps Electron que escuchan en `127.0.0.1` para su
IPC). **No** da acceso al localhost del host (otro netns). Documentar en README.

## Estructura del proyecto

```
faraday/
  go.mod                       // module github.com/ensamblatec/faraday
  Makefile                     // CGO_ENABLED=0
  README.md
  cmd/faraday/main.go          // si FARADAY_CHILD → childMain, si no → cli.Main
  internal/cli/                // dispatch: run, list, save, version, help
  internal/sandbox/
      runner.go bwrap.go flatpak.go probe.go nsrunner.go child.go
  internal/profile/profile.go  // load/save YAML, resolve, merge CLI↔perfil
  profiles/spotify.yaml
```

### CLI

- `faraday run [--loopback] -- <cmd> [args...]` — ad-hoc.
- `faraday run <perfil>` — carga `~/.config/faraday/profiles/<perfil>.yaml`.
- `faraday list` / `faraday save <nombre> [--loopback] -- <cmd>` / `version` / `help`.

Perfil YAML:
```yaml
description: Spotify sin red (solo música descargada)
flatpak: com.spotify.Client
loopback: false
```

## Dependencias

`github.com/vishvananda/netlink` (pure Go) · `gopkg.in/yaml.v3` ·
`golang.org/x/sys/unix` · CLI con stdlib `flag` + dispatcher manual.

## Verificación end-to-end

- `faraday run -- curl -sS --max-time 5 https://example.com` → falla (sin red).
- `faraday run -- ip addr` → solo `lo` (DOWN; UP con `--loopback`).
- `faraday run spotify` → abre, reproduce música descargada, 0 conexiones salientes
  (comparar `ss -tnp` con Spotify normal); ventana visible (Wayland) y audio (PipeWire).
- Híbrido: `--force-bwrap` confirma que bwrap también bloquea.
- `go test ./...` (unit CLI/perfiles; integración linux: aislamiento de red).

## Riesgos / notas

- Flatpak anidado → por eso se delega a `flatpak run --unshare=network`.
- AppArmor/userns puede restringir en otras máquinas (`kernel.apparmor_restrict_unprivileged_userns`)
  → existe fallback bwrap + mensaje de error con guía.
- `lo` interno ≠ localhost del host.
- No se aíslan FS/IPC/PID a propósito (GUI/audio/D-Bus). Un modo "paranoico" rompería audio.
