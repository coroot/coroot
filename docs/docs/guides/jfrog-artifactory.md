---
sidebar_position: 2
---

# Using JFrog Artifactory as a registry for Coroot

This guide walks through setting up Coroot in a Kubernetes cluster using JFrog Artifactory as a container image registry.
This is common in air-gapped environments or organizations that require all images to be pulled from an internal registry.

## Prerequisites

- A Kubernetes cluster with `kubectl` and Helm.
- A JFrog Artifactory instance (cloud or self-hosted).

## Step 1: Create a remote Docker repository

Create a **Remote** Docker repository in Artifactory that proxies `ghcr.io/coroot`.
Artifactory will automatically cache images on first pull, so no manual mirroring is required.

1. Go to **Administration** → **Repositories** → **Create a Repository** → **Remote**.

<img alt="Create a repository" src="/img/docs/guides/jfrog/create-repo.png" class="card w-1200"/>

2. Select **Docker** as the package type.
3. Set the **Repository Key** (e.g., `coroot`).
4. Set the **URL** to `https://ghcr.io`.

<img alt="Remote repository configuration" src="/img/docs/guides/jfrog/remote-repo-config.png" class="card w-1200"/>

5. Click **Create Remote Repository**.

## Step 2: Get the Docker client configuration

After creating the repository, Artifactory provides the Docker client configuration with the authentication token.

1. Go to **Application** → **Artifactory** → **Repositories**, find the `coroot` repository, and click **Set Up Client**.
2. Select **Docker** as the client type.

<img alt="Docker client setup" src="/img/docs/guides/jfrog/docker-client-setup.png" class="card w-1200"/>

Copy the JSON configuration shown in the dialog — you'll use it to create the Kubernetes pull secret in the next step.

## Step 3: Create the Kubernetes pull secret

Create a `kubernetes.io/dockerconfigjson` secret in the namespace where Coroot will be deployed,
using the JSON from the previous step:

```bash
cat <<'EOF' | kubectl create secret generic coroot-registry-auth -n coroot \
  --type=kubernetes.io/dockerconfigjson --from-file=.dockerconfigjson=/dev/stdin
{
  "auths": {
    "example.jfrog.io": {
      "auth": "<token from the Artifactory dialog>",
    }
  }
}
EOF
```

## Step 4: Install the operator

Install the Coroot operator with the custom registry configuration:

```bash
helm repo add coroot https://coroot.github.io/helm-charts
helm repo update coroot

helm install -n coroot --create-namespace coroot-operator coroot/coroot-operator \
  --set registry.url=https://example.jfrog.io/coroot \
  --set registry.pullSecret=coroot-registry-auth
```

The operator will now:
- Discover new component versions by querying your Artifactory registry.
- Use your registry for all component image paths.
- Attach the pull secret to all component pods so kubelet can authenticate when pulling images.

## Step 5: Deploy Coroot

Create the Coroot custom resource as usual:

```yaml
apiVersion: coroot.com/v1
kind: Coroot
metadata:
  name: coroot
  namespace: coroot
spec: {}
```

The operator will pull all images through your Artifactory instance. You can verify by checking the pod image references:

```bash
kubectl get pods -n coroot -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[*].image}{"\n"}{end}'
```

All images should point to `example.jfrog.io/coroot/...`.

## Self-signed certificates

If your Artifactory instance uses a self-signed TLS certificate, enable TLS skip verify:

```bash
helm upgrade -n coroot coroot-operator coroot/coroot-operator \
  --set registry.url=https://example.jfrog.io/coroot \
  --set registry.pullSecret=coroot-registry-auth \
  --set registry.tlsSkipVerify=true
```
