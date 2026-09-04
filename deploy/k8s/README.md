# Kubernetes manifests

Plain manifests (no Helm/Kustomize) for running the API itself on a
cluster. **Postgres, Redis, and Jaeger are deliberately not included here**
— running stateful services as raw Kubernetes Deployments is the standard
anti-pattern real clusters avoid; use your cloud provider's managed
Postgres/Redis, or a proper operator/Helm chart for each, and point this
app at them via `configmap.yaml`'s `DB_HOST`/`REDIS_ADDR` and an OTel
Collector for `OTEL_EXPORTER_OTLP_ENDPOINT`.

## Apply order

```bash
kubectl apply -f configmap.yaml
kubectl create secret generic taskflow-api-secrets \
  --from-literal=JWT_SECRET="$(openssl rand -base64 48)" \
  --from-literal=DB_PASSWORD="<real password>"
# (do NOT apply secret.example.yaml directly — it's a template, not a real secret)
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
kubectl apply -f hpa.yaml
kubectl apply -f pdb.yaml
```

## Before this is actually production-ready on your cluster

- Replace the `image:` placeholder in `deployment.yaml` once a container
  registry is configured (the CI pipeline currently builds and scans the
  image but doesn't push anywhere).
- Point `DB_HOST` / `REDIS_ADDR` / `OTEL_EXPORTER_OTLP_ENDPOINT` in
  `configmap.yaml` at your actual managed services.
- `readOnlyRootFilesystem: true` is set in `deployment.yaml`; the app
  writes nothing to disk at runtime, so this should need no further
  changes, but verify against your actual base image/entrypoint if you
  change the Dockerfile.
- Consider a NetworkPolicy restricting egress to just Postgres/Redis/the
  OTel collector, and ingress to just your ingress controller.
