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

The operator needs connection details for your GARM server. Edit the manager deployment to set the required arguments:

```bash
kubectl -n garm-operator-system edit deployment garm-operator-controller-manager
```

Set these arguments on the manager container:

```yaml
args:
  - --garm-server=http://garm-server.garm-server.svc:9997
  - --garm-username=admin
  - --garm-password=your-password
  - --operator-watch-namespace=garm-operator-system
```

Alternatively, use environment variables (`GARM_SERVER`, `GARM_USERNAME`, `GARM_PASSWORD`, `OPERATOR_WATCH_NAMESPACE`) or a config file. See the [configuration parsing guide](config/configuration-parsing.md) for all options.

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

For GitHub App authentication, use `authType: app` and provide the app private key, app ID, and installation ID:

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
```

The `name` field (`my-repo`) must match the repository name on GitHub. The `owner` field is the GitHub user or organization that owns the repository.

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
```

You can also use `Organization` or `Enterprise` CRDs with Gitea credentials in the same way.

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
  tags:
    - linux
    - ubuntu
    - small
```

This works identically for Gitea-backed repositories. The `githubScopeRef` points to the same `Repository` CR regardless of whether it uses GitHub or Gitea credentials.

Verify the pool was created:

```bash
kubectl -n garm-operator-system get pool
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
