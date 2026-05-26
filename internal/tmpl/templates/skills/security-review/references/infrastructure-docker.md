# Docker / container images — security overlay

## Base image discipline
- Pin base images by digest (`FROM image@sha256:...`), not floating
  tags. `:latest` and `:3` are CI hazards.
- Prefer minimal bases: `distroless`, `alpine` (with attention to
  musl compatibility), `*-slim`. Fewer packages → less attack
  surface and fewer CVE hits.
- Refresh base images on a schedule; pinning by digest is incompatible
  with "set and forget" — a rebuild cadence is mandatory.

## Dockerfile hygiene
- **Non-root.** Create a dedicated user and `USER <uid>` before the
  process starts. Running as UID 0 is a finding unless justified
  (privileged host ops, bind to privileged ports the orchestrator
  will not remap).
- **Explicit WORKDIR / COPY scope.** Do not `COPY . /app` — use
  `.dockerignore` and narrow COPY paths to avoid shipping secrets,
  test fixtures, and local config.
- **Multi-stage builds.** Build dependencies and toolchains stay in
  the builder stage; the final image contains only runtime artefacts.
- **Secrets never baked in.** Build args are visible in image
  history. Use BuildKit `--mount=type=secret` for build-time
  secrets, and runtime secret mounts for runtime.

## Runtime hardening
- `--read-only` root filesystem with explicit `tmpfs` mounts for
  `/tmp`, `/run`, and any app-specific writable path.
- `--cap-drop ALL` then `--cap-add` only the capabilities needed.
  `NET_BIND_SERVICE` is the common reason to add back.
- `--security-opt no-new-privileges:true` always.
- Do not run with `--privileged`. If the workload seems to need it
  (GPUs, fuse, nested containers), look for a supported orchestrator
  primitive first.
- `--pids-limit`, `--memory`, `--cpus` set to bound runaway
  resource use.

## Orchestration (Kubernetes)
- `securityContext` on every pod: `runAsNonRoot: true`,
  `readOnlyRootFilesystem: true`, `allowPrivilegeEscalation: false`,
  `capabilities.drop: ["ALL"]`.
- `seccompProfile.type: RuntimeDefault` (or `Localhost` with a
  narrower profile).
- Network policies: default-deny ingress / egress; grant
  namespace-scoped allowances.
- Secrets as `Secret` objects mounted as files or env; do not
  `kubectl exec cat /etc/passwd` patterns for secret retrieval.
- Resource requests and limits set; pods without limits can starve
  neighbours on the node.

## Supply chain
- Build provenance (SLSA / in-toto / sigstore) for production images.
- Sign images (`cosign sign`) and verify signatures at admission.
- SBOM produced at build (`syft`, `docker sbom`) and tracked.
- Scan images (`trivy`, `grype`) in CI; gate release on
  high / critical findings unless accepted with a named waiver.

## Private registry and pull
- Use a private registry with authentication for internal images.
- `ImagePullPolicy: IfNotPresent` in prod only when the tag is
  pinned to a digest; otherwise `Always`.
- Image promotion from staging to prod goes through a verified
  signed pipeline, not manual retag.

## Review cues
- Grep Dockerfiles for `USER root`, `chmod 777`, `curl | sh`,
  `apt-get install` without `--no-install-recommends`,
  `pip install` without a pinned lockfile, `ADD http://` (use
  `curl` + verification instead).
- Look for multi-stage builds that forget to drop the builder
  secrets in the final stage.
- Review `docker run` / compose / Helm values for missing
  `securityContext`, missing resource limits, exposed host paths.

## Anti-patterns
- Sharing the host network namespace without a clear reason.
- Mounting the Docker socket (`/var/run/docker.sock`) into app
  containers — that is root on the host.
- Using `:latest` tags in production deployment manifests.
- Treating "the cluster is private" as a sufficient substitute for
  pod security context.
