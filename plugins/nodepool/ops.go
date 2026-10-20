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
)

func ptr[T any](v T) *T { return &v }

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func i32(p *int32) string {
	if p == nil {
		return ""
	}
	return fmt.Sprint(*p)
}

func props(p *armcontainerservice.AgentPool) *armcontainerservice.ManagedClusterAgentPoolProfileProperties {
	if p.Properties == nil {
		p.Properties = &armcontainerservice.ManagedClusterAgentPoolProfileProperties{}
	}
	return p.Properties
}

func power(p *armcontainerservice.ManagedClusterAgentPoolProfileProperties) string {
	if p.PowerState == nil || p.PowerState.Code == nil {
		return ""
	}
	return string(*p.PowerState.Code)
}

func mode(p *armcontainerservice.ManagedClusterAgentPoolProfileProperties) string {
	if p.Mode == nil {
		return ""
	}
	return string(*p.Mode)
}

func writeJSON(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func listPools(ctx context.Context, api poolAPI, out io.Writer, format string) error {
	pools, err := api.List(ctx)
	if err != nil {
		return err
	}
	if format == "json" {
		return writeJSON(out, pools)
	}
	tw := tabwriter.NewWriter(out, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tMODE\tVM SIZE\tCOUNT\tVERSION\tPOWER\tSTATE")
	for _, p := range pools {
		pp := props(p)
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			str(p.Name), mode(pp), str(pp.VMSize), i32(pp.Count),
			str(pp.CurrentOrchestratorVersion), power(pp), str(pp.ProvisioningState))
	}
	return tw.Flush()
}

func showPool(ctx context.Context, api poolAPI, out io.Writer, name, format string) error {
	p, err := api.Get(ctx, name)
	if err != nil {
		return err
	}
	if format == "json" {
		return writeJSON(out, p)
	}
	pp := props(p)
	rows := [][2]string{
		{"Name", str(p.Name)},
		{"Mode", mode(pp)},
		{"VM Size", str(pp.VMSize)},
		{"Count", i32(pp.Count)},
		{"Autoscaling", fmt.Sprint(pp.EnableAutoScaling != nil && *pp.EnableAutoScaling)},
		{"Min Count", i32(pp.MinCount)},
		{"Max Count", i32(pp.MaxCount)},
		{"Kubernetes Version", str(pp.CurrentOrchestratorVersion)},
		{"Target Version", str(pp.OrchestratorVersion)},
		{"Node Image", str(pp.NodeImageVersion)},
		{"Power State", power(pp)},
		{"Provisioning State", str(pp.ProvisioningState)},
		{"Labels", formatLabels(pp.NodeLabels)},
		{"Taints", strings.Join(derefAll(pp.NodeTaints), ",")},
	}
	if pp.UpgradeSettings != nil {
		rows = append(rows, [2]string{"Max Surge", str(pp.UpgradeSettings.MaxSurge)})
	}
	tw := tabwriter.NewWriter(out, 0, 8, 2, ' ', 0)
	for _, r := range rows {
		fmt.Fprintf(tw, "%s:\t%s\n", r[0], r[1])
	}
	return tw.Flush()
}

func describePool(ctx context.Context, api poolAPI, out io.Writer, name, format string) error {
	p, err := api.Get(ctx, name)
	if err != nil {
		return err
	}
	up, err := api.GetUpgradeProfile(ctx, name)
	if err != nil {
		return fmt.Errorf("getting upgrade profile: %w", err)
	}
	if format == "json" {
		return writeJSON(out, map[string]any{"agentPool": p, "upgradeProfile": up})
	}

	pp := props(p)
	rows := [][2]string{
		{"Name", str(p.Name)},
		{"ID", str(p.ID)},
		{"Mode", mode(pp)},
		{"OS Type", enumStr(pp.OSType)},
		{"VM Size", str(pp.VMSize)},
		{"OS Disk Size (GB)", i32(pp.OSDiskSizeGB)},
		{"Max Pods", i32(pp.MaxPods)},
		{"Zones", strings.Join(derefAll(pp.AvailabilityZones), ",")},
		{"Subnet", str(pp.VnetSubnetID)},
		{"Count", i32(pp.Count)},
		{"Autoscaling", fmt.Sprint(pp.EnableAutoScaling != nil && *pp.EnableAutoScaling)},
		{"Min Count", i32(pp.MinCount)},
		{"Max Count", i32(pp.MaxCount)},
		{"Kubernetes Version", str(pp.CurrentOrchestratorVersion)},
		{"Target Version", str(pp.OrchestratorVersion)},
		{"Node Image", str(pp.NodeImageVersion)},
		{"Power State", power(pp)},
		{"Provisioning State", str(pp.ProvisioningState)},
		{"Labels", formatLabels(pp.NodeLabels)},
		{"Taints", strings.Join(derefAll(pp.NodeTaints), ",")},
	}
	if pp.UpgradeSettings != nil {
		rows = append(rows,
			[2]string{"Max Surge", str(pp.UpgradeSettings.MaxSurge)},
			[2]string{"Drain Timeout (min)", i32(pp.UpgradeSettings.DrainTimeoutInMinutes)})
	}
	if up.Properties != nil {
		rows = append(rows, [2]string{"Latest Node Image", str(up.Properties.LatestNodeImageVersion)})
		var versions []string
		for _, u := range up.Properties.Upgrades {
			v := str(u.KubernetesVersion)
			if u.IsPreview != nil && *u.IsPreview {
				v += " (preview)"
			}
			versions = append(versions, v)
		}
		avail := strings.Join(versions, ", ")
		if avail == "" {
			avail = "none"
		}
		rows = append(rows, [2]string{"Available Upgrades", avail})
	}
	tw := tabwriter.NewWriter(out, 0, 8, 2, ' ', 0)
	for _, r := range rows {
		fmt.Fprintf(tw, "%s:\t%s\n", r[0], r[1])
	}
	return tw.Flush()
}

func enumStr[T ~string](p *T) string {
	if p == nil {
		return ""
	}
	return string(*p)
}

func derefAll(in []*string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s != nil {
			out = append(out, *s)
		}
	}
	return out
}

