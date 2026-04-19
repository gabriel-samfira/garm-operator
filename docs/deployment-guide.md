<!-- SPDX-License-Identifier: MIT -->

# Deployment Guide

This guide walks through building the garm-operator, deploying it to a Kubernetes cluster with an existing GARM server, and creating resources for both GitHub and Gitea forges.

<!-- toc -->
- [Prerequisites](#prerequisites)
- [Build the Operator Image](#build-the-operator-image)
- [Deploy the Operator](#deploy-the-operator)
- [Configure the GARM Server](#configure-the-garm-server)
- [GitHub Setup](#github-setup)
  - [Create a GitHub Endpoint](#create-a-github-endpoint)
  - [Create GitHub Credentials](#create-github-credentials)
  - [Add a GitHub Repository](#add-a-github-repository)
- [Gitea Setup](#gitea-setup)
  - [Create a Gitea Endpoint](#create-a-gitea-endpoint)
  - [Create Gitea Credentials](#create-gitea-credentials)
  - [Add a Gitea Repository](#add-a-gitea-repository)
- [Create a Pool](#create-a-pool)
- [Create a Scale Set](#create-a-scale-set)
- [Scale Runners](#scale-runners)
<!-- /toc -->

## Prerequisites

- A Kubernetes cluster with `kubectl` configured
- [cert-manager](https://cert-manager.io/docs/installation/) installed (required for validating webhooks)
- A running GARM server reachable from the cluster
- A container registry you can push to
- A configured webhook on your GitHub/Gitea instance pointing to your GARM server's webhook URL (see [GARM webhook docs](https://github.com/cloudbase/garm/blob/main/doc/webhooks.md))

## Build the Operator Image

```bash
# Build and run tests
make docker-build IMG=your-registry.com/garm-operator:latest

# Push to your registry
make docker-push IMG=your-registry.com/garm-operator:latest
```

For multi-architecture builds (amd64, arm64, etc.):

```bash
make docker-buildx IMG=your-registry.com/garm-operator:latest
```

## Deploy the Operator

Install the CRDs and deploy the operator:

```bash
# Install CRDs
make install

# Deploy the operator (sets the image in the manager deployment)
make deploy IMG=your-registry.com/garm-operator:latest
```

The deployment template includes environment variables with empty defaults for GARM connection details. After deploying, configure the operator with your GARM server credentials:

```bash
kubectl -n garm-operator-system set env deployment/garm-operator-controller-manager \
  GARM_SERVER=https://garm.example.com \
  GARM_USERNAME=admin \
  GARM_PASSWORD=your-password
```

The following environment variables are pre-configured in the deployment template:

| Variable | Default | Description |
|----------|---------|-------------|
| `GARM_SERVER` | (empty, required) | URL of your GARM server |
| `GARM_USERNAME` | (empty, required) | GARM admin username |
| `GARM_PASSWORD` | (empty, required) | GARM admin password |
| `OPERATOR_WATCH_NAMESPACE` | (auto-detected) | Namespace to watch, defaults to the deployment's own namespace |
| `OPERATOR_RUNNER_RECONCILIATION` | `true` | Sync runner state from GARM into Kubernetes Runner CRs |

To disable runner reconciliation:

```bash
kubectl -n garm-operator-system set env deployment/garm-operator-controller-manager \
  OPERATOR_RUNNER_RECONCILIATION=false
```

Alternatively, use CLI flags or a config file. See the [configuration parsing guide](config/configuration-parsing.md) for all options.

Verify the operator is running:

```bash
kubectl -n garm-operator-system get pods
```

## Configure the GARM Server

Create a `GarmServerConfig` to configure callback, metadata, and webhook URLs on your GARM server:

```yaml
apiVersion: garm-operator.mercedes-benz.com/v1beta1
kind: GarmServerConfig
metadata:
  name: garm-server-config
  namespace: garm-operator-system
spec:
  callbackUrl: http://garm-server.garm-server.svc:9997/api/v1/callbacks
  metadataUrl: http://garm-server.garm-server.svc:9997/api/v1/metadata
  webhookUrl: http://garm-server.garm-server.svc:9997/webhook
  # Optional: required when using agent mode on runners
  agentUrl: http://garm-server.garm-server.svc:9997/api/v1/agent
  # Optional: URL to fetch GARM agent tool releases from
  # garmAgentReleasesUrl: "https://github.com/cloudbase/garm/releases"
  # Optional: automatically sync GARM agent tools
  # syncGarmAgentTools: true
```

Verify it was reconciled:

```bash
kubectl -n garm-operator-system get garmserverconfig
```

## GitHub Setup

### Create a GitHub Endpoint

```yaml
apiVersion: garm-operator.mercedes-benz.com/v1beta1
kind: GitHubEndpoint
metadata:
  name: github
  namespace: garm-operator-system
spec:
  description: "github.com"
  apiBaseUrl: "https://api.github.com"
  uploadBaseUrl: "https://uploads.github.com"
  baseUrl: "https://github.com"
```

For GitHub Enterprise Server, replace the URLs with your instance's URLs and optionally add a CA certificate bundle:

```yaml
spec:
  apiBaseUrl: "https://github.example.com/api/v3"
  uploadBaseUrl: "https://github.example.com/api/uploads"
  baseUrl: "https://github.example.com"
  caCertBundleSecretRef:
    name: ghes-ca-bundle
    key: ca.crt
```

### Create GitHub Credentials

Create a Kubernetes Secret with your Personal Access Token (PAT), then reference it in a `GitHubCredential`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: github-pat
  namespace: garm-operator-system
data:
  token: <base64-encoded-PAT>
---
apiVersion: garm-operator.mercedes-benz.com/v1beta1
kind: GitHubCredential
metadata:
  name: github-pat
  namespace: garm-operator-system
spec:
  description: "GitHub PAT credentials"
  endpointRef:
    apiGroup: garm-operator.mercedes-benz.com
    kind: GitHubEndpoint
    name: github
  authType: pat
  secretRef:
    name: github-pat
    key: token
```

For GitHub App authentication, create a secret with the app's private key PEM file, then use `authType: app`:

```bash
kubectl -n garm-operator-system create secret generic github-app-key \
  --from-file=privateKey=/path/to/your-app.private-key.pem
```

Or with a manifest:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: github-app-key
  namespace: garm-operator-system
stringData:
  privateKey: |
    -----BEGIN RSA PRIVATE KEY-----
    ...your private key content...
    -----END RSA PRIVATE KEY-----
```

Then reference it in the `GitHubCredential`:

```yaml
spec:
  authType: app
  appId: 12345
  installationId: 67890
  secretRef:
    name: github-app-key
    key: privateKey
```

Verify credentials are ready:

```bash
kubectl -n garm-operator-system get githubcredential -o wide
```

### Add a GitHub Repository

Create a webhook secret and the Repository CR:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: repo-webhook-secret
  namespace: garm-operator-system
data:
  webhookSecret: <base64-encoded-webhook-secret>
---
apiVersion: garm-operator.mercedes-benz.com/v1beta1
kind: Repository
metadata:
  name: my-repo
  namespace: garm-operator-system
spec:
  owner: "my-github-org"
  credentialsRef:
    apiGroup: garm-operator.mercedes-benz.com
    kind: GitHubCredential
    name: github-pat
  webhookSecretRef:
    name: repo-webhook-secret
    key: webhookSecret
  # Optional: enable agent mode for runners in this repository
  # agentMode: true
```

The `name` field (`my-repo`) must match the repository name on GitHub. The `owner` field is the GitHub user or organization that owns the repository.

The optional `agentMode` field enables GARM agent mode for runners scoped to this entity. When agent mode is enabled, pools under this entity can use `enableShell` to allow shell access on runners. Agent mode can also be set on `Organization` and `Enterprise` CRDs.

## Gitea Setup

### Create a Gitea Endpoint

```yaml
apiVersion: garm-operator.mercedes-benz.com/v1beta1
kind: GiteaEndpoint
metadata:
  name: my-gitea
  namespace: garm-operator-system
spec:
  description: "My Gitea instance"
  apiBaseUrl: "https://gitea.example.com/api/v1"
  baseUrl: "https://gitea.example.com"
```

Optional fields:

```yaml
spec:
  caCertBundleSecretRef:
    name: gitea-ca-bundle
    key: ca.crt
  toolsMetadataUrl: "https://gitea.example.com/tools/metadata"
  useInternalToolsMetadata: true
```

Verify the endpoint is ready:

```bash
kubectl -n garm-operator-system get giteaendpoint
```

### Create Gitea Credentials

Gitea only supports PAT authentication (no GitHub App equivalent):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gitea-pat
  namespace: garm-operator-system
data:
  token: <base64-encoded-gitea-token>
---
apiVersion: garm-operator.mercedes-benz.com/v1beta1
kind: GiteaCredential
metadata:
  name: gitea-pat
  namespace: garm-operator-system
spec:
  description: "Gitea PAT credentials"
  endpointRef:
    apiGroup: garm-operator.mercedes-benz.com
    kind: GiteaEndpoint
    name: my-gitea
  authType: pat
  secretRef:
    name: gitea-pat
    key: token
```

Verify:

```bash
kubectl -n garm-operator-system get giteacredential -o wide
```

### Add a Gitea Repository

Gitea repositories use the same `Repository` CRD as GitHub. Point the `credentialsRef` to a `GiteaCredential` instead of a `GitHubCredential`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gitea-repo-webhook-secret
  namespace: garm-operator-system
data:
  webhookSecret: <base64-encoded-webhook-secret>
---
apiVersion: garm-operator.mercedes-benz.com/v1beta1
kind: Repository
metadata:
  name: my-gitea-repo
  namespace: garm-operator-system
spec:
  owner: "my-gitea-org"
  credentialsRef:
    apiGroup: garm-operator.mercedes-benz.com
    kind: GiteaCredential
    name: gitea-pat
  webhookSecretRef:
    name: gitea-repo-webhook-secret
    key: webhookSecret
  # Optional: enable agent mode for runners in this repository
  # agentMode: true
```

You can also use `Organization` or `Enterprise` CRDs with Gitea credentials in the same way. All entity types support the optional `agentMode` field.

## Create a Pool

Pools define runner groups scoped to a Repository, Organization, or Enterprise. First create an `Image` CR, then the `Pool`:

```yaml
apiVersion: garm-operator.mercedes-benz.com/v1beta1
kind: Image
metadata:
  name: ubuntu-2204
  namespace: garm-operator-system
spec:
  tag: linux-ubuntu-22.04
---
apiVersion: garm-operator.mercedes-benz.com/v1beta1
kind: Pool
metadata:
  name: my-pool
  namespace: garm-operator-system
spec:
  githubScopeRef:
    apiGroup: garm-operator.mercedes-benz.com
    kind: Repository        # or Organization, Enterprise
    name: my-repo           # name of the Repository/Organization/Enterprise CR
  providerName: openstack   # must match a provider configured in your GARM server
  imageName: ubuntu-2204    # references the Image CR name
  flavor: small
  osType: linux
  osArch: amd64
  minIdleRunners: 2
  maxRunners: 4
  enabled: true
  runnerBootstrapTimeout: 20
  # Optional: allow shell access on runners (requires agentMode on the parent entity)
  # enableShell: true
  tags:
    - linux
    - ubuntu
    - small
```

The optional `enableShell` field allows shell access on runners in this pool. It only takes effect when `agentMode: true` is set on the parent entity (Repository, Organization, or Enterprise). Without agent mode enabled on the parent, the setting is accepted but has no effect.

This works identically for Gitea-backed repositories. The `githubScopeRef` points to the same `Repository` CR regardless of whether it uses GitHub or Gitea credentials.

Verify the pool was created:

```bash
kubectl -n garm-operator-system get pool
```

## Create a Scale Set

Scale Sets are an alternative to Pools for managing runners. They follow the same scoping model (Repository, Organization, or Enterprise) but use GitHub's Actions Runner Scale Set feature for more efficient runner lifecycle management.

```yaml
apiVersion: garm-operator.mercedes-benz.com/v1beta1
kind: ScaleSet
metadata:
  name: my-scaleset
  namespace: garm-operator-system
spec:
  githubScopeRef:
    apiGroup: garm-operator.mercedes-benz.com
    kind: Repository        # or Organization, Enterprise
    name: my-repo           # name of the Repository/Organization/Enterprise CR
  name: my-scaleset
  providerName: openstack   # must match a provider configured in your GARM server
  imageName: ubuntu-2204    # references the Image CR name
  flavor: small
  osType: linux
  osArch: amd64
  minIdleRunners: 2
  maxRunners: 4
  enabled: true
  runnerBootstrapTimeout: 20
  # Optional fields:
  # enableShell: true         # requires agentMode on the parent entity
  # runnerPrefix: "my-prefix"
  # githubRunnerGroup: ""
  # disableUpdate: false
  # customLabels:             # additional labels for the scale set (create-only)
  #   - self-hosted
  #   - linux
  #   - x64
```

Like Pools, Scale Sets reference a `githubScopeRef` to define which entity (Repository, Organization, or Enterprise) the runners belong to. The `imageName` must reference an existing `Image` CR.

The optional `customLabels` field allows you to specify additional labels for the scale set. The scale set name is always included as a system label automatically. Custom labels can only be set when creating the scale set and are read-only after that — the GitHub API does not support updating labels on an existing scale set.

Verify the scale set was created:

```bash
kubectl -n garm-operator-system get scaleset
```

## Scale Runners

There are two ways to scale the number of idle runners in a pool:

**Option 1: Edit the Pool spec**

```bash
kubectl -n garm-operator-system patch pool my-pool --type merge -p '{"spec":{"minIdleRunners":6}}'
```

**Option 2: Use `kubectl scale`**

Pools support the Kubernetes scale subresource:

```bash
kubectl -n garm-operator-system scale pool my-pool --replicas=6
```

This sets `minIdleRunners` to 6. GARM will then create additional runners to meet the new minimum.

To scale down:

```bash
kubectl -n garm-operator-system scale pool my-pool --replicas=2
```

The operator will remove idle runners older than `minIdleRunnersAge` (default 5 minutes) until the count reaches the target.

To scale to zero (removes all idle runners immediately):

```bash
kubectl -n garm-operator-system scale pool my-pool --replicas=0
```

Note: `minIdleRunners` must always be less than or equal to `maxRunners`.
