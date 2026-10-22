package main

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rnemeth90/kubectl-plugins/internal/kubeutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const schedMsg = "0/5 nodes are available: 2 Insufficient cpu, 1 Insufficient memory, 2 node(s) had untolerated taint {dedicated: gpu}, 1 node(s) didn't match Pod's node affinity/selector. preemption: 0/5 nodes are available: 5 No preemption victims found for incoming pod."

func pendingPod(ns, name, msg string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name, CreationTimestamp: metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{{
				Type: corev1.PodScheduled, Status: corev1.ConditionFalse,
				Reason: corev1.PodReasonUnschedulable, Message: msg,
			}},
		},
	}
}

func TestSplitReasons(t *testing.T) {
	got := splitReasons(schedMsg)
	want := []string{
		"2 Insufficient cpu",
		"1 Insufficient memory",
		"2 node(s) had untolerated taint {dedicated: gpu}",
		"1 node(s) didn't match Pod's node affinity/selector",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSplitReasonsTaintWithComma(t *testing.T) {
	got := splitReasons("0/2 nodes are available: 2 node(s) had untolerated taint {a: b, c: d}.")
	if len(got) != 1 {
		t.Fatalf("got %q, want one reason", got)
	}
}

func TestCauses(t *testing.T) {
	got := causes(schedMsg)
	want := []string{"Insufficient cpu", "Insufficient memory", "Taints", "NodeAffinity"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDiagnose(t *testing.T) {
	now := time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC)
	scheduled := pendingPod("a", "s", schedMsg)
	scheduled.Spec.NodeName = "node1"
	running := pendingPod("a", "r", schedMsg)
	running.Status.Phase = corev1.PodRunning
	noCond := pendingPod("a", "n", "")
	noCond.Status.Conditions = nil

	if f, ok := diagnose(pendingPod("a", "p", schedMsg), now); !ok || f.age != 2*time.Hour {
		t.Errorf("unschedulable: ok=%v age=%v", ok, f.age)
	}
	for name, p := range map[string]corev1.Pod{"scheduled": scheduled, "running": running, "no condition": noCond} {
		if _, ok := diagnose(p, now); ok {
			t.Errorf("%s pod should not be reported", name)
		}
	}
}

func TestRun(t *testing.T) {
	now := time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC)
	a, b := pendingPod("a", "web", schedMsg), pendingPod("b", "db", "0/1 nodes are available: 1 node(s) were unschedulable.")
	ok := pendingPod("a", "fine", "")
	ok.Status.Phase = corev1.PodRunning
	cs := fake.NewSimpleClientset(&a, &b, &ok)

	var out bytes.Buffer
	if err := run(context.Background(), cs, &out, "a", kubeutil.Scope{}, now); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "web") || strings.Contains(s, "db") || strings.Contains(s, "fine") || strings.Contains(s, "NAMESPACE") {
		t.Errorf("unexpected output:\n%s", s)
	}

	out.Reset()
	if err := run(context.Background(), cs, &out, "a", kubeutil.Scope{AllNamespaces: true}, now); err != nil {
		t.Fatal(err)
	}
	s = out.String()
	if !strings.Contains(s, "NAMESPACE") || !strings.Contains(s, "Cordoned") || !strings.Contains(s, "2h") {
		t.Errorf("unexpected all-namespaces output:\n%s", s)
	}

	out.Reset()
	if err := run(context.Background(), fake.NewSimpleClientset(), &out, "a", kubeutil.Scope{}, now); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No unschedulable pods.") {
		t.Errorf("empty output = %q", out.String())
	}
}