func formatLabels(m map[string]*string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+str(m[k]))
	}
	return strings.Join(parts, ",")
}

// parseLabels parses "k=v,k2=v2".
func parseLabels(s string) (map[string]*string, error) {
	out := map[string]*string{}
	if s == "" {
		return out, nil
	}
	for _, kv := range strings.Split(s, ",") {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("invalid label %q: expected key=value", kv)
		}
		out[k] = ptr(v)
	}
	return out, nil
}

// parseTaints validates "key=value:Effect" entries.
func parseTaints(s string) ([]*string, error) {
	out := []*string{}
	if s == "" {
		return out, nil
	}
	for _, t := range strings.Split(s, ",") {
		kv, effect, ok := strings.Cut(t, ":")
		if !ok || kv == "" || !validEffect(effect) {
			return nil, fmt.Errorf("invalid taint %q: expected key=value:NoSchedule|PreferNoSchedule|NoExecute", t)
		}
		out = append(out, ptr(t))
	}
	return out, nil
}

func validEffect(e string) bool {
	return e == "NoSchedule" || e == "PreferNoSchedule" || e == "NoExecute"
}

type addOptions struct {
	vmSize      string
	count       int32
	mode        string
	version     string
	labels      string
	taints      string
	autoscale   bool
	minCount    int32
	maxCount    int32
	maxSurge    string
	osType      string
	maxPods     int32
	zones       []string
	subnetID    string
	osDiskSizeG int32
}

func addPool(ctx context.Context, api poolAPI, out io.Writer, name string, o addOptions, wait bool) error {
	labels, err := parseLabels(o.labels)
	if err != nil {
		return err
	}
	taints, err := parseTaints(o.taints)
	if err != nil {
		return err
	}
	if o.autoscale && (o.minCount <= 0 || o.maxCount < o.minCount) {
		return fmt.Errorf("--enable-cluster-autoscaler requires --min-count >= 1 and --max-count >= --min-count")
	}
	pm := armcontainerservice.AgentPoolMode(o.mode)
	if pm != armcontainerservice.AgentPoolModeUser && pm != armcontainerservice.AgentPoolModeSystem {
		return fmt.Errorf("invalid --mode %q: expected User or System", o.mode)
	}

	pp := &armcontainerservice.ManagedClusterAgentPoolProfileProperties{
		VMSize:            ptr(o.vmSize),
		Count:             ptr(o.count),
		Mode:              &pm,
		Type:              ptr(armcontainerservice.AgentPoolTypeVirtualMachineScaleSets),
		EnableAutoScaling: ptr(o.autoscale),
		NodeLabels:        labels,
		NodeTaints:        taints,
	}
	if o.autoscale {
		pp.MinCount = ptr(o.minCount)
		pp.MaxCount = ptr(o.maxCount)
	}
	if o.version != "" {
		pp.OrchestratorVersion = ptr(o.version)
	}
	if o.maxSurge != "" {
		pp.UpgradeSettings = &armcontainerservice.AgentPoolUpgradeSettings{MaxSurge: ptr(o.maxSurge)}
	}
	if o.osType != "" {
		pp.OSType = ptr(armcontainerservice.OSType(o.osType))
	}
	if o.maxPods > 0 {
		pp.MaxPods = ptr(o.maxPods)
	}
	if len(o.zones) > 0 {
		for _, z := range o.zones {
			pp.AvailabilityZones = append(pp.AvailabilityZones, ptr(z))
		}
	}
	if o.subnetID != "" {
		pp.VnetSubnetID = ptr(o.subnetID)
	}
	if o.osDiskSizeG > 0 {
		pp.OSDiskSizeGB = ptr(o.osDiskSizeG)
	}

	if _, err := api.CreateOrUpdate(ctx, name, armcontainerservice.AgentPool{Properties: pp}, wait); err != nil {
		return err
	}
	return done(out, "created", name, wait)
}

