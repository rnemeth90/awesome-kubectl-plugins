package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func pod(ns, name string, phase corev1.PodPhase, lbls map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name, Labels: lbls},
		Status:     corev1.PodStatus{Phase: phase},
	}
}

func deployment(ns, name string, match map[string]string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Spec:       appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{MatchLabels: match}},
	}
}

func replicaSet(ns, name string, match map[string]string) *appsv1.ReplicaSet {
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Spec:       appsv1.ReplicaSetSpec{Selector: &metav1.LabelSelector{MatchLabels: match}},
	}
}

func TestParseResource(t *testing.T) {
	tests := []struct {
		name     string
		arg      string
		wantKind string
		wantName string
		wantErr  bool
	}{
		{name: "deployment", arg: "deployment/web", wantKind: kindDeployment, wantName: "web"},
		{name: "deployments plural", arg: "deployments/web", wantKind: kindDeployment, wantName: "web"},
		{name: "deploy alias", arg: "deploy/web", wantKind: kindDeployment, wantName: "web"},
		{name: "replicaset", arg: "replicaset/web-abc", wantKind: kindReplicaSet, wantName: "web-abc"},
		{name: "rs alias", arg: "rs/web-abc", wantKind: kindReplicaSet, wantName: "web-abc"},
		{name: "case insensitive kind", arg: "Deploy/Web", wantKind: kindDeployment, wantName: "Web"},
		{name: "missing slash", arg: "web", wantErr: true},
		{name: "missing name", arg: "deploy/", wantErr: true},
		{name: "unsupported kind", arg: "svc/web", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, name, err := parseResource(tt.arg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseResource(%q) error = %v, wantErr %v", tt.arg, err, tt.wantErr)
			}
			if kind != tt.wantKind || name != tt.wantName {
				t.Errorf("parseResource(%q) = (%q, %q), want (%q, %q)", tt.arg, kind, name, tt.wantKind, tt.wantName)
			}
		})
	}
}

func TestRun(t *testing.T) {
	web := map[string]string{"app": "web"}
	db := map[string]string{"app": "db"}

	objects := []runtime.Object{
		deployment("prod", "web", web),
		deployment("prod", "db", db),
		replicaSet("prod", "web-abc", web),
		pod("prod", "web-1", corev1.PodRunning, web),
		pod("prod", "web-2", corev1.PodRunning, web),
		pod("prod", "web-3", corev1.PodPending, web),
		pod("prod", "db-1", corev1.PodRunning, db),
		pod("dev", "web-1", corev1.PodRunning, web),
	}

	tests := []struct {
		name     string
		ns       string
		resource string
		want     []string
		wantNot  []string
		wantErr  string
	}{
		{
			name: "all pods in namespace",
			ns:   "prod",
			want: []string{`Pods in namespace "prod": 4`, "Pending    1", "Running    3"},
		},
		{
			name: "namespace isolation",
			ns:   "dev",
			want: []string{`Pods in namespace "dev": 1`, "Running    1"},
		},
		{
			name: "empty namespace",
			ns:   "empty",
			want: []string{`Pods in namespace "empty": 0`},
		},
		{
			name:     "deployment selector",
			ns:       "prod",
			resource: "deploy/web",
			want:     []string{`deployment "web" in namespace "prod": 3`, "Running    2", "Pending    1"},
			wantNot:  []string{"db"},
		},
		{
			name:     "other deployment selector",
			ns:       "prod",
			resource: "deployment/db",
			want:     []string{`deployment "db" in namespace "prod": 1`},
		},
		{
			name:     "replicaset selector",
			ns:       "prod",
			resource: "rs/web-abc",
			want:     []string{`replicaset "web-abc" in namespace "prod": 3`},
		},
		{
			name:     "deployment not found",
			ns:       "prod",
			resource: "deploy/missing",
			wantErr:  "not found",
		},
		{
			name:     "replicaset in wrong namespace",
			ns:       "dev",
			resource: "rs/web-abc",
			wantErr:  "not found",
		},
		{
			name:     "invalid resource format",
			ns:       "prod",
			resource: "web",
			wantErr:  "invalid resource",
		},
		{
			name:     "unsupported kind",
			ns:       "prod",
			resource: "svc/web",
			wantErr:  "unsupported resource kind",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset(objects...)
			var out bytes.Buffer

			err := run(context.Background(), client, &out, tt.ns, tt.resource)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("run() error = %v, want error containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("run() unexpected error: %v", err)
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

func TestNewCmdArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "help", args: []string{"--help"}},
		{name: "too many args", args: []string{"deploy/a", "deploy/b"}, wantErr: true},
		{name: "unknown flag", args: []string{"--nope"}, wantErr: true},
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
