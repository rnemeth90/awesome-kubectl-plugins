package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/rnemeth90/kubectl-plugins/internal/aksutil"
)

type fakeCluster struct {
	mc        armcontainerservice.ManagedCluster
	started   int
	stopped   int
	rotated   int
	lastPut   armcontainerservice.ManagedCluster
	lastWait  bool
	profile   *armcontainerservice.ManagedClusterUpgradeProfile
	getErr    error
	putCalled bool
}

func newFake(power armcontainerservice.Code) *fakeCluster {
	return &fakeCluster{
		mc: armcontainerservice.ManagedCluster{
			Name:     aksutil.Ptr("prod"),
			Location: aksutil.Ptr("eastus"),
			Properties: &armcontainerservice.ManagedClusterProperties{
				KubernetesVersion:        aksutil.Ptr("1.29"),
				CurrentKubernetesVersion: aksutil.Ptr("1.29.5"),
				PowerState:               &armcontainerservice.PowerState{Code: &power},
			},
		},
		profile: &armcontainerservice.ManagedClusterUpgradeProfile{Properties: &armcontainerservice.ManagedClusterUpgradeProfileProperties{
			ControlPlaneProfile: &armcontainerservice.ManagedClusterPoolUpgradeProfile{Upgrades: []*armcontainerservice.ManagedClusterPoolUpgradeProfileUpgradesItem{
				{KubernetesVersion: aksutil.Ptr("1.30.1")},
				{KubernetesVersion: aksutil.Ptr("1.31.0"), IsPreview: aksutil.Ptr(true)},
			}},
			AgentPoolProfiles: []*armcontainerservice.ManagedClusterPoolUpgradeProfile{
				{Name: aksutil.Ptr("nodepool1"), Upgrades: nil},
			},
		}},
	}
}

func (f *fakeCluster) Get(context.Context) (*armcontainerservice.ManagedCluster, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	cp := f.mc
	pp := *f.mc.Properties
	cp.Properties = &pp
	return &cp, nil
}
func (f *fakeCluster) GetUpgradeProfile(context.Context) (*armcontainerservice.ManagedClusterUpgradeProfile, error) {
	return f.profile, nil
}
func (f *fakeCluster) CreateOrUpdate(_ context.Context, mc armcontainerservice.ManagedCluster, wait bool) error {
	f.lastPut, f.lastWait, f.putCalled = mc, wait, true
	return nil
}
func (f *fakeCluster) Start(context.Context, bool) error              { f.started++; return nil }
func (f *fakeCluster) Stop(context.Context, bool) error               { f.stopped++; return nil }
func (f *fakeCluster) RotateCertificates(context.Context, bool) error { f.rotated++; return nil }

func TestShowAndDescribe(t *testing.T) {
	f := newFake(armcontainerservice.CodeRunning)
	var buf bytes.Buffer
	if err := showCluster(context.Background(), f, &buf, "table"); err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"prod", "eastus", "1.29.5", "Running"} {
		if !strings.Contains(buf.String(), w) {
			t.Errorf("show missing %q:\n%s", w, buf.String())
		}
	}
	buf.Reset()
	if err := describeCluster(context.Background(), f, &buf, "table"); err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"1.30.1, 1.31.0 (preview)", "Pool nodepool1 Upgrades", "none"} {
		if !strings.Contains(buf.String(), w) {
			t.Errorf("describe missing %q:\n%s", w, buf.String())
		}
	}
	buf.Reset()
	if err := showCluster(context.Background(), f, &buf, "json"); err != nil || !strings.Contains(buf.String(), `"name": "prod"`) {
		t.Errorf("json: %v\n%s", err, buf.String())
	}
	f.getErr = fmt.Errorf("boom")
	if err := showCluster(context.Background(), f, &buf, "table"); err == nil {
		t.Error("expected error")
	}
}

func TestUpgrade(t *testing.T) {
	f := newFake(armcontainerservice.CodeRunning)
	var buf bytes.Buffer
	if err := upgradeCluster(context.Background(), f, &buf, "1.30.1", false); err != nil {
		t.Fatal(err)
	}
	if *f.lastPut.Properties.KubernetesVersion != "1.30.1" || f.lastWait {
		t.Errorf("unexpected put: %+v wait=%v", f.lastPut.Properties, f.lastWait)
	}
	if err := upgradeCluster(context.Background(), f, &buf, "", true); err == nil {
		t.Error("expected error for empty version")
	}
}

func TestPower(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer

	running := newFake(armcontainerservice.CodeRunning)
	if err := setPower(ctx, running, &buf, true, true); err != nil || running.started != 0 {
		t.Errorf("start on running should be a no-op: %v", err)
	}
	if err := setPower(ctx, running, &buf, false, true); err != nil || running.stopped != 1 {
		t.Errorf("stop failed: %v", err)
	}

	stopped := newFake(armcontainerservice.CodeStopped)
	if err := setPower(ctx, stopped, &buf, false, true); err != nil || stopped.stopped != 0 {
		t.Errorf("stop on stopped should be a no-op: %v", err)
	}
	if err := setPower(ctx, stopped, &buf, true, true); err != nil || stopped.started != 1 {
		t.Errorf("start failed: %v", err)
	}
}

func TestRotateCerts(t *testing.T) {
	ctx := context.Background()
	f := newFake(armcontainerservice.CodeRunning)
	var buf bytes.Buffer
	if err := rotateCerts(ctx, f, strings.NewReader("n\n"), &buf, false, true); err == nil || f.rotated != 0 {
		t.Error("should abort on 'n'")
	}
	if err := rotateCerts(ctx, f, strings.NewReader("y\n"), &buf, false, true); err != nil || f.rotated != 1 {
		t.Errorf("rotate on 'y': %v", err)
	}
	if err := rotateCerts(ctx, f, strings.NewReader(""), &buf, true, true); err != nil || f.rotated != 2 {
		t.Errorf("rotate with --yes: %v", err)
	}
}

type fakeLister []*armcontainerservice.ManagedCluster

func (l fakeLister) List(context.Context, string) ([]*armcontainerservice.ManagedCluster, error) {
	return l, nil
}

func TestListClusters(t *testing.T) {
	l := fakeLister{{Name: aksutil.Ptr("a"), Location: aksutil.Ptr("eastus"), ID: aksutil.Ptr("/id/a")}}
	var buf bytes.Buffer
	if err := listClusters(context.Background(), l, &buf, "", "table"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "NAME") || !strings.Contains(buf.String(), "eastus") {
		t.Errorf("unexpected output:\n%s", buf.String())
	}
}
