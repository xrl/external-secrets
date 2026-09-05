# xrl native integration images

This fork-only pipeline is independent of the pending SDK cache API. It builds
**all providers**, using Go **1.26.5**, on native `ubuntu-24.04` (AMD64) and
`ubuntu-24.04-arm` (ARM64) hosts. There is no QEMU, provider stripping, WASM
optimization, runtime memory tuning, or dependency change. The upstream
Dockerfile, image defaults, and main-branch build behavior are unchanged.

`.github/workflows/xrl-images.yml` runs on pushes to `xrl/integration`, PRs
against it, and manual dispatch. Build jobs have only `contents: read`, do not
log in, and export checked local images as short-lived workflow artifacts.
Only `push` or `workflow_dispatch` on **exactly** `refs/heads/xrl/integration`
in **exactly** `xrl/external-secrets` can start the separate write-permission
publisher jobs. These load the images from this run/attempt; they do not rebuild.
Legacy CI excludes integration PRs, and legacy image publishing/signing and
release promotion are upstream-repository-only. Do not enable a legacy fork
publisher with `GHCR_USERNAME` or `IS_FORK`.

## Image identity and evidence

The sole destination is `ghcr.io/xrl/external-secrets`. Each run produces:

- `v2.9.0-xrl.<run_id>.<run_attempt>-g<12-character-commit-sha>-amd64`
- `v2.9.0-xrl.<run_id>.<run_attempt>-g<12-character-commit-sha>-arm64`
- `v2.9.0-xrl.<run_id>.<run_attempt>-g<12-character-commit-sha>` (two-platform index)

No `latest`, `main`, `v2.9.0`, or upstream release tags are published. OCI labels
record source, full revision, and version. Job evidence includes host CPU details,
native binary `file`/`go version -m`, commands, image config, archive checksum,
child digests and the final index. The index is assembled from **produced child
digests**, not mutable tags, then checked for exactly the expected Linux
AMD64/ARM64 digest pairs. For reruns, choose **re-run all jobs**: attempt-scoped
artifacts deliberately prevent mixing previous build attempts with new tags.

The owner must make the GHCR package public manually; the workflow does not
change permissions. The final job requires anonymous index access and pulls both
children by digest using an empty Docker credential config. An initially private
package will fail this gate even after upload. No cluster deployment or
publication was performed as part of implementing this pipeline.

## Local targets and SDK handoff

Use `make xrl.images.test xrl.images.lint` for identity/manifest/guard tests,
actionlint, and shell syntax checks. `make xrl.tools.actionlint` optionally installs
the pinned linter into `bin/`. Native Linux hosts with Go 1.26.5 and Docker Buildx
can run:

```sh
make xrl.image.build xrl.image.verify XRL_ARCH=arm64 XRL_TAG=local
```

`hack/xrl-images.mk` is an opt-in Make include. `xrl.native.check` refuses a
non-Linux host, an architecture mismatch, or a different Go toolchain.
`xrl.image.build` uses the existing `build-<arch>` target (including generated
code), explicitly retaining `all_providers`. `Dockerfile.xrl.dockerignore` is a
small allowlist independent of the upstream context exclusions.

Two concrete seams are reserved for the later provider/SDK writer:

1. `hack/xrl-image-prepare.sh BINARY IMAGE_ROOT` executes during the native
   `Dockerfile.xrl` preparation stage with networking disabled. It currently
   performs **only `--help`** and creates an empty image root. Once a real SDK
   API/CLI exists, this hook should populate its cache beneath `IMAGE_ROOT`;
   that tree is copied to `/` in the final distroless image. Ensure files are
   readable by UID/GID 65534 without runtime writes. Extend the context allowlist
   only if additional explicit build inputs are needed.
2. `hack/xrl-image-verify.sh IMAGE` starts a fresh process with `--network=none`,
   `--read-only`, UID/GID 65534, no capabilities and no writable mounts/tmpfs.
   Replace the current `--help` smoke command with the actual offline
   **require-hit** command. Keep sandbox flags, fail on misses, and prove the
   fresh process used the embedded cache. No SDK method or command is assumed
   here; these images do **not** currently claim cache population or cache hits.

Preserve the original shipping `core.wasm` SHA-256
`23d115f4ac7519b48172df3e8615945572dbda7033d51b44c9490fd533ae0f23`.
The later integration must verify it against the dependency's embedded artifact;
this CI-only stage does not modify or inspect module-cache source. The deployed
baseline uses Wazero 1.12.0. A Wazero compiled cache may depend on architecture,
CPU features, runtime/compiler version, and WASM identity: a native GitHub runner
is **not** proof of compatibility with every same-architecture deployment CPU.
Validate require-hit on representative target CPUs and reject incompatible
entries; never hide a miss with network access or runtime writes.

## Remaining merge gates

Independent review, native CI builds, public GHCR pulls, actual SDK preparation
and sandboxed require-hit validation remain required. At final integration run
`make reviewable`, `make test`, and (after committing) `make check-diff`; report
missing prerequisites and do not replace tools or regenerate unrelated changes
to force a pass. PRs targeting integration intentionally no longer run legacy CI,
so these repository-wide gates must be supplied at the final integration stage.
