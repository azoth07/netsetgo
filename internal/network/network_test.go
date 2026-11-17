package network

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func staticProfile() Profile {
	return Profile{
		AdapterID: "12",
		Addresses: []Address{{IP: "192.168.1.20", Mask: "255.255.255.0"}},
		Gateways:  []string{"192.168.1.1"},
		DNS:       []string{"223.5.5.5", "1.1.1.1"},
		Targets:   []string{"example.com", "192.168.1.1"},
	}
}

func TestValidateRejectsInvalidProfiles(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Profile)
	}{
		{"zero index", func(profile *Profile) { profile.AdapterID = "0" }},
		{"leading zero index", func(profile *Profile) { profile.AdapterID = "012" }},
		{"oversized index", func(profile *Profile) { profile.AdapterID = "4294967296" }},
		{"injected index", func(profile *Profile) { profile.AdapterID = "12;whoami" }},
		{"oversized name", func(profile *Profile) { profile.Name = strings.Repeat("a", 129) }},
		{"control name", func(profile *Profile) { profile.Name = "a\nb" }},
		{"invalid UTF8", func(profile *Profile) { profile.Name = string([]byte{255}) }},
		{"no address", func(profile *Profile) { profile.Addresses = nil }},
		{"DHCP static address", func(profile *Profile) { profile.DHCP = true }},
		{"DHCP gateway", func(profile *Profile) { profile.DHCP = true; profile.Addresses = nil }},
		{"IPv6", func(profile *Profile) { profile.Addresses[0].IP = "::1" }},
		{"leading zero IPv4", func(profile *Profile) { profile.Addresses[0].IP = "192.168.001.20" }},
		{"loopback", func(profile *Profile) { profile.Addresses[0].IP = "127.0.0.1" }},
		{"multicast", func(profile *Profile) { profile.Addresses[0].IP = "224.0.0.1" }},
		{"network", func(profile *Profile) { profile.Addresses[0].IP = "192.168.1.0" }},
		{"broadcast", func(profile *Profile) { profile.Addresses[0].IP = "192.168.1.255" }},
		{"noncontiguous mask", func(profile *Profile) { profile.Addresses[0].Mask = "255.0.255.0" }},
		{"zero mask", func(profile *Profile) { profile.Addresses[0].Mask = "0.0.0.0" }},
		{"prefix as mask", func(profile *Profile) { profile.Addresses[0].Mask = "24" }},
		{"duplicate IP", func(profile *Profile) { profile.Addresses = append(profile.Addresses, profile.Addresses[0]) }},
		{"too many IPs", func(profile *Profile) { profile.Addresses = make([]Address, 17) }},
		{"offlink gateway", func(profile *Profile) { profile.Gateways = []string{"10.0.0.1"} }},
		{"self gateway", func(profile *Profile) { profile.Gateways = []string{"192.168.1.20"} }},
		{"network gateway", func(profile *Profile) { profile.Gateways = []string{"192.168.1.0"} }},
		{"broadcast gateway", func(profile *Profile) { profile.Gateways = []string{"192.168.1.255"} }},
		{"duplicate gateway", func(profile *Profile) { profile.Gateways = []string{"192.168.1.1", "192.168.1.1"} }},
		{"too many gateways", func(profile *Profile) { profile.Gateways = make([]string, 5) }},
		{"DNS hostname", func(profile *Profile) { profile.DNS = []string{"dns.example.com"} }},
		{"DNS unspecified", func(profile *Profile) { profile.DNS = []string{"0.0.0.0"} }},
		{"duplicate DNS", func(profile *Profile) { profile.DNS = []string{"1.1.1.1", "1.1.1.1"} }},
		{"too many DNS", func(profile *Profile) { profile.DNS = make([]string, 9) }},
		{"too many targets", func(profile *Profile) { profile.Targets = make([]string, 17) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profile := staticProfile()
			test.change(&profile)
			if err := Validate(profile); err == nil {
				t.Fatal("expected validation error")
			}
			if commands, err := Plan(profile); err == nil || commands != nil {
				t.Fatal("invalid profile produced commands")
			}
		})
	}
}

