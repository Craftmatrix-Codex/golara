# Deployment

The current Node control plane and native Studio deployment remain the rollback path. The Go platform will first run as an isolated build and local integration target.

The production target is one reproducible multi-stage Docker build containing:

- Studio frontend build artifacts.
- A statically linked Go platform binary.
- Minimal NGINX runtime configuration.
- No Node or Go toolchain in the final runtime image.

Deployment is allowed only after the compatibility, security, Docker, and live verification gates are recorded.
