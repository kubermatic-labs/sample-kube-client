# Kubermatic Kubernetes client example

This repository provides an example of how to create a KKP cluster at a GCP cloud provider using the Kubernetes API.

NOTE: The example works only with KKP 2.21, and it's a TECHNOLOGY PREVIEW.
Not all features are supported and bugs can occur. 

Steps the client executes:
1. Creates a project
2. Creates a user cluster
3. Initializes a user cluster client 
4. Creates two nodes
5. Creates a nginx pod

# How to run the client

```bash
go run . \
--gcp-service-account=<path-to-sa> \
--gcp-network=<gcp-network> \
--gcp-subnet=<gcp-subnet> \
--seed-kubeconfig=<path-to-seed-kubeconfig>
```
