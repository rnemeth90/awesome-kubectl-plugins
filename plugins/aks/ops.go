package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/rnemeth90/kubectl-plugins/internal/aksutil"
)

func writeJSON(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func i32(p *int32) string {
	if p == nil {
		return ""
	}
	return fmt.Sprint(*p)
}

func enumStr[T ~string](p *T) string {
	if p == nil {
		return ""
	}
	return string(*p)
}

func done(out io.Writer, verb, name string, wait bool) error {
	if wait {
		_, err := fmt.Fprintf(out, "cluster %q %s\n", name, verb)
		return err
	}
	_, err := fmt.Fprintf(out, "cluster %q: %s accepted, not waiting for completion\n", name, verb)
	return err
}

func clusterRows(mc *armcontainerservice.ManagedCluster) [][2]string {
	rows := [][2]string{
		{"Name", aksutil.Str(mc.Name)},
		{"Location", aksutil.Str(mc.Location)},
	}
	if mc.SKU != nil {
		rows = append(rows, [2]string{"SKU Tier", enumStr(mc.SKU.Tier)})
	}
	if p := mc.Properties; p != nil {
		power := ""
		if p.PowerState != nil {
			power = enumStr(p.PowerState.Code)
		}
		fqdn := aksutil.Str(p.Fqdn)
		if fqdn == "" {
			fqdn = aksutil.Str(p.PrivateFQDN)
		}
		rows = append(rows,
			[2]string{"Kubernetes Version", aksutil.Str(p.KubernetesVersion)},
			[2]string{"Current Version", aksutil.Str(p.CurrentKubernetesVersion)},
			[2]string{"Power State", power},
			[2]string{"Provisioning State", aksutil.Str(p.ProvisioningState)},
			[2]string{"FQDN", fqdn},
			[2]string{"Node Resource Group", aksutil.Str(p.NodeResourceGroup)},
			[2]string{"Node Pools", fmt.Sprint(len(p.AgentPoolProfiles))},
		)
		if p.AutoUpgradeProfile != nil {
			rows = append(rows,
				[2]string{"Auto-upgrade Channel", enumStr(p.AutoUpgradeProfile.UpgradeChannel)},
				[2]string{"Node OS Upgrade Channel", enumStr(p.AutoUpgradeProfile.NodeOSUpgradeChannel)})
		}
		if p.NetworkProfile != nil {
			rows = append(rows,
				[2]string{"Network Plugin", enumStr(p.NetworkProfile.NetworkPlugin)},
				[2]string{"Network Policy", enumStr(p.NetworkProfile.NetworkPolicy)})
		}
	}
	return rows
}

func printRows(out io.Writer, rows [][2]string) error {
	tw := tabwriter.NewWriter(out, 0, 8, 2, ' ', 0)
	for _, r := range rows {
		fmt.Fprintf(tw, "%s:\t%s\n", r[0], r[1])
	}
	return tw.Flush()
}

func showCluster(ctx context.Context, api clusterAPI, out io.Writer, format string) error {
	mc, err := api.Get(ctx)
	if err != nil {
		return err
	}
	if format == "json" {
		return writeJSON(out, mc)
	}
	return printRows(out, clusterRows(mc))
}

func upgradeVersions(items []*armcontainerservice.ManagedClusterPoolUpgradeProfileUpgradesItem) string {
	var vs []string
	for _, u := range items {
		v := aksutil.Str(u.KubernetesVersion)
		if u.IsPreview != nil && *u.IsPreview {
			v += " (preview)"
		}
		vs = append(vs, v)
	}
	if len(vs) == 0 {
		return "none"
	}
	return strings.Join(vs, ", ")
}

func describeCluster(ctx context.Context, api clusterAPI, out io.Writer, format string) error {
	mc, err := api.Get(ctx)
	if err != nil {
		return err
	}
	up, err := api.GetUpgradeProfile(ctx)
	if err != nil {
		return fmt.Errorf("getting upgrade profile: %w", err)
	}
	if format == "json" {
		return writeJSON(out, map[string]any{"cluster": mc, "upgradeProfile": up})
	}
	rows := clusterRows(mc)
	if up.Properties != nil {
		if cp := up.Properties.ControlPlaneProfile; cp != nil {
			rows = append(rows, [2]string{"Control Plane Upgrades", upgradeVersions(cp.Upgrades)})
		}
		pools := append([]*armcontainerservice.ManagedClusterPoolUpgradeProfile(nil), up.Properties.AgentPoolProfiles...)
		sort.Slice(pools, func(i, j int) bool { return aksutil.Str(pools[i].Name) < aksutil.Str(pools[j].Name) })
		for _, p := range pools {
			rows = append(rows, [2]string{"Pool " + aksutil.Str(p.Name) + " Upgrades", upgradeVersions(p.Upgrades)})
		}
	}
	return printRows(out, rows)
}

func upgradeCluster(ctx context.Context, api clusterAPI, out io.Writer, version string, wait bool) error {
	if version == "" {
		return fmt.Errorf("--kubernetes-version is required")
	}
	mc, err := api.Get(ctx)
	if err != nil {
		return err
	}
	if mc.Properties == nil {
		mc.Properties = &armcontainerservice.ManagedClusterProperties{}
	}
	mc.Properties.KubernetesVersion = aksutil.Ptr(version)
	if err := api.CreateOrUpdate(ctx, *mc, wait); err != nil {
		return err
	}
	return done(out, "upgraded", aksutil.Str(mc.Name), wait)
}

func isRunning(mc *armcontainerservice.ManagedCluster) bool {
	p := mc.Properties
	return p == nil || p.PowerState == nil || p.PowerState.Code == nil || *p.PowerState.Code == armcontainerservice.CodeRunning
}

func setPower(ctx context.Context, api clusterAPI, out io.Writer, start, wait bool) error {
	mc, err := api.Get(ctx)
	if err != nil {
		return err
	}
	name := aksutil.Str(mc.Name)
	if start && isRunning(mc) {
		_, err := fmt.Fprintf(out, "cluster %q is already Running\n", name)
		return err
	}
	if !start && !isRunning(mc) {
		_, err := fmt.Fprintf(out, "cluster %q is already Stopped\n", name)
		return err
	}
	if start {
		if err := api.Start(ctx, wait); err != nil {
			return err
		}
		return done(out, "started", name, wait)
	}
	if err := api.Stop(ctx, wait); err != nil {
		return err
	}
	return done(out, "stopped", name, wait)
}

func rotateCerts(ctx context.Context, api clusterAPI, in io.Reader, out io.Writer, yes, wait bool) error {
	mc, err := api.Get(ctx)
	if err != nil {
		return err
	}
	name := aksutil.Str(mc.Name)
	if !yes {
		fmt.Fprintf(out, "Rotate certificates of cluster %q? This restarts the cluster. [y/N]: ", name)
		line, _ := bufio.NewReader(in).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(line)); a != "y" && a != "yes" {
			return fmt.Errorf("aborted")
		}
	}
	if err := api.RotateCertificates(ctx, wait); err != nil {
		return err
	}
	return done(out, "certificates rotated", name, wait)
}

// listClusters prints clusters returned by the lister, grouped as returned.
func listClusters(ctx context.Context, lister aksutil.ClusterLister, out io.Writer, rg, format string) error {
	clusters, err := lister.List(ctx, rg)
	if err != nil {
		return err
	}
	return printClusters(out, clusters, format)
}

func printClusters(out io.Writer, clusters []*armcontainerservice.ManagedCluster, format string) error {
	if format == "json" {
		return writeJSON(out, clusters)
	}
	tw := tabwriter.NewWriter(out, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tLOCATION\tVERSION\tPOWER\tSTATE\tID")
	for _, c := range clusters {
		ver, power, state := "", "", ""
		if p := c.Properties; p != nil {
			ver = aksutil.Str(p.CurrentKubernetesVersion)
			state = aksutil.Str(p.ProvisioningState)
			if p.PowerState != nil {
				power = enumStr(p.PowerState.Code)
			}
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", aksutil.Str(c.Name), aksutil.Str(c.Location), ver, power, state, aksutil.Str(c.ID))
	}
	return tw.Flush()
}
