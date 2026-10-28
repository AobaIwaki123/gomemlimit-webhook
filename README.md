# gomemlimit-webhook

Lightweight Kubernetes Mutating Admission Webhook that automatically calculates and injects `GOMEMLIMIT` and `GOGC` environment variables into container specifications based on their memory limits.

Built with **zero heavy framework dependencies** (pure Go `net/http` and official `k8s.io/api` types) to achieve minimal footprint (~9.8MB static binary) and fast cold start.

---

## Background & Problem

In Kubernetes environments, Go applications often risk being killed by the Linux kernel OOM Killer (`SIGKILL`, Exit Code 137) because the Go runtime GC is not automatically aware of container cgroups limits.

Go 1.19+ introduced the `GOMEMLIMIT` soft memory limit (`runtime/debug.SetMemoryLimit`). Setting `GOMEMLIMIT` to **85% - 90%** of the container's `resources.limits.memory` allows the Go GC to run aggressively before hitting the container limit, preventing OOM kills.

While applications can import in-process libraries (like `go.uber.org/automemlimit`), cluster operators running heterogeneous or third-party containers need an infrastructure-level mechanism to enforce this policy automatically.

---

## Features

- **Pure Go & Minimal Dependencies**: Uses Go standard library (`net/http`) and official Kubernetes API definitions (`k8s.io/api/admission/v1`). No `controller-runtime` bloat.
- **Micro Footprint**: ~9.8 MB static binary stripped of DWARF symbols, packaged in a Distroless non-root image.
- **In-Memory TLS Support**: Built-in self-signed CA & server certificate generation for zero-dependency bootstrapping, alongside standard Secret mount support.
- **RFC 6902 Compliant**: Generates precise JSON Patches for container `env` fields.
- **Configurable**: Supports per-pod ratio customization and custom `GOGC` values via annotations.

---

## Annotations

| Annotation | Type / Default | Description |
| :--- | :--- | :--- |
| `gomemlimit.aooba.net/inject` | `string` (`"true"` / `"false"`) | Opt-in webhook injection for the Pod |
| `gomemlimit.aooba.net/ratio` | `string` (`"0.90"`) | Ratio of memory limit to allocate to `GOMEMLIMIT` (e.g., `0.85`) |
| `gomemlimit.aooba.net/gogc` | `string` (Optional) | Custom `GOGC` target value to inject (e.g., `100`, `off`) |

---

## Quick Start

### 1. Build and Test

```bash
# Run unit & integration tests with race detector
make test

# Build optimized static binary
make build-stripped
```

### 2. Deploy to Kubernetes

Deploy the webhook and its self-signed TLS certificates to your cluster:

```bash
./scripts/deploy-local.sh
```

### 3. Verification

Apply a sample pod with memory limits:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: sample-go-app
  annotations:
    gomemlimit.aooba.net/inject: "true"
    gomemlimit.aooba.net/ratio: "0.85"
spec:
  containers:
    - name: app
      image: alpine:latest
      command: ["sh", "-c", "env | grep -E 'GOMEMLIMIT|GOGC' && sleep 3600"]
      resources:
        limits:
          memory: "256Mi"
```

Inspect the injected environment variable:

```bash
kubectl get pod sample-go-app -o jsonpath='{.spec.containers[0].env}'
# Result: [{"name":"GOMEMLIMIT","value":"217MiB"}]
```

---

## References

- [Go GC Guide (Official Documentation)](https://go.dev/doc/gc-guide)
- [Dynamic Admission Control (Kubernetes Official Documentation)](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/)
- [RFC 6902 JavaScript Object Notation (JSON) Patch](https://datatracker.ietf.org/doc/html/rfc6902)

---

## License

MIT License. See [LICENSE](LICENSE) for details.
