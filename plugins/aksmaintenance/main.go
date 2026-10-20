package main

import (
	"context"
	"io"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/rnemeth90/kubectl-plugins/internal/aksutil"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const example = `  # List maintenance configurations of the current cluster
  kubectl aks-maintenance list

  # Weekly node OS upgrade window: Sundays 02:00 for 4 hours
  kubectl aks-maintenance add node-os --day Sunday --start-time 02:00 --duration-hours 4

  # Weekly auto-upgrade window with a UTC offset
  kubectl aks-maintenance add auto-upgrade --day Saturday --start-time 01:00 --utc-offset -05:00

  # Default (planned maintenance) config: Mondays, hours 1 and 2 UTC
  kubectl aks-maintenance add default --day Monday --hours 1,2

  kubectl aks-maintenance show node-os
  kubectl aks-maintenance delete node-os`

type azureMaint struct {
	client        *armcontainerservice.MaintenanceConfigurationsClient
	resourceGroup string
	cluster       string
}

func newAzureMaint(sub string, cred azcore.TokenCredential, rg, cluster string) (*azureMaint, error) {
	c, err := armcontainerservice.NewMaintenanceConfigurationsClient(sub, cred, nil)
	if err != nil {
		return nil, err
	}
	return &azureMaint{client: c, resourceGroup: rg, cluster: cluster}, nil
}

func (a *azureMaint) List(ctx context.Context) ([]*armcontainerservice.MaintenanceConfiguration, error) {
	var out []*armcontainerservice.MaintenanceConfiguration
	pager := a.client.NewListByManagedClusterPager(a.resourceGroup, a.cluster, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Value...)
	}
	return out, nil
}

func (a *azureMaint) Get(ctx context.Context, name string) (*armcontainerservice.MaintenanceConfiguration, error) {
	resp, err := a.client.Get(ctx, a.resourceGroup, a.cluster, name, nil)
	if err != nil {
		return nil, err
	}
	return &resp.MaintenanceConfiguration, nil
}

func (a *azureMaint) CreateOrUpdate(ctx context.Context, name string, cfg armcontainerservice.MaintenanceConfiguration) error {
	_, err := a.client.CreateOrUpdate(ctx, a.resourceGroup, a.cluster, name, cfg, nil)
	return err
}

func (a *azureMaint) Delete(ctx context.Context, name string) error {
	_, err := a.client.Delete(ctx, a.resourceGroup, a.cluster, name, nil)
	return err
}

func main() {
	if err := newCmd(os.Stdout).Execute(); err != nil {
		os.Exit(1)
	}
}

func newCmd(out io.Writer) *cobra.Command {
	configFlags := genericclioptions.NewConfigFlags(true)
	configFlags.ClusterName = nil
	g := &aksutil.Flags{}

	root := &cobra.Command{
		Use:          "kubectl aks-maintenance",
		Short:        "Manage AKS planned maintenance windows",
		Example:      example,
		SilenceUsage: true,
	}
	root.SetOut(out)
	configFlags.AddFlags(root.PersistentFlags())
	g.Register(root.PersistentFlags())

	connect := func(ctx context.Context) (maintAPI, error) {
		t, cred, err := aksutil.Connect(ctx, g, func() string {
			rc, err := configFlags.ToRESTConfig()
			if err != nil {
				return ""
			}
			return rc.Host
		})
		if err != nil {
			return nil, err
		}
		return newAzureMaint(t.Subscription, cred, t.ResourceGroup, t.Cluster)
	}
	verb := func(use, short string, args cobra.PositionalArgs, fn func(ctx context.Context, api maintAPI, name string) error) *cobra.Command {
		return &cobra.Command{
			Use: use, Short: short, Args: args,
			RunE: func(cmd *cobra.Command, a []string) error {
				api, err := connect(cmd.Context())
				if err != nil {
					return err
				}
				name := ""
				if len(a) > 0 {
					name = a[0]
				}
				return fn(cmd.Context(), api, name)
			},
		}
	}

	o := addOptions{}
	add := verb("add NAME", "Create or replace a maintenance configuration (default|auto-upgrade|node-os)", cobra.ExactArgs(1),
		func(ctx context.Context, api maintAPI, n string) error { return addConfig(ctx, api, out, n, o) })
	f := add.Flags()
	f.StringVar(&o.day, "day", "", "Day of week, e.g. Sunday")
	f.StringVar(&o.startTime, "start-time", "02:00", "Window start time HH:mm (auto-upgrade, node-os)")
	f.Int32Var(&o.durationHours, "duration-hours", 4, "Window length in hours, 4-24 (auto-upgrade, node-os)")
	f.Int32Var(&o.intervalWeeks, "interval-weeks", 1, "Weeks between windows (auto-upgrade, node-os)")
	f.StringVar(&o.utcOffset, "utc-offset", "", "UTC offset +/-HH:mm for the start time (auto-upgrade, node-os)")
	f.Int32SliceVar(&o.hours, "hours", nil, "UTC hour slots 0-23 (default configuration)")
	_ = add.MarkFlagRequired("day")

	root.AddCommand(
		verb("list", "List maintenance configurations", cobra.NoArgs, func(ctx context.Context, api maintAPI, _ string) error {
			return listConfigs(ctx, api, out, g.Output)
		}),
		verb("show NAME", "Show a maintenance configuration", cobra.ExactArgs(1), func(ctx context.Context, api maintAPI, n string) error {
			return showConfig(ctx, api, out, n, g.Output)
		}),
		add,
		verb("delete NAME", "Delete a maintenance configuration", cobra.ExactArgs(1), func(ctx context.Context, api maintAPI, n string) error {
			return deleteConfig(ctx, api, out, n)
		}),
	)
	return root
}
