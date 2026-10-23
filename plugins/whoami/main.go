package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/spf13/cobra"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

const example = `  # Show who you are authenticated as
  kubectl whoami

  # Use another context
  kubectl whoami --context prod

  # Machine-readable output
  kubectl whoami -o json`

type options struct {
	output string
}

// clusterInfo describes the kubeconfig target the identity was resolved against.
type clusterInfo struct {
	Context string
	Cluster string
	Server  string
}

type identity struct {
	Context  string              `json:"context"`
	Cluster  string              `json:"cluster"`
	Server   string              `json:"server"`
	Username string              `json:"username"`
	UID      string              `json:"uid,omitempty"`
	Groups   []string            `json:"groups"`
	Extra    map[string][]string `json:"extra,omitempty"`
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
		Use:          "kubectl whoami [flags]",
		Short:        "Show the current user, groups and context",
		Example:      example,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.output != "" && opts.output != "json" {
				return fmt.Errorf("unsupported output format %q (use json)", opts.output)
			}
			client, info, err := newClient(configFlags)
			if err != nil {
				return err
			}
			return run(cmd.Context(), client, out, info, opts)
		},
	}
	configFlags.AddFlags(cmd.Flags())
	cmd.Flags().StringVarP(&opts.output, "output", "o", "", "Output format: json")
	return cmd
}

func newClient(configFlags *genericclioptions.ConfigFlags) (kubernetes.Interface, clusterInfo, error) {
	restConfig, err := configFlags.ToRESTConfig()
	if err != nil {
		return nil, clusterInfo{}, err
	}
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, clusterInfo{}, err
	}
	info := clusterInfo{Server: restConfig.Host}
	if raw, err := configFlags.ToRawKubeConfigLoader().RawConfig(); err == nil {
		info.Context = raw.CurrentContext
		if configFlags.Context != nil && *configFlags.Context != "" {
			info.Context = *configFlags.Context
		}
		if c, ok := raw.Contexts[info.Context]; ok {
			info.Cluster = c.Cluster
		}
	}
	return client, info, nil
}

func run(ctx context.Context, client kubernetes.Interface, out io.Writer, info clusterInfo, opts options) error {
	review, err := client.AuthenticationV1().SelfSubjectReviews().Create(ctx, &authenticationv1.SelfSubjectReview{}, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("self subject review failed (requires Kubernetes 1.28+): %w", err)
	}

	ui := review.Status.UserInfo
	id := identity{
		Context:  info.Context,
		Cluster:  info.Cluster,
		Server:   info.Server,
		Username: ui.Username,
		UID:      ui.UID,
		Groups:   append([]string{}, ui.Groups...),
	}
	sort.Strings(id.Groups)
	if len(ui.Extra) > 0 {
		id.Extra = map[string][]string{}
		for k, v := range ui.Extra {
			id.Extra[k] = v
		}
	}

	if opts.output == "json" {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(id)
	}

	fmt.Fprintf(out, "Context:  %s\n", id.Context)
	fmt.Fprintf(out, "Cluster:  %s\n", id.Cluster)
	fmt.Fprintf(out, "Server:   %s\n", id.Server)
	fmt.Fprintf(out, "Username: %s\n", id.Username)
	if id.UID != "" {
		fmt.Fprintf(out, "UID:      %s\n", id.UID)
	}
	fmt.Fprintln(out, "Groups:")
	for _, g := range id.Groups {
		fmt.Fprintf(out, "  %s\n", g)
	}
	if len(id.Extra) > 0 {
		keys := make([]string, 0, len(id.Extra))
		for k := range id.Extra {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Fprintln(out, "Extra:")
		for _, k := range keys {
			fmt.Fprintf(out, "  %s: %v\n", k, id.Extra[k])
		}
	}
	return nil
}
