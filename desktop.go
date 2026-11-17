package main

import (
	"fmt"
	"strings"

	"netsetgo/internal/network"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/theme"
)

type desktop struct {
	window                               *mygo.Window
	saved                                []network.Profile
	adapters                             []network.Adapter
	id, name, adapterID, mode            string
	addresses, gateways, dns, targets    string
	importPath, directory, logs, message string
	busy, admin, loaded, failed          bool
	confirmApply, confirmDelete          bool
	pending                              network.Profile
	preview                              string
}

func newDesktop() *desktop {
	return &desktop{name: "新网络方案", mode: "dhcp", targets: "1.1.1.1\nwww.microsoft.com"}
}

func (state *desktop) render(c *ui.Context) {
	theme.Apply(c)
	tokens := theme.TokensOf(c)
	ui.Column(c).Fill().Background(tokens.BgLayout).TextColor(tokens.Text).FontSize(14).Children(func() {
		ui.Row(c).Padding(20, 28).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "NetSetGo").FontSize(26).FontWeight(700)
				ui.Text(c, "网络随场景切换，让连接井然有序").TextColor(tokens.TextSecondary).FontSize(13)
			})
			label := "标准权限 · 应用配置需管理员启动"
			if state.admin {
				label = "管理员权限"
			}
			ui.Text(c, label).Padding(8, 12).Radius(8).Background(tokens.FillTertiary).FontSize(12)
		})
		ui.Row(c).Grow(1).Basis(0).MinHeight(0).Padding(0, 20, 20, 20).Gap(18).AlignItems(ui.Stretch).Children(func() {
			state.sidebar(c.Key("sidebar"))
			ui.Scroll(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
				ui.Column(c).Gap(16).Children(func() {
					state.status(c)
					state.adapterCard(c.Key("adapters"))
					state.editor(c.Key("editor"))
					state.logCard(c.Key("logs"))
				})
			})
		})
		state.dialogs(c.Key("dialogs"))
	})
}

func panel(c *ui.Context, title, subtitle string, content func()) {
	tokens := theme.TokensOf(c)
	ui.Column(c).Padding(20).Gap(14).Radius(14).Border(1, tokens.BorderSecondary).Background(tokens.BgContainer).Children(func() {
		ui.Text(c, title).FontSize(17).FontWeight(600)
		if subtitle != "" {
			ui.Text(c, subtitle).FontSize(12).TextColor(tokens.TextSecondary)
		}
		content()
	})
}

func (state *desktop) button(c *ui.Context, label string, primary bool, disabled bool) bool {
	variant := action.Default
	if primary {
		variant = action.Primary
	}
	return action.Button(c, label, action.ButtonProps{Variant: variant}).Disabled(state.busy || disabled).Clicked()
}

func (state *desktop) sidebar(c *ui.Context) {
	tokens := theme.TokensOf(c)
	ui.Column(c).Width(248).Shrink(0).Padding(18).Gap(16).Radius(14).
		Background(tokens.BgContainer).Border(1, tokens.BorderSecondary).Children(func() {
		ui.Text(c, "已保存的方案").FontWeight(600).FontSize(17)
		ui.Text(c, fmt.Sprintf("%d 个场景 · 本机保存", len(state.saved))).TextColor(tokens.TextSecondary).FontSize(12)
		if state.button(c.Key("new"), "＋ 新建方案", false, false) {
			state.setProfile(network.Profile{Name: "新网络方案", DHCP: true})
		}
		if state.button(c.Key("default"), "创建默认方案", false, !state.loaded || state.selectedAdapter() == nil) {
			state.createDefault()
		}
		ui.Scroll(c).Grow(1).Basis(0).MinHeight(0).Children(func() {
			ui.Column(c).Gap(8).Children(func() {
				if len(state.saved) == 0 {
					ui.Text(c, "还没有保存的方案\n编辑右侧配置，保存常用网络。").TextColor(tokens.TextSecondary).FontSize(13)
				}
				for _, profile := range state.saved {
					variant := action.Default
					if profile.ID == state.id {
						variant = action.Filled
					}
					ui.Row(c.Key(profile.ID)).Gap(4).AlignItems(ui.Center).Children(func() {
						if action.Button(c.Key("select"), profile.Name, action.ButtonProps{Variant: variant}).
							Grow(1).Basis(0).MinWidth(0).Disabled(state.busy).Clicked() {
							state.setProfile(profile)
						}
						if state.button(c.Key("apply"), "应用", false, false) {
							state.setProfile(profile)
							state.plan()
						}
						if state.button(c.Key("delete"), "删除", false, !state.loaded) {
							state.setProfile(profile)
							state.confirmDelete = true
						}
					})
				}
			})
		})
		ui.Text(c, "导入 INI 配置").FontWeight(600)
		selected := "尚未选择 INI 文件"
		if state.importPath != "" {
			selected = "已选择：" + state.importPath
		}
		ui.Text(c, selected).FontSize(12).TextColor(tokens.TextSecondary)
		if state.button(c.Key("choose-ini"), "选择 INI 文件", false, false) {
			state.chooseINI()
		}
		if state.button(c.Key("import"), "导入并编辑", false, false) {
			state.importINI()
		}
		ui.Text(c, "选择文件后导入，随后请选择本机网卡。").FontSize(12).TextColor(tokens.TextSecondary)
		if state.directory != "" {
			ui.Text(c, "保存位置\n"+state.directory).FontSize(11).TextColor(tokens.TextTertiary)
		}
	})
}

