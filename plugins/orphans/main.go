package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

const example = `  # Find unused ConfigMaps, Secrets, PVCs and Services in the current namespace
  kubectl orphans

  # Across all namespaces
  kubectl orphans -A

  # Only Secrets and PVCs
  kubectl orphans --type secrets,pvcs`

var allTypes = []string{"configmaps", "secrets", "pvcs", "services"}

type options struct {
	allNamespaces bool
	types         []string
}

type orphan struct {
	Namespace string
	Kind      string
	Name      string
	Reason    string
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
		Use:          "kubectl orphans [flags]",
		Short:        "Find unused ConfigMaps, Secrets, PVCs and Services with no endpoints",
		Example:      example,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, t := range opts.types {
				if !contains(allTypes, t) {
					return fmt.Errorf("unknown type %q (valid: %s)", t, strings.Join(allTypes, ", "))
				}
			}
			client, ns, err := newClient(configFlags)
			if err != nil {
				return err
			}
			return run(cmd.Context(), client, out, ns, opts)
		},
	}
	configFlags.AddFlags(cmd.Flags())
	cmd.Flags().BoolVarP(&opts.allNamespaces, "all-namespaces", "A", false, "Scan all namespaces")
	cmd.Flags().StringSliceVar(&opts.types, "type", allTypes, "Resource types to check: configmaps,secrets,pvcs,services")
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

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// refSet records "namespace/name" keys referenced by pods.
type refSet map[string]bool

func key(ns, name string) string { return ns + "/" + name }

type refs struct {
	configMaps, secrets, pvcs refSet
}

// collectRefs walks every pod and records the ConfigMaps, Secrets and PVCs it uses.
func collectRefs(pods []corev1.Pod) refs {
	r := refs{refSet{}, refSet{}, refSet{}}
	for _, p := range pods {
		ns := p.Namespace
		for _, s := range p.Spec.ImagePullSecrets {
			r.secrets[key(ns, s.Name)] = true
		}
		for _, v := range p.Spec.Volumes {
			if v.ConfigMap != nil {
				r.configMaps[key(ns, v.ConfigMap.Name)] = true
			}
			if v.Secret != nil {
				r.secrets[key(ns, v.Secret.SecretName)] = true
			}
			if v.PersistentVolumeClaim != nil {
				r.pvcs[key(ns, v.PersistentVolumeClaim.ClaimName)] = true
			}
			if v.Projected != nil {
				for _, src := range v.Projected.Sources {
					if src.ConfigMap != nil {
						r.configMaps[key(ns, src.ConfigMap.Name)] = true
					}
					if src.Secret != nil {
						r.secrets[key(ns, src.Secret.Name)] = true
					}
				}
			}
		}
		containers := append(append([]corev1.Container{}, p.Spec.InitContainers...), p.Spec.Containers...)
		for _, ec := range p.Spec.EphemeralContainers {
			containers = append(containers, corev1.Container(ec.EphemeralContainerCommon))
		}
		for _, c := range containers {
			for _, ef := range c.EnvFrom {
				if ef.ConfigMapRef != nil {
					r.configMaps[key(ns, ef.ConfigMapRef.Name)] = true
				}
				if ef.SecretRef != nil {
					r.secrets[key(ns, ef.SecretRef.Name)] = true
				}
			}
			for _, e := range c.Env {
				if e.ValueFrom == nil {
					continue
				}
				if e.ValueFrom.ConfigMapKeyRef != nil {
					r.configMaps[key(ns, e.ValueFrom.ConfigMapKeyRef.Name)] = true
				}
				if e.ValueFrom.SecretKeyRef != nil {
					r.secrets[key(ns, e.ValueFrom.SecretKeyRef.Name)] = true
				}
			}
		}
	}
	return r
}

// isSystemSecret reports Secrets managed by the platform that no pod references by design.
func isSystemSecret(s corev1.Secret) bool {
	return s.Type == corev1.SecretTypeServiceAccountToken || s.Type == "helm.sh/release.v1"
}

func hasReadyEndpoint(slices []discoveryv1.EndpointSlice) bool {
	for _, sl := range slices {
		for _, ep := range sl.Endpoints {
			if ep.Conditions.Ready == nil || *ep.Conditions.Ready {
				return true
			}
		}
	}
	return false
}

func run(ctx context.Context, client kubernetes.Interface, out io.Writer, ns string, opts options) error {
	if opts.allNamespaces {
		ns = metav1.NamespaceAll
	}
	want := func(t string) bool { return contains(opts.types, t) }

	var found []orphan

	if want("configmaps") || want("secrets") || want("pvcs") {
		podList, err := client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		r := collectRefs(podList.Items)

		if want("configmaps") {
			list, err := client.CoreV1().ConfigMaps(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}
			for _, cm := range list.Items {
				if cm.Name == "kube-root-ca.crt" || r.configMaps[key(cm.Namespace, cm.Name)] {
					continue
				}
				found = append(found, orphan{cm.Namespace, "ConfigMap", cm.Name, "not referenced by any pod"})
			}
		}
		if want("secrets") {
			list, err := client.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}
			for _, s := range list.Items {
				if isSystemSecret(s) || r.secrets[key(s.Namespace, s.Name)] {
					continue
				}
				found = append(found, orphan{s.Namespace, "Secret", s.Name, "not referenced by any pod"})
			}
		}
		if want("pvcs") {
			list, err := client.CoreV1().PersistentVolumeClaims(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}
			for _, p := range list.Items {
				if r.pvcs[key(p.Namespace, p.Name)] {
					continue
				}
				found = append(found, orphan{p.Namespace, "PVC", p.Name, "not mounted by any pod"})
			}
		}
	}

	if want("services") {
		svcs, err := client.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, s := range svcs.Items {
			if s.Spec.Type == corev1.ServiceTypeExternalName {
				continue
			}
			slices, err := client.DiscoveryV1().EndpointSlices(s.Namespace).List(ctx, metav1.ListOptions{
				LabelSelector: discoveryv1.LabelServiceName + "=" + s.Name,
			})
			if err != nil {
				return err
			}
			if !hasReadyEndpoint(slices.Items) {
				found = append(found, orphan{s.Namespace, "Service", s.Name, "no ready endpoints"})
			}
		}
	}

	if len(found) == 0 {
		fmt.Fprintln(out, "No orphaned resources found.")
		return nil
	}

	sort.Slice(found, func(i, j int) bool {
		a, b := found[i], found[j]
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Name < b.Name
	})

	tw := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	if opts.allNamespaces {
		fmt.Fprintln(tw, "NAMESPACE\tKIND\tNAME\tREASON")
	} else {
		fmt.Fprintln(tw, "KIND\tNAME\tREASON")
	}
	for _, o := range found {
		if opts.allNamespaces {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", o.Namespace, o.Kind, o.Name, o.Reason)
		} else {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", o.Kind, o.Name, o.Reason)
		}
	}
	return tw.Flush()
}
