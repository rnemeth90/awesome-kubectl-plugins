package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/rnemeth90/kubectl-plugins/internal/kubeutil"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

const example = `  # Unhealthy pods in the current namespace
  kubectl unhealthy

  # Across all namespaces
  kubectl unhealthy -A

  # Only pods matching a label selector
  kubectl unhealthy -l app=web`

func main() {
	if err := newCmd(os.Stdout).Execute(); err != nil {
		os.Exit(1)
	}
}

func newCmd(out io.Writer) *cobra.Command {
	configFlags := genericclioptions.NewConfigFlags(true)
	var scope kubeutil.Scope

	cmd := &cobra.Command{
		Use:          "kubectl unhealthy [flags]",
		Short:        "List pods that are not Ready, crash-looping, pending or failed, with the reason",
		Example:      example,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cs, ns, err := kubeutil.NewClient(configFlags)
			if err != nil {
				return err
			}
			return run(cmd.Context(), cs, out, ns, scope)
		},
	}
	configFlags.AddFlags(cmd.Flags())
	cmd.Flags().BoolVarP(&scope.AllNamespaces, "all-namespaces", "A", false, "Search all namespaces")
	cmd.Flags().StringVarP(&scope.Selector, "selector", "l", "", "Label selector to filter pods")
	return cmd
}

type finding struct {
	namespace, name, ready, status, reason string
}

// diagnose returns a finding for an unhealthy pod, or false when the pod is fine.
func diagnose(p corev1.Pod) (finding, bool) {
	f := finding{namespace: p.Namespace, name: p.Name}
	total := len(p.Spec.Containers)
	ready := 0
	for _, cs := range p.Status.ContainerStatuses {
		if cs.Ready {
			ready++
		}
	}
	f.ready = fmt.Sprintf("%d/%d", ready, total)

	if p.Status.Phase == corev1.PodSucceeded {
		return f, false
	}
	if p.DeletionTimestamp != nil {
		f.status, f.reason = "Terminating", "pod is being deleted"
		return f, true
	}
	if p.Status.Phase == corev1.PodFailed {
		f.status = "Failed"
		f.reason = firstNonEmpty(p.Status.Reason, p.Status.Message)
		return f, true
	}

	statuses := append(append([]corev1.ContainerStatus{}, p.Status.InitContainerStatuses...), p.Status.ContainerStatuses...)
	for _, cs := range statuses {
		if w := cs.State.Waiting; w != nil && w.Reason != "" && w.Reason != "PodInitializing" {
			f.status = w.Reason
			f.reason = fmt.Sprintf("container %s: %s", cs.Name, firstNonEmpty(w.Message, w.Reason))
			return f, true
		}
		if t := cs.State.Terminated; t != nil && t.ExitCode != 0 {
			f.status = firstNonEmpty(t.Reason, "Error")
			f.reason = fmt.Sprintf("container %s exited with code %d", cs.Name, t.ExitCode)
			return f, true
		}
	}

	if p.Status.Phase == corev1.PodPending {
		f.status = "Pending"
		for _, c := range p.Status.Conditions {
			if c.Status == corev1.ConditionFalse && c.Reason != "" {
				f.reason = fmt.Sprintf("%s: %s", c.Reason, c.Message)
				return f, true
			}
		}
		return f, true
	}

	if ready < total {
		f.status = "NotReady"
		for _, c := range p.Status.Conditions {
			if c.Type == corev1.ContainersReady && c.Status == corev1.ConditionFalse {
				f.reason = firstNonEmpty(c.Message, c.Reason)
			}
		}
		if f.reason == "" {
			f.reason = "containers not ready"
		}
		return f, true
	}
	return f, false
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

func run(ctx context.Context, cs kubernetes.Interface, out io.Writer, ns string, scope kubeutil.Scope) error {
	pods, err := kubeutil.ListPods(ctx, cs, ns, scope)
	if err != nil {
		return err
	}
	var found []finding
	for _, p := range pods {
		if f, bad := diagnose(p); bad {
			found = append(found, f)
		}
	}
	if len(found) == 0 {
		fmt.Fprintln(out, "All pods healthy.")
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
		fmt.Fprintln(tw, "NAMESPACE\tNAME\tREADY\tSTATUS\tREASON")
	} else {
		fmt.Fprintln(tw, "NAME\tREADY\tSTATUS\tREASON")
	}
	for _, f := range found {
		if scope.AllNamespaces {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", f.namespace, f.name, f.ready, f.status, f.reason)
		} else {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", f.name, f.ready, f.status, f.reason)
		}
	}
	return tw.Flush()
}
