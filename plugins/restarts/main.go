package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/rnemeth90/kubectl-plugins/internal/kubeutil"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

const example = `  # Pods ranked by restart count in the current namespace
  kubectl restarts

  # Across all namespaces, top 10
  kubectl restarts -A --limit 10

  # Only pods with at least 5 restarts
  kubectl restarts -A --min 5`

type options struct {
	scope kubeutil.Scope
	min   int
	limit int
}

func main() {
	if err := newCmd(os.Stdout).Execute(); err != nil {
		os.Exit(1)
	}
}

func newCmd(out io.Writer) *cobra.Command {
	configFlags := genericclioptions.NewConfigFlags(true)
	var opts options

	cmd := &cobra.Command{
		Use:          "kubectl restarts [flags]",
		Short:        "Rank pods by container restarts, with the last termination reason",
		Example:      example,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cs, ns, err := kubeutil.NewClient(configFlags)
			if err != nil {
				return err
			}
			return run(cmd.Context(), cs, out, ns, opts, time.Now())
		},
	}
	configFlags.AddFlags(cmd.Flags())
	cmd.Flags().BoolVarP(&opts.scope.AllNamespaces, "all-namespaces", "A", false, "Search all namespaces")
	cmd.Flags().StringVarP(&opts.scope.Selector, "selector", "l", "", "Label selector to filter pods")
	cmd.Flags().IntVar(&opts.min, "min", 1, "Only show pods with at least this many restarts")
	cmd.Flags().IntVar(&opts.limit, "limit", 20, "Maximum rows to show (0 for no limit)")
	return cmd
}

type row struct {
	namespace, name string
	restarts        int32
	container       string
	reason          string
	exitCode        int32
	finished        time.Time
}

// summarize totals a pod's restarts and finds its most recent termination.
func summarize(p corev1.Pod) row {
	r := row{namespace: p.Namespace, name: p.Name}
	all := append(append([]corev1.ContainerStatus{}, p.Status.InitContainerStatuses...), p.Status.ContainerStatuses...)
	for _, cs := range all {
		r.restarts += cs.RestartCount
		t := cs.LastTerminationState.Terminated
		if t == nil {
			continue
		}
		if r.container == "" || t.FinishedAt.Time.After(r.finished) {
			r.container = cs.Name
			r.reason = t.Reason
			r.exitCode = t.ExitCode
			r.finished = t.FinishedAt.Time
		}
	}
	return r
}

func ago(t, now time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := now.Sub(t).Round(time.Second)
	switch {
	case d >= 48*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d >= time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d >= time.Minute:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	return fmt.Sprintf("%ds ago", int(d.Seconds()))
}

func run(ctx context.Context, cs kubernetes.Interface, out io.Writer, ns string, opts options, now time.Time) error {
	pods, err := kubeutil.ListPods(ctx, cs, ns, opts.scope)
	if err != nil {
		return err
	}
	var rows []row
	for _, p := range pods {
		if r := summarize(p); r.restarts > 0 && int(r.restarts) >= opts.min {
			rows = append(rows, r)
		}
	}
	if len(rows) == 0 {
		fmt.Fprintln(out, "No pods with restarts.")
		return nil
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].restarts != rows[j].restarts {
			return rows[i].restarts > rows[j].restarts
		}
		if rows[i].namespace != rows[j].namespace {
			return rows[i].namespace < rows[j].namespace
		}
		return rows[i].name < rows[j].name
	})
	if opts.limit > 0 && len(rows) > opts.limit {
		rows = rows[:opts.limit]
	}

	tw := tabwriter.NewWriter(out, 0, 8, 2, ' ', 0)
	if opts.scope.AllNamespaces {
		fmt.Fprint(tw, "NAMESPACE\t")
	}
	fmt.Fprintln(tw, "NAME\tRESTARTS\tCONTAINER\tLAST REASON\tEXIT\tLAST RESTART")
	for _, r := range rows {
		if opts.scope.AllNamespaces {
			fmt.Fprintf(tw, "%s\t", r.namespace)
		}
		reason, exit := r.reason, fmt.Sprint(r.exitCode)
		if r.container == "" {
			reason, exit = "-", "-"
		}
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\t%s\n", r.name, r.restarts, orDash(r.container), orDash(reason), exit, ago(r.finished, now))
	}
	return tw.Flush()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
