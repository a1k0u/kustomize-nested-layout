# Kustomize Nested Layout

> Caution. Work in progress. Just proof of concept. Use at your own risk.

### Layout

Help to support layout with nested overlays without extra base directories.

```
kubernetes
├── development
│   ├── betas
│   │   └── kustomization.yaml
│   ├── testing
│   │   ├── deployment.yml
│   │   └── kustomization.yaml
│   ├── deployment.yml
│   ├── hpa_scaled_object.yml
│   └── kustomization.yaml
├── production
│   ├── cron-cluster
│   │   ├── us-east-2
│   │   │   ├── deployment.yml
│   │   │   └── kustomization.yaml
│   │   ├── deployment.yml
│   │   └── kustomization.yaml
│   ├── deployment.yml
│   ├── hpa_scaled_object.yml
│   └── kustomization.yaml
├── deployment.yml
├── hpa_scaled_object.yml
├── ingress.yml
├── service.yml
└── kustomization.yaml
```

Build `kubernetes/development/betas` with kustomize:
```
go run main.go --root kubernetes --build kubernetes/development/betas
```

### Generation

Help to generate `resources` and `patches` fields in `kustomization.yaml` files if they are empty, or if no file exists, it will be created.

```
kubernetes
├── development
│   ├── betas
│   │   └── kustomization.yaml
│   ├── testing
│   │   ├── deployment.yml
│   │   └── kustomization.yaml
│   ├── deployment.yml
│   ├── hpa_scaled_object.yml
│   └── kustomization.yaml
├── production
│   ├── cron-cluster
│   │   ├── us-east-2
│   │   │   └── deployment.yml
│   │   └── deployment.yml
│   ├── deployment.yml
│   └── hpa_scaled_object.yml
├── deployment.yml
├── hpa_scaled_object.yml
├── ingress.yml
└── service.yml
```

If it is root kustomization, it will generate `resources` field with YAML files from the same directory.
Otherwise, it will be inherited from nearest parent kustomization.

If it is not root kustomization, it will generate `patches` field with YAML files from the same directory.

```
go run main.go --root kubernetes --build kubernetes/production/cron-cluster --generate-resources --generate-patches
```
