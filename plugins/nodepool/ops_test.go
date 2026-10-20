package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
)

type fakePools struct {
	pools     map[string]*armcontainerservice.AgentPool
	puts      []string
	deleted   []string
	imaged    []string
	aborted   []string
	lastWait  bool
	getErr    error
	lastModel armcontainerservice.AgentPool
}

func newFakePools(ps ...*armcontainerservice.AgentPool) *fakePools {
	f := &fakePools{pools: map[string]*armcontainerservice.AgentPool{}}
	for _, p := range ps {
		f.pools[*p.Name] = p
	}
	return f
}

func pool(name string, count int32, autoscale bool) *armcontainerservice.AgentPool {
	m := armcontainerservice.AgentPoolModeUser
	return &armcontainerservice.AgentPool{
		Name: ptr(name),
		Properties: &armcontainerservice.ManagedClusterAgentPoolProfileProperties{
			Count: ptr(count), VMSize: ptr("Standard_DS2_v2"), Mode: &m,
			EnableAutoScaling:          ptr(autoscale),
			CurrentOrchestratorVersion: ptr("1.29.0"),
		},
	}
}

func (f *fakePools) List(context.Context) ([]*armcontainerservice.AgentPool, error) {
	var out []*armcontainerservice.AgentPool
	for _, p := range f.pools {
		out = append(out, p)
	}
	return out, nil
}

func (f *fakePools) Get(_ context.Context, name string) (*armcontainerservice.AgentPool, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	p, ok := f.pools[name]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	cp := *p
	pp := *p.Properties
	cp.Properties = &pp
	return &cp, nil
}

func (f *fakePools) GetUpgradeProfile(context.Context, string) (*armcontainerservice.AgentPoolUpgradeProfile, error) {
	return &armcontainerservice.AgentPoolUpgradeProfile{Properties: &armcontainerservice.AgentPoolUpgradeProfileProperties{
		LatestNodeImageVersion: ptr("AKSUbuntu-2204-202401"),
		Upgrades: []*armcontainerservice.AgentPoolUpgradeProfilePropertiesUpgradesItem{
			{KubernetesVersion: ptr("1.30.0")},
			{KubernetesVersion: ptr("1.31.0"), IsPreview: ptr(true)},
		},
	}}, nil
}

func (f *fakePools) CreateOrUpdate(_ context.Context, name string, p armcontainerservice.AgentPool, wait bool) (*armcontainerservice.AgentPool, error) {
	f.puts = append(f.puts, name)
	f.lastModel = p
	f.lastWait = wait
	return &p, nil
}

func (f *fakePools) Delete(_ context.Context, name string, _ bool) error {
	f.deleted = append(f.deleted, name)
	return nil
}

func (f *fakePools) UpgradeNodeImage(_ context.Context, name string, _ bool) error {
	f.imaged = append(f.imaged, name)
	return nil
}

func (f *fakePools) AbortLatestOperation(_ context.Context, name string, _ bool) error {
	f.aborted = append(f.aborted, name)
	return nil
}