func (state *desktop) status(c *ui.Context) {
	tokens := theme.TokensOf(c)
	text := state.message
	if first, _, found := strings.Cut(text, "\n"); found {
		text = first + "（详情见操作日志）"
	}
	if text == "" {
		text = "就绪 · 编辑和保存方案不会更改当前网络"
	}
	color := tokens.TextSecondary
	if state.failed {
		color = tokens.Error
	}
	if state.busy {
		text = "处理中，请稍候…"
	}
	ui.Text(c, text).TextColor(color).Padding(4, 2)
}

func (state *desktop) adapterCard(c *ui.Context) {
	panel(c, "01  选择网络适配器", "网卡索引是配置的实际目标，请在应用前核对。", func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			options := make([]field.SelectOption, 0, len(state.adapters))
			for _, adapter := range state.adapters {
				options = append(options, field.SelectOption{Value: adapter.ID, Label: adapter.Name + "  ·  " + adapter.Status + "  (#" + adapter.ID + ")"})
			}
			field.Select(c.Key("adapter"), &state.adapterID, field.SelectProps{Options: options, Placeholder: "请选择本机网卡", Disabled: state.busy}).Grow(1)
			if state.button(c.Key("refresh"), "刷新网卡", false, false) {
				state.refresh()
			}
		})
		if adapter := state.selectedAdapter(); adapter != nil {
			tokens := theme.TokensOf(c)
			mode := "静态地址"
			if adapter.DHCP {
				mode = "DHCP 自动获取"
			}
			ui.Column(c).Padding(12).Gap(6).Radius(8).Background(tokens.FillQuaternary).Children(func() {
				ui.Text(c, adapter.Description).FontWeight(500)
				ui.Text(c, "状态："+adapter.Status+"    MAC："+adapter.MAC+"    "+mode).FontSize(12)
				ui.Text(c, "IPv4："+formatAddresses(adapter.Addresses)+"\n网关："+strings.Join(adapter.Gateways, "、")+"\nDNS："+strings.Join(adapter.DNS, "、")).FontSize(12).TextColor(tokens.TextSecondary)
			})
			if state.button(c.Key("read-current"), "将当前网卡配置填入编辑器", false, false) {
				state.readAdapter(*adapter)
			}
		} else {
			ui.Text(c, "尚未选择可用网卡；导入或载入方案后请核对绑定。").FontSize(12).TextColor(theme.TokensOf(c).TextSecondary)
		}
	})
}

func input(c *ui.Context, label string, value *string, placeholder string, disabled bool) {
	ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, label).FontSize(13).FontWeight(500)
		field.Input(c, value, field.InputProps{Placeholder: placeholder}).Disabled(disabled)
	})
}

func area(c *ui.Context, label string, value *string, placeholder string, disabled bool) {
	ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, label).FontSize(13).FontWeight(500)
		field.TextArea(c, value, field.TextAreaProps{Placeholder: placeholder, MinRows: 2, MaxRows: 5}).Disabled(disabled)
	})
}

