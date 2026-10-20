---
title: "AKS targeting"
description: "Shared flags and cluster discovery for the AKS plugins."
category: Guide
weight: 2
---

`nodepool`, `aks`, `aks-versions` and `aks-maintenance` all talk to Azure's management API and share these flags.

| Flag | Description |
|---|---|
| `--subscription` | Azure subscription ID |
| `-g, --resource-group` | Resource group of the AKS cluster |
| `--cluster` | AKS cluster name |
| `-o, --output` | `table` (default) or `json` |
| `--no-wait` | Return as soon as Azure accepts a long-running operation |

The kubeconfig flags `--context` and `--kubeconfig` also work. The kubeconfig `--cluster` flag is not available because `--cluster` means the AKS cluster name.

## How the cluster is found

If you pass both `-g` and `--cluster`, no lookup is needed. Otherwise the plugin:

1. Reads the API server host of your current kube context.
2. Lists AKS clusters and matches the host against each cluster's FQDN, private FQDN or portal FQDN. If you gave `--cluster`, it matches by name instead.
3. Uses the resource group and name of the single match.

## Which subscriptions are searched

1. `--subscription`, if given.
2. Otherwise `AZURE_SUBSCRIPTION_ID`, if set.
3. Otherwise every subscription in `~/.azure/azureProfile.json`, default first. The first match wins.

If nothing matches, pass `--subscription`, `--cluster` and `-g` explicitly.

## Authentication

The plugins use the Azure SDK's default credential chain, so `az login`, environment variables and managed identity all work.
