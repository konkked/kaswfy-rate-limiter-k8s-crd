# RateLimiter Operator README

Welcome to the `RateLimiter` Operator! This tool extends Kubernetes to manage rate limiting for high-volume web applications. With the `RateLimitConfig` Custom Resource Definition (CRD), you can configure Envoy sidecars to enforce request limits dynamically—keeping your cluster stable and tenants in check.

This README outlines all configurable options available in the `RateLimitConfig` CRD and related Deployment settings.

## Configurable Options

The `RateLimitConfig` CRD lets you define how Envoy rate-limits traffic for a specific `Deployment`. Below are the options you can set under `spec`, along with Deployment-level port configurations.

### RateLimitConfig Spec Options

| **Field**                  | **Type**              | **Description**                                                                                   | **Default**       | **Example**                       |
|----------------------------|-----------------------|---------------------------------------------------------------------------------------------------|-------------------|-----------------------------------|
| `deploymentName`           | `string`              | The target `Deployment` to apply rate limiting to—e.g., `web-app`. Required.                | None (required)   | `"web-app"`                  |
| `maxRequestsByIpRps`       | `*int32`              | Maximum requests per second by client IP—optional; omit or set to `null` to disable.             | `null`            | `1000`                            |
| `maxRequestsByUserRps`     | `*int32`              | Maximum requests per second by user (via `X-User-ID` header)—optional; omit to disable.          | `null`            | `500`                             |
| `maxRequestsByLikeRouteRps`| `*int32`              | Maximum requests per second by route pattern (e.g., `/space/*`)—optional; omit to disable.       | `null`            | `2000`                            |
| `envoyPort`                | `*int32`              | Port where Envoy listens for incoming traffic—optional; defaults to 8080 if unset.               | `8080`            | `9090`                            |
| `clusterName`              | `string`              | The backend cluster Envoy proxies traffic to—e.g., `web-app`. Required.                    | None (required)   | `"web-app"`                  |
| `headerRateLimits`         | `[]HeaderRateLimit`   | Custom rate limits based on request headers—optional; leave empty to skip. See below for details.| `[]` (empty)      | See below                         |

#### `headerRateLimits` Sub-Fields
Nested under `spec.headerRateLimits`, this array allows rate limiting by custom headers (e.g., `X-Tenant-ID`):

| **Sub-Field**  | **Type**  | **Description**                                      | **Default** | **Example**       |
|----------------|-----------|------------------------------------------------------|-------------|-------------------|
| `headerName`   | `string`  | The header to limit by—e.g., `X-Tenant-ID`. Required.| None        | `"X-Tenant-ID"`   |
| `rps`          | `int32`   | Requests per second for this header. Required.       | None        | `200`             |

**Example:**
```yaml
headerRateLimits:
- headerName: "X-Tenant-ID"
  rps: 200
- headerName: "X-API-Key"
  rps: 150
```

### Deployment-Level Port Options
These options are configured in the target `Deployment` (e.g., `a-app-web`) and work with the Operator’s Envoy sidecar:

| **Field**      | **Type**  | **Description**                                      | **Default** | **Example** |
|----------------|-----------|------------------------------------------------------|-------------|-------------|
| `ingressPort`  | `*int32`  | Port where traffic enters the Deployment—set via Service targeting `envoyPort`. Optional. | `80` (Service) | `8081` |
| `egressPort`   | `*int32`  | Port where Envoy sends traffic to the app—set in the app container. Optional. | `3000` (app) | `3001` |

## Example Usage

Here’s how to use `RateLimitConfig` with an app `Deployment`:

### Sample `RateLimitConfig`
```yaml
apiVersion: kaswfy.io/v1
kind: RateLimitConfig
metadata:
  name: web-app-ratelimit
  namespace: default
spec:
  deploymentName: "web-app"
  maxRequestsByIpRps: 1000
  maxRequestsByUserRps: 500
  maxRequestsByLikeRouteRps: 2000
  envoyPort: 9090
  clusterName: "web-app-cluster"
  headerRateLimits:
  - headerName: "X-Tenant-ID"
    rps: 200
```

### Corresponding `Deployment` and `Service`
- **Deployment:**
  ```yaml
  apiVersion: apps/v1
  kind: Deployment
  metadata:
    name: web-app
  spec:
    replicas: 3
    selector:
      matchLabels:
        app: web-app
    template:
      metadata:
        labels:
          app: web-app
      spec:
        containers:
        - name: web-app-a
          image: miniflex-app:latest
          ports:
          - containerPort: 3000  # egressPort—where Envoy sends traffic
  ```
- **Service:**
  ```yaml
  apiVersion: v1
  kind: Service
  metadata:
    name: web-app
  spec:
    selector:
      app: web-app
    ports:
    - port: 8081            # ingressPort—external traffic entry
      targetPort: 9090      # Matches envoyPort from RateLimitConfig
  ```

## How It Works
- **Apply Config:** `kubectl apply -f ratelimitconfig.yaml`—Operator injects an Envoy sidecar into `web-app`.
- **Traffic Flow:** External requests → Service (`:8081`) → Envoy (`:9090`) → App (`:3000`)—rate limits applied by Envoy.
- **Limits:** Configurable via IP, user ID, route patterns, and custom headers—e.g., tenant-specific limits on `/space/123`.

## Getting Started
1. **Deploy Operator:**
   ```bash
   helm install rate-limiter charts/rate-limiter-operator
   ```
2. **Apply Sample:**
   ```bash
   kubectl apply -f config/samples/kaswfy_v1_ratelimitconfig.yaml
   ```
3. **Check It Out:**
   ```bash
   kubectl get pods -l app=web-app -o wide  # Look for envoy-sidecar
   ```

## Notes
- **Required Fields:** `deploymentName` and `clusterName` must be set—others are optional.
- **Port Config:** `ingressPort` and `egressPort` are set in the `Deployment` and `Service`—align with `envoyPort` for smooth traffic flow.

Need more details? Check the code in `api/v1/ratelimitconfig_types.go` or ping me—happy rate limiting!