func TestListAndShow(t *testing.T) {
	f := newFakePools(pool("user1", 3, false))
	var buf bytes.Buffer
	if err := listPools(context.Background(), f, &buf, "table"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "user1") || !strings.Contains(buf.String(), "NAME") {
		t.Errorf("unexpected list output:\n%s", buf.String())
	}
	buf.Reset()
	if err := showPool(context.Background(), f, &buf, "user1", "json"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"name": "user1"`) {
		t.Errorf("unexpected json:\n%s", buf.String())
	}
}

func TestScale(t *testing.T) {
	f := newFakePools(pool("a", 3, false), pool("auto", 3, true))
	var buf bytes.Buffer
	if err := scalePool(context.Background(), f, &buf, "a", 7, true); err != nil {
		t.Fatal(err)
	}
	if *f.lastModel.Properties.Count != 7 {
		t.Errorf("count = %d, want 7", *f.lastModel.Properties.Count)
	}
	if err := scalePool(context.Background(), f, &buf, "auto", 7, true); err == nil {
		t.Error("expected error scaling an autoscaled pool")
	}
	if err := scalePool(context.Background(), f, &buf, "a", -1, true); err == nil {
		t.Error("expected error for negative count")
	}
}

func TestAdd(t *testing.T) {
	f := newFakePools()
	var buf bytes.Buffer
	o := addOptions{vmSize: "Standard_D4s_v3", count: 2, mode: "User", labels: "a=b", taints: "k=v:NoSchedule"}
	if err := addPool(context.Background(), f, &buf, "new", o, false); err != nil {
		t.Fatal(err)
	}
	pp := f.lastModel.Properties
	if *pp.VMSize != "Standard_D4s_v3" || *pp.NodeLabels["a"] != "b" || *pp.NodeTaints[0] != "k=v:NoSchedule" {
		t.Errorf("unexpected model: %+v", pp)
	}
	if f.lastWait {
		t.Error("wait should be false")
	}

	bad := []addOptions{
		{vmSize: "x", mode: "Other"},
		{vmSize: "x", mode: "User", autoscale: true},
		{vmSize: "x", mode: "User", taints: "bad"},
		{vmSize: "x", mode: "User", labels: "bad"},
	}
	for i, b := range bad {
		if err := addPool(context.Background(), f, &buf, "n", b, true); err == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
}

func TestUpdate(t *testing.T) {
	ctx := context.Background()
	f := newFakePools(pool("a", 3, false))
	var buf bytes.Buffer
	one, five := int32(1), int32(5)

	if err := updatePool(ctx, f, &buf, "a", updateOptions{enableAutoscaler: true, minCount: &one, maxCount: &five}, true); err != nil {
		t.Fatal(err)
	}
	pp := f.lastModel.Properties
	if !*pp.EnableAutoScaling || *pp.MinCount != 1 || *pp.MaxCount != 5 {
		t.Errorf("autoscaler not set: %+v", pp)
	}

	if err := updatePool(ctx, f, &buf, "a", updateOptions{}, true); err == nil {
		t.Error("expected nothing-to-update error")
	}
	if err := updatePool(ctx, f, &buf, "a", updateOptions{minCount: &one}, true); err == nil {
		t.Error("expected error: min-count without autoscaler")
	}
	if err := updatePool(ctx, f, &buf, "a", updateOptions{enableAutoscaler: true, disableAutoscaler: true}, true); err == nil {
		t.Error("expected mutual exclusion error")
	}
	empty := ""
	if err := updatePool(ctx, f, &buf, "a", updateOptions{taints: &empty}, true); err != nil {
		t.Fatal(err)
	}
	if len(f.lastModel.Properties.NodeTaints) != 0 {
		t.Error("taints should be cleared")
	}
}

func TestUpgradeAndPower(t *testing.T) {
	ctx := context.Background()
	f := newFakePools(pool("a", 3, false))
	var buf bytes.Buffer
	if err := upgradePool(ctx, f, &buf, "a", "1.30.0", true); err != nil {
		t.Fatal(err)
	}
	if *f.lastModel.Properties.OrchestratorVersion != "1.30.0" {
		t.Error("version not set")
	}
	if err := upgradePool(ctx, f, &buf, "a", "", true); err == nil {
		t.Error("expected error for empty version")
	}
	if err := setPower(ctx, f, &buf, "a", armcontainerservice.CodeStopped, true); err != nil {
		t.Fatal(err)
	}
	if *f.lastModel.Properties.PowerState.Code != armcontainerservice.CodeStopped {
		t.Error("power state not set")
	}
	if err := upgradeNodeImage(ctx, f, &buf, "a", true); err != nil || len(f.imaged) != 1 {
		t.Errorf("node image upgrade failed: %v", err)
	}
	if err := cancelOperation(ctx, f, &buf, "a", true); err != nil || len(f.aborted) != 1 {
		t.Errorf("cancel failed: %v", err)
	}
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	f := newFakePools(pool("a", 3, false))
	var buf bytes.Buffer
	if err := deletePool(ctx, f, strings.NewReader("n\n"), &buf, "a", false, true); err == nil || len(f.deleted) != 0 {
		t.Error("delete should abort on 'n'")
	}
	if err := deletePool(ctx, f, strings.NewReader("y\n"), &buf, "a", false, true); err != nil || len(f.deleted) != 1 {
		t.Errorf("delete on 'y' failed: %v", err)
	}
	if err := deletePool(ctx, f, strings.NewReader(""), &buf, "b", true, true); err != nil || len(f.deleted) != 2 {
		t.Errorf("delete with --yes failed: %v", err)
	}
}

func TestDescribe(t *testing.T) {
	f := newFakePools(pool("user1", 3, false))
	var buf bytes.Buffer
	if err := describePool(context.Background(), f, &buf, "user1", "table"); err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"Name:", "user1", "AKSUbuntu-2204-202401", "1.30.0, 1.31.0 (preview)"} {
		if !strings.Contains(buf.String(), w) {
			t.Errorf("output missing %q:\n%s", w, buf.String())
		}
	}
	buf.Reset()
	if err := describePool(context.Background(), f, &buf, "user1", "json"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "upgradeProfile") {
		t.Errorf("json missing upgradeProfile:\n%s", buf.String())
	}
	if err := describePool(context.Background(), f, &buf, "missing", "table"); err == nil {
		t.Error("expected error for missing pool")
	}
}

func TestTaint(t *testing.T) {
	ctx := context.Background()
	p := pool("a", 3, false)
	p.Properties.NodeTaints = []*string{ptr("sku=gpu:NoSchedule"), ptr("x=1:NoExecute")}
	f := newFakePools(p)
	var buf bytes.Buffer

	if err := taintPool(ctx, f, &buf, "a", []string{"sku=cpu:NoSchedule", "y=2:PreferNoSchedule"}, true); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(derefAll(f.lastModel.Properties.NodeTaints), ",")
	if got != "x=1:NoExecute,sku=cpu:NoSchedule,y=2:PreferNoSchedule" {
		t.Errorf("after add: %s", got)
	}
	if err := taintPool(ctx, f, &buf, "a", []string{"sku-", "x:NoExecute-"}, true); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(derefAll(f.lastModel.Properties.NodeTaints), ","); got != "" {
		t.Errorf("after remove: %s", got)
	}
	for _, bad := range [][]string{{}, {"nope-"}, {"bad"}, {"k=v:Wrong"}} {
		if err := taintPool(ctx, f, &buf, "a", bad, true); err == nil {
			t.Errorf("expected error for %v", bad)
		}
	}
}

func TestUntaint(t *testing.T) {
	ctx := context.Background()
	p := pool("a", 3, false)
	p.Properties.NodeTaints = []*string{ptr("sku=gpu:NoSchedule"), ptr("x=1:NoExecute")}
	f := newFakePools(p)
	var buf bytes.Buffer
	if err := untaintPool(ctx, f, &buf, "a", []string{"sku", "x:NoExecute"}, true); err != nil {
		t.Fatal(err)
	}
	if len(f.lastModel.Properties.NodeTaints) != 0 {
		t.Error("taints should be removed")
	}
	if err := untaintPool(ctx, f, &buf, "a", []string{"sku-"}, true); err == nil {
		t.Error("expected error for trailing dash")
	}
}
