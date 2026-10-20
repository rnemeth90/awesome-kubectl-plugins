package main

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
)

// poolAPI is the subset of the AKS agent pool API used by the verbs. Mutating
// calls block until the long-running operation finishes when wait is true.
type poolAPI interface {
	List(ctx context.Context) ([]*armcontainerservice.AgentPool, error)
	Get(ctx context.Context, name string) (*armcontainerservice.AgentPool, error)
	GetUpgradeProfile(ctx context.Context, name string) (*armcontainerservice.AgentPoolUpgradeProfile, error)
	CreateOrUpdate(ctx context.Context, name string, pool armcontainerservice.AgentPool, wait bool) (*armcontainerservice.AgentPool, error)
	Delete(ctx context.Context, name string, wait bool) error
	UpgradeNodeImage(ctx context.Context, name string, wait bool) error
	AbortLatestOperation(ctx context.Context, name string, wait bool) error
}

type azurePools struct {
	client        *armcontainerservice.AgentPoolsClient
	resourceGroup string
	cluster       string
}

func newAzurePools(sub string, cred azcore.TokenCredential, rg, cluster string) (*azurePools, error) {
	c, err := armcontainerservice.NewAgentPoolsClient(sub, cred, nil)
	if err != nil {
		return nil, err
	}
	return &azurePools{client: c, resourceGroup: rg, cluster: cluster}, nil
}

func (a *azurePools) List(ctx context.Context) ([]*armcontainerservice.AgentPool, error) {
	var out []*armcontainerservice.AgentPool
	pager := a.client.NewListPager(a.resourceGroup, a.cluster, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Value...)
	}
	return out, nil
}

func (a *azurePools) Get(ctx context.Context, name string) (*armcontainerservice.AgentPool, error) {
	resp, err := a.client.Get(ctx, a.resourceGroup, a.cluster, name, nil)
	if err != nil {
		return nil, err
	}
	return &resp.AgentPool, nil
}

func (a *azurePools) GetUpgradeProfile(ctx context.Context, name string) (*armcontainerservice.AgentPoolUpgradeProfile, error) {
	resp, err := a.client.GetUpgradeProfile(ctx, a.resourceGroup, a.cluster, name, nil)
	if err != nil {
		return nil, err
	}
	return &resp.AgentPoolUpgradeProfile, nil
}

func (a *azurePools) CreateOrUpdate(ctx context.Context, name string, pool armcontainerservice.AgentPool, wait bool) (*armcontainerservice.AgentPool, error) {
	poller, err := a.client.BeginCreateOrUpdate(ctx, a.resourceGroup, a.cluster, name, pool, nil)
	if err != nil {
		return nil, err
	}
	if !wait {
		return nil, nil
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &resp.AgentPool, nil
}

func (a *azurePools) Delete(ctx context.Context, name string, wait bool) error {
	poller, err := a.client.BeginDelete(ctx, a.resourceGroup, a.cluster, name, nil)
	if err != nil {
		return err
	}
	if !wait {
		return nil
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

func (a *azurePools) UpgradeNodeImage(ctx context.Context, name string, wait bool) error {
	poller, err := a.client.BeginUpgradeNodeImageVersion(ctx, a.resourceGroup, a.cluster, name, nil)
	if err != nil {
		return err
	}
	if !wait {
		return nil
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

func (a *azurePools) AbortLatestOperation(ctx context.Context, name string, wait bool) error {
	poller, err := a.client.BeginAbortLatestOperation(ctx, a.resourceGroup, a.cluster, name, nil)
	if err != nil {
		return err
	}
	if !wait {
		return nil
	}
	_, err = poller.PollUntilDone(ctx, nil)
	return err
}
