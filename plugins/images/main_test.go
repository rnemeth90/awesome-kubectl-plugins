package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/rnemeth90/kubectl-plugins/internal/kubeutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestIsMutable(t *testing.T) {
	cases := map[string]bool{
		"nginx":                              true,
		"nginx:latest":                       true,
		"nginx:1.27":                         false,
		"localhost:5000/app":                 true,
		"localhost:5000/app:v1":              false,
		"reg.io/app@sha256:abc":              false,
		"reg.io/app:latest@sha256:abc":       false,
		"mcr.microsoft.com/oss/kube/pause:3": false,
	}
	for img, want := range cases {
		if got := isMutable(img); got != want {
			t.Errorf("isMutable(%q) = %v, want %v", img, got, want)
		}
	}
}

func pod(ns, name string, images ...string) *corev1.Pod {
	p := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name}}
	for _, i := range images {
		p.Spec.Containers = append(p.Spec.Containers, corev1.Container{Name: "c", Image: i})
	}
	return p
}

func TestRun(t *testing.T) {
	cs := fake.NewSimpleClientset(
		pod("a", "p1", "nginx:1.27", "busybox"),
		pod("b", "p2", "nginx:1.27"),
	)
	var out bytes.Buffer
	if err := run(context.Background(), cs, &out, "", options{scope: kubeutil.Scope{AllNamespaces: true}}); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if strings.Index(s, "nginx:1.27") > strings.Index(s, "busybox") || !strings.Contains(s, "a,b") || !strings.Contains(s, "mutable tag") {
		t.Errorf("unexpected output:\n%s", s)
	}

	out.Reset()
	if err := run(context.Background(), cs, &out, "", options{scope: kubeutil.Scope{AllNamespaces: true}, latest: true}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "nginx") || !strings.Contains(out.String(), "busybox") {
		t.Errorf("--latest filter wrong:\n%s", out.String())
	}
}
