package network

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Address struct {
	IP   string
	Mask string
}

type Profile struct {
	ID        string
	Name      string
	AdapterID string
	DHCP      bool
	Addresses []Address
	Gateways  []string
	DNS       []string
	Targets   []string
}

type Adapter struct {
	ID          string
	Name        string
	Description string
	Status      string
	MAC         string
	DHCP        bool
	Addresses   []Address
	Gateways    []string
	DNS         []string
}

type Command struct {
	Program string
	Args    []string
}

type Result struct {
	Success bool
	Message string
	Output  string
}

var ErrUnsupported = errors.New("当前系统不支持 Windows 网络配置")

func Validate(profile Profile) error {
	if !validText(profile.ID, 128) || !validText(profile.Name, 128) {
		return errors.New("配置 ID 和名称不得超过 128 字节或包含控制字符")
	}
	index, err := strconv.ParseUint(profile.AdapterID, 10, 32)
	if err != nil || index == 0 || strconv.FormatUint(index, 10) != profile.AdapterID {
		return errors.New("网卡 ID 必须为规范的正整数接口索引")
	}
	if len(profile.Addresses) > 16 || len(profile.Gateways) > 4 || len(profile.DNS) > 8 {
		return errors.New("最多支持 16 个地址、4 个网关和 8 个 DNS")
	}
	if profile.DHCP && (len(profile.Addresses) != 0 || len(profile.Gateways) != 0) {
		return errors.New("DHCP 模式不能同时指定静态地址或网关")
	}
	if !profile.DHCP && len(profile.Addresses) == 0 {
		return errors.New("静态模式至少需要一个 IPv4 地址")
	}
	prefixes := make([]netip.Prefix, 0, len(profile.Addresses))
	seen := make(map[string]bool)
	for _, address := range profile.Addresses {
		ip, err := ipv4(address.IP)
		if err != nil || ip.IsLoopback() {
			return fmt.Errorf("无效的网卡 IPv4 地址：%q", address.IP)
		}
		bits, err := maskBits(address.Mask)
		if err != nil {
			return err
		}
		prefix := netip.PrefixFrom(ip, bits)
		if !usableHost(prefix, ip) {
			return fmt.Errorf("地址不能为子网网络地址或广播地址：%s", address.IP)
		}
		if seen[address.IP] {
			return fmt.Errorf("IPv4 地址重复：%s", address.IP)
		}
		seen[address.IP] = true
		prefixes = append(prefixes, prefix)
	}
	if err := validateIPs(profile.Gateways, "网关"); err != nil {
		return err
	}
	for _, gateway := range profile.Gateways {
		ip, _ := ipv4(gateway)
		reachable := false
		for _, prefix := range prefixes {
			if prefix.Contains(ip) && usableHost(prefix, ip) && !seen[gateway] {
				reachable = true
			}
		}
		if ip.IsLoopback() || !reachable {
			return fmt.Errorf("网关 %s 必须在某个静态地址的子网内，且不能是本机、网络或广播地址", gateway)
		}
	}
	if err := validateIPs(profile.DNS, "DNS"); err != nil {
		return err
	}
	return validateTargets(profile.Targets)
}

func validText(value string, max int) bool {
	return len(value) <= max && utf8.ValidString(value) && !strings.ContainsFunc(value, unicode.IsControl)
}

func ipv4(value string) (netip.Addr, error) {
	ip, err := netip.ParseAddr(value)
	if err != nil || !ip.Is4() {
		return netip.Addr{}, errors.New("必须是规范的 IPv4 地址")
	}
	octets := ip.As4()
	if octets[0] == 0 || octets[0] >= 224 {
		return netip.Addr{}, errors.New("必须是单播 IPv4 地址")
	}
	return ip, nil
}

func maskBits(value string) (int, error) {
	ip, err := netip.ParseAddr(value)
	if err == nil && ip.Is4() {
		octets := ip.As4()
		ones, bits := net.IPMask(octets[:]).Size()
		if bits == 32 && ones > 0 {
			return ones, nil
		}
	}
	return 0, fmt.Errorf("无效的连续 IPv4 子网掩码：%q", value)
}

func usableHost(prefix netip.Prefix, ip netip.Addr) bool {
	if prefix.Bits() >= 31 {
		return true
	}
	base := prefix.Masked().Addr().As4()
	last := base
	for bit := prefix.Bits(); bit < 32; bit++ {
		last[bit/8] |= 1 << (7 - bit%8)
	}
	return ip != prefix.Masked().Addr() && ip != netip.AddrFrom4(last)
}