func TestTargetValidation(t *testing.T) {
	for _, target := range []string{
		"", "-n", "a b", "a'b", "a;whoami", "$(whoami)", "https://example.com",
		"example.com:80", "a..b", "a_.com", "*.com", "a-.com", "::1",
		"999.1.1.1", "127.01.0.1", "0.0.0.0", "255.255.255.255", "224.0.0.1",
		strings.Repeat("a", 64) + ".com", strings.Repeat("a.", 127), "中文.com",
	} {
		if err := validateTargets([]string{target}); err == nil {
			t.Errorf("accepted invalid target %q", target)
		}
	}
	for _, target := range []string{"localhost", "example.com", "host-1.example", "127.0.0.1", "1.1.1.1"} {
		if err := validateTargets([]string{target}); err != nil {
			t.Errorf("rejected target %q: %v", target, err)
		}
	}
}

func TestPointToPointAndHostMasks(t *testing.T) {
	for _, address := range []Address{
		{IP: "10.0.0.0", Mask: "255.255.255.254"},
		{IP: "10.0.0.1", Mask: "255.255.255.255"},
	} {
		profile := Profile{AdapterID: "1", Addresses: []Address{address}}
		if err := Validate(profile); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPlanStatic(t *testing.T) {
	profile := staticProfile()
	profile.Name = "中文名称 ' & $(ignored)"
	profile.Addresses = append(profile.Addresses, Address{IP: "10.0.0.20", Mask: "255.255.255.0"})
	profile.Gateways = append(profile.Gateways, "10.0.0.1")
	commands, err := Plan(profile)
	if err != nil {
		t.Fatal(err)
	}
	expected := []Command{
		netsh("set", "address", "name=12", "source=static", "address=192.168.1.20", "mask=255.255.255.0", "gateway=none", "store=persistent"),
		netsh("add", "address", "name=12", "address=10.0.0.20", "mask=255.255.255.0", "store=persistent"),
		netsh("add", "route", "prefix=0.0.0.0/0", "interface=12", "nexthop=192.168.1.1", "metric=10", "store=persistent"),
		netsh("add", "route", "prefix=0.0.0.0/0", "interface=12", "nexthop=10.0.0.1", "metric=11", "store=persistent"),
		netsh("set", "dnsservers", "name=12", "source=static", "address=223.5.5.5", "validate=no"),
		netsh("add", "dnsservers", "name=12", "address=1.1.1.1", "index=2", "validate=no"),
	}
	if !reflect.DeepEqual(commands, expected) {
		t.Fatalf("unexpected plan: %#v", commands)
	}
	for _, command := range commands {
		if strings.Contains(strings.Join(command.Args, " "), profile.Name) {
			t.Fatal("profile name entered command")
		}
	}
	again, err := Plan(profile)
	if err != nil || !reflect.DeepEqual(again, commands) {
		t.Fatal("plan is not deterministic")
	}
}

func TestPlanDNSModes(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		source  string
		address string
	}{
		{"DHCP DNS", Profile{AdapterID: "12", DHCP: true}, "source=dhcp", ""},
		{"DHCP manual DNS", Profile{AdapterID: "12", DHCP: true, DNS: []string{"1.1.1.1"}}, "source=static", "address=1.1.1.1"},
		{"static empty DNS", Profile{AdapterID: "12", Addresses: staticProfile().Addresses}, "source=static", "address=none"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			commands, err := Plan(test.profile)
			if err != nil {
				t.Fatal(err)
			}
			last := strings.Join(commands[len(commands)-1].Args, " ")
			if !strings.Contains(last, test.source) || !strings.Contains(last, test.address) {
				t.Fatalf("unexpected DNS plan %s", last)
			}
			if test.profile.DHCP && !reflect.DeepEqual(commands[0], netsh("set", "address", "name=12", "source=dhcp")) {
				t.Fatal("missing DHCP command")
			}
		})
	}
}

type fakeRunner struct {
	commands []Command
	failAt   int
	err      error
	adapters string
	after    func(int)
}

