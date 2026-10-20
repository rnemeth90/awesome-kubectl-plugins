package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/metadata"
)

const example = `  # Count pods in the current namespace
  kubectl count pods

  # Count services in a specific namespace
  kubectl count services -n kube-system

  # Count deployments across all namespaces
  kubectl count deployments -A

  # Count pods matching a label selector
  kubectl count pods -l app=web

  # Cluster-scoped resources work too
  kubectl count nodes`

type options struct {
	allNamespaces bool
	selector      string
}

func main() {
	if err := newCmd(os.Stdout).Execute(); err != nil {
		os.Exit(1)
	}
}

func newCmd(out io.Writer) *cobra.Command {
	configFlags := genericclioptions.NewConfigFlags(true)
	opts := options{}

	cmd := &cobra.Command{
		Use:          "kubectl count RESOURCE [flags]",
		Short:        "Count resources of any type in a namespace or cluster",
		Example:      example,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, mapper, ns, err := newClient(configFlags)
			if err != nil {
				return err
			}
			return run(cmd.Context(), client, mapper, out, ns, args[0], opts)
		},
	}
	configFlags.AddFlags(cmd.Flags())
	cmd.Flags().BoolVarP(&opts.allNamespaces, "all-namespaces", "A", false, "Count across all namespaces")
	cmd.Flags().StringVarP(&opts.selector, "selector", "l", "", "Label selector to filter resources")
	return cmd
}

func newClient(configFlags *genericclioptions.ConfigFlags) (metadata.Interface, meta.RESTMapper, string, error) {
	restConfig, err := configFlags.ToRESTConfig()
	if err != nil {
		return nil, nil, "", err
	}
	client, err := metadata.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, "", err
	}
	mapper, err := configFlags.ToRESTMapper()
	if err != nil {
		return nil, nil, "", err
	}
	ns, _, err := configFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return nil, nil, "", err
	}
	return client, mapper, ns, nil
}

// resolve maps a user-supplied resource name (plural, singular, short name,
// or resource.group) to its REST mapping.
func resolve(mapper meta.RESTMapper, arg string) (*meta.RESTMapping, error) {
	gvr, gr := schema.ParseResourceArg(arg)
	if gvr == nil {
		gvr = &schema.GroupVersionResource{Group: gr.Group, Resource: gr.Resource}
	}
	gvk, err := mapper.KindFor(*gvr)
	if err != nil {
		return nil, fmt.Errorf("unknown resource type %q: %w", arg, err)
	}
	return mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
}

// countResources lists metadata for the resource and tallies it per namespace.
// An empty ns lists across all namespaces (or the whole cluster).
func countResources(ctx context.Context, client metadata.Interface, gvr schema.GroupVersionResource, ns, selector string) (int, map[string]int, error) {
	byNS := map[string]int{}
	total := 0
	opts := metav1.ListOptions{LabelSelector: selector, Limit: 500}
	for {
		list, err := client.Resource(gvr).Namespace(ns).List(ctx, opts)
		if err != nil {
			return 0, nil, err
		}
		for _, item := range list.Items {
			byNS[item.Namespace]++
			total++
		}
		if list.Continue == "" {
			return total, byNS, nil
		}
		opts.Continue = list.Continue
	}
}

func run(ctx context.Context, client metadata.Interface, mapper meta.RESTMapper, out io.Writer, ns, resource string, opts options) error {
	mapping, err := resolve(mapper, resource)
	if err != nil {
		return err
	}

	namespaced := mapping.Scope.Name() == meta.RESTScopeNameNamespace
	listNS := ""
	scope := "cluster"
	switch {
	case !namespaced:
	case opts.allNamespaces:
		scope = "all namespaces"
	default:
		listNS = ns
		scope = fmt.Sprintf("namespace %q", ns)
	}

	total, byNS, err := countResources(ctx, client, mapping.Resource, listNS, opts.selector)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "%s in %s: %d\n", mapping.GroupVersionKind.Kind, scope, total)
	if namespaced && opts.allNamespaces {
		names := make([]string, 0, len(byNS))
		for n := range byNS {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			fmt.Fprintf(out, "  %-30s %d\n", n, byNS[n])
		}
	}
	return nil
}