func validateIPs(values []string, label string) error {
	seen := make(map[string]bool)
	for _, value := range values {
		if _, err := ipv4(value); err != nil {
			return fmt.Errorf("%s 包含无效的 IPv4 地址：%q", label, value)
		}
		if seen[value] {
			return fmt.Errorf("%s 地址重复：%s", label, value)
		}
		seen[value] = true
	}
	return nil
}

func validateTargets(targets []string) error {
	if len(targets) > 16 {
		return errors.New("最多支持 16 个连通检查目标")
	}
	for _, target := range targets {
		if _, err := ipv4(target); err == nil {
			continue
		}
		if !validHostname(target) {
			return fmt.Errorf("检查目标必须是 IPv4 或 ASCII 主机名：%q", target)
		}
	}
	return nil
}

func validHostname(value string) bool {
	if len(value) == 0 || len(value) > 253 {
		return false
	}
	hasLetter := false
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			letter := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z'
			if !letter && !(char >= '0' && char <= '9') && char != '-' {
				return false
			}
			hasLetter = hasLetter || letter
		}
	}
	return hasLetter
}

func powershell(script string) Command {
	return Command{
		Program: "powershell.exe",
		Args: []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-Command",
			"$ErrorActionPreference='Stop'; [Console]::OutputEncoding=New-Object System.Text.UTF8Encoding($false); " + script},
	}
}

func netsh(args ...string) Command {
	return Command{Program: "netsh.exe", Args: append([]string{"interface", "ipv4"}, args...)}
}

func Plan(profile Profile) ([]Command, error) {
	if err := Validate(profile); err != nil {
		return nil, err
	}
	index := profile.AdapterID
	name := "name=" + index
	var commands []Command
	if profile.DHCP {
		commands = append(commands, netsh("set", "address", name, "source=dhcp"))
	} else {
		for offset, address := range profile.Addresses {
			if offset == 0 {
				commands = append(commands, netsh("set", "address", name, "source=static", "address="+address.IP, "mask="+address.Mask, "gateway=none", "store=persistent"))
			} else {
				commands = append(commands, netsh("add", "address", name, "address="+address.IP, "mask="+address.Mask, "store=persistent"))
			}
		}
		for offset, gateway := range profile.Gateways {
			commands = append(commands, netsh("add", "route", "prefix=0.0.0.0/0", "interface="+index, "nexthop="+gateway, "metric="+strconv.Itoa(10+offset), "store=persistent"))
		}
	}
	if len(profile.DNS) == 0 {
		if profile.DHCP {
			commands = append(commands, netsh("set", "dnsservers", name, "source=dhcp"))
		} else {
			commands = append(commands, netsh("set", "dnsservers", name, "source=static", "address=none", "validate=no"))
		}
	} else {
		for offset, dns := range profile.DNS {
			if offset == 0 {
				commands = append(commands, netsh("set", "dnsservers", name, "source=static", "address="+dns, "validate=no"))
			} else {
				commands = append(commands, netsh("add", "dnsservers", name, "address="+dns, "index="+strconv.Itoa(offset+1), "validate=no"))
			}
		}
	}
	return commands, nil
}

type runner func(context.Context, Command) (string, error)

type service struct {
	run runner
}

func ListAdapters(ctx context.Context) ([]Adapter, error) {
	return (service{run: platformRun}).listAdapters(ctx)
}

func Apply(ctx context.Context, profile Profile) (Result, error) {
	return (service{run: platformRun}).apply(ctx, profile)
}

func Check(ctx context.Context, targets []string) (string, error) {
	return (service{run: platformRun}).check(ctx, targets)
}

func IsAdmin() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return (service{run: platformRun}).admin(ctx) == nil
}

func (s service) admin(ctx context.Context) error {
	_, err := s.run(ctx, powershell("if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw '需要管理员权限' }"))
	return err
}

