// Package kubeutil holds client setup and pod listing shared by the
// Kubernetes-API plugins.
package kubeutil

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

// Scope selects which pods to list.
type Scope struct {
	AllNamespaces bool
	Selector      string
}

// NewClient returns a clientset and the effective namespace from the kubeconfig flags.
func NewClient(cf *genericclioptions.ConfigFlags) (kubernetes.Interface, string, error) {
	cfg, err := cf.ToRESTConfig()
	if err != nil {
		return nil, "", err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, "", err
	}
	ns, _, err := cf.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return nil, "", err
	}
	return cs, ns, nil
}

// ListPods lists pods in ns (or every namespace when scope.AllNamespaces), following pagination.
func ListPods(ctx context.Context, cs kubernetes.Interface, ns string, scope Scope) ([]corev1.Pod, error) {
	if scope.AllNamespaces {
		ns = metav1.NamespaceAll
	}
	var pods []corev1.Pod
	opts := metav1.ListOptions{LabelSelector: scope.Selector, Limit: 500}
	for {
		list, err := cs.CoreV1().Pods(ns).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		pods = append(pods, list.Items...)
		if list.Continue == "" {
			return pods, nil
		}
		opts.Continue = list.Continue
	}
}
