package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"strconv"
	"strings"
	"time"

	"netsetgo/internal/network"
	"netsetgo/internal/profiles"

	"github.com/egoist/mygo"
)

func (state *desktop) run(label string, work func(context.Context) func()) {
	if state.busy {
		return
	}
	state.busy = true
	state.record(label, false)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		complete := work(ctx)
		mygo.RunOnMain(func() {
			state.busy = false
			complete()
			if state.window != nil {
				state.window.Invalidate()
			}
		})
	}()
}

func (state *desktop) record(message string, failed bool) {
	state.message, state.failed = message, failed
	state.logs = time.Now().Format("15:04:05") + "  " + message + "\n\n" + state.logs
	if len(state.logs) > 64000 {
		runes := []rune(state.logs)
		if len(runes) > 32000 {
			state.logs = string(runes[:32000]) + "\n（较早日志已截断）"
		}
	}
}

func (state *desktop) initialize() {
	state.run("正在读取本机网卡与已保存方案", func(ctx context.Context) func() {
		saved, loadErr := profiles.Load()
		adapters, adapterErr := network.ListAdapters(ctx)
		directory, dirErr := profiles.Directory()
		admin := network.IsAdmin()
		return func() {
			state.admin, state.directory = admin, directory
			state.loaded = loadErr == nil
			if loadErr == nil {
				state.saved = saved
			}
			if adapterErr == nil {
				state.adapters = adapters
			}
			state.record("初始化完成", false)
			if loadErr != nil {
				state.record("方案读取失败，已禁止保存以保护原文件："+loadErr.Error(), true)
			}
			if dirErr != nil {
				state.record("无法获取保存目录："+dirErr.Error(), true)
			}
			if adapterErr != nil {
				state.record("读取网卡失败："+adapterErr.Error(), true)
			}
		}
	})
}

func (state *desktop) refresh() {
	state.run("正在刷新网卡", func(ctx context.Context) func() {
		adapters, err := network.ListAdapters(ctx)
		return func() {
			if err != nil {
				state.record("刷新网卡失败："+err.Error(), true)
				return
			}
			state.adapters = adapters
			state.record(fmt.Sprintf("已刷新 %d 个网卡", len(adapters)), false)
		}
	})
}

func (state *desktop) setProfile(profile network.Profile) {
	state.id, state.name, state.adapterID = profile.ID, profile.Name, profile.AdapterID
	state.mode = "static"
	if profile.DHCP {
		state.mode = "dhcp"
	}
	state.addresses = formatAddresses(profile.Addresses)
	state.gateways = strings.Join(profile.Gateways, "\n")
	state.dns = strings.Join(profile.DNS, "\n")
	state.targets = strings.Join(profile.Targets, "\n")
}

func (state *desktop) readAdapter(adapter network.Adapter) {
	state.setProfile(network.Profile{
		ID: state.id, Name: state.name, AdapterID: adapter.ID, DHCP: adapter.DHCP,
		Addresses: adapter.Addresses, Gateways: adapter.Gateways, DNS: adapter.DNS,
		Targets: splitValues(state.targets),
	})
	state.record("已将网卡当前配置填入编辑器，尚未保存或应用", false)
}

func splitValues(value string) []string {
	return strings.FieldsFunc(value, func(char rune) bool {
		return char == '\n' || char == '\r' || char == ',' || char == '，' || char == ';' || char == '；' || char == ' ' || char == '\t'
	})
}

func (state *desktop) profile() (network.Profile, error) {
	profile := network.Profile{
		ID: state.id, Name: strings.TrimSpace(state.name), AdapterID: state.adapterID,
		DHCP: state.mode == "dhcp", Targets: splitValues(state.targets),
	}
	if profile.Name == "" {
		return profile, fmt.Errorf("请输入方案名称")
	}
	if state.selectedAdapter() == nil {
		return profile, fmt.Errorf("请选择本机可用网卡")
	}
	if !profile.DHCP {
		for index, line := range strings.Split(state.addresses, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.FieldsFunc(line, func(char rune) bool {
				return char == '/' || char == ' ' || char == '\t'
			})
			if len(parts) != 2 {
				return profile, fmt.Errorf("IPv4 第 %d 行应填写“IP / 子网掩码”", index+1)
			}
			profile.Addresses = append(profile.Addresses, network.Address{IP: parts[0], Mask: parts[1]})
		}
		profile.Gateways, profile.DNS = splitValues(state.gateways), splitValues(state.dns)
	}
	if err := network.Validate(profile); err != nil {
		return profile, err
	}
	return profile, nil
}

func (state *desktop) save() {
	if !state.loaded {
		state.record("方案文件未成功读取，无法安全保存", true)
		return
	}
	profile, err := state.profile()
	if err != nil {
		state.record("保存失败："+err.Error(), true)
		return
	}
	state.saveProfile(profile)
}

func (state *desktop) createDefault() {
	if !state.loaded {
		state.record("方案文件未成功读取，无法安全保存", true)
		return
	}
	adapter := state.selectedAdapter()
	if adapter == nil {
		state.record("请先选择要保存的网卡", true)
		return
	}
	profile := network.Profile{
		Name:      "默认",
		AdapterID: adapter.ID,
		DHCP:      adapter.DHCP,
		Targets:   splitValues(state.targets),
	}
	if !profile.DHCP {
		profile.Addresses = adapter.Addresses
		profile.Gateways = adapter.Gateways
		profile.DNS = adapter.DNS
	}
	if err := network.Validate(profile); err != nil {
		state.record("创建默认方案失败："+err.Error(), true)
		return
	}
	for _, saved := range state.saved {
		if saved.Name == profile.Name {
			profile.ID = saved.ID
			break
		}
	}
	state.saveProfile(profile)
}