func (state *desktop) editor(c *ui.Context) {
	panel(c, "02  配置网络方案", "所有更改先预览、再确认。应用配置可能中断当前连接。", func() {
		input(c.Key("name"), "方案名称", &state.name, "例如：办公室 / 实验室 / 家庭", state.busy)
		field.Select(c.Key("mode"), &state.mode, field.SelectProps{
			Options:  []field.SelectOption{{Value: "dhcp", Label: "DHCP · 自动获取 IPv4 和 DNS"}, {Value: "static", Label: "静态配置 · 手动设置 IPv4 和 DNS"}},
			Disabled: state.busy,
		})
		if state.mode == "dhcp" {
			ui.Text(c, "自动获取地址与 DNS；下面保留的静态参数不会应用。").FontSize(12).TextColor(theme.TokensOf(c).TextSecondary)
		}
		area(c.Key("addresses"), "IPv4 地址 / 子网掩码 · 每行一组", &state.addresses,
			"192.168.1.100 / 255.255.255.0\n192.168.2.100 / 255.255.255.0", state.busy || state.mode == "dhcp")
		ui.Row(c).Gap(14).AlignItems(ui.Stretch).Children(func() {
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
				area(c.Key("gateways"), "默认网关 · 每行一个", &state.gateways, "192.168.1.1", state.busy || state.mode == "dhcp")
			})
			ui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
				area(c.Key("dns"), "DNS 服务器 · 静态模式留空会清空 DNS", &state.dns, "1.1.1.1\n8.8.8.8", state.busy || state.mode == "dhcp")
			})
		})
		area(c.Key("targets"), "连通性检测目标 · 每行一个 IP 或域名", &state.targets, "192.168.1.1\nwww.microsoft.com", state.busy)
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if state.button(c.Key("save"), "保存方案", false, !state.loaded) {
				state.save()
			}
			if state.button(c.Key("delete"), "删除方案", false, state.id == "" || !state.loaded) {
				state.confirmDelete = true
			}
			if state.button(c.Key("check"), "检测连通性", false, false) {
				state.check()
			}
			ui.Text(c, "确认后才会修改当前网卡").FontSize(12).TextColor(theme.TokensOf(c).TextSecondary).Grow(1)
			if action.Button(c.Key("preview"), "应用方案", action.ButtonProps{
				Variant: action.Primary,
				Size:    theme.Large,
			}).Width(132).Disabled(state.busy).Clicked() {
				state.plan()
			}
		})
		if !state.admin {
			ui.Text(c, "当前不是管理员：可编辑、保存、检测和预览；应用网络配置需要以管理员身份重新启动。").
				FontSize(12).TextColor(theme.TokensOf(c).Warning)
		}
	})
}

func (state *desktop) logCard(c *ui.Context) {
	panel(c, "操作日志", "记录真实执行结果；发生失败时，请查看完整输出。", func() {
		ui.Scroll(c).Height(180).Children(func() {
			text := state.logs
			if text == "" {
				text = "暂无操作记录。"
			}
			ui.Text(c, text).FontSize(12)
		})
	})
}

func (state *desktop) dialogs(c *ui.Context) {
	if feedback.Modal(c.Key("apply"), &state.confirmApply, feedback.ModalProps{
		Title: "确认应用网络配置", Width: 740, NoFooter: true,
	}, func() {
		ui.Column(c).Gap(14).Children(func() {
			target := state.pending.AdapterID
			for _, adapter := range state.adapters {
				if adapter.ID == state.pending.AdapterID {
					target = adapter.Name + " (#" + adapter.ID + ") · " + adapter.Description
				}
			}
			ui.Text(c, "方案："+state.pending.Name+"\n目标网卡："+target).FontWeight(600)
			ui.Text(c, "应用将替换此网卡的 IPv4 地址和默认网关，网络可能断开。失败时停止，已执行的修改不会自动回滚。")
			ui.Scroll(c).MaxHeight(300).Children(func() {
				ui.Text(c, state.preview).FontSize(12)
			})
			if !state.admin {
				ui.Text(c, "权限不足：请关闭程序，并以管理员身份重新启动。").TextColor(theme.TokensOf(c).Warning)
			}
			ui.Row(c).Gap(10).Justify(ui.End).Children(func() {
				if state.button(c.Key("cancel"), "返回修改", false, false) {
					state.confirmApply = false
				}
				if state.button(c.Key("confirm"), "确认并应用", true, !state.admin) {
					state.confirmApply = false
					state.apply()
				}
			})
		})
	}) {
		state.confirmApply = false
	}
	if feedback.Modal(c.Key("delete"), &state.confirmDelete, feedback.ModalProps{
		Title: "删除已保存方案", OkText: "确认删除", CancelText: "取消",
	}, func() {
		ui.Text(c, "确定删除已保存的方案“"+state.name+"”吗？此操作不会修改网卡配置。")
	}) && !state.busy {
		state.confirmDelete = false
		state.remove()
	}
}

func (state *desktop) selectedAdapter() *network.Adapter {
	for index := range state.adapters {
		if state.adapters[index].ID == state.adapterID {
			return &state.adapters[index]
		}
	}
	return nil
}

func formatAddresses(addresses []network.Address) string {
	lines := make([]string, 0, len(addresses))
	for _, address := range addresses {
		lines = append(lines, address.IP+" / "+address.Mask)
	}
	return strings.Join(lines, "\n")
}