func done(out io.Writer, verb, name string, wait bool) error {
	if wait {
		_, err := fmt.Fprintf(out, "nodepool %q %s\n", name, verb)
		return err
	}
	_, err := fmt.Fprintf(out, "nodepool %q: %s accepted, not waiting for completion\n", name, verb)
	return err
}

func deletePool(ctx context.Context, api poolAPI, in io.Reader, out io.Writer, name string, yes, wait bool) error {
	if !yes {
		fmt.Fprintf(out, "Delete nodepool %q? [y/N]: ", name)
		line, _ := bufio.NewReader(in).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(line)); a != "y" && a != "yes" {
			return fmt.Errorf("aborted")
		}
	}
	if err := api.Delete(ctx, name, wait); err != nil {
		return err
	}
	return done(out, "deleted", name, wait)
}

func scalePool(ctx context.Context, api poolAPI, out io.Writer, name string, count int32, wait bool) error {
	if count < 0 {
		return fmt.Errorf("--count must be >= 0")
	}
	p, err := api.Get(ctx, name)
	if err != nil {
		return err
	}
	pp := props(p)
	if pp.EnableAutoScaling != nil && *pp.EnableAutoScaling {
		return fmt.Errorf("nodepool %q has the cluster autoscaler enabled; use `update --min-count/--max-count` or disable it first", name)
	}
	pp.Count = ptr(count)
	if _, err := api.CreateOrUpdate(ctx, name, *p, wait); err != nil {
		return err
	}
	return done(out, "scaled", name, wait)
}

type updateOptions struct {
	enableAutoscaler  bool
	disableAutoscaler bool
	minCount          *int32
	maxCount          *int32
	labels            *string
	taints            *string
	maxSurge          *string
	mode              *string
}

func updatePool(ctx context.Context, api poolAPI, out io.Writer, name string, o updateOptions, wait bool) error {
	if o.enableAutoscaler && o.disableAutoscaler {
		return fmt.Errorf("--enable-cluster-autoscaler and --disable-cluster-autoscaler are mutually exclusive")
	}
	if o.enableAutoscaler == false && o.disableAutoscaler == false && o.minCount == nil && o.maxCount == nil &&
		o.labels == nil && o.taints == nil && o.maxSurge == nil && o.mode == nil {
		return fmt.Errorf("nothing to update: specify at least one flag")
	}
	p, err := api.Get(ctx, name)
	if err != nil {
		return err
	}
	pp := props(p)

	switch {
	case o.enableAutoscaler:
		pp.EnableAutoScaling = ptr(true)
	case o.disableAutoscaler:
		pp.EnableAutoScaling = ptr(false)
		pp.MinCount, pp.MaxCount = nil, nil
	}
	if o.minCount != nil {
		pp.MinCount = o.minCount
	}
	if o.maxCount != nil {
		pp.MaxCount = o.maxCount
	}
	if pp.EnableAutoScaling != nil && *pp.EnableAutoScaling {
		if pp.MinCount == nil || pp.MaxCount == nil || *pp.MinCount < 1 || *pp.MaxCount < *pp.MinCount {
			return fmt.Errorf("autoscaler requires --min-count >= 1 and --max-count >= --min-count")
		}
	} else if o.minCount != nil || o.maxCount != nil {
		return fmt.Errorf("--min-count/--max-count require the cluster autoscaler; add --enable-cluster-autoscaler")
	}
	if o.labels != nil {
		if pp.NodeLabels, err = parseLabels(*o.labels); err != nil {
			return err
		}
	}
	if o.taints != nil {
		if pp.NodeTaints, err = parseTaints(*o.taints); err != nil {
			return err
		}
	}
	if o.maxSurge != nil {
		if pp.UpgradeSettings == nil {
			pp.UpgradeSettings = &armcontainerservice.AgentPoolUpgradeSettings{}
		}
		pp.UpgradeSettings.MaxSurge = o.maxSurge
	}
	if o.mode != nil {
		m := armcontainerservice.AgentPoolMode(*o.mode)
		if m != armcontainerservice.AgentPoolModeUser && m != armcontainerservice.AgentPoolModeSystem {
			return fmt.Errorf("invalid --mode %q: expected User or System", *o.mode)
		}
		pp.Mode = &m
	}

	if _, err := api.CreateOrUpdate(ctx, name, *p, wait); err != nil {
		return err
	}
	return done(out, "updated", name, wait)
}

