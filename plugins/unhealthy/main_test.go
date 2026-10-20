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

func pod(ns, name string, phase corev1.PodPhase, cs ...corev1.ContainerStatus) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c"}}},
		Status:     corev1.PodStatus{Phase: phase, ContainerStatuses: cs},
	}
}

func TestDiagnose(t *testing.T) {
	ready := corev1.ContainerStatus{Name: "c", Ready: true}
	crash := corev1.ContainerStatus{Name: "c", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}}
	notReady := corev1.ContainerStatus{Name: "c"}
	terminating := pod("a", "t", corev1.PodRunning, ready)
	now := metav1.Now()
	terminating.DeletionTimestamp = &now
	pending := pod("a", "p", corev1.PodPending)
	pending.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: "Unschedulable", Message: "no nodes"}}

	tests := []struct {
		name   string
		pod    corev1.Pod
		bad    bool
		status string
	}{
		{"healthy", pod("a", "ok", corev1.PodRunning, ready), false, ""},
		{"completed", pod("a", "done", corev1.PodSucceeded), false, ""},
		{"crashloop", pod("a", "x", corev1.PodRunning, crash), true, "CrashLoopBackOff"},
		{"not ready", pod("a", "n", corev1.PodRunning, notReady), true, "NotReady"},
		{"failed", pod("a", "f", corev1.PodFailed), true, "Failed"},
		{"pending", pending, true, "Pending"},
		{"terminating", terminating, true, "Terminating"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, bad := diagnose(tc.pod)
			if bad != tc.bad || f.status != tc.status {
				t.Fatalf("got bad=%v status=%q, want bad=%v status=%q", bad, f.status, tc.bad, tc.status)
			}
		})
	}
	if f, _ := diagnose(pending); !strings.Contains(f.reason, "Unschedulable") {
		t.Errorf("pending reason = %q", f.reason)
	}
}

func TestRun(t *testing.T) {
	good := pod("a", "ok", corev1.PodRunning, corev1.ContainerStatus{Name: "c", Ready: true})
	bad := pod("b", "bad", corev1.PodFailed)
	cs := fake.NewSimpleClientset(&good, &bad)

	var out bytes.Buffer
	if err := run(context.Background(), cs, &out, "", kubeutil.Scope{AllNamespaces: true}); err != nil {
		t.Fatal(err)
	}
	if s := out.String(); !strings.Contains(s, "bad") || strings.Contains(s, " ok ") {
		t.Errorf("unexpected output:\n%s", s)
	}

	out.Reset()
	if err := run(context.Background(), cs, &out, "a", kubeutil.Scope{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "All pods healthy") {
		t.Errorf("unexpected output:\n%s", out.String())
	}
}
