## Structure

The `config` folder contains the Kustomize resources that are used to assemble the operator's deployment units

```
.
├── certmanager   # Kustomize options dealing with cert-manager
├── crd           # Kustomize options for our CRDs
│   ├── bases     # auto generated based on the code annotations (`make manifests`)
│   └── patches   # patches to apply to the generated CRD
├── default       # Kustomize's "entry point", generating the distribution YAML file
├── manager       # the operator's Deployment
├── manifests     # the resulting CSV
│   └── bases
├── prometheus    # ServiceMonitor that exposes our operator's metrics
├── rbac          # RBAC rules
├── samples       # Set of examples of how to accomplish specific scenarios. Those are bundled in the final CSV
└── webhook       # Webhook configuration and service
```

## Generated files

`make update` regenerates the files here that are derived from the code (`crd/bases`,
`rbac/role.yaml`, `webhook/manifests.yaml`, the CSV bases under `manifests`), and the
bundles under `bundle/`. Its result depends only on checked-in files.

Nothing else writes to this folder. Targets that need a setting for one run, such as
the operator image for `make deploy` or the operator arguments for the e2e tests,
generate a kustomize overlay under `local/kustomize/`, which git ignores, and build
from that. `make reset` removes those overlays.
