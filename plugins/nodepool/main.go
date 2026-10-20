package main

import (
	"context"
	"io"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/rnemeth90/kubectl-plugins/internal/aksutil"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const example = `  # List node pools of the cluster in the current kube context
  kubectl nodepool list

  # Show one node pool as JSON
  kubectl nodepool show userpool -o json

  # Add a node pool with the autoscaler
  kubectl nodepool add gpu --vm-size Standard_NC6s_v3 --enable-cluster-autoscaler --min-count 1 --max-count 5

  # Taint a pool (trailing '-' removes)
  kubectl nodepool taint gpu sku=gpu:NoSchedule
  kubectl nodepool taint gpu sku-
  kubectl nodepool untaint gpu sku:NoSchedule

  # Scale, upgrade, stop
  kubectl nodepool scale userpool --count 5
  kubectl nodepool upgrade userpool --kubernetes-version 1.30.4
  kubectl nodepool stop userpool

  # Target a specific cluster explicitly
  kubectl nodepool list --subscription <id> -g my-rg --cluster my-aks`

func main() {
	if err := newCmd(os.Stdin, os.Stdout).Execute(); err != nil {
		os.Exit(1)
	}
}

func newCmd(in io.Reader, out io.Writer) *cobra.Command {
	// Only kubeconfig/context flags are exposed; --cluster is reserved for the AKS cluster name.
	configFlags := genericclioptions.NewConfigFlags(true)
	configFlags.ClusterName = nil
	g := &aksutil.Flags{}

	root := &cobra.Command{
		Use:          "kubectl nodepool",
		Short:        "Manage AKS node pools",
		Example:      example,
		SilenceUsage: true,
	}
	root.SetOut(out)
	configFlags.AddFlags(root.PersistentFlags())
	g.Register(root.PersistentFlags())

	connect := func(ctx context.Context) (poolAPI, error) {
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
		return newAzurePools(t.Subscription, cred, t.ResourceGroup, t.Cluster)
	}

	// verb wraps a one-pool-name command with connection setup.
	verb := func(use, short string, args cobra.PositionalArgs, fn func(ctx context.Context, api poolAPI, name string) error) *cobra.Command {
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
	wait := g.Wait

	root.AddCommand(
		verb("list", "List node pools", cobra.NoArgs, func(ctx context.Context, api poolAPI, _ string) error {
			return listPools(ctx, api, out, g.Output)
		}),
		verb("show NAME", "Show a node pool", cobra.ExactArgs(1), func(ctx context.Context, api poolAPI, n string) error {
			return showPool(ctx, api, out, n, g.Output)
		}),
		verb("describe NAME", "Show detailed node pool properties and available upgrades", cobra.ExactArgs(1), func(ctx context.Context, api poolAPI, n string) error {
			return describePool(ctx, api, out, n, g.Output)
		}),
		verb("upgrade-node-image NAME", "Upgrade the node image of a pool", cobra.ExactArgs(1), func(ctx context.Context, api poolAPI, n string) error {
			return upgradeNodeImage(ctx, api, out, n, wait())
		}),
		verb("start NAME", "Start a stopped pool", cobra.ExactArgs(1), func(ctx context.Context, api poolAPI, n string) error {
			return setPower(ctx, api, out, n, armcontainerservice.CodeRunning, wait())
		}),
		verb("stop NAME", "Stop a pool", cobra.ExactArgs(1), func(ctx context.Context, api poolAPI, n string) error {
			return setPower(ctx, api, out, n, armcontainerservice.CodeStopped, wait())
		}),
		verb("cancel-operation NAME", "Cancel the latest operation on a pool", cobra.ExactArgs(1), func(ctx context.Context, api poolAPI, n string) error {
			return cancelOperation(ctx, api, out, n, wait())
		}),
	)

	// Commands with their own flags.
	var count int32
	scale := verb("scale NAME", "Set the node count of a pool", cobra.ExactArgs(1), func(ctx context.Context, api poolAPI, n string) error {
		return scalePool(ctx, api, out, n, count, wait())
	})
	scale.Flags().Int32Var(&count, "count", 0, "Desired node count")
	_ = scale.MarkFlagRequired("count")

	var version string
	upgrade := verb("upgrade NAME", "Upgrade the Kubernetes version of a pool", cobra.ExactArgs(1), func(ctx context.Context, api poolAPI, n string) error {
		return upgradePool(ctx, api, out, n, version, wait())
	})
	upgrade.Flags().StringVar(&version, "kubernetes-version", "", "Target Kubernetes version")
	_ = upgrade.MarkFlagRequired("kubernetes-version")

	add := addOptions{}
	addCmd := verb("add NAME", "Add a node pool", cobra.ExactArgs(1), func(ctx context.Context, api poolAPI, n string) error {
		return addPool(ctx, api, out, n, add, wait())
	})
	af := addCmd.Flags()
	af.StringVar(&add.vmSize, "vm-size", "Standard_DS2_v2", "VM size")
	af.Int32Var(&add.count, "count", 3, "Initial node count")
	af.StringVar(&add.mode, "mode", "User", "Pool mode: User|System")
	af.StringVar(&add.version, "kubernetes-version", "", "Kubernetes version (default: cluster version)")
	af.StringVar(&add.labels, "labels", "", "Node labels: key=value,key2=value2")
	af.StringVar(&add.taints, "taints", "", "Node taints: key=value:NoSchedule,...")
	af.BoolVar(&add.autoscale, "enable-cluster-autoscaler", false, "Enable the cluster autoscaler")
	af.Int32Var(&add.minCount, "min-count", 0, "Autoscaler minimum node count")
	af.Int32Var(&add.maxCount, "max-count", 0, "Autoscaler maximum node count")
	af.StringVar(&add.maxSurge, "max-surge", "", "Max surge during upgrades, e.g. 33% or 2")
	af.StringVar(&add.osType, "os-type", "", "OS type: Linux|Windows")
	af.Int32Var(&add.maxPods, "max-pods", 0, "Maximum pods per node")
	af.StringSliceVar(&add.zones, "zones", nil, "Availability zones, e.g. 1,2,3")
	af.StringVar(&add.subnetID, "vnet-subnet-id", "", "Subnet resource ID")
	af.Int32Var(&add.osDiskSizeG, "os-disk-size", 0, "OS disk size in GB")

	taint := &cobra.Command{
		Use:   "taint NAME KEY=VALUE:EFFECT... | KEY[:EFFECT]-...",
		Short: "Add or remove node taints on a pool (kubectl taint syntax)",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, a []string) error {
			api, err := connect(cmd.Context())
			if err != nil {
				return err
			}
			return taintPool(cmd.Context(), api, out, a[0], a[1:], wait())
		},
	}

	untaint := &cobra.Command{
		Use:   "untaint NAME KEY[:EFFECT]...",
		Short: "Remove node taints from a pool",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, a []string) error {
			api, err := connect(cmd.Context())
			if err != nil {
				return err
			}
			return untaintPool(cmd.Context(), api, out, a[0], a[1:], wait())
		},
	}

	var yes bool
	del := verb("delete NAME", "Delete a node pool", cobra.ExactArgs(1), func(ctx context.Context, api poolAPI, n string) error {
		return deletePool(ctx, api, in, out, n, yes, wait())
	})
	del.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the confirmation prompt")

	var up updateOptions
	var minC, maxC int32
	var labels, taints, surge, pmode string
	updCmd := &cobra.Command{Use: "update NAME", Short: "Update autoscaler, labels, taints, surge, or mode", Args: cobra.ExactArgs(1)}
	updCmd.RunE = func(cmd *cobra.Command, a []string) error {
		f := cmd.Flags()
		up.minCount, up.maxCount, up.labels, up.taints, up.maxSurge, up.mode = nil, nil, nil, nil, nil, nil
		if f.Changed("min-count") {
			up.minCount = &minC
		}
		if f.Changed("max-count") {
			up.maxCount = &maxC
		}
		if f.Changed("labels") {
			up.labels = &labels
		}
		if f.Changed("taints") {
			up.taints = &taints
		}
		if f.Changed("max-surge") {
			up.maxSurge = &surge
		}
		if f.Changed("mode") {
			up.mode = &pmode
		}
		api, err := connect(cmd.Context())
		if err != nil {
			return err
		}
		return updatePool(cmd.Context(), api, out, a[0], up, wait())
	}
	uf := updCmd.Flags()
	uf.BoolVar(&up.enableAutoscaler, "enable-cluster-autoscaler", false, "Enable the cluster autoscaler")
	uf.BoolVar(&up.disableAutoscaler, "disable-cluster-autoscaler", false, "Disable the cluster autoscaler")
	uf.Int32Var(&minC, "min-count", 0, "Autoscaler minimum node count")
	uf.Int32Var(&maxC, "max-count", 0, "Autoscaler maximum node count")
	uf.StringVar(&labels, "labels", "", "Replace node labels: key=value,... (empty clears)")
	uf.StringVar(&taints, "taints", "", "Replace node taints: key=value:Effect,... (empty clears)")
	uf.StringVar(&surge, "max-surge", "", "Max surge during upgrades")
	uf.StringVar(&pmode, "mode", "", "Pool mode: User|System")

	root.AddCommand(scale, upgrade, addCmd, del, updCmd, taint, untaint)
	return root
}
