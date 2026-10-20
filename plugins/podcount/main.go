package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

const example = `  # Count all pods in the current namespace
  kubectl podcount

  # Count pods in a specific namespace
  kubectl podcount -n kube-system

  # Count pods belonging to a deployment
  kubectl podcount deployment/web

  # Count pods belonging to a replicaset
  kubectl podcount rs/web-5d8f7c9b6`

const (
	kindDeployment = "deployment"
	kindReplicaSet = "replicaset"
)

func main() {
	if err := newCmd(os.Stdout).Execute(); err != nil {
		os.Exit(1)
	}
}

func newCmd(out io.Writer) *cobra.Command {
	configFlags := genericclioptions.NewConfigFlags(true)

	cmd := &cobra.Command{
		Use:          "kubectl podcount [deployment/NAME | replicaset/NAME]",
		Short:        "Count pods in a namespace, deployment, or replicaset",
		Example:      example,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ns, err := newClient(configFlags)
			if err != nil {
				return err
			}
			resource := ""
			if len(args) == 1 {
				resource = args[0]
			}
			return run(cmd.Context(), client, out, ns, resource)
		},
	}
	configFlags.AddFlags(cmd.Flags())
	return cmd
}

func newClient(configFlags *genericclioptions.ConfigFlags) (kubernetes.Interface, string, error) {
	restConfig, err := configFlags.ToRESTConfig()
	if err != nil {
		return nil, "", err
	}
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, "", err
	}
	ns, _, err := configFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return nil, "", err
	}
	return client, ns, nil
}

// parseResource splits "kind/name" and normalizes kind aliases.
func parseResource(arg string) (kind, name string, err error) {
	k, name, ok := strings.Cut(arg, "/")
	if !ok || name == "" {
		return "", "", fmt.Errorf("invalid resource %q: expected deployment/NAME or replicaset/NAME", arg)
	}
	switch strings.ToLower(k) {
	case "deployment", "deployments", "deploy":
		return kindDeployment, name, nil
	case "replicaset", "replicasets", "rs":
		return kindReplicaSet, name, nil
	default:
		return "", "", fmt.Errorf("unsupported resource kind %q: expected deployment or replicaset", k)
	}
}

// selectorFor returns the pod selector of the named workload.
func selectorFor(ctx context.Context, client kubernetes.Interface, ns, kind, name string) (labels.Selector, error) {
	var sel *metav1.LabelSelector
	switch kind {
	case kindDeployment:
		d, err := client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		sel = d.Spec.Selector
	case kindReplicaSet:
		rs, err := client.AppsV1().ReplicaSets(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		sel = rs.Spec.Selector
	}
	return metav1.LabelSelectorAsSelector(sel)
}

// countPods lists pods matching selector and tallies them by phase.
func countPods(ctx context.Context, client kubernetes.Interface, ns string, selector labels.Selector) (int, map[corev1.PodPhase]int, error) {
	byPhase := map[corev1.PodPhase]int{}
	total := 0
	opts := metav1.ListOptions{LabelSelector: selector.String(), Limit: 500}
	for {
		pods, err := client.CoreV1().Pods(ns).List(ctx, opts)
		if err != nil {
			return 0, nil, err
		}
		for _, p := range pods.Items {
			byPhase[p.Status.Phase]++
			total++
		}
		if pods.Continue == "" {
			return total, byPhase, nil
		}
		opts.Continue = pods.Continue
	}
}

// run counts pods in ns, optionally scoped to a deployment/replicaset (resource).
func run(ctx context.Context, client kubernetes.Interface, out io.Writer, ns, resource string) error {
	selector := labels.Everything()
	scope := fmt.Sprintf("namespace %q", ns)

	if resource != "" {
		kind, name, err := parseResource(resource)
		if err != nil {
			return err
		}
		if selector, err = selectorFor(ctx, client, ns, kind, name); err != nil {
			return err
		}
		scope = fmt.Sprintf("%s %q in namespace %q", kind, name, ns)
	}

	total, byPhase, err := countPods(ctx, client, ns, selector)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Pods in %s: %d\n", scope, total)
	phases := make([]string, 0, len(byPhase))
	for ph := range byPhase {
		phases = append(phases, string(ph))
	}
	sort.Strings(phases)
	for _, ph := range phases {
		fmt.Fprintf(out, "  %-10s %d\n", ph, byPhase[corev1.PodPhase(ph)])
	}
	return nil
}
