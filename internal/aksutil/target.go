// Package aksutil holds Azure/AKS targeting shared by the AKS plugins.
package aksutil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/spf13/pflag"
)

// Target identifies an AKS cluster.
type Target struct {
	Subscription  string
	ResourceGroup string
	Cluster       string
}

// ErrNoMatch is returned (wrapped) when no cluster matches the target.
var ErrNoMatch = errors.New("no matching AKS cluster")

// CandidateSubscriptions returns the subscriptions to search: just the
// explicit one if given (flag or env), otherwise every subscription in the
// Azure CLI profile with the default first.
func CandidateSubscriptions(flag string) ([]string, error) {
	if flag != "" {
		return []string{flag}, nil
	}
	if v := os.Getenv("AZURE_SUBSCRIPTION_ID"); v != "" {
		return []string{v}, nil
	}
	if home, err := os.UserHomeDir(); err == nil {
		if subs := profileSubscriptions(filepath.Join(home, ".azure", "azureProfile.json")); len(subs) > 0 {
			return subs, nil
		}
	}
	return nil, fmt.Errorf("no subscription: use --subscription, AZURE_SUBSCRIPTION_ID, or `az login`")
}

func profileSubscriptions(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	// The Azure CLI writes this file with a UTF-8 BOM.
	data = []byte(strings.TrimPrefix(string(data), "\ufeff"))
	var profile struct {
		Subscriptions []struct {
			ID        string `json:"id"`
			IsDefault bool   `json:"isDefault"`
		} `json:"subscriptions"`
	}
	if json.Unmarshal(data, &profile) != nil {
		return nil
	}
	var subs []string
	for _, s := range profile.Subscriptions {
		if s.IsDefault {
			subs = append([]string{s.ID}, subs...)
		} else {
			subs = append(subs, s.ID)
		}
	}
	return subs
}

// ResolveTarget fills in the resource group and cluster name. When both are
// given no lookup is needed; otherwise clusters in the subscription are
// matched by name or by the API server host of the current kube context.
func ResolveTarget(ctx context.Context, lister ClusterLister, t Target, apiHost string) (Target, error) {
	if t.ResourceGroup != "" && t.Cluster != "" {
		return t, nil
	}
	clusters, err := lister.List(ctx, t.ResourceGroup)
	if err != nil {
		return t, fmt.Errorf("listing AKS clusters: %w", err)
	}

	var matches []*armcontainerservice.ManagedCluster
	for _, c := range clusters {
		if c.Name == nil {
			continue
		}
		if t.Cluster != "" {
			if strings.EqualFold(*c.Name, t.Cluster) {
				matches = append(matches, c)
			}
		} else if hostMatches(c, apiHost) {
			matches = append(matches, c)
		}
	}

	switch len(matches) {
	case 0:
		if t.Cluster != "" {
			return t, fmt.Errorf("AKS cluster %q not found in subscription %s: %w", t.Cluster, t.Subscription, ErrNoMatch)
		}
		return t, fmt.Errorf("no AKS cluster in subscription %s matches API server %q: %w", t.Subscription, apiHost, ErrNoMatch)
	case 1:
		id, err := arm.ParseResourceID(*matches[0].ID)
		if err != nil {
			return t, err
		}
		t.ResourceGroup = id.ResourceGroupName
		t.Cluster = *matches[0].Name
		return t, nil
	default:
		return t, fmt.Errorf("%d clusters match; narrow with --resource-group", len(matches))
	}
}

func hostMatches(c *armcontainerservice.ManagedCluster, host string) bool {
	if host == "" || c.Properties == nil {
		return false
	}
	for _, f := range []*string{c.Properties.Fqdn, c.Properties.PrivateFQDN, c.Properties.AzurePortalFQDN} {
		if f != nil && strings.EqualFold(*f, host) {
			return true
		}
	}
	return false
}

// ClusterLister lists managed clusters, optionally limited to a resource group.
type ClusterLister interface {
	List(ctx context.Context, resourceGroup string) ([]*armcontainerservice.ManagedCluster, error)
}

