package aksutil

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
)

type fakeClusters []*armcontainerservice.ManagedCluster

func (f fakeClusters) List(context.Context, string) ([]*armcontainerservice.ManagedCluster, error) {
	return f, nil
}

func cluster(name, rg, fqdn string) *armcontainerservice.ManagedCluster {
	return &armcontainerservice.ManagedCluster{
		Name:       Ptr(name),
		ID:         Ptr("/subscriptions/s/resourceGroups/" + rg + "/providers/Microsoft.ContainerService/managedClusters/" + name),
		Properties: &armcontainerservice.ManagedClusterProperties{Fqdn: Ptr(fqdn)},
	}
}

func TestResolveTarget(t *testing.T) {
	ctx := context.Background()
	cl := fakeClusters{cluster("one", "rg1", "one-abc.hcp.eastus.azmk8s.io"), cluster("two", "rg2", "two-def.hcp.eastus.azmk8s.io")}

	got, err := ResolveTarget(ctx, cl, Target{}, "two-def.hcp.eastus.azmk8s.io")
	if err != nil || got.Cluster != "two" || got.ResourceGroup != "rg2" {
		t.Errorf("host match: %+v, %v", got, err)
	}
	got, err = ResolveTarget(ctx, cl, Target{Cluster: "ONE"}, "")
	if err != nil || got.ResourceGroup != "rg1" {
		t.Errorf("name match: %+v, %v", got, err)
	}
	got, err = ResolveTarget(ctx, nil, Target{ResourceGroup: "x", Cluster: "y"}, "")
	if err != nil || got.Cluster != "y" {
		t.Errorf("explicit: %+v, %v", got, err)
	}
	if _, err = ResolveTarget(ctx, cl, Target{}, "other.example.com"); err == nil {
		t.Error("expected no-match error")
	}
	if _, err = ResolveTarget(ctx, cl, Target{Cluster: "nope"}, ""); err == nil {
		t.Error("expected not-found error")
	}
}

func TestHostOnly(t *testing.T) {
	for in, want := range map[string]string{
		"https://a.b.io:443": "a.b.io",
		"a.b.io":             "a.b.io",
		"https://a.b.io":     "a.b.io",
	} {
		if got := HostOnly(in); got != want {
			t.Errorf("HostOnly(%q) = %q, want %q", in, got, want)
		}
	}
}
