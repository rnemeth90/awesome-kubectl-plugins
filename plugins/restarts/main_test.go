package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rnemeth90/kubectl-plugins/internal/kubeutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var now = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func pod(ns, name string, restarts int32, reason string, finished time.Time) corev1.Pod {
	cs := corev1.ContainerStatus{Name: "app", RestartCount: restarts}
	if reason != "" {
		cs.LastTerminationState.Terminated = &corev1.ContainerStateTerminated{
			Reason: reason, ExitCode: 137, FinishedAt: metav1.NewTime(finished),
		}
	}
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		Status:     corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{cs}},
	}
}

func TestSummarize(t *testing.T) {
	p := pod("a", "p", 3, "OOMKilled", now.Add(-time.Hour))
	p.Status.ContainerStatuses = append(p.Status.ContainerStatuses, corev1.ContainerStatus{
		Name: "side", RestartCount: 2,
		LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
			Reason: "Error", ExitCode: 1, FinishedAt: metav1.NewTime(now.Add(-time.Minute)),
		}},
	})
	r := summarize(p)
	if r.restarts != 5 || r.container != "side" || r.reason != "Error" {
		t.Fatalf("got %+v", r)
	}
}

func TestAgo(t *testing.T) {
	cases := map[time.Duration]string{30 * time.Second: "30s ago", 5 * time.Minute: "5m ago", 3 * time.Hour: "3h ago", 72 * time.Hour: "3d ago"}
	for d, want := range cases {
		if got := ago(now.Add(-d), now); got != want {
			t.Errorf("ago(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestRunRanksAndFilters(t *testing.T) {
	a := pod("a", "low", 2, "Error", now.Add(-time.Hour))
	b := pod("a", "high", 9, "OOMKilled", now.Add(-time.Minute))
	c := pod("a", "none", 0, "", time.Time{})
	cs := fake.NewSimpleClientset(&a, &b, &c)

	var out bytes.Buffer
	opts := options{min: 1, limit: 20}
	if err := run(context.Background(), cs, &out, "a", opts, now); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if strings.Contains(s, "none") || strings.Index(s, "high") > strings.Index(s, "low") || !strings.Contains(s, "OOMKilled") {
		t.Errorf("unexpected output:\n%s", s)
	}

	out.Reset()
	opts = options{scope: kubeutil.Scope{}, min: 1, limit: 1}
	if err := run(context.Background(), cs, &out, "a", opts, now); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "low") {
		t.Errorf("limit not applied:\n%s", out.String())
	}
}
