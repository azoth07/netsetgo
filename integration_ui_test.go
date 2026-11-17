package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"netsetgo/internal/network"
)

func TestDesktopConfirmationCanCancelWithoutApplying(t *testing.T) {
	state := newDesktop()
	state.confirmApply = true
	state.pending = network.Profile{Name: "测试方案", AdapterID: "12", DHCP: true}
	state.preview = "netsh interface ipv4 set address name=12 source=dhcp"
	tester := ui.NewTester(state.render, 1280, 900)
	if !tester.HasText("确认应用网络配置") {
		t.Fatal("confirmation dialog missing")
	}
	if err := tester.Click("返回修改"); err != nil {
		t.Fatal(err)
	}
	if state.confirmApply || state.busy {
		t.Fatal("cancel must close confirmation without starting work")
	}
}

func TestDesktopRejectsUnavailableAdapter(t *testing.T) {
	state := newDesktop()
	state.adapterID = "12"
	state.adapters = []network.Adapter{{ID: "13", Name: "测试网卡"}}
	if _, err := state.profile(); err == nil {
		t.Fatal("missing adapter accepted")
	}
}

func TestDesktopRenderSizes(t *testing.T) {
	for _, size := range [][2]int{{1280, 900}, {1000, 700}} {
		state := newDesktop()
		state.loaded = true
		state.admin = true
		state.mode = "static"
		state.adapterID = "12"
		state.adapters = []network.Adapter{{
			ID: "12", Name: "以太网", Description: "测试网卡 · 仅渲染示例", Status: "Up",
			Addresses: []network.Address{{IP: "192.168.1.100", Mask: "255.255.255.0"}},
			Gateways:  []string{"192.168.1.1"}, DNS: []string{"1.1.1.1"},
		}}
		state.readAdapter(state.adapters[0])
		tester := ui.NewTester(state.render, size[0], size[1])
		if !tester.HasText("NetSetGo") || !tester.HasText("已保存的方案") {
			t.Fatal("main interface missing")
		}
		if directory := os.Getenv("NETSETGO_RENDER_DIR"); directory != "" && size[0] == 1280 {
			if err := os.MkdirAll(directory, 0700); err != nil {
				t.Fatal(err)
			}
			file, err := os.Create(filepath.Join(directory, "desktop.png"))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(file, tester.Image())
			file.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}
