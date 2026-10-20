package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	metafake "k8s.io/client-go/metadata/fake"
)

var (
	podGVR  = schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	svcGVR  = schema.GroupVersionResource{Version: "v1", Resource: "services"}
	nodeGVR = schema.GroupVersionResource{Version: "v1", Resource: "nodes"}
)

func testMapper() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper(nil)
	m.AddSpecific(schema.GroupVersionKind{Version: "v1", Kind: "Pod"}, podGVR, schema.GroupVersionResource{Version: "v1", Resource: "pod"}, meta.RESTScopeNamespace)
	m.AddSpecific(schema.GroupVersionKind{Version: "v1", Kind: "Service"}, svcGVR, schema.GroupVersionResource{Version: "v1", Resource: "service"}, meta.RESTScopeNamespace)
	m.AddSpecific(schema.GroupVersionKind{Version: "v1", Kind: "Node"}, nodeGVR, schema.GroupVersionResource{Version: "v1", Resource: "node"}, meta.RESTScopeRoot)
	return m
}

func obj(kind, ns, name string, lbls map[string]string) *metav1.PartialObjectMetadata {
	return &metav1.PartialObjectMetadata{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: kind},
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name, Labels: lbls},
	}
}

func newFake(objs ...runtime.Object) *metafake.FakeMetadataClient {
	scheme := metafake.NewTestScheme()
	_ = metav1.AddMetaToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	return metafake.NewSimpleMetadataClient(scheme, objs...)
}

func TestResolve(t *testing.T) {
	m := testMapper()
	tests := []struct {
		arg      string
		wantKind string
		wantErr  bool
	}{
		{arg: "pods", wantKind: "Pod"},
		{arg: "pod", wantKind: "Pod"},
		{arg: "services", wantKind: "Service"},
		{arg: "nodes", wantKind: "Node"},
		{arg: "widgets", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			got, err := resolve(m, tt.arg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolve(%q) error = %v, wantErr %v", tt.arg, err, tt.wantErr)
			}
			if err == nil && got.GroupVersionKind.Kind != tt.wantKind {
				t.Errorf("kind = %q, want %q", got.GroupVersionKind.Kind, tt.wantKind)
			}
		})
	}
}

func TestRun(t *testing.T) {
	objs := []runtime.Object{
		obj("Pod", "default", "a", map[string]string{"app": "web"}),
		obj("Pod", "default", "b", nil),
		obj("Pod", "kube-system", "c", nil),
		obj("Service", "kube-system", "dns", nil),
		obj("Node", "", "n1", nil),
		obj("Node", "", "n2", nil),
	}

	tests := []struct {
		name     string
		resource string
		ns       string
		opts     options
		want     []string
		wantErr  bool
	}{
		{name: "namespace", resource: "pods", ns: "default", want: []string{`Pod in namespace "default": 2`}},
		{name: "other namespace", resource: "services", ns: "kube-system", want: []string{`Service in namespace "kube-system": 1`}},
		{name: "selector", resource: "pods", ns: "default", opts: options{selector: "app=web"}, want: []string{`Pod in namespace "default": 1`}},
		{name: "all namespaces", resource: "pods", ns: "default", opts: options{allNamespaces: true},
			want: []string{"Pod in all namespaces: 3", "default", "kube-system"}},
		{name: "cluster scoped ignores namespace", resource: "nodes", ns: "default", want: []string{"Node in cluster: 2"}},
		{name: "unknown", resource: "widgets", ns: "default", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := run(context.Background(), newFake(objs...), testMapper(), &buf, tt.ns, tt.resource, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Fatalf("run() error = %v, wantErr %v", err, tt.wantErr)
			}
			for _, w := range tt.want {
				if !strings.Contains(buf.String(), w) {
					t.Errorf("output missing %q:\n%s", w, buf.String())
				}
			}
		})
	}
}
