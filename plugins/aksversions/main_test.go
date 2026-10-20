package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/rnemeth90/kubectl-plugins/internal/aksutil"
)

type fakeVersions []*armcontainerservice.KubernetesVersion

func (f fakeVersions) List(context.Context, string) ([]*armcontainerservice.KubernetesVersion, error) {
	return f, nil
}

func sample() fakeVersions {
	lts := armcontainerservice.KubernetesSupportPlanAKSLongTermSupport
	std := armcontainerservice.KubernetesSupportPlanKubernetesOfficial
	return fakeVersions{
		{Version: aksutil.Ptr("1.9"), PatchVersions: map[string]*armcontainerservice.KubernetesPatchVersion{"1.9.1": {}}},
		{
			Version:      aksutil.Ptr("1.30"),
			IsDefault:    aksutil.Ptr(true),
			Capabilities: &armcontainerservice.KubernetesVersionCapabilities{SupportPlan: []*armcontainerservice.KubernetesSupportPlan{&std, &lts}},
			PatchVersions: map[string]*armcontainerservice.KubernetesPatchVersion{
				"1.30.2":  {},
				"1.30.10": {Upgrades: []*string{aksutil.Ptr("1.31.1")}},
			},
		},
		{Version: aksutil.Ptr("1.31"), IsPreview: aksutil.Ptr(true)},
	}
}

func TestRunTable(t *testing.T) {
	var buf bytes.Buffer
	if err := run(context.Background(), sample(), &buf, "eastus", "table", false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// Numeric sort, newest first: 1.31, 1.30, 1.9.
	if !strings.HasPrefix(lines[1], "1.31") || !strings.HasPrefix(lines[2], "1.30") || !strings.HasPrefix(lines[3], "1.9") {
		t.Errorf("unexpected order:\n%s", out)
	}
	if !strings.Contains(out, "1.30.10, 1.30.2") {
		t.Errorf("patches not numerically sorted:\n%s", out)
	}
	if !strings.Contains(out, "KubernetesOfficial,AKSLongTermSupport") {
		t.Errorf("support plan missing:\n%s", out)
	}
}

func TestRunPatches(t *testing.T) {
	var buf bytes.Buffer
	if err := run(context.Background(), sample(), &buf, "eastus", "table", true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "1.30.10") || !strings.Contains(buf.String(), "1.31.1") {
		t.Errorf("unexpected output:\n%s", buf.String())
	}
}

func TestRunJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := run(context.Background(), sample(), &buf, "eastus", "json", false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"version": "1.30"`) {
		t.Errorf("unexpected json:\n%s", buf.String())
	}
}
