# xrl native integration images

This fork-only pipeline prepares the original 1Password SDK native cache. It builds
**all providers**, using Go **1.26.5**, on native `ubuntu-24.04` (AMD64) and
`ubuntu-24.04-arm` (ARM64) hosts. There is no QEMU, provider stripping, WASM
optimization or runtime memory tuning. Only the SDK/runtime fork replacements change. The upstream
Dockerfile, image defaults, and main-branch build behavior are unchanged.

`.github/workflows/xrl-images.yml` runs on pushes to `xrl/integration`, PRs
against it, and manual dispatch. Build jobs have only `contents: read`, do not
log in, and export checked local images as short-lived workflow artifacts.
Only `push` or `workflow_dispatch` on **exactly** `refs/heads/xrl/integration`
in **exactly** `xrl/external-secrets` can start the separate write-permission
publisher jobs. These load the images from this run/attempt; they do not rebuild.
Legacy CI and credential-bearing e2e exclude integration PRs; upstream tests
remain unchanged. Legacy image publishing/signing and
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

## Cache contract and ownership

The controller flag `--onepassword-sdk-cache-dir=/var/cache/onepassword-sdk`
explicitly opts in. The empty default retains the upstream SDK path. No chart
API changes are needed: configure the existing `extraArgs` value only after target
preflight. A configured controller prepares one process-lifetime owned runtime
before Kubernetes configuration, manager creation, reconciliation or readiness.
Missing, incompatible or corrupt entries abort startup, with no compilation or
read-write fallback. Both store validation and normal client creation use the
same owner; per-store client and secret caches are unchanged. The shared owner
is deliberately not closed on manager shutdown: a shutdown timeout need not mean
all reconcilers have stopped. Process exit reclaims it. Failed preparation and
standalone CLI owners are closed in SDK ownership order.

`external-secrets onepassword-sdk-cache prepare --cache-dir DIRECTORY` populates
using the exact controller binary, embedded artifact and SDK configuration.
`check` instead requires a hit. Neither command creates a manager, discovers
Kubernetes configuration, reads credentials or authenticates a client. CLI output
records OS/architecture and ARM64 LSE availability; success is printed only after
preparation and cleanup succeed.

`hack/xrl-image-prepare.sh` runs `prepare` in Docker with `--network=none`. Its
output tree is root-owned, directories 0555 and files 0444, readable by UID 65534.
`hack/xrl-image-verify.sh` starts fresh UID/GID 65534 processes with no network,
writable root, capabilities, mounts or tmpfs. It requires a cache hit, checks an
existing-but-unpopulated directory fails, and checks normal controller startup
fails on the same miss before Kubernetes configuration. The image does not turn
on the controller flag by default. Native CI also runs `make xrl.cache.test`.

`make xrl.cache.digest` checks the dependency's embedded `core.wasm` SHA-256:
`23d115f4ac7519b48172df3e8615945572dbda7033d51b44c9490fd533ae0f23`.
No optimized WASM or benchmark binary contributes cache entries. Root and provider
modules both pin `github.com/xrl/onepassword-sdk-go v0.4.1-xrl.0` and
`github.com/xrl/wazero v1.12.0-xrl.0`; replacements from dependencies are
not inherited. `make xrl.cache.provider.test` and `xrl.cache.command.test` exercise
those module boundaries independently. `make xrl.cache.build` builds the complete
all-provider executable locally without unrelated generated-file prerequisites.

## CPU compatibility is an acceptance gate

The pinned runtime retains `wazevo.fileCacheKey`'s CPU feature bits, module identity
and compiler-format magic. ARM64 `platform.loadCpuFeatureFlags` maps
`cpu.ARM64.HasATOMICS` (LSE) to `CpuFeatureArm64Atomic`; cache keys therefore differ
between LSE and non-LSE hosts. Do not normalize keys, disable feature detection,
or infer compatibility from `linux/arm64` alone. AMD64 feature keys also remain
unchanged. Cache input is trusted executable data: checksums detect corruption,
not malicious replacement. Only trusted build artifacts belong in the image.

The owner must run the image's credential-free `check` on a representative target
CPU with the same read-only/no-network restrictions before enabling the flag.
Record both build and target `arm64-lse` output and a successful fresh-process
require-hit (not just `lscpu` or architecture). This is the proof of LSE/config
compatibility; mismatches must block rollout, not trigger recompilation. CI proves
only compatibility with its own native runner. No target compatibility is claimed
from local Darwin checks or from packaging alone.

## Remaining merge gates

Independent review, native CI builds, public GHCR pulls, actual SDK preparation
and sandboxed require-hit validation remain required. At final integration run
`make reviewable`, `make test`, and (after committing) `make check-diff`; report
missing prerequisites and do not replace tools or regenerate unrelated changes
to force a pass. PRs targeting integration intentionally no longer run legacy CI,
so these repository-wide gates must be supplied at the final integration stage.