func upgradePool(ctx context.Context, api poolAPI, out io.Writer, name, version string, wait bool) error {
	if version == "" {
		return fmt.Errorf("--kubernetes-version is required")
	}
	p, err := api.Get(ctx, name)
	if err != nil {
		return err
	}
	props(p).OrchestratorVersion = ptr(version)
	if _, err := api.CreateOrUpdate(ctx, name, *p, wait); err != nil {
		return err
	}
	return done(out, "upgraded", name, wait)
}

func upgradeNodeImage(ctx context.Context, api poolAPI, out io.Writer, name string, wait bool) error {
	if err := api.UpgradeNodeImage(ctx, name, wait); err != nil {
		return err
	}
	return done(out, "node image upgraded", name, wait)
}

func setPower(ctx context.Context, api poolAPI, out io.Writer, name string, code armcontainerservice.Code, wait bool) error {
	p, err := api.Get(ctx, name)
	if err != nil {
		return err
	}
	pp := props(p)
	if pp.PowerState != nil && pp.PowerState.Code != nil && *pp.PowerState.Code == code {
		_, err := fmt.Fprintf(out, "nodepool %q is already %s\n", name, code)
		return err
	}
	pp.PowerState = &armcontainerservice.PowerState{Code: &code}
	if _, err := api.CreateOrUpdate(ctx, name, *p, wait); err != nil {
		return err
	}
	verb := "started"
	if code == armcontainerservice.CodeStopped {
		verb = "stopped"
	}
	return done(out, verb, name, wait)
}

func cancelOperation(ctx context.Context, api poolAPI, out io.Writer, name string, wait bool) error {
	if err := api.AbortLatestOperation(ctx, name, wait); err != nil {
		return err
	}
	return done(out, "operation cancelled", name, wait)
}

// taintKey returns the "key" and "effect" of a "key[=value]:Effect" taint.
func taintKey(t string) (key, effect string) {
	kv, effect, _ := strings.Cut(t, ":")
	key, _, _ = strings.Cut(kv, "=")
	return key, effect
}

// applyTaints applies kubectl-style specs to the existing taints: "key=value:Effect"
// adds (replacing a taint with the same key and effect), "key:Effect-" removes
// that taint and "key-" removes every taint with that key.
func applyTaints(existing []*string, specs []string) ([]*string, error) {
	cur := derefAll(existing)
	for _, spec := range specs {
		if strings.HasSuffix(spec, "-") {
			rm := strings.TrimSuffix(spec, "-")
			key, effect := taintKey(rm)
			if key == "" || (effect != "" && !validEffect(effect)) {
				return nil, fmt.Errorf("invalid taint removal %q: expected key- or key:Effect-", spec)
			}
			kept := cur[:0:0]
			removed := false
			for _, t := range cur {
				k, e := taintKey(t)
				if k == key && (effect == "" || e == effect) {
					removed = true
					continue
				}
				kept = append(kept, t)
			}
			if !removed {
				return nil, fmt.Errorf("taint %q not found", rm)
			}
			cur = kept
			continue
		}
		if _, err := parseTaints(spec); err != nil {
			return nil, err
		}
		key, effect := taintKey(spec)
		kept := cur[:0:0]
		for _, t := range cur {
			if k, e := taintKey(t); k == key && e == effect {
				continue
			}
			kept = append(kept, t)
		}
		cur = append(kept, spec)
	}
	out := make([]*string, 0, len(cur))
	for _, t := range cur {
		out = append(out, ptr(t))
	}
	return out, nil
}

func taintPool(ctx context.Context, api poolAPI, out io.Writer, name string, specs []string, wait bool) error {
	if len(specs) == 0 {
		return fmt.Errorf("at least one taint is required, e.g. key=value:NoSchedule or key-")
	}
	p, err := api.Get(ctx, name)
	if err != nil {
		return err
	}
	pp := props(p)
	if pp.NodeTaints, err = applyTaints(pp.NodeTaints, specs); err != nil {
		return err
	}
	if _, err := api.CreateOrUpdate(ctx, name, *p, wait); err != nil {
		return err
	}
	return done(out, "tainted", name, wait)
}

// untaintPool removes taints given as "key" or "key:Effect".
func untaintPool(ctx context.Context, api poolAPI, out io.Writer, name string, keys []string, wait bool) error {
	specs := make([]string, len(keys))
	for i, k := range keys {
		if strings.HasSuffix(k, "-") {
			return fmt.Errorf("invalid taint %q: untaint takes key or key:Effect without a trailing '-'", k)
		}
		specs[i] = k + "-"
	}
	return taintPool(ctx, api, out, name, specs, wait)
}