func (state *desktop) saveProfile(profile network.Profile) {
	if profile.ID == "" {
		var identifier [16]byte
		if _, err := rand.Read(identifier[:]); err != nil {
			state.record("创建方案标识失败："+err.Error(), true)
			return
		}
		profile.ID = fmt.Sprintf("%x", identifier)
	}
	next := append([]network.Profile(nil), state.saved...)
	replaced := false
	for index := range next {
		if next[index].ID == profile.ID {
			next[index], replaced = profile, true
			break
		}
	}
	if !replaced {
		next = append(next, profile)
	}
	state.run("正在保存方案", func(context.Context) func() {
		err := profiles.Save(next)
		return func() {
			if err != nil {
				state.record("保存方案失败："+err.Error(), true)
				return
			}
			state.saved, state.id = next, profile.ID
			state.setProfile(profile)
			state.record("已保存方案："+profile.Name, false)
		}
	})
}

func (state *desktop) remove() {
	if !state.loaded || state.id == "" {
		return
	}
	next := make([]network.Profile, 0, len(state.saved))
	for _, profile := range state.saved {
		if profile.ID != state.id {
			next = append(next, profile)
		}
	}
	state.run("正在删除方案", func(context.Context) func() {
		err := profiles.Save(next)
		return func() {
			if err != nil {
				state.record("删除方案失败："+err.Error(), true)
				return
			}
			state.saved, state.id = next, ""
			state.record("已删除保存的方案，编辑器内容保留", false)
		}
	})
}

func (state *desktop) importINI() {
	path := strings.Trim(strings.TrimSpace(state.importPath), "\"")
	if path == "" {
		state.record("请输入 INI 文件路径", true)
		return
	}
	state.run("正在导入 INI 配置", func(context.Context) func() {
		profile, err := profiles.ImportINI(path)
		return func() {
			if err != nil {
				state.record("导入失败："+err.Error(), true)
				return
			}
			profile.ID, profile.AdapterID = "", ""
			state.setProfile(profile)
			state.record("INI 已导入编辑器，请选择本机网卡后保存或预览", false)
		}
	})
}

func (state *desktop) chooseINI() {
	if state.busy {
		return
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:      state.window,
		Title:       "选择网络配置 INI",
		ButtonLabel: "选择",
		Filters:     []mygo.FileFilter{{Name: "INI 配置文件", Extensions: []string{"ini"}}, {Name: "所有文件", Extensions: []string{"*"}}},
		Multiple:    false,
	})
	if err != nil {
		state.record("选择 INI 文件失败："+err.Error(), true)
		return
	}
	if len(paths) == 0 {
		return
	}
	state.importPath = paths[0]
	state.record("已选择 INI 文件："+paths[0], false)
	if state.window != nil {
		state.window.Invalidate()
	}
}

func (state *desktop) check() {
	targets := splitValues(state.targets)
	if len(targets) == 0 {
		state.record("请至少填写一个检测目标", true)
		return
	}
	state.run("正在检测连通性", func(ctx context.Context) func() {
		output, err := network.Check(ctx, targets)
		return func() {
			if err != nil {
				state.record("检测失败："+err.Error()+"\n"+output, true)
				return
			}
			state.record("检测完成\n"+output, false)
		}
	})
}

func (state *desktop) plan() {
	profile, err := state.profile()
	if err != nil {
		state.record("无法生成预览："+err.Error(), true)
		return
	}
	state.run("正在生成命令预览", func(context.Context) func() {
		commands, err := network.Plan(profile)
		return func() {
			if err != nil {
				state.record("生成预览失败："+err.Error(), true)
				return
			}
			if len(commands) == 0 {
				state.record("没有可执行命令，已取消应用", true)
				return
			}
			lines := make([]string, 0, len(commands))
			for _, command := range commands {
				parts := []string{strconv.Quote(command.Program)}
				for _, arg := range command.Args {
					parts = append(parts, strconv.Quote(arg))
				}
				lines = append(lines, strings.Join(parts, " "))
			}
			state.pending, state.preview = profile, strings.Join(lines, "\n\n")
			state.confirmApply = true
			state.record("命令预览已生成，等待确认；尚未修改网络", false)
		}
	})
}

func (state *desktop) apply() {
	if !state.admin {
		state.record("权限不足，请以管理员身份重新启动", true)
		return
	}
	profile := state.pending
	state.run("正在应用网络配置："+profile.Name, func(ctx context.Context) func() {
		result, err := network.Apply(ctx, profile)
		adapters, refreshErr := network.ListAdapters(ctx)
		return func() {
			if refreshErr == nil {
				state.adapters = adapters
			}
			message := result.Message
			if result.Output != "" {
				message += "\n" + result.Output
			}
			if err != nil {
				state.record("应用失败，配置可能仅部分生效："+err.Error()+"\n"+message, true)
			} else if !result.Success {
				state.record("应用未成功，配置可能仅部分生效：\n"+message, true)
			} else {
				state.record("配置已应用\n"+message, false)
			}
			if refreshErr != nil {
				state.record("应用后无法刷新网卡状态："+refreshErr.Error(), true)
			}
		}
	})
}
