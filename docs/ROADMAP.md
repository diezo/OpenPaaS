# OpenPaaS roadmap

This file tracks planned ideas that aren't built yet.

## Auto-redeploy on image push (via the deploy API)

**Idea (from the project author, Nov 2026):** when a new image is pushed, services should redeploy themselves, with no SSH and no CLI.

### Flow
1. The user's CI (for example a GitHub Actions workflow) builds and pushes a new image to the **same tag** the service was created with, usually `:latest`.
2. CI then calls the OpenPaaS deploy API on the VM at `https://deploy.<domain>`, naming the service: for example `POST /services/<slug>/redeploy`.
3. OpenPaaS then:
   1. pulls the same image reference again, which fetches the new `:latest`;
   2. deletes the old container;
   3. starts a new container from the new image with the same port mapping.

### Why `op create` recommends `:latest`
With a fixed tag such as `:v1.2`, re-pulling would return the same image every time. With `:latest`, a redeploy is simply "pull and restart", and CI only needs to push and call one URL.

### Current state
- `op create` already stores each service's image reference, internal port and host port in `/etc/openpaas/services.json`. A redeploy has everything it needs.
- The deploy API (`op serve`, behind `deploy.<domain>`) exists but only answers `hello`. No endpoints are built yet.

### Open questions for when this is built
- **Auth:** the API is public. Each service probably needs its own deploy token: create it on `op create` and show it once, for CI to store as a secret. Compare the token in constant time, and add rate limiting.
- **Zero-downtime swap:**
  1. Start the new container on a fresh port and wait for it to be healthy.
  2. Point the nginx site at the new port and reload.
  3. Remove the old container.
  4. Update `host_port` in the store.

  This avoids the gap a plain "delete then start" would cause.
- **Failed deploys:** if the new container doesn't come up, keep the old one running and report the error to CI as a non-2xx response with the logs tail.
- **Concurrency:** two redeploys of the same service must not race (use a per-service lock).
- **A ready-made GitHub Actions snippet**, for example a `curl` step, documented in the README.
- **Wildcard TLS** (`*.domain`), so new services don't each need a certbot run.
