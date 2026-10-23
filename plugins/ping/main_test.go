package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	utilexec "k8s.io/client-go/util/exec"
)

type fakeExec struct {
	cmd       []string
	container string
	err       error
}

func (f *fakeExec) Exec(_ context.Context, _, _, container string, cmd []string, out, _ io.Writer) error {
	f.cmd, f.container = cmd, container
	io.WriteString(out, "PING ok\n")
	return f.err
}

func podFixture(phase corev1.PodPhase) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "web"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}, {Name: "side"}}},
		Status:     corev1.PodStatus{Phase: phase},
	}
}

func TestNewCmdFlags(t *testing.T) {
	cmd := newCmd(io.Discard, io.Discard)
	for _, f := range []string{"count", "size", "timeout", "container", "namespace"} {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("flag --%s missing", f)
		}
	}
}

func TestValidate(t *testing.T) {
	ok := options{count: 4, timeout: 2}
	for _, d := range []string{
		"8.8.8.8", "10.0.0.1", "255.255.255.255",
		"example.com", "example.com.", "kubernetes", "my-svc",
		"my-svc.my-ns.svc.cluster.local", "my-svc.my-ns.svc", "pod-0.headless.ns.svc.cluster.local", "A1.b2.io",
	} {
		if err := validate(d, ok); err != nil {
			t.Errorf("validate(%q) = %v", d, err)
		}
	}
	for _, d := range []string{
		"", ".", "8.8.8", "256.1.1.1", "123", "::1", "::ffff:1.2.3.4", "fe80::1",
		"-c", "-bad.example.com", "bad-.example.com", "a..b", "a b", "1.2.3.4; rm -rf /",
		"exa_mple.com", "ex$(id).com", "host/name", strings.Repeat("a", 64) + ".com",
	} {
		if err := validate(d, ok); err == nil {
			t.Errorf("validate(%q) should fail", d)
		}
	}
	bad := []options{
		{count: 0, timeout: 2},
		{count: maxCount + 1, timeout: 2},
		{count: 1, size: -1, timeout: 2},
		{count: 1, size: maxSize + 1, timeout: 2},
		{count: 1, timeout: 0},
	}
	for _, o := range bad {
		if err := validate("1.1.1.1", o); err == nil {
			t.Errorf("validate with %+v should fail", o)
		}
	}
}

func TestBuildCommand(t *testing.T) {
	got := buildCommand("1.2.3.4", options{count: 3, timeout: 2})
	want := []string{"ping", "-c", "3", "-W", "2", "1.2.3.4"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	got = buildCommand("1.2.3.4", options{count: 5, timeout: 1, size: 1400})
	want = []string{"ping", "-c", "5", "-W", "1", "-s", "1400", "1.2.3.4"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRun(t *testing.T) {
	cs := fake.NewSimpleClientset(podFixture(corev1.PodRunning))
	ex := &fakeExec{}
	var out, errOut bytes.Buffer
	o := options{count: 2, timeout: 2, size: 64, container: "side"}
	if err := run(context.Background(), cs, ex, &out, &errOut, "ns", "web", "9.9.9.9", o); err != nil {
		t.Fatal(err)
	}
	if ex.container != "side" || ex.cmd[len(ex.cmd)-1] != "9.9.9.9" || !strings.Contains(out.String(), "PING ok") {
		t.Errorf("unexpected exec: %+v out=%q", ex, out.String())
	}
}

func TestRunErrors(t *testing.T) {
	o := options{count: 1, timeout: 1}
	ctx := context.Background()
	var out bytes.Buffer

	cs := fake.NewSimpleClientset(podFixture(corev1.PodRunning))
	if err := run(ctx, cs, &fakeExec{}, &out, &out, "ns", "web", "not an ip", o); err == nil {
		t.Error("bad destination should fail")
	}
	if err := run(ctx, cs, &fakeExec{}, &out, &out, "ns", "missing", "1.1.1.1", o); err == nil {
		t.Error("missing pod should fail")
	}
	o.container = "nope"
	if err := run(ctx, cs, &fakeExec{}, &out, &out, "ns", "web", "1.1.1.1", o); err == nil {
		t.Error("unknown container should fail")
	}
	o.container = ""

	pending := fake.NewSimpleClientset(podFixture(corev1.PodPending))
	if err := run(ctx, pending, &fakeExec{}, &out, &out, "ns", "web", "1.1.1.1", o); err == nil {
		t.Error("non-running pod should fail")
	}

	exit := &fakeExec{err: utilexec.CodeExitError{Err: errors.New("exit 1"), Code: 1}}
	err := run(ctx, cs, exit, &out, &out, "ns", "web", "1.1.1.1", o)
	if exitCode(err) != 1 {
		t.Errorf("exit code not propagated: %v", err)
	}
	other := &fakeExec{err: errors.New("exec: \"ping\": executable file not found")}
	if err := run(ctx, cs, other, &out, &out, "ns", "web", "1.1.1.1", o); err == nil || !strings.Contains(err.Error(), "installed") {
		t.Errorf("missing-binary hint absent: %v", err)
	}
}
