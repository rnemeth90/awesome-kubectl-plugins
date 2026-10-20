package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/rnemeth90/kubectl-plugins/internal/aksutil"
)

const (
	nameDefault  = "default"
	nameAutoUpg  = "aksManagedAutoUpgradeSchedule"
	nameNodeOS   = "aksManagedNodeOSUpgradeSchedule"
	aliasAutoUpg = "auto-upgrade"
	aliasNodeOS  = "node-os"
)

// maintAPI is the subset of the AKS maintenance configuration API used by the verbs.
type maintAPI interface {
	List(ctx context.Context) ([]*armcontainerservice.MaintenanceConfiguration, error)
	Get(ctx context.Context, name string) (*armcontainerservice.MaintenanceConfiguration, error)
	CreateOrUpdate(ctx context.Context, name string, cfg armcontainerservice.MaintenanceConfiguration) error
	Delete(ctx context.Context, name string) error
}

// normalizeName maps friendly aliases to the fixed ARM configuration names.
func normalizeName(n string) (string, error) {
	switch strings.ToLower(n) {
	case nameDefault:
		return nameDefault, nil
	case aliasAutoUpg, strings.ToLower(nameAutoUpg):
		return nameAutoUpg, nil
	case aliasNodeOS, strings.ToLower(nameNodeOS):
		return nameNodeOS, nil
	}
	return "", fmt.Errorf("unsupported configuration %q: expected %s, %s or %s", n, nameDefault, aliasAutoUpg, aliasNodeOS)
}

var timeRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
var offsetRe = regexp.MustCompile(`^[+-]([01]\d|2[0-3]):[0-5]\d$`)

type addOptions struct {
	day           string
	startTime     string
	durationHours int32
	intervalWeeks int32
	utcOffset     string
	hours         []int32
}

func parseDay(d string) (armcontainerservice.WeekDay, error) {
	for _, w := range armcontainerservice.PossibleWeekDayValues() {
		if strings.EqualFold(string(w), d) {
			return w, nil
		}
	}
	return "", fmt.Errorf("invalid --day %q: expected Monday..Sunday", d)
}

// buildConfig creates the ARM model for name. The default configuration uses
// weekly hour slots (UTC); the managed schedules use a recurring window.
func buildConfig(name string, o addOptions) (armcontainerservice.MaintenanceConfiguration, error) {
	var cfg armcontainerservice.MaintenanceConfiguration
	day, err := parseDay(o.day)
	if err != nil {
		return cfg, err
	}
	if name == nameDefault {
		if len(o.hours) == 0 {
			return cfg, fmt.Errorf("--hours is required for the default configuration, e.g. --hours 1,2")
		}
		slots := make([]*int32, 0, len(o.hours))
		for _, h := range o.hours {
			if h < 0 || h > 23 {
				return cfg, fmt.Errorf("invalid hour %d: expected 0-23", h)
			}
			slots = append(slots, aksutil.Ptr(h))
		}
		cfg.Properties = &armcontainerservice.MaintenanceConfigurationProperties{
			TimeInWeek: []*armcontainerservice.TimeInWeek{{Day: &day, HourSlots: slots}},
		}
		return cfg, nil
	}

	if !timeRe.MatchString(o.startTime) {
		return cfg, fmt.Errorf("invalid --start-time %q: expected HH:mm", o.startTime)
	}
	if o.durationHours < 4 || o.durationHours > 24 {
		return cfg, fmt.Errorf("invalid --duration-hours %d: expected 4-24", o.durationHours)
	}
	if o.intervalWeeks < 1 {
		return cfg, fmt.Errorf("invalid --interval-weeks %d: expected >= 1", o.intervalWeeks)
	}
	if o.utcOffset != "" && !offsetRe.MatchString(o.utcOffset) {
		return cfg, fmt.Errorf("invalid --utc-offset %q: expected +/-HH:mm", o.utcOffset)
	}
	w := &armcontainerservice.MaintenanceWindow{
		DurationHours: aksutil.Ptr(o.durationHours),
		StartTime:     aksutil.Ptr(o.startTime),
		Schedule: &armcontainerservice.Schedule{
			Weekly: &armcontainerservice.WeeklySchedule{DayOfWeek: &day, IntervalWeeks: aksutil.Ptr(o.intervalWeeks)},
		},
	}
	if o.utcOffset != "" {
		w.UTCOffset = aksutil.Ptr(o.utcOffset)
	}
	cfg.Properties = &armcontainerservice.MaintenanceConfigurationProperties{MaintenanceWindow: w}
	return cfg, nil
}