const adapterScript = `$interfaces = @(Get-NetIPInterface -AddressFamily IPv4)
$allAddresses = @(Get-NetIPAddress -AddressFamily IPv4)
$allRoutes = @(Get-NetRoute -AddressFamily IPv4)
$items = @(Get-NetAdapter -IncludeHidden | Sort-Object ifIndex | ForEach-Object {
    $adapter = $_
    $ip = $interfaces | Where-Object {$_.InterfaceIndex -eq $adapter.ifIndex}
    if ($ip) {
        $addresses = @($allAddresses | Where-Object {$_.InterfaceIndex -eq $adapter.ifIndex} | ForEach-Object {
            $prefix = $_.PrefixLength
            $mask = @(0..3 | ForEach-Object {
                $bits = [Math]::Min(8, [Math]::Max(0, $prefix - 8 * $_))
                [int](256 - [Math]::Pow(2, 8 - $bits))
            }) -join '.'
            @{IP = [string]$_.IPAddress; Mask = $mask}
        })
        $gateways = @($allRoutes |
            Where-Object {$_.InterfaceIndex -eq $adapter.ifIndex -and $_.DestinationPrefix -eq '0.0.0.0/0'} |
            Sort-Object RouteMetric | Select-Object -ExpandProperty NextHop -Unique)
        $dns = @((Get-DnsClientServerAddress -InterfaceIndex $adapter.ifIndex -AddressFamily IPv4).ServerAddresses)
        @{ID = [string]$adapter.ifIndex; Name = [string]$adapter.Name;
          Description = [string]$adapter.InterfaceDescription; Status = [string]$adapter.Status;
          MAC = [string]$adapter.MacAddress; DHCP = [bool]($ip.Dhcp -eq 'Enabled');
          Addresses = $addresses; Gateways = $gateways; DNS = $dns}
    }
})
ConvertTo-Json -InputObject $items -Depth 5 -Compress`

func (s service) listAdapters(ctx context.Context) ([]Adapter, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	output, err := s.run(ctx, powershell(adapterScript))
	if err != nil {
		return nil, fmt.Errorf("读取网卡失败：%w；%s", err, output)
	}
	var adapters []Adapter
	if err := json.Unmarshal([]byte(strings.TrimPrefix(output, "\ufeff")), &adapters); err != nil {
		return nil, fmt.Errorf("解析网卡信息失败：%w", err)
	}
	if adapters == nil {
		adapters = []Adapter{}
	}
	return adapters, nil
}

func (s service) apply(ctx context.Context, profile Profile) (Result, error) {
	commands, err := Plan(profile)
	if err != nil {
		return Result{Message: err.Error()}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := s.admin(ctx); err != nil {
		return Result{Message: "管理员权限检查失败，未执行配置"}, fmt.Errorf("管理员权限检查失败：%w", err)
	}
	adapters, err := s.listAdapters(ctx)
	if err != nil {
		return Result{Message: "网卡检查失败，未执行配置"}, err
	}
	found := false
	for _, adapter := range adapters {
		found = found || adapter.ID == profile.AdapterID
	}
	if !found {
		err := errors.New("目标网卡已不存在，未执行配置")
		return Result{Message: err.Error()}, err
	}
	var output strings.Builder
	for offset, command := range commands {
		if err := ctx.Err(); err != nil {
			return failedApply(offset, len(commands), output.String(), err)
		}
		text, err := s.run(ctx, command)
		fmt.Fprintf(&output, "步骤 %d/%d：\n%s\n", offset+1, len(commands), text)
		if err != nil {
			return failedApply(offset, len(commands), output.String(), err)
		}
	}
	return Result{Success: true, Message: "网络配置命令执行成功；连通性请单独检查", Output: output.String()}, nil
}

func failedApply(completed, total int, output string, cause error) (Result, error) {
	message := fmt.Sprintf("配置失败：已完成 %d/%d 步；失败步骤也可能产生修改，未自动回滚，请检查当前网络配置", completed, total)
	return Result{Message: message, Output: output}, fmt.Errorf("%s：%w", message, cause)
}

func (s service) check(ctx context.Context, targets []string) (string, error) {
	if err := validateTargets(targets); err != nil {
		return "", err
	}
	if len(targets) == 0 {
		return "未指定连通检查目标", nil
	}
	var output strings.Builder
	var failures []error
	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			failures = append(failures, err)
			break
		}
		checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		text, err := s.run(checkCtx, powershell("if (-not (Test-Connection -ComputerName '"+target+"' -Count 1 -Quiet -ErrorAction Stop)) { throw '未收到 ICMP 响应' }; '收到 ICMP 响应'"))
		cancel()
		if err != nil {
			fmt.Fprintf(&output, "%s：检查失败（可能禁用 ICMP）\n%s\n", target, text)
			failures = append(failures, fmt.Errorf("%s：%w", target, err))
		} else {
			fmt.Fprintf(&output, "%s：可达\n%s\n", target, text)
		}
	}
	return output.String(), errors.Join(failures...)
}
