package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/rnemeth90/kubectl-plugins/internal/kubeutil"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

const example = `  # Images used in the current namespace
  kubectl images

  # Across the cluster
  kubectl images -A

  # Only images using :latest or no tag
  kubectl images -A --latest`

type options struct {
	scope  kubeutil.Scope
	latest bool
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
		Use:          "kubectl images [flags]",
		Short:        "List container images in use and flag :latest or untagged ones",
		Example:      example,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cs, ns, err := kubeutil.NewClient(configFlags)
			if err != nil {
				return err
			}
			return run(cmd.Context(), cs, out, ns, opts)
		},
	}
	configFlags.AddFlags(cmd.Flags())
	cmd.Flags().BoolVarP(&opts.scope.AllNamespaces, "all-namespaces", "A", false, "Search all namespaces")
	cmd.Flags().StringVarP(&opts.scope.Selector, "selector", "l", "", "Label selector to filter pods")
	cmd.Flags().BoolVar(&opts.latest, "latest", false, "Only show images using :latest or no tag")
	return cmd
}

// isMutable reports whether an image reference is unpinned: tagged :latest or
// untagged, and not pinned by digest.
func isMutable(image string) bool {
	if strings.Contains(image, "@") {
		return false
	}
	name := image[strings.LastIndex(image, "/")+1:]
	i := strings.LastIndex(name, ":")
	return i < 0 || name[i+1:] == "latest"
}

type usage struct {
	pods       map[string]bool
	namespaces map[string]bool
}

func collect(pods []corev1.Pod) map[string]*usage {
	byImage := map[string]*usage{}
	for _, p := range pods {
		key := p.Namespace + "/" + p.Name
		add := func(cs []corev1.Container) {
			for _, c := range cs {
				u := byImage[c.Image]
				if u == nil {
					u = &usage{pods: map[string]bool{}, namespaces: map[string]bool{}}
					byImage[c.Image] = u
				}
				u.pods[key] = true
				u.namespaces[p.Namespace] = true
			}
		}
		add(p.Spec.InitContainers)
		add(p.Spec.Containers)
	}
	return byImage
}

func run(ctx context.Context, cs kubernetes.Interface, out io.Writer, ns string, opts options) error {
	pods, err := kubeutil.ListPods(ctx, cs, ns, opts.scope)
	if err != nil {
		return err
	}
	byImage := collect(pods)

	images := make([]string, 0, len(byImage))
	for img := range byImage {
		if opts.latest && !isMutable(img) {
			continue
		}
		images = append(images, img)
	}
	if len(images) == 0 {
		fmt.Fprintln(out, "No images found.")
		return nil
	}
	sort.Slice(images, func(i, j int) bool {
		a, b := byImage[images[i]], byImage[images[j]]
		if len(a.pods) != len(b.pods) {
			return len(a.pods) > len(b.pods)
		}
		return images[i] < images[j]
	})

	tw := tabwriter.NewWriter(out, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "IMAGE\tPODS\tNAMESPACES\tNOTE")
	for _, img := range images {
		u := byImage[img]
		note := ""
		if isMutable(img) {
			note = "mutable tag"
		}
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n", img, len(u.pods), joinKeys(u.namespaces), note)
	}
	return tw.Flush()
}

func joinKeys(m map[string]bool) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > 3 {
		return fmt.Sprintf("%s +%d more", strings.Join(keys[:3], ","), len(keys)-3)
	}
	return strings.Join(keys, ",")
}
