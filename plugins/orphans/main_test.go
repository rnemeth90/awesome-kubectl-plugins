package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func om(ns, name string) metav1.ObjectMeta { return metav1.ObjectMeta{Namespace: ns, Name: name} }

func slice(ns, svc string, ready bool) *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: svc + "-abc", Labels: map[string]string{discoveryv1.LabelServiceName: svc}},
		Endpoints:  []discoveryv1.Endpoint{{Conditions: discoveryv1.EndpointConditions{Ready: &ready}}},
	}
}

func usingPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: om("default", "app"),
		Spec: corev1.PodSpec{
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: "pull"}},
			Volumes: []corev1.Volume{
				{Name: "cm", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "vol-cm"}}}},
				{Name: "pvc", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data"}}},
				{Name: "proj", VolumeSource: corev1.VolumeSource{Projected: &corev1.ProjectedVolumeSource{Sources: []corev1.VolumeProjection{
					{Secret: &corev1.SecretProjection{LocalObjectReference: corev1.LocalObjectReference{Name: "proj-secret"}}},
				}}}},
			},
			Containers: []corev1.Container{{
				Name:    "c",
				EnvFrom: []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "env-secret"}}}},
				Env: []corev1.EnvVar{{Name: "X", ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "env-cm"}, Key: "k"},
				}}},
			}},
		},
	}
}

func TestRun(t *testing.T) {
	objs := []runtime.Object{
		usingPod(),
		&corev1.ConfigMap{ObjectMeta: om("default", "vol-cm")},
		&corev1.ConfigMap{ObjectMeta: om("default", "env-cm")},
		&corev1.ConfigMap{ObjectMeta: om("default", "unused-cm")},
		&corev1.ConfigMap{ObjectMeta: om("default", "kube-root-ca.crt")},
		&corev1.Secret{ObjectMeta: om("default", "pull")},
		&corev1.Secret{ObjectMeta: om("default", "env-secret")},
		&corev1.Secret{ObjectMeta: om("default", "proj-secret")},
		&corev1.Secret{ObjectMeta: om("default", "unused-secret")},
		&corev1.Secret{ObjectMeta: om("default", "sa-token"), Type: corev1.SecretTypeServiceAccountToken},
		&corev1.PersistentVolumeClaim{ObjectMeta: om("default", "data")},
		&corev1.PersistentVolumeClaim{ObjectMeta: om("default", "unused-pvc")},
		&corev1.Service{ObjectMeta: om("default", "healthy")},
		&corev1.Service{ObjectMeta: om("default", "dead")},
		&corev1.Service{ObjectMeta: om("default", "notready")},
		&corev1.Service{ObjectMeta: om("default", "ext"), Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeExternalName}},
		slice("default", "healthy", true),
		slice("default", "notready", false),
		&corev1.ConfigMap{ObjectMeta: om("other", "other-cm")},
	}

	tests := []struct {
		name    string
		opts    options
		ns      string
		want    []string
		wantNot []string
	}{
		{
			name:    "all types in namespace",
			opts:    options{types: allTypes},
			ns:      "default",
			want:    []string{"unused-cm", "unused-secret", "unused-pvc", "dead", "notready"},
			wantNot: []string{"vol-cm", "env-cm", "kube-root-ca.crt", "pull", "env-secret", "proj-secret", "sa-token", "data ", "healthy", "ext", "other-cm", "NAMESPACE"},
		},
		{
			name:    "type filter",
			opts:    options{types: []string{"pvcs"}},
			ns:      "default",
			want:    []string{"unused-pvc"},
			wantNot: []string{"unused-cm", "unused-secret", "dead"},
		},
		{
			name: "all namespaces adds column",
			opts: options{types: []string{"configmaps"}, allNamespaces: true},
			ns:   "default",
			want: []string{"NAMESPACE", "other-cm", "unused-cm"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := run(context.Background(), fake.NewSimpleClientset(objs...), &out, tt.ns, tt.opts); err != nil {
				t.Fatal(err)
			}
			for _, s := range tt.want {
				if !strings.Contains(out.String(), s) {
					t.Errorf("output missing %q\ngot:\n%s", s, out.String())
				}
			}
			for _, s := range tt.wantNot {
				if strings.Contains(out.String(), s) {
					t.Errorf("output unexpectedly contains %q\ngot:\n%s", s, out.String())
				}
			}
		})
	}
}

func TestRunNone(t *testing.T) {
	var out bytes.Buffer
	if err := run(context.Background(), fake.NewSimpleClientset(), &out, "default", options{types: allTypes}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No orphaned resources") {
		t.Errorf("unexpected output: %s", out.String())
	}
}

func TestRunAPIError(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("boom")
	})
	if err := run(context.Background(), client, &bytes.Buffer{}, "default", options{types: allTypes}); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewCmdArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "help", args: []string{"--help"}},
		{name: "unexpected positional arg", args: []string{"extra"}, wantErr: true},
		{name: "unknown flag", args: []string{"--nope"}, wantErr: true},
		{name: "bad type", args: []string{"--type", "pods"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := newCmd(&out)
			cmd.SetArgs(tt.args)
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			if err := cmd.Execute(); (err != nil) != tt.wantErr {
				t.Fatalf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