func (fake *fakeRunner) run(ctx context.Context, command Command) (string, error) {
	fake.commands = append(fake.commands, command)
	call := len(fake.commands)
	if fake.after != nil {
		fake.after(call)
	}
	if call == fake.failAt {
		return "模拟错误输出", fake.err
	}
	if strings.Contains(strings.Join(command.Args, " "), "ConvertTo-Json") {
		return fake.adapters, nil
	}
	return "模拟成功", nil
}

func TestApplyStopsAtEveryFailure(t *testing.T) {
	profile := staticProfile()
	commands, err := Plan(profile)
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("simulated failure")
	for failAt := 1; failAt <= len(commands)+2; failAt++ {
		fake := &fakeRunner{failAt: failAt, err: failure, adapters: `[{"ID":"12"}]`}
		result, err := (service{run: fake.run}).apply(context.Background(), profile)
		if !errors.Is(err, failure) || result.Success || len(fake.commands) != failAt {
			t.Fatalf("failure at %d: result=%+v err=%v calls=%d", failAt, result, err, len(fake.commands))
		}
		if failAt > 2 && (!strings.Contains(result.Message, "未自动回滚") || !strings.Contains(result.Output, "模拟错误输出")) {
			t.Fatalf("missing partial failure detail: %+v", result)
		}
	}
}

func TestApplyPreconditionsAndSuccess(t *testing.T) {
	fake := &fakeRunner{adapters: `[{"ID":"12"}]`}
	client := service{run: fake.run}
	if _, err := client.apply(context.Background(), Profile{}); err == nil || len(fake.commands) != 0 {
		t.Fatal("invalid profile executed")
	}
	for _, adapters := range []string{`[]`, `[{"ID":"99"}]`, `broken json`} {
		fake := &fakeRunner{adapters: adapters}
		result, err := (service{run: fake.run}).apply(context.Background(), staticProfile())
		if err == nil || result.Success || len(fake.commands) != 2 {
			t.Fatalf("unsafe execution for adapters %q", adapters)
		}
	}
	result, err := client.apply(context.Background(), staticProfile())
	plan, _ := Plan(staticProfile())
	if err != nil || !result.Success || !reflect.DeepEqual(fake.commands[2:], plan) {
		t.Fatalf("unexpected successful execution: %+v %v", result, err)
	}
}

func TestApplyCancellationStopsMutations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fake := &fakeRunner{adapters: `[{"ID":"12"}]`, after: func(call int) {
		if call == 3 {
			cancel()
		}
	}}
	result, err := (service{run: fake.run}).apply(ctx, staticProfile())
	if !errors.Is(err, context.Canceled) || result.Success || len(fake.commands) != 3 {
		t.Fatalf("cancellation did not stop execution: %+v %v", result, err)
	}
}

func TestListAdaptersJSON(t *testing.T) {
	for _, data := range []string{
		`[]`,
		`[{"ID":"12","Name":"以太网","DHCP":true,"Addresses":[{"IP":"192.168.1.20","Mask":"255.255.255.0"}],"DNS":["1.1.1.1"]}]`,
		`[{"ID":"12"},{"ID":"13"}]`,
	} {
		fake := &fakeRunner{adapters: data}
		adapters, err := (service{run: fake.run}).listAdapters(context.Background())
		if err != nil || adapters == nil {
			t.Fatalf("failed to parse %s: %v", data, err)
		}
	}
}

func TestCheckValidationAndFailure(t *testing.T) {
	failure := errors.New("ICMP blocked")
	fake := &fakeRunner{failAt: 1, err: failure}
	service := service{run: fake.run}
	if _, err := service.check(context.Background(), []string{"ok.com", "';whoami"}); err == nil || len(fake.commands) != 0 {
		t.Fatal("invalid targets executed")
	}
	if _, err := service.check(context.Background(), nil); err != nil || len(fake.commands) != 0 {
		t.Fatal("empty targets executed")
	}
	output, err := service.check(context.Background(), []string{"example.com", "1.1.1.1"})
	if !errors.Is(err, failure) || len(fake.commands) != 2 || !strings.Contains(output, "1.1.1.1：可达") {
		t.Fatalf("missing check results: %s %v", output, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.check(ctx, []string{"example.com"}); !errors.Is(err, context.Canceled) || len(fake.commands) != 2 {
		t.Fatal("canceled check executed")
	}
}
