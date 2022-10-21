# Single namespace deployment

Before you apply the manifests, you must set the `TS_AUTH_KEY` in `subnet-router.yaml`.

```
NAMESPACE=<tailscale-namespace> # replace with the namespace in which you want to deploy the Tailscale router
kubectl create namespace "${NAMESPACE}"
kubectl -n "${NAMESPACE}" apply -f subnet-router.yaml
```

# Watch for services in an additional namespace

Setup RBAC in the additional namespace:

```
NAMESPACE=<tailscale-namespace> # the namespace in which the Tailscale router runs
TARGET_NAMESPACE=<target-namespace> # the namespace in which you want to discover services
sed "s/TS_NAMESPACE/${NAMESPACE}/" additional_ns_rbac.yaml | \
  kubectl -n "${TARGET_NAMESPACE}" apply f -
```

Patch the router deployment:

```
sed "s/TS_NAMESPACE/${NAMESPACE}/" deploy-patch.json | \
sed "s/ADD_NAMESPACE/${TARGET_NAMESPACE}/" >deploy-patch-${TARGET_NAMESPACE}.json

kubectl -n "${NAMESPACE}" patch deploy/tailscale-namespace-router \
  --type=json \
  --patch-file=deploy-patch-${TARGET_NAMESPACE}.json
```

Please note that the patch expects that the Tailscale router deployment
matches the deployment provided in this directory and will completely remove and replace the existing `TARGET_NAMESPACE` environment variable for the service-observer container with

```
- name: TARGET_NAMESPACE
  value: ${NAMESPACE},${TARGET_NAMESPACE}
```

Please modify the deployment manually (for example with `kubectl edit`) if you want to add multiple additional namespaces.