// Clusters implements ClusterLister against Azure.
type Clusters struct {
	client *armcontainerservice.ManagedClustersClient
}

// NewClusters creates a ClusterLister for a subscription.
func NewClusters(sub string, cred azcore.TokenCredential) (*Clusters, error) {
	c, err := armcontainerservice.NewManagedClustersClient(sub, cred, nil)
	if err != nil {
		return nil, err
	}
	return &Clusters{client: c}, nil
}

func (a *Clusters) List(ctx context.Context, resourceGroup string) ([]*armcontainerservice.ManagedCluster, error) {
	var out []*armcontainerservice.ManagedCluster
	if resourceGroup != "" {
		pager := a.client.NewListByResourceGroupPager(resourceGroup, nil)
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return nil, err
			}
			out = append(out, page.Value...)
		}
		return out, nil
	}
	pager := a.client.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Value...)
	}
	return out, nil
}

// HostOnly extracts the hostname from an API server URL or host[:port].
func HostOnly(h string) string {
	if !strings.Contains(h, "://") {
		h = "https://" + h
	}
	u, err := url.Parse(h)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// Flags are the Azure targeting and output flags shared by the AKS plugins.
type Flags struct {
	Subscription  string
	ResourceGroup string
	Cluster       string
	Output        string
	NoWait        bool
}

// Register adds the shared flags to fs.
func (f *Flags) Register(fs *pflag.FlagSet) {
	fs.StringVar(&f.Subscription, "subscription", "", "Azure subscription ID (default: AZURE_SUBSCRIPTION_ID, else search all az profile subscriptions)")
	fs.StringVarP(&f.ResourceGroup, "resource-group", "g", "", "Resource group of the AKS cluster")
	fs.StringVar(&f.Cluster, "cluster", "", "AKS cluster name (default: inferred from the current kube context)")
	fs.StringVarP(&f.Output, "output", "o", "table", "Output format: table|json")
	fs.BoolVar(&f.NoWait, "no-wait", false, "Do not wait for long-running operations to finish")
}

// Wait reports whether long-running operations should be awaited.
func (f *Flags) Wait() bool { return !f.NoWait }

// Connect resolves the target cluster, searching candidate subscriptions in
// turn, and returns it with the credential used. kubeHost is only called when
// the cluster must be inferred from the kube context.
func Connect(ctx context.Context, f *Flags, kubeHost func() string) (Target, azcore.TokenCredential, error) {
	subs, err := CandidateSubscriptions(f.Subscription)
	if err != nil {
		return Target{}, nil, err
	}
	cred, err := NewCredential()
	if err != nil {
		return Target{}, nil, err
	}
	host := ""
	if (f.Cluster == "" || f.ResourceGroup == "") && kubeHost != nil {
		host = HostOnly(kubeHost())
	}
	var lastErr error
	for _, sub := range subs {
		clusters, err := NewClusters(sub, cred)
		if err != nil {
			return Target{}, nil, err
		}
		t, err := ResolveTarget(ctx, clusters, Target{sub, f.ResourceGroup, f.Cluster}, host)
		if errors.Is(err, ErrNoMatch) {
			lastErr = err
			continue
		}
		if err != nil {
			return Target{}, nil, err
		}
		return t, cred, nil
	}
	if len(subs) > 1 {
		return Target{}, nil, fmt.Errorf("no AKS cluster found in %d subscriptions (host %q); use --subscription, --cluster and --resource-group", len(subs), host)
	}
	return Target{}, nil, fmt.Errorf("%w; use --subscription, --cluster and --resource-group", lastErr)
}

// Ptr returns a pointer to v.
func Ptr[T any](v T) *T { return &v }

// Str dereferences a string pointer, returning "" for nil.
func Str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// NewCredential returns the default Azure credential chain (az login works).
func NewCredential() (azcore.TokenCredential, error) {
	return azidentity.NewDefaultAzureCredential(nil)
}