func describeSchedule(c *armcontainerservice.MaintenanceConfiguration) string {
	p := c.Properties
	if p == nil {
		return ""
	}
	if w := p.MaintenanceWindow; w != nil && w.Schedule != nil {
		s := w.Schedule
		offset := aksutil.Str(w.UTCOffset)
		if offset == "" {
			offset = "+00:00"
		}
		dur := ""
		if w.DurationHours != nil {
			dur = fmt.Sprintf(" for %dh", *w.DurationHours)
		}
		at := fmt.Sprintf("at %s (UTC%s)%s", aksutil.Str(w.StartTime), offset, dur)
		switch {
		case s.Weekly != nil:
			return fmt.Sprintf("every %s week(s) on %s %s", i32(s.Weekly.IntervalWeeks), enumStr(s.Weekly.DayOfWeek), at)
		case s.Daily != nil:
			return fmt.Sprintf("every %s day(s) %s", i32(s.Daily.IntervalDays), at)
		case s.AbsoluteMonthly != nil:
			return fmt.Sprintf("every %s month(s) on day %s %s", i32(s.AbsoluteMonthly.IntervalMonths), i32(s.AbsoluteMonthly.DayOfMonth), at)
		case s.RelativeMonthly != nil:
			return fmt.Sprintf("every %s month(s) on %s %s %s", i32(s.RelativeMonthly.IntervalMonths),
				enumStr(s.RelativeMonthly.WeekIndex), enumStr(s.RelativeMonthly.DayOfWeek), at)
		}
	}
	var parts []string
	for _, t := range p.TimeInWeek {
		var hs []string
		for _, h := range t.HourSlots {
			if h != nil {
				hs = append(hs, fmt.Sprint(*h))
			}
		}
		parts = append(parts, fmt.Sprintf("%s hours %s UTC", enumStr(t.Day), strings.Join(hs, ",")))
	}
	return strings.Join(parts, "; ")
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

func listConfigs(ctx context.Context, api maintAPI, out io.Writer, format string) error {
	cs, err := api.List(ctx)
	if err != nil {
		return err
	}
	if format == "json" {
		return writeJSON(out, cs)
	}
	sort.Slice(cs, func(i, j int) bool { return aksutil.Str(cs[i].Name) < aksutil.Str(cs[j].Name) })
	tw := tabwriter.NewWriter(out, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tSCHEDULE")
	for _, c := range cs {
		fmt.Fprintf(tw, "%s\t%s\n", aksutil.Str(c.Name), describeSchedule(c))
	}
	return tw.Flush()
}

func showConfig(ctx context.Context, api maintAPI, out io.Writer, name, format string) error {
	n, err := normalizeName(name)
	if err != nil {
		return err
	}
	c, err := api.Get(ctx, n)
	if err != nil {
		return err
	}
	if format == "json" {
		return writeJSON(out, c)
	}
	tw := tabwriter.NewWriter(out, 0, 8, 2, ' ', 0)
	fmt.Fprintf(tw, "Name:\t%s\nSchedule:\t%s\n", aksutil.Str(c.Name), describeSchedule(c))
	if c.Properties != nil {
		for _, ns := range c.Properties.NotAllowedTime {
			if ns.Start != nil && ns.End != nil {
				fmt.Fprintf(tw, "Not Allowed:\t%s to %s\n", ns.Start.Format("2006-01-02 15:04"), ns.End.Format("2006-01-02 15:04"))
			}
		}
	}
	return tw.Flush()
}

func addConfig(ctx context.Context, api maintAPI, out io.Writer, name string, o addOptions) error {
	n, err := normalizeName(name)
	if err != nil {
		return err
	}
	cfg, err := buildConfig(n, o)
	if err != nil {
		return err
	}
	if err := api.CreateOrUpdate(ctx, n, cfg); err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "maintenance configuration %q saved\n", n)
	return err
}

func deleteConfig(ctx context.Context, api maintAPI, out io.Writer, name string) error {
	n, err := normalizeName(name)
	if err != nil {
		return err
	}
	if err := api.Delete(ctx, n); err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "maintenance configuration %q deleted\n", n)
	return err
}

func writeJSON(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
