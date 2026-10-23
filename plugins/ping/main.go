package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	utilexec "k8s.io/client-go/util/exec"
)

const example = `  # Ping an IP from a pod in the current namespace
  kubectl ping web-7d4b9 8.8.8.8

  # Ping a DNS name, resolved from inside the pod (cluster-internal names work)
  kubectl ping web-7d4b9 my-svc.my-ns.svc.cluster.local
  kubectl ping web-7d4b9 example.com

  # Another namespace, a specific container, 10 packets
  kubectl ping web-7d4b9 10.0.0.5 -n prod -c app --count 10

  # Larger packets (payload bytes) with a 1s per-packet timeout
  kubectl ping web-7d4b9 10.0.0.5 --size 1400 --timeout 1`

const (
	maxCount = 1000
	maxSize  = 65507 // largest ICMP payload that fits in an IPv4 packet
)

type options struct {
	container string
	count     int
	size      int
	timeout   int
}

// executor runs a command in a pod container, streaming output to out/errOut.
type executor interface {
	Exec(ctx context.Context, ns, pod, container string, cmd []string, out, errOut io.Writer) error
}

func main() {
	if err := newCmd(os.Stdout, os.Stderr).Execute(); err != nil {
		os.Exit(exitCode(err))
	}
}

func exitCode(err error) int {
	if ce, ok := err.(utilexec.CodeExitError); ok {
		return ce.Code
	}
	return 1
}

func newCmd(out, errOut io.Writer) *cobra.Command {
	configFlags := genericclioptions.NewConfigFlags(true)
	opts := options{}

	cmd := &cobra.Command{
		Use:          "kubectl ping POD DESTINATION [flags]",
		Short:        "Ping an IPv4 address or DNS name from inside a pod",
		Example:      example,
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := configFlags.ToRESTConfig()
			if err != nil {
				return err
			}
			cs, err := kubernetes.NewForConfig(cfg)
			if err != nil {
				return err
			}
			ns, _, err := configFlags.ToRawKubeConfigLoader().Namespace()
			if err != nil {
				return err
			}
			return run(cmd.Context(), cs, spdyExecutor{cs: cs, cfg: cfg}, out, errOut, ns, args[0], args[1], opts)
		},
	}
	configFlags.AddFlags(cmd.Flags())
	cmd.Flags().StringVarP(&opts.container, "container", "c", "", "Container to run ping in (default: the pod's default container)")
	cmd.Flags().IntVar(&opts.count, "count", 4, "Number of packets to send")
	cmd.Flags().IntVar(&opts.size, "size", 0, "Payload size in bytes (0 uses the ping default)")
	cmd.Flags().IntVar(&opts.timeout, "timeout", 2, "Seconds to wait for each reply")
	return cmd
}

// validate checks the destination and numeric options before anything is
// sent to the cluster.
func validate(dest string, o options) error {
	if !validDestination(dest) {
		return fmt.Errorf("%q is not a valid IPv4 address or DNS name", dest)
	}
	if o.count < 1 || o.count > maxCount {
		return fmt.Errorf("--count must be between 1 and %d", maxCount)
	}
	if o.size < 0 || o.size > maxSize {
		return fmt.Errorf("--size must be between 0 and %d", maxSize)
	}
	if o.timeout < 1 {
		return fmt.Errorf("--timeout must be at least 1")
	}
	return nil
}

// validDestination accepts a dotted-quad IPv4 address or an RFC 1123 DNS name
// (including single-label and cluster-internal names such as
// my-svc.my-ns.svc.cluster.local). IPv6 and anything that could be parsed as
// a ping option are rejected.
func validDestination(d string) bool {
	if ip := net.ParseIP(d); ip != nil {
		return ip.To4() != nil && !strings.Contains(d, ":")
	}
	name := strings.TrimSuffix(d, ".")
	if name == "" || len(name) > 253 {
		return false
	}
	labels := strings.Split(name, ".")
	for _, l := range labels {
		if l == "" || len(l) > 63 || l[0] == '-' || l[len(l)-1] == '-' {
			return false
		}
		for _, c := range l {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	// A DNS name never ends in an all-numeric label, so this rejects
	// malformed addresses like "256.1.1.1" and "8.8.8".
	return strings.Trim(labels[len(labels)-1], "0123456789") != ""
}

func buildCommand(dest string, o options) []string {
	cmd := []string{"ping", "-c", strconv.Itoa(o.count), "-W", strconv.Itoa(o.timeout)}
	if o.size > 0 {
		cmd = append(cmd, "-s", strconv.Itoa(o.size))
	}
	return append(cmd, dest)
}

func run(ctx context.Context, cs kubernetes.Interface, ex executor, out, errOut io.Writer, ns, podName, dest string, o options) error {
	if err := validate(dest, o); err != nil {
		return err
	}
	pod, err := cs.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("pod %s/%s is %s, not Running", ns, podName, pod.Status.Phase)
	}
	if o.container != "" && !hasContainer(pod, o.container) {
		return fmt.Errorf("container %q not found in pod %s/%s", o.container, ns, podName)
	}

	fmt.Fprintf(errOut, "Pinging %s from pod %s/%s\n", dest, ns, podName)
	err = ex.Exec(ctx, ns, podName, o.container, buildCommand(dest, o), out, errOut)
	if err != nil {
		if _, ok := err.(utilexec.CodeExitError); ok {
			return err
		}
		return fmt.Errorf("running ping in pod (is ping installed in the container?): %w", err)
	}
	return nil
}

func hasContainer(p *corev1.Pod, name string) bool {
	for _, c := range p.Spec.Containers {
		if c.Name == name {
			return true
		}
	}
	return false
}

type spdyExecutor struct {
	cs  kubernetes.Interface
	cfg *rest.Config
}

func (e spdyExecutor) Exec(ctx context.Context, ns, pod, container string, cmd []string, out, errOut io.Writer) error {
	req := e.cs.CoreV1().RESTClient().Post().
		Resource("pods").Namespace(ns).Name(pod).SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   cmd,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(e.cfg, "POST", req.URL())
	if err != nil {
		return err
	}
	return exec.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: out, Stderr: errOut})
}
