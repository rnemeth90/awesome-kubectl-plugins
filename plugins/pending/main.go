package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/rnemeth90/kubectl-plugins/internal/kubeutil"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

const example = `  # Unschedulable pods in the current namespace
  kubectl pending

  # Across all namespaces
  kubectl pending -A

  # Only pods matching a label selector
  kubectl pending -l app=web`

func main() {
	if err := newCmd(os.Stdout).Execute(); err != nil {
		os.Exit(1)
	}
}

func newCmd(out io.Writer) *cobra.Command {
	configFlags := genericclioptions.NewConfigFlags(true)
	var scope kubeutil.Scope

	cmd := &cobra.Command{
		Use:          "kubectl pending [flags]",
		Short:        "Explain why pods are unschedulable (taints, insufficient resources, affinity)",
		Example:      example,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cs, ns, err := kubeutil.NewClient(configFlags)
			if err != nil {
				return err
			}
			return run(cmd.Context(), cs, out, ns, scope, time.Now())
		},
	}
	configFlags.AddFlags(cmd.Flags())
	cmd.Flags().BoolVarP(&scope.AllNamespaces, "all-namespaces", "A", false, "Search all namespaces")
	cmd.Flags().StringVarP(&scope.Selector, "selector", "l", "", "Label selector to filter pods")
	return cmd
}

type finding struct {
	namespace, name string
	age             time.Duration
	causes          []string
	detail          string
}

// e.g. "0/5 nodes are available: "
var nodesPrefix = regexp.MustCompile(`^\d+/\d+ nodes are available:\s*`)

// classify maps one scheduler reason to a short cause label.
func classify(reason string) string {
	r := strings.ToLower(reason)
	switch {
	case strings.Contains(r, "insufficient"):
		return "Insufficient " + strings.TrimSpace(r[strings.Index(r, "insufficient")+len("insufficient"):])
	case strings.Contains(r, "untolerated taint"):
		return "Taints"
	case strings.Contains(r, "node affinity") || strings.Contains(r, "node selector"):
		return "NodeAffinity"
	case strings.Contains(r, "pod affinity") || strings.Contains(r, "anti-affinity") || strings.Contains(r, "topology spread"):
		return "PodAffinity"
	case strings.Contains(r, "unschedulable"):
		return "Cordoned"
	case strings.Contains(r, "persistentvolumeclaim") || strings.Contains(r, "volume node affinity") || strings.Contains(r, "volume binding"):
		return "Volumes"
	case strings.Contains(r, "ports"):
		return "HostPorts"
	}
	return "Other"
}

// splitReasons splits a scheduler message into per-node-group reasons, dropping the
// "N/M nodes are available" prefix and any trailing preemption note. Commas inside
// braces (taints) do not split.
func splitReasons(msg string) []string {
	if i := strings.Index(msg, "preemption:"); i >= 0 {
		msg = msg[:i]
	}
	msg = strings.TrimSpace(msg)
	msg = strings.TrimSuffix(strings.TrimSpace(nodesPrefix.ReplaceAllString(msg, "")), ".")

	var parts []string
	depth, start := 0, 0
	for i, c := range msg {
		switch c {
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, msg[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, msg[start:])

	var out []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// causes returns the distinct cause labels for a scheduler message, in order of first appearance.
func causes(msg string) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range splitReasons(msg) {
		if c := classify(r); !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}

// diagnose returns a finding for a pod the scheduler could not place, or false otherwise.
func diagnose(p corev1.Pod, now time.Time) (finding, bool) {
	if p.Status.Phase != corev1.PodPending || p.Spec.NodeName != "" || p.DeletionTimestamp != nil {
		return finding{}, false
	}
	for _, c := range p.Status.Conditions {
		if c.Type == corev1.PodScheduled && c.Status == corev1.ConditionFalse && c.Reason == corev1.PodReasonUnschedulable {
			f := finding{namespace: p.Namespace, name: p.Name, detail: c.Message, causes: causes(c.Message)}
			if len(f.causes) == 0 {
				f.causes = []string{"Other"}
			}
			if !p.CreationTimestamp.IsZero() {
				f.age = now.Sub(p.CreationTimestamp.Time)
			}
			return f, true
		}
	}
	return finding{}, false
}

func humanAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func run(ctx context.Context, cs kubernetes.Interface, out io.Writer, ns string, scope kubeutil.Scope, now time.Time) error {
	pods, err := kubeutil.ListPods(ctx, cs, ns, scope)
	if err != nil {
		return err
	}
	var found []finding
	for _, p := range pods {
		if f, bad := diagnose(p, now); bad {
			found = append(found, f)
		}
	}
	if len(found) == 0 {
		fmt.Fprintln(out, "No unschedulable pods.")
		return nil
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].namespace != found[j].namespace {
			return found[i].namespace < found[j].namespace
		}
		return found[i].name < found[j].name
	})

	tw := tabwriter.NewWriter(out, 0, 8, 2, ' ', 0)
	if scope.AllNamespaces {
		fmt.Fprintln(tw, "NAMESPACE\tNAME\tAGE\tCAUSE\tDETAIL")
	} else {
		fmt.Fprintln(tw, "NAME\tAGE\tCAUSE\tDETAIL")
	}
	for _, f := range found {
		cause, detail := strings.Join(f.causes, ","), strings.Join(splitReasons(f.detail), "; ")
		if scope.AllNamespaces {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", f.namespace, f.name, humanAge(f.age), cause, detail)
		} else {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", f.name, humanAge(f.age), cause, detail)
		}
	}
	return tw.Flush()
}
