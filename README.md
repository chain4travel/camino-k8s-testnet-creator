# Grungni
> "Grungni's Trunnion!" — Popular Dwarf invocation <br>
> Grungni is the Dwarf Ancestor God of mining, artisans and smiths.


This tool creates camino test networks on a camino cluster

# k8s requirements
- access to a k8s cluster with the following access (get, watch, list, create, delete)
    - Namespaces
    - ConfigMaps
    - Secrets
    - StatefulSets
    - Ingress
    - Services
- ngnix ingress controller (will be generic in the future, just now its easier to hardcode pathe resolution :eyes: -> "Ingress Gateways")
- cert managaer installed to resolve certificate requests

# c4t specific tools
- kubectl v1.25.1
- gcloud with gke-gcloud-auth-plugin
- go v1.19.1