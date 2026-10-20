package main

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
)

type fakeMaint struct {
	cfgs    map[string]armcontainerservice.MaintenanceConfiguration
	deleted []string
}

func newFake() *fakeMaint {
	return &fakeMaint{cfgs: map[string]armcontainerservice.MaintenanceConfiguration{}}
}

func (f *fakeMaint) List(context.Context) ([]*armcontainerservice.MaintenanceConfiguration, error) {
	var out []*armcontainerservice.MaintenanceConfiguration
	for n, c := range f.cfgs {
		c := c
		c.Name = &n
		out = append(out, &c)
	}
	return out, nil
}
func (f *fakeMaint) Get(_ context.Context, n string) (*armcontainerservice.MaintenanceConfiguration, error) {
	c, ok := f.cfgs[n]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	c.Name = &n
	return &c, nil
}
func (f *fakeMaint) CreateOrUpdate(_ context.Context, n string, c armcontainerservice.MaintenanceConfiguration) error {
	f.cfgs[n] = c
	return nil
}
func (f *fakeMaint) Delete(_ context.Context, n string) error {
	delete(f.cfgs, n)
	f.deleted = append(f.deleted, n)
	return nil
}

func TestNormalizeName(t *testing.T) {
	for in, want := range map[string]string{
		"default": nameDefault, "Auto-Upgrade": nameAutoUpg, "node-os": nameNodeOS,
		"aksManagedAutoUpgradeSchedule": nameAutoUpg, "aksmanagednodeosupgradeschedule": nameNodeOS,
	} {
		got, err := normalizeName(in)
		if err != nil || got != want {
			t.Errorf("normalizeName(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := normalizeName("other"); err == nil {
		t.Error("expected error for unknown name")
	}
}

func TestAddWindowAndShow(t *testing.T) {
	ctx := context.Background()
	f := newFake()
	var buf bytes.Buffer
	o := addOptions{day: "sunday", startTime: "02:00", durationHours: 4, intervalWeeks: 2, utcOffset: "-05:00"}
	if err := addConfig(ctx, f, &buf, "node-os", o); err != nil {
		t.Fatal(err)
	}
	w := f.cfgs[nameNodeOS].Properties.MaintenanceWindow
	if *w.Schedule.Weekly.DayOfWeek != armcontainerservice.WeekDaySunday || *w.Schedule.Weekly.IntervalWeeks != 2 || *w.UTCOffset != "-05:00" {
		t.Errorf("unexpected window: %+v", w)
	}
	buf.Reset()
	if err := showConfig(ctx, f, &buf, "node-os", "table"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "every 2 week(s) on Sunday at 02:00 (UTC-05:00) for 4h") {
		t.Errorf("unexpected show:\n%s", buf.String())
	}
	buf.Reset()
	if err := listConfigs(ctx, f, &buf, "table"); err != nil || !strings.Contains(buf.String(), nameNodeOS) {
		t.Errorf("list: %v\n%s", err, buf.String())
	}
}

func TestAddDefault(t *testing.T) {
	f := newFake()
	var buf bytes.Buffer
	if err := addConfig(context.Background(), f, &buf, "default", addOptions{day: "Monday", hours: []int32{1, 2}}); err != nil {
		t.Fatal(err)
	}
	tw := f.cfgs[nameDefault].Properties.TimeInWeek
	if len(tw) != 1 || len(tw[0].HourSlots) != 2 {
		t.Errorf("unexpected time in week: %+v", tw)
	}
	buf.Reset()
	_ = showConfig(context.Background(), f, &buf, "default", "table")
	if !strings.Contains(buf.String(), "Monday hours 1,2 UTC") {
		t.Errorf("unexpected show:\n%s", buf.String())
	}
}

func TestAddValidation(t *testing.T) {
	ok := addOptions{day: "Monday", startTime: "02:00", durationHours: 4, intervalWeeks: 1}
	cases := map[string]struct {
		name string
		mut  func(*addOptions)
	}{
		"bad day":          {"node-os", func(o *addOptions) { o.day = "Funday" }},
		"bad time":         {"node-os", func(o *addOptions) { o.startTime = "25:00" }},
		"short window":     {"node-os", func(o *addOptions) { o.durationHours = 2 }},
		"bad interval":     {"node-os", func(o *addOptions) { o.intervalWeeks = 0 }},
		"bad offset":       {"node-os", func(o *addOptions) { o.utcOffset = "5" }},
		"default no hours": {"default", func(o *addOptions) {}},
		"bad hour":         {"default", func(o *addOptions) { o.hours = []int32{24} }},
		"unknown name":     {"weird", func(o *addOptions) {}},
	}
	for n, c := range cases {
		t.Run(n, func(t *testing.T) {
			o := ok
			c.mut(&o)
			var buf bytes.Buffer
			if err := addConfig(context.Background(), newFake(), &buf, c.name, o); err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestDelete(t *testing.T) {
	f := newFake()
	f.cfgs[nameNodeOS] = armcontainerservice.MaintenanceConfiguration{}
	var buf bytes.Buffer
	if err := deleteConfig(context.Background(), f, &buf, "node-os"); err != nil || len(f.deleted) != 1 || f.deleted[0] != nameNodeOS {
		t.Errorf("delete failed: %v %v", err, f.deleted)
	}
}
