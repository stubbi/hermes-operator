# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [0.2.1](https://github.com/stubbi/hermes-operator/compare/v0.2.0...v0.2.1) (2026-11-14)


### Features

* **networkpolicy:** add egressPolicy to make the TCP/443 baseline restrictable ([#179](https://github.com/stubbi/hermes-operator/issues/179)) ([4ed75da](https://github.com/stubbi/hermes-operator/commit/4ed75da9f685a1fca189e465e7ce3239aadb4db7))


### Bug Fixes

* **backup:** make backup jobs run in the default restic image ([#182](https://github.com/stubbi/hermes-operator/issues/182)) ([d29171c](https://github.com/stubbi/hermes-operator/commit/d29171c89a4b18a4a6252ad5603faa0a97287919))
* **ci:** resolve envtest assets before running tests ([#194](https://github.com/stubbi/hermes-operator/issues/194)) ([e9a1c7e](https://github.com/stubbi/hermes-operator/commit/e9a1c7e32a247379baa161e1b263d30fbcfa0f5c))
* **ci:** validate Helm RBAC permission comparisons ([#198](https://github.com/stubbi/hermes-operator/issues/198)) ([b226b80](https://github.com/stubbi/hermes-operator/commit/b226b806b7bc8226ee01bc317b18295b6308ca44))
* **ci:** verify legacy release artifacts and preserve signing failures ([#201](https://github.com/stubbi/hermes-operator/issues/201)) ([aed77f4](https://github.com/stubbi/hermes-operator/commit/aed77f455489821a177394ecb6400ed19d1b82be))
* **deps:** align built artifacts with Go 1.27.4 ([#204](https://github.com/stubbi/hermes-operator/issues/204)) ([a1ed60a](https://github.com/stubbi/hermes-operator/commit/a1ed60a341ecc89d87406106d3cda1dee17ccc53))
* **deps:** resolve Go advisory findings and check shipped targets ([#200](https://github.com/stubbi/hermes-operator/issues/200)) ([10acdb7](https://github.com/stubbi/hermes-operator/commit/10acdb7c5d839cfc5e344f3d630bba118f8db12f))

## [0.2.0](https://github.com/paperclipinc/hermes-operator/compare/v0.1.20...v0.2.0) (2026-09-06)


### ⚠ BREAKING CHANGES

* **upgrade:** since #151, instances that leave `spec.resources` unset render CPU and memory requests and limits on the agent container instead of running unbounded. An instance whose working set exceeds 4Gi of memory, or that relied on bursting past 2 CPUs, will be OOM-killed or throttled after this upgrade. Set `spec.resources` explicitly, or `spec.resources.applyOperatorDefaults: false`, before upgrading such an instance. Clusters relying on a namespace LimitRange to supply these values should note that the operator now wins over the LimitRange default. See docs/upgrade-notes.md.

### Features

* **resources:** give the agent container a default request/limit floor ([#151](https://github.com/paperclipinc/hermes-operator/issues/151)) ([35100b1](https://github.com/paperclipinc/hermes-operator/commit/35100b152a3024e1a9ec0c93d5e000c4f633191f))


### Bug Fixes

* **bundle:** keep the OLM CSV operator image refs on the current release ([#150](https://github.com/paperclipinc/hermes-operator/issues/150)) ([7a9bbab](https://github.com/paperclipinc/hermes-operator/commit/7a9bbab132b0b5ba57d84731e1ce8c46515e5ef3))
* **release:** bump the OLM CSV generically, the yaml updater drops the markers ([#153](https://github.com/paperclipinc/hermes-operator/issues/153)) ([14d41d2](https://github.com/paperclipinc/hermes-operator/commit/14d41d247069e9f6abc2c1dc145fc7e073b508e6))


### Documentation

* **upgrade:** add upgrade notes, starting with the agent resource floor ([#154](https://github.com/paperclipinc/hermes-operator/issues/154)) ([f2528e3](https://github.com/paperclipinc/hermes-operator/commit/f2528e3852f79168155a786a4f68c8e867354f30))

## [0.1.20](https://github.com/paperclipinc/hermes-operator/compare/v0.1.19...v0.1.20) (2026-07-28)


### Bug Fixes

* **chart:** stop rendering a doubled v in the operator image tag ([#131](https://github.com/paperclipinc/hermes-operator/issues/131)) ([7a724e5](https://github.com/paperclipinc/hermes-operator/commit/7a724e5976a9cb9297bc9a2d85c7e69ae72a23fe))

## [0.1.19](https://github.com/paperclipinc/hermes-operator/compare/v0.1.18...v0.1.19) (2026-07-28)


### Features

* **scheduling:** add spec.scheduling.runtimeClassName ([#123](https://github.com/paperclipinc/hermes-operator/issues/123)) ([4462cb7](https://github.com/paperclipinc/hermes-operator/commit/4462cb7179effefa4f458297ce3b1f3afb582507))


### Bug Fixes

* **chart:** render pullable default image tags ([#122](https://github.com/paperclipinc/hermes-operator/issues/122)) ([88445b1](https://github.com/paperclipinc/hermes-operator/commit/88445b1972610aafd8bd0992233458c3db421354))

## [0.1.18](https://github.com/paperclipinc/hermes-operator/compare/v0.1.17...v0.1.18) (2026-06-19)


### Features

* **runtime:** run agents on the upstream s6 image — instance reaches Ready (gateway API + /health) ([#90](https://github.com/paperclipinc/hermes-operator/issues/90)) ([12dccb4](https://github.com/paperclipinc/hermes-operator/commit/12dccb40a00f66a34b80267932cda2e3c3898220))


### Bug Fixes

* **agent-image:** ship pyproject.toml + uv.lock at /opt/venv-template/ ([#68](https://github.com/paperclipinc/hermes-operator/issues/68)) ([#85](https://github.com/paperclipinc/hermes-operator/issues/85)) ([39ef52c](https://github.com/paperclipinc/hermes-operator/commit/39ef52c09efa10becf02a2238f8637be1c6bfc0a))
* **agent-image:** ship the uv binary in the runtime stage ([#68](https://github.com/paperclipinc/hermes-operator/issues/68)) ([#88](https://github.com/paperclipinc/hermes-operator/issues/88)) ([13fb1e5](https://github.com/paperclipinc/hermes-operator/commit/13fb1e512cfea258359aa9744750e51299baf25e))
* **backup:** bound the on-delete finalizer so a failing snapshot can't make an instance undeletable ([#94](https://github.com/paperclipinc/hermes-operator/issues/94)) ([40d6099](https://github.com/paperclipinc/hermes-operator/commit/40d609947b0659dfb747e8cc57686584ae9d94cb))

## [0.1.17](https://github.com/paperclipinc/hermes-operator/compare/v0.1.16...v0.1.17) (2026-06-16)


### Bug Fixes

* **operatorhub:** ship ci.yaml to redhat submission + add reviewers ([#82](https://github.com/paperclipinc/hermes-operator/issues/82)) ([9e9cafe](https://github.com/paperclipinc/hermes-operator/commit/9e9cafe13428bb8c3d41cc3e895f334d11a2d3e2))

## [0.1.16](https://github.com/paperclipinc/hermes-operator/compare/v0.1.15...v0.1.16) (2026-06-16)


### Bug Fixes

* **bundle:** declare com.redhat.openshift.versions for OpenShift cert ([#80](https://github.com/paperclipinc/hermes-operator/issues/80)) ([47d5ff4](https://github.com/paperclipinc/hermes-operator/commit/47d5ff4a5c1b547290acfe24e0628b357415062c))

## [0.1.15](https://github.com/paperclipinc/hermes-operator/compare/v0.1.14...v0.1.15) (2026-06-16)


### Features

* **chart:** world-class Artifact Hub metadata + cosign chart signing ([#78](https://github.com/paperclipinc/hermes-operator/issues/78)) ([4e38216](https://github.com/paperclipinc/hermes-operator/commit/4e382164d94142eb205b9ace554e36c66438f7a8))


### Bug Fixes

* **agent-image:** broken runtime venv + invalid smoke workflow YAML ([#66](https://github.com/paperclipinc/hermes-operator/issues/66)) ([ec6016c](https://github.com/paperclipinc/hermes-operator/commit/ec6016c141813a84f673d9c7cc77b824abbc180a))

## [0.1.14](https://github.com/paperclipinc/hermes-operator/compare/v0.1.13...v0.1.14) (2026-06-10)


### Features

* spec.tailscale.mode=serve (private tailnet exposure) ([#63](https://github.com/paperclipinc/hermes-operator/issues/63)) ([da7c307](https://github.com/paperclipinc/hermes-operator/commit/da7c307ecf1c738c145cc096cb064a2fdb1e6d45))

## [0.1.13](https://github.com/paperclipinc/hermes-operator/compare/v0.1.12...v0.1.13) (2026-06-04)


### Bug Fixes

* **olm:** use the official Hermes logo for the bundle icon ([#60](https://github.com/paperclipinc/hermes-operator/issues/60)) ([45c2f38](https://github.com/paperclipinc/hermes-operator/commit/45c2f38e1e89fafe8443fbed9b98d6c2da288b15))

## [0.1.12](https://github.com/paperclipinc/hermes-operator/compare/v0.1.11...v0.1.12) (2026-06-03)


### Features

* optional Gateway API HTTPRoute for HermesInstance ([#59](https://github.com/paperclipinc/hermes-operator/issues/59)) ([ff5d781](https://github.com/paperclipinc/hermes-operator/commit/ff5d7817a991a64c6e124190787d103dea9f40e0))
* tier-1/2 parity (digest-pin, bench, image CEL, zombie reaping, Grafana, docs-site) ([#56](https://github.com/paperclipinc/hermes-operator/issues/56)) ([712ed2b](https://github.com/paperclipinc/hermes-operator/commit/712ed2b1928d363abade3fd3caf5158db798d5fc))


### Bug Fixes

* **olm:** declare webhookDefinitions so OLM provisions the webhook cert ([#57](https://github.com/paperclipinc/hermes-operator/issues/57)) ([4420511](https://github.com/paperclipinc/hermes-operator/commit/44205118a0fd3cd3d08c847449684f472ebb14b3))

## [0.1.11](https://github.com/paperclipinc/hermes-operator/compare/v0.1.10...v0.1.11) (2026-06-03)


### Bug Fixes

* **olm:** correct CSV image refs so OperatorHub submission passes ([#54](https://github.com/paperclipinc/hermes-operator/issues/54)) ([edaa87d](https://github.com/paperclipinc/hermes-operator/commit/edaa87ddf333a7610e3d451666f02c4bd8362880))

## [0.1.10](https://github.com/paperclipinc/hermes-operator/compare/v0.1.9...v0.1.10) (2026-06-03)


### Bug Fixes

* **agent-image:** native pinned uv relock + bump uv to 0.11.7 ([#48](https://github.com/paperclipinc/hermes-operator/issues/48)) ([6e2615f](https://github.com/paperclipinc/hermes-operator/commit/6e2615f29de4e8e92d6fc0b9fdb0b0a2eac0978f))
* **agent-image:** relock uses uv image entrypoint (unblocks hermes-agent publish) ([#46](https://github.com/paperclipinc/hermes-operator/issues/46)) ([9e456b1](https://github.com/paperclipinc/hermes-operator/commit/9e456b199741b0e279fa24587a1317be15f82f81))
* **agent-image:** verify uv.lock by git tag (unblocks publish) ([#50](https://github.com/paperclipinc/hermes-operator/issues/50)) ([3cbcc7f](https://github.com/paperclipinc/hermes-operator/commit/3cbcc7f3fc17216d7fd24aceeeb5124966ab485c))

## [0.1.9](https://github.com/paperclipinc/hermes-operator/compare/v0.1.8...v0.1.9) (2026-05-26)


### Bug Fixes

* **ci:** operatorhub-submit uses classic PAT with public_repo scope ([#39](https://github.com/paperclipinc/hermes-operator/issues/39)) ([33273ad](https://github.com/paperclipinc/hermes-operator/commit/33273adf3bd6e0149ef174c12a434a5f06dcbde6))
* **ci:** release-please auto-creates tag via commit-message detection ([#38](https://github.com/paperclipinc/hermes-operator/issues/38)) ([e36a8d4](https://github.com/paperclipinc/hermes-operator/commit/e36a8d471732109c0308a4b7642e8112b8b6ac58))

## [0.1.8](https://github.com/paperclipinc/hermes-operator/compare/v0.1.7...v0.1.8) (2026-05-26)


### Bug Fixes

* rename stubbi → paperclipinc across chart, CI, docs, Go module ([de3e0bd](https://github.com/paperclipinc/hermes-operator/commit/de3e0bd1f3e0ae8b1ee01b8748f005053f1f3b65))

## [0.1.7](https://github.com/paperclipinc/hermes-operator/compare/v0.1.6...v0.1.7) (2026-05-20)


### Bug Fixes

* **ci:** drop --remote from gh repo fork in operatorhub-submit ([#29](https://github.com/paperclipinc/hermes-operator/issues/29)) ([55bab59](https://github.com/paperclipinc/hermes-operator/commit/55bab593d92088ce31ff46925ca4a5b6a37a80b3))

## [0.1.6](https://github.com/paperclipinc/hermes-operator/compare/v0.1.5...v0.1.6) (2026-05-13)


### Features

* hermes-operator v1.0.0 — plans 1–7 implementation ([#1](https://github.com/paperclipinc/hermes-operator/issues/1)) ([a99ad5c](https://github.com/paperclipinc/hermes-operator/commit/a99ad5c9e2d684c6a7fbd0a5884e23a982d449f0))


### Bug Fixes

* **ci:** semantic Bundle RBAC sync check (was: file-diff flake) ([#16](https://github.com/paperclipinc/hermes-operator/issues/16)) ([3b3b5a6](https://github.com/paperclipinc/hermes-operator/commit/3b3b5a6e83046ccf041340e082e0165234d3ceb3))
* **release:** build container images via docker/build-push-action ([#23](https://github.com/paperclipinc/hermes-operator/issues/23)) ([d1b4862](https://github.com/paperclipinc/hermes-operator/commit/d1b4862f018d4b6c1d657d8485ce4f417f754030))
* **release:** run make installer via goreleaser before:hooks ([#25](https://github.com/paperclipinc/hermes-operator/issues/25)) ([5bcd154](https://github.com/paperclipinc/hermes-operator/commit/5bcd1548d9b80b23809b171b13d7ec71a80724af))
* **release:** trigger v0.1.2 (v0.1.1 tag has broken release.yaml) ([#18](https://github.com/paperclipinc/hermes-operator/issues/18)) ([e3f8289](https://github.com/paperclipinc/hermes-operator/commit/e3f8289b26d2dcc581702c138549c90914b747bf))
* **release:** use --skip=validate instead of throw-away commit ([#20](https://github.com/paperclipinc/hermes-operator/issues/20)) ([61f2099](https://github.com/paperclipinc/hermes-operator/commit/61f20999031ba61f13d2996b7b235026ab701e5a))

## [0.1.5](https://github.com/paperclipinc/hermes-operator/compare/v0.1.4...v0.1.5) (2026-05-13)


### Bug Fixes

* **release:** run make installer via goreleaser before:hooks ([#25](https://github.com/paperclipinc/hermes-operator/issues/25)) ([5bcd154](https://github.com/paperclipinc/hermes-operator/commit/5bcd1548d9b80b23809b171b13d7ec71a80724af))

## [0.1.4](https://github.com/paperclipinc/hermes-operator/compare/v0.1.3...v0.1.4) (2026-05-13)


### Bug Fixes

* **release:** build container images via docker/build-push-action ([#23](https://github.com/paperclipinc/hermes-operator/issues/23)) ([d1b4862](https://github.com/paperclipinc/hermes-operator/commit/d1b4862f018d4b6c1d657d8485ce4f417f754030))

## [0.1.3](https://github.com/paperclipinc/hermes-operator/compare/v0.1.2...v0.1.3) (2026-05-13)


### Bug Fixes

* **release:** use --skip=validate instead of throw-away commit ([#20](https://github.com/paperclipinc/hermes-operator/issues/20)) ([61f2099](https://github.com/paperclipinc/hermes-operator/commit/61f20999031ba61f13d2996b7b235026ab701e5a))

## [0.1.2](https://github.com/paperclipinc/hermes-operator/compare/v0.1.1...v0.1.2) (2026-05-13)


### Bug Fixes

* **ci:** semantic Bundle RBAC sync check (was: file-diff flake) ([#16](https://github.com/paperclipinc/hermes-operator/issues/16)) ([3b3b5a6](https://github.com/paperclipinc/hermes-operator/commit/3b3b5a6e83046ccf041340e082e0165234d3ceb3))
* **release:** trigger v0.1.2 (v0.1.1 tag has broken release.yaml) ([#18](https://github.com/paperclipinc/hermes-operator/issues/18)) ([e3f8289](https://github.com/paperclipinc/hermes-operator/commit/e3f8289b26d2dcc581702c138549c90914b747bf))

## [0.1.1](https://github.com/paperclipinc/hermes-operator/compare/v0.1.0...v0.1.1) (2026-05-13)


### Features

* hermes-operator v1.0.0: plans 1-7 implementation ([#1](https://github.com/paperclipinc/hermes-operator/issues/1)) ([a99ad5c](https://github.com/paperclipinc/hermes-operator/commit/a99ad5c9e2d684c6a7fbd0a5884e23a982d449f0))

## [1.0.0](https://github.com/paperclipinc/hermes-operator/releases/tag/v1.0.0) (2026-05-12)

First public release. The Kubernetes operator for
[nousresearch/hermes-agent](https://github.com/nousresearch/hermes-agent),
shipping with full feature parity to openclaw-operator v0.32 adapted to
hermes-agent's Python/uv runtime, plus hermes-specific surfaces:
multi-platform gateways (Telegram/Discord/Slack/WhatsApp/Signal), a
Honcho profile-store companion, an SSA-based `HermesSelfConfig` API for
agent-initiated mutations, and a one-shot OpenClaw → Hermes migration
path.

The v1 stability contract: API versioning policy, deprecation policy,
exhaustive condition catalogue, conversion-webhook scaffolding: is in
place from day one. See
[docs/api-versioning.md](docs/api-versioning.md) and
[docs/deprecations.md](docs/deprecations.md).

Inspired by [openclaw-rocks/openclaw-operator](https://github.com/openclaw-rocks/openclaw-operator).
Concrete lessons baked in: SSA from day one on the SelfConfig path
(openclaw #433), explicit Kubernetes defaults set in every builder
(generation-thrash regressions never shipped), finalizer mutations via
`r.Patch` rather than `r.Update` (openclaw #437), foreign-annotation
preservation (openclaw #446), zombie-process reaper (openclaw #471),
namespace-scoped RBAC opt-in (openclaw #469), ClusterRole aggregation
labels (openclaw #479), and read-only root filesystem with explicit
writable subPaths (openclaw #458).

### Highlights

- **CRDs (`hermes.agent/v1`):** `HermesInstance` (namespaced),
  `HermesSelfConfig` (namespaced, SSA-applied), `HermesClusterDefaults`
  (cluster-scoped singleton `cluster`).
- **Workload:** StatefulSet (single replica by default; opt-in HPA),
  default-deny NetworkPolicy + per-gateway allow rules, PDB
  auto-managed when `replicas > 1`, read-only root filesystem with
  writable `emptyDir`s for `/tmp` and `~/.config`.
- **Multi-platform gateways:** Telegram, Discord, Slack, WhatsApp,
  Signal: each with its own Secret reference, rotatable independently.
- **Day-2 operations:** S3-compatible backups (scheduled / on-delete /
  pre-update), declarative one-shot restore (`spec.restoreFrom`),
  OCI-registry-driven auto-update with probe-failure rollback, one-shot
  OpenClaw → Hermes migration (sibling or S3 source).
- **GitOps coexistence:** SSA on the SelfConfig path under field manager
  `hermes.agent/selfconfig`; FluxCD/Argo own the same instance for
  other fields without flap.
- **Distribution:** Helm chart, OLM bundle (OperatorHub submission),
  plain kustomize manifests, multi-arch (`amd64`+`arm64`) Cosign-signed
  images with SPDX SBOM attestation.
- **Testing:** unit, envtest, e2e (kind), conformance (negative +
  idempotency + upgrade matrix + GitOps + failure injection),
  benchmarks, gosec + Trivy, Reconcile Guard CI, Helm RBAC sync check.
- **Documentation:** [design spec](docs/superpowers/specs/2026-05-12-hermes-operator-design.md),
  [API reference](docs/api-reference.md),
  [condition catalogue](docs/conditions.md),
  [API versioning policy](docs/api-versioning.md),
  [deprecation policy](docs/deprecations.md),
  [9 worked examples](examples/),
  [Grafana dashboard](docs/grafana/),
  [public roadmap](ROADMAP.md).

### Supported Kubernetes versions

1.28, 1.29, 1.30, 1.31, 1.32.

### Known limitations / deferred items

- **`examples/` directory** will be populated with 9 worked YAML recipes
  in v1.1. The directory structure and README index are committed; the
  individual example files are a follow-up cycle.
- **Grafana dashboard library** (`docs/grafana/`) will expand to per-
  instance drilldown and per-gateway health dashboards in v1.1. The
  operator-overview dashboard is in v1.0.0.
- **OperatorHub submission** requires manual steps (submission PR to
  OperatorHub community-operators repo). The OLM bundle is committed and
  tested; the actual submission is a human-in-the-loop step after this
  release.

### Inspiration and prior art

This is a clean-room operator built specifically for hermes-agent. The
[openclaw-rocks/openclaw-operator](https://github.com/openclaw-rocks/openclaw-operator)
project: which shipped a similar lifecycle operator for OpenClaw,
evolving through v0.5 → v0.32 with substantial production feedback:
served as the reference for the *shape* of this product: which surfaces
matter, which lessons stick, and which guardrails are non-negotiable.
The hermes-specific surfaces (Python/uv runtime, multi-platform
gateways, Honcho, SSA-based SelfConfig with `profiles` action, declarative
migration importer) are new. The v1 stability contract is also new; it
is the single most important thing this operator does differently from
the v0.x grind.

For the full list of openclaw lessons that informed v1, see
[docs/superpowers/specs/2026-05-12-hermes-operator-design.md](docs/superpowers/specs/2026-05-12-hermes-operator-design.md)
§1.G3 and §7.2 ("Reconciliation rules").
