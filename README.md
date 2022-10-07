# camino-k8s-testnet-creator (camktncr) 
(formerly grungni) 
> "grungnis Trunnion!" — Popular Dwarf invocation <br>
> grungnis the Dwarf Ancestor God of mining, artisans and smiths.

This tool creates camino test networks on a k8s cluster

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
- some domain pointing to the lb

# c4t specific tools
- kubectl v1.25.1
- gcloud with gke-gcloud-auth-plugin
- go v1.19.1

# Quickstart
## You have access to the cluster and are allowed to write
Make sure the cluster has the nginx ingress controller and cert manager installed and has a domain pointing there.
The ergonomics of the tool are not final yet, but at the moment the creation of networks is seperated into two steps.
- Creation of genesis block and validator certificates
- Creation of network resources on the cluster e.g. nodes, api-nodes, https endpoints & certificates,

Accomplishing the first step is to run `camktncr generate <network-name>`. That will generate you a default network with 20 certificates that have funds in the genesis block. Check out the help with the `--help` flag to check out how to addjust this.
After that you can create the network with `camktncr k8s create <network-name>`. Also here you can check out the `--help` flag for further help
The networks api nodes will be available under `https://<domain>/<network-name>` and for things that need to be static like keystore operations `https://<domain>/<network-name>/static` will always route to the same node. To test a different version use the `--image` flag to start the nodes with a specific image. The binary will always default to the version it supports the genesis block for. 
When you are done please delete the network via `camktncr k8s delete <network-name>`, be carefull, this gets rid of everything in the namespace. If you only want to delete some parts of the network, use the `kubectl` tool. All relavant resources are properly labeled.

# Caveats
- cluster-issuer for the cert-manager is hardcoded
- the resources are encapsulated by namespace and not threadsafe, please choose names that are not existing already
- the delete operation get rid of the whole namespace, anything else in there will also be deleted
- changes to the genesis block require an update of the testnet creator