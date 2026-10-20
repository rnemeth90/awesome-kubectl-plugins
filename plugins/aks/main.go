package main

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/rnemeth90/kubectl-plugins/internal/aksutil"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const example = `  # List AKS clusters in your subscriptions
  kubectl aks list

  # Show / describe the cluster of the current kube context
  kubectl aks show
  kubectl aks describe

  # Upgrade, stop, start, rotate certificates
  kubectl aks upgrade --kubernetes-version 1.30.4
  kubectl aks stop
  kubectl aks start
  kubectl aks rotate-certs

  # Target a specific cluster explicitly
  kubectl aks show --subscription <id> -g my-rg --cluster my-aks`

func main() {
	if err := newCmd(os.Stdin, os.Stdout).Execute(); err != nil {
		os.Exit(1)
	}
}

func newCmd(in io.Reader, out io.Writer) *cobra.Command {
	// --cluster is reserved for the AKS cluster name, so the kubeconfig one is dropped.
	configFlags := genericclioptions.NewConfigFlags(true)
	configFlags.ClusterName = nil
	g := &aksutil.Flags{}

	root := &cobra.Command{
		Use:          "kubectl aks",
		Short:        "Manage AKS clusters",
		Example:      example,
		SilenceUsage: true,
	}
	root.SetOut(out)
	configFlags.AddFlags(root.PersistentFlags())
	g.Register(root.PersistentFlags())

	kubeHost := func() string {
		rc, err := configFlags.ToRESTConfig()
		if err != nil {
			return ""
		}
		return rc.Host
	}
	connect := func(ctx context.Context) (clusterAPI, error) {
		t, cred, err := aksutil.Connect(ctx, g, kubeHost)
		if err != nil {
			return nil, err
		}
		return newAzureCluster(t.Subscription, cred, t.ResourceGroup, t.Cluster)
	}
	run := func(use, short string, fn func(ctx context.Context, api clusterAPI) error) *cobra.Command {
		return &cobra.Command{
			Use: use, Short: short, Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				api, err := connect(cmd.Context())
				if err != nil {
					return err
				}
				return fn(cmd.Context(), api)
			},
		}
	}

	list := &cobra.Command{
		Use: "list", Short: "List AKS clusters", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runList(cmd.Context(), g, out)
		},
	}

	var version string
	upgrade := run("upgrade", "Upgrade the cluster Kubernetes version", func(ctx context.Context, api clusterAPI) error {
		return upgradeCluster(ctx, api, out, version, g.Wait())
	})
	upgrade.Flags().StringVar(&version, "kubernetes-version", "", "Target Kubernetes version")
	_ = upgrade.MarkFlagRequired("kubernetes-version")

	var yes bool
	rotate := run("rotate-certs", "Rotate the cluster certificates", func(ctx context.Context, api clusterAPI) error {
		return rotateCerts(ctx, api, in, out, yes, g.Wait())
	})
	rotate.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the confirmation prompt")

	root.AddCommand(
		list,
		run("show", "Show the cluster", func(ctx context.Context, api clusterAPI) error {
			return showCluster(ctx, api, out, g.Output)
		}),
		run("describe", "Show cluster details and available upgrades", func(ctx context.Context, api clusterAPI) error {
			return describeCluster(ctx, api, out, g.Output)
		}),
		upgrade,
		run("start", "Start a stopped cluster", func(ctx context.Context, api clusterAPI) error {
			return setPower(ctx, api, out, true, g.Wait())
		}),
		run("stop", "Stop the cluster", func(ctx context.Context, api clusterAPI) error {
			return setPower(ctx, api, out, false, g.Wait())
		}),
		rotate,
	)
	return root
}

// runList lists clusters in the given subscription, or in every candidate subscription.
func runList(ctx context.Context, g *aksutil.Flags, out io.Writer) error {
	subs, err := aksutil.CandidateSubscriptions(g.Subscription)
	if err != nil {
		return err
	}
	cred, err := aksutil.NewCredential()
	if err != nil {
		return err
	}
	var all []*armcontainerservice.ManagedCluster
	for _, sub := range subs {
		l, err := aksutil.NewClusters(sub, cred)
		if err != nil {
			return err
		}
		cs, err := l.List(ctx, g.ResourceGroup)
		if err != nil {
			if len(subs) > 1 {
				continue // subscription not accessible with this credential
			}
			return err
		}
		all = append(all, cs...)
	}
	if len(all) == 0 && len(subs) > 1 {
		return errors.New("no AKS clusters found in any accessible subscription")
	}
	return printClusters(out, all, g.Output)
}
