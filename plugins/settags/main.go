package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/rnemeth90/kubectl-plugins/internal/kubeutil"
	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

const example = `  # Update two deployments in every namespace
  kubectl set-tags api=v1.4.2 web=v2.0.0

  # Read pairs from a file (one name=tag per line)
  kubectl set-tags -f tags.txt

  # Only some namespaces, preview first
  kubectl set-tags -n team-a,team-b --dry-run api=v1.4.2`

var tagRe = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$`)

type options struct {
	namespaces []string // empty means all namespaces
	file       string
	dryRun     bool
	yes        bool
}

// change is one container image update.
type change struct {
	Namespace, Deployment, Container, Old, New string
}

func main() {
	if err := newCmd(os.Stdout, os.Stdin).Execute(); err != nil {
		os.Exit(1)
	}
}

func newCmd(out io.Writer, in io.Reader) *cobra.Command {
	configFlags := genericclioptions.NewConfigFlags(true)
	var opts options
	var allNS bool

	cmd := &cobra.Command{
		Use:          "kubectl set-tags [flags] NAME=TAG [NAME=TAG ...]",
		Short:        "Update image tags for named deployments across namespaces",
		Long:         "Update image tags for deployments by name across many namespaces. Only the tag changes; the image repository is kept. Searches every namespace unless -n is given (comma-separated list allowed).",
		Example:      example,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !allNS && cmd.Flags().Changed("namespace") && configFlags.Namespace != nil {
				for _, n := range strings.Split(*configFlags.Namespace, ",") {
					if n = strings.TrimSpace(n); n != "" {
						opts.namespaces = append(opts.namespaces, n)
					}
				}
			}
			tags, err := collectTags(args, opts.file)
			if err != nil {
				return err
			}
			cs, _, err := kubeutil.NewClient(configFlags)
			if err != nil {
				return err
			}
			return run(cmd.Context(), cs, out, in, tags, opts)
		},
	}
	configFlags.AddFlags(cmd.Flags())
	cmd.Flags().BoolVarP(&allNS, "all-namespaces", "A", false, "All namespaces (the default; overrides -n)")
	cmd.Flags().StringVarP(&opts.file, "file", "f", "", "File with one NAME=TAG per line (# comments allowed)")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Show the planned changes and exit")
	cmd.Flags().BoolVarP(&opts.yes, "yes", "y", false, "Do not prompt for confirmation")
	return cmd
}

// collectTags merges NAME=TAG args with pairs read from file.
func collectTags(args []string, file string) (map[string]string, error) {
	pairs := append([]string(nil), args...)
	if file != "" {
		f, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		fp, err := readPairs(f)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, fp...)
	}
	return parsePairs(pairs)
}

func readPairs(r io.Reader) ([]string, error) {
	var pairs []string
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		if line = strings.TrimSpace(line); line != "" {
			pairs = append(pairs, line)
		}
	}
	return pairs, sc.Err()
}

func parsePairs(pairs []string) (map[string]string, error) {
	if len(pairs) == 0 {
		return nil, fmt.Errorf("no NAME=TAG pairs given")
	}
	tags := map[string]string{}
	for _, p := range pairs {
		name, tag, ok := strings.Cut(p, "=")
		name, tag = strings.TrimSpace(name), strings.TrimSpace(tag)
		if !ok || name == "" || tag == "" {
			return nil, fmt.Errorf("invalid pair %q (expected NAME=TAG)", p)
		}
		if !tagRe.MatchString(tag) {
			return nil, fmt.Errorf("invalid tag in %q", p)
		}
		if old, dup := tags[name]; dup && old != tag {
			return nil, fmt.Errorf("conflicting tags for %q: %s and %s", name, old, tag)
		}
		tags[name] = tag
	}
	return tags, nil
}

// imageRepo strips any tag or digest from an image reference.
func imageRepo(image string) string {
	if i := strings.Index(image, "@"); i >= 0 {
		image = image[:i]
	}
	slash := strings.LastIndex(image, "/")
	if c := strings.LastIndex(image, ":"); c > slash {
		image = image[:c]
	}
	return image
}

// plan computes the container updates for deployments whose name has a tag.
// Containers are updated when they share the repository of the first one.
func plan(deps []appsv1.Deployment, tags map[string]string) []change {
	var changes []change
	for _, d := range deps {
		tag, ok := tags[d.Name]
		cs := d.Spec.Template.Spec.Containers
		if !ok || len(cs) == 0 {
			continue
		}
		base := imageRepo(cs[0].Image)
		for _, c := range cs {
			if imageRepo(c.Image) != base {
				continue
			}
			changes = append(changes, change{d.Namespace, d.Name, c.Name, c.Image, base + ":" + tag})
		}
	}
	sort.SliceStable(changes, func(i, j int) bool {
		a, b := changes[i], changes[j]
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		if a.Deployment != b.Deployment {
			return a.Deployment < b.Deployment
		}
		return a.Container < b.Container
	})
	return changes
}

func listDeployments(ctx context.Context, cs kubernetes.Interface, namespaces []string) ([]appsv1.Deployment, error) {
	targets := namespaces
	if len(targets) == 0 {
		targets = []string{metav1.NamespaceAll}
	}
	var deps []appsv1.Deployment
	for _, ns := range targets {
		opts := metav1.ListOptions{Limit: 500}
		for {
			list, err := cs.AppsV1().Deployments(ns).List(ctx, opts)
			if err != nil {
				return nil, err
			}
			deps = append(deps, list.Items...)
			if list.Continue == "" {
				break
			}
			opts.Continue = list.Continue
		}
	}
	return deps, nil
}

func apply(ctx context.Context, cs kubernetes.Interface, c change) error {
	patch, err := json.Marshal(map[string]any{"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
		"containers": []map[string]string{{"name": c.Container, "image": c.New}},
	}}}})
	if err != nil {
		return err
	}
	_, err = cs.AppsV1().Deployments(c.Namespace).Patch(ctx, c.Deployment, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	return err
}

func run(ctx context.Context, cs kubernetes.Interface, out io.Writer, in io.Reader, tags map[string]string, opts options) error {
	deps, err := listDeployments(ctx, cs, opts.namespaces)
	if err != nil {
		return err
	}

	found := map[string]bool{}
	for _, d := range deps {
		found[d.Name] = true
	}
	var missing []string
	for name := range tags {
		if !found[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	for _, name := range missing {
		fmt.Fprintf(out, "warning: no deployment named %q found\n", name)
	}

	changes := plan(deps, tags)
	if len(changes) == 0 {
		fmt.Fprintln(out, "Nothing to update.")
		return nil
	}

	tw := tabwriter.NewWriter(out, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "NAMESPACE\tDEPLOYMENT\tCONTAINER\tCURRENT\tNEW")
	for _, c := range changes {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", c.Namespace, c.Deployment, c.Container, c.Old, c.New)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if opts.dryRun {
		return nil
	}

	if !opts.yes {
		fmt.Fprint(out, "Apply these changes? [y/N] ")
		ans, _ := bufio.NewReader(in).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(ans)); a != "y" && a != "yes" {
			return fmt.Errorf("aborted")
		}
	}

	failed := 0
	for _, c := range changes {
		if err := apply(ctx, cs, c); err != nil {
			failed++
			fmt.Fprintf(out, "FAILED  %s/%s (%s): %v\n", c.Namespace, c.Deployment, c.Container, err)
			continue
		}
		fmt.Fprintf(out, "updated %s/%s (%s) -> %s\n", c.Namespace, c.Deployment, c.Container, c.New)
	}
	if failed > 0 {
		return fmt.Errorf("%d update(s) failed", failed)
	}
	return nil
}
