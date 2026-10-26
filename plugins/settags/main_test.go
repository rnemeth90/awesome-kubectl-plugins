package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestImageRepo(t *testing.T) {
	cases := map[string]string{
		"nginx":                        "nginx",
		"nginx:1.27":                   "nginx",
		"localhost:5000/app":           "localhost:5000/app",
		"localhost:5000/app:v1":        "localhost:5000/app",
		"reg.io/app@sha256:abc":        "reg.io/app",
		"reg.io/app:latest@sha256:abc": "reg.io/app",
	}
	for in, want := range cases {
		if got := imageRepo(in); got != want {
			t.Errorf("imageRepo(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParsePairs(t *testing.T) {
	tags, err := parsePairs([]string{"api=v1", "web=2.0"})
	if err != nil || tags["api"] != "v1" || tags["web"] != "2.0" {
		t.Fatalf("got %v, %v", tags, err)
	}
	for _, bad := range [][]string{nil, {"api"}, {"=v1"}, {"api="}, {"api=a b"}, {"api=v1", "api=v2"}} {
		if _, err := parsePairs(bad); err == nil {
			t.Errorf("parsePairs(%v) should fail", bad)
		}
	}
}

func TestReadPairs(t *testing.T) {
	got, err := readPairs(strings.NewReader("# header\napi=v1  # trailing\n\n  web=v2\n"))
	if err != nil || len(got) != 2 || got[0] != "api=v1" || got[1] != "web=v2" {
		t.Fatalf("got %v, %v", got, err)
	}
}

func dep(ns, name string, images ...string) *appsv1.Deployment {
	d := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name}}
	for i, img := range images {
		d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers,
			corev1.Container{Name: string(rune('a' + i)), Image: img})
	}
	return d
}

func image(t *testing.T, cs *fake.Clientset, ns, name, ctr string) string {
	t.Helper()
	d, err := cs.AppsV1().Deployments(ns).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range d.Spec.Template.Spec.Containers {
		if c.Name == ctr {
			return c.Image
		}
	}
	t.Fatalf("container %s not found", ctr)
	return ""
}

func TestRunUpdatesAcrossNamespaces(t *testing.T) {
	cs := fake.NewSimpleClientset(
		dep("a", "api", "reg.io/api:v1"),
		dep("b", "api", "reg.io/api:v0", "reg.io/sidecar:1"),
		dep("a", "web", "reg.io/web:1"),
		dep("a", "other", "reg.io/other:1"),
	)
	var out bytes.Buffer
	tags := map[string]string{"api": "v2", "web": "3", "ghost": "1"}
	if err := run(context.Background(), cs, &out, strings.NewReader(""), tags, options{yes: true}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ ns, name, ctr, want string }{
		{"a", "api", "a", "reg.io/api:v2"},
		{"b", "api", "a", "reg.io/api:v2"},
		{"b", "api", "b", "reg.io/sidecar:1"},
		{"a", "web", "a", "reg.io/web:3"},
		{"a", "other", "a", "reg.io/other:1"},
	} {
		if got := image(t, cs, c.ns, c.name, c.ctr); got != c.want {
			t.Errorf("%s/%s/%s = %s, want %s", c.ns, c.name, c.ctr, got, c.want)
		}
	}
	if !strings.Contains(out.String(), `no deployment named "ghost"`) {
		t.Errorf("missing warning:\n%s", out.String())
	}
}

func TestRunNamespaceFilterAndDryRun(t *testing.T) {
	cs := fake.NewSimpleClientset(dep("a", "api", "r/api:v1"), dep("b", "api", "r/api:v1"))
	var out bytes.Buffer
	if err := run(context.Background(), cs, &out, strings.NewReader(""), map[string]string{"api": "v2"},
		options{namespaces: []string{"a"}, dryRun: true}); err != nil {
		t.Fatal(err)
	}
	if got := image(t, cs, "a", "api", "a"); got != "r/api:v1" {
		t.Errorf("dry-run changed image: %s", got)
	}

	out.Reset()
	if err := run(context.Background(), cs, &out, strings.NewReader("y\n"), map[string]string{"api": "v2"},
		options{namespaces: []string{"a"}}); err != nil {
		t.Fatal(err)
	}
	if image(t, cs, "a", "api", "a") != "r/api:v2" || image(t, cs, "b", "api", "a") != "r/api:v1" {
		t.Error("prompted apply wrong")
	}
}

func TestRunAbort(t *testing.T) {
	cs := fake.NewSimpleClientset(dep("a", "api", "r/api:v1"))
	var out bytes.Buffer
	err := run(context.Background(), cs, &out, strings.NewReader("n\n"), map[string]string{"api": "v2"}, options{})
	if err == nil || image(t, cs, "a", "api", "a") != "r/api:v1" {
		t.Errorf("expected abort without change, err=%v", err)
	}
}
