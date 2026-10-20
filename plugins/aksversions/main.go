package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/rnemeth90/kubectl-plugins/internal/aksutil"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const example = `  # Kubernetes versions available in the region of the current cluster
  kubectl aks-versions

  # A specific region, including patch versions and their upgrade paths
  kubectl aks-versions --location eastus --patches

  # JSON output
  kubectl aks-versions --location westeurope -o json`

// versionsAPI lists the Kubernetes versions AKS supports in a region.
type versionsAPI interface {
	List(ctx context.Context, location string) ([]*armcontainerservice.KubernetesVersion, error)
}

type azureVersions struct {
	client *armcontainerservice.ManagedClustersClient
}

func (a azureVersions) List(ctx context.Context, location string) ([]*armcontainerservice.KubernetesVersion, error) {
	resp, err := a.client.ListKubernetesVersions(ctx, location, nil)
	if err != nil {
		return nil, err
	}
	return resp.Values, nil
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
	var location string
	var patches bool

	cmd := &cobra.Command{
		Use:          "kubectl aks-versions",
		Short:        "List Kubernetes versions available in an Azure region",
		Example:      example,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			var cred azcore.TokenCredential
			var sub string
			if location == "" {
				t, c, err := aksutil.Connect(ctx, g, func() string {
					rc, err := configFlags.ToRESTConfig()
					if err != nil {
						return ""
					}
					return rc.Host
				})
				if err != nil {
					return fmt.Errorf("%w (or pass --location)", err)
				}
				mcs, err := armcontainerservice.NewManagedClustersClient(t.Subscription, c, nil)
				if err != nil {
					return err
				}
				resp, err := mcs.Get(ctx, t.ResourceGroup, t.Cluster, nil)
				if err != nil {
					return err
				}
				location, cred, sub = aksutil.Str(resp.Location), c, t.Subscription
			} else {
				subs, err := aksutil.CandidateSubscriptions(g.Subscription)
				if err != nil {
					return err
				}
				if cred, err = aksutil.NewCredential(); err != nil {
					return err
				}
				sub = subs[0]
			}
			client, err := armcontainerservice.NewManagedClustersClient(sub, cred, nil)
			if err != nil {
				return err
			}
			return run(ctx, azureVersions{client}, out, location, g.Output, patches)
		},
	}
	configFlags.AddFlags(cmd.Flags())
	g.Register(cmd.Flags())
	cmd.Flags().StringVar(&location, "location", "", "Azure region, e.g. eastus (default: region of the current cluster)")
	cmd.Flags().BoolVar(&patches, "patches", false, "Show patch versions and their upgrade paths")
	return cmd
}

// versionKey orders dotted versions numerically.
func versionKey(v string) []int {
	var out []int
	for _, p := range strings.Split(v, ".") {
		n, _ := strconv.Atoi(p)
		out = append(out, n)
	}
	return out
}

func versionLess(a, b string) bool {
	ka, kb := versionKey(a), versionKey(b)
	for i := 0; i < len(ka) && i < len(kb); i++ {
		if ka[i] != kb[i] {
			return ka[i] < kb[i]
		}
	}
	return len(ka) < len(kb)
}

func flag(b *bool) string {
	if b != nil && *b {
		return "yes"
	}
	return ""
}

func run(ctx context.Context, api versionsAPI, out io.Writer, location, format string, patches bool) error {
	vs, err := api.List(ctx, location)
	if err != nil {
		return err
	}
	sort.Slice(vs, func(i, j int) bool { return versionLess(aksutil.Str(vs[j].Version), aksutil.Str(vs[i].Version)) })
	if format == "json" {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(vs)
	}

	tw := tabwriter.NewWriter(out, 0, 8, 2, ' ', 0)
	if patches {
		fmt.Fprintln(tw, "VERSION\tPATCH\tUPGRADES TO")
		for _, v := range vs {
			pv := make([]string, 0, len(v.PatchVersions))
			for p := range v.PatchVersions {
				pv = append(pv, p)
			}
			sort.Slice(pv, func(i, j int) bool { return versionLess(pv[j], pv[i]) })
			for _, p := range pv {
				var ups []string
				for _, u := range v.PatchVersions[p].Upgrades {
					ups = append(ups, aksutil.Str(u))
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", aksutil.Str(v.Version), p, strings.Join(ups, ", "))
			}
		}
		return tw.Flush()
	}

	fmt.Fprintln(tw, "VERSION\tDEFAULT\tPREVIEW\tSUPPORT PLAN\tPATCHES")
	for _, v := range vs {
		var plans []string
		if v.Capabilities != nil {
			for _, sp := range v.Capabilities.SupportPlan {
				if sp != nil {
					plans = append(plans, string(*sp))
				}
			}
		}
		pv := make([]string, 0, len(v.PatchVersions))
		for p := range v.PatchVersions {
			pv = append(pv, p)
		}
		sort.Slice(pv, func(i, j int) bool { return versionLess(pv[j], pv[i]) })
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", aksutil.Str(v.Version), flag(v.IsDefault), flag(v.IsPreview),
			strings.Join(plans, ","), strings.Join(pv, ", "))
	}
	return tw.Flush()
}
