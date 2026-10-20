package main

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
)

// clusterAPI is the subset of the AKS managed cluster API used by the verbs.
// Mutating calls block until the long-running operation finishes when wait is true.
type clusterAPI interface {
	Get(ctx context.Context) (*armcontainerservice.ManagedCluster, error)
	GetUpgradeProfile(ctx context.Context) (*armcontainerservice.ManagedClusterUpgradeProfile, error)
	CreateOrUpdate(ctx context.Context, mc armcontainerservice.ManagedCluster, wait bool) error
	Start(ctx context.Context, wait bool) error
	Stop(ctx context.Context, wait bool) error
	RotateCertificates(ctx context.Context, wait bool) error
}

type azureCluster struct {
	client        *armcontainerservice.ManagedClustersClient
	resourceGroup string
	name          string
}

func newAzureCluster(sub string, cred azcore.TokenCredential, rg, name string) (*azureCluster, error) {
	c, err := armcontainerservice.NewManagedClustersClient(sub, cred, nil)
	if err != nil {
		return nil, err
	}
	return &azureCluster{client: c, resourceGroup: rg, name: name}, nil
}

func (a *azureCluster) Get(ctx context.Context) (*armcontainerservice.ManagedCluster, error) {
	resp, err := a.client.Get(ctx, a.resourceGroup, a.name, nil)
	if err != nil {
		return nil, err
	}
	return &resp.ManagedCluster, nil
}

func (a *azureCluster) GetUpgradeProfile(ctx context.Context) (*armcontainerservice.ManagedClusterUpgradeProfile, error) {
	resp, err := a.client.GetUpgradeProfile(ctx, a.resourceGroup, a.name, nil)
	if err != nil {
		return nil, err
	}
	return &resp.ManagedClusterUpgradeProfile, nil
}

func (a *azureCluster) CreateOrUpdate(ctx context.Context, mc armcontainerservice.ManagedCluster, wait bool) error {
	p, err := a.client.BeginCreateOrUpdate(ctx, a.resourceGroup, a.name, mc, nil)
	if err != nil {
		return err
	}
	if !wait {
		return nil
	}
	_, err = p.PollUntilDone(ctx, nil)
	return err
}

func (a *azureCluster) Start(ctx context.Context, wait bool) error {
	p, err := a.client.BeginStart(ctx, a.resourceGroup, a.name, nil)
	if err != nil {
		return err
	}
	if !wait {
		return nil
	}
	_, err = p.PollUntilDone(ctx, nil)
	return err
}

func (a *azureCluster) Stop(ctx context.Context, wait bool) error {
	p, err := a.client.BeginStop(ctx, a.resourceGroup, a.name, nil)
	if err != nil {
		return err
	}
	if !wait {
		return nil
	}
	_, err = p.PollUntilDone(ctx, nil)
	return err
}

func (a *azureCluster) RotateCertificates(ctx context.Context, wait bool) error {
	p, err := a.client.BeginRotateClusterCertificates(ctx, a.resourceGroup, a.name, nil)
	if err != nil {
		return err
	}
	if !wait {
		return nil
	}
	_, err = p.PollUntilDone(ctx, nil)
	return err
}
