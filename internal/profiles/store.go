package profiles

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"gopkg.in/ini.v1"
	"netsetgo/internal/network"
)

const maxFileSize = 1024 * 1024

func Directory() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "NetSetGo"), nil
}

func Load() ([]network.Profile, error) {
	directory, err := Directory()
	if err != nil {
		return nil, err
	}
	return load(filepath.Join(directory, "profiles.json"))
}

func load(path string) ([]network.Profile, error) {
	data, err := readFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []network.Profile{}, nil
	}
	if err != nil {
		return nil, err
	}
	var profiles []network.Profile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return nil, fmt.Errorf("方案文件损坏，未覆盖原文件：%w", err)
	}
	if err := validateProfiles(profiles); err != nil {
		return nil, err
	}
	return profiles, nil
}

func Save(profiles []network.Profile) error {
	directory, err := Directory()
	if err != nil {
		return err
	}
	return save(filepath.Join(directory, "profiles.json"), profiles)
}

func save(path string, profiles []network.Profile) error {
	if err := validateProfiles(profiles); err != nil {
		return err
	}
	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > maxFileSize {
		return errors.New("方案文件超过 1 MiB 限制")
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".profiles-*.json")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func validateProfiles(profiles []network.Profile) error {
	if len(profiles) > 200 {
		return errors.New("最多保存 200 个方案")
	}
	ids := make(map[string]bool, len(profiles))
	for _, profile := range profiles {
		if profile.ID == "" || ids[profile.ID] {
			return errors.New("方案编号为空或重复")
		}
		ids[profile.ID] = true
		if strings.TrimSpace(profile.Name) == "" || len([]rune(profile.Name)) > 80 {
			return errors.New("方案名称须为 1–80 个字符")
		}
		candidate := profile
		if candidate.AdapterID == "" {
			candidate.AdapterID = "1"
		}
		if err := network.Validate(candidate); err != nil {
			return fmt.Errorf("方案 %q 无效：%w", profile.Name, err)
		}
	}
	return nil
}

func readFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxFileSize {
		return nil, errors.New("配置文件超过 1 MiB 限制")
	}
	if !utf8.Valid(data) {
		return nil, errors.New("配置文件必须使用 UTF-8 编码")
	}
	return data, nil
}

func ImportINI(path string) (network.Profile, error) {
	profile := network.Profile{Name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))}
	data, err := readFile(path)
	if err != nil {
		return profile, err
	}
	config, err := ini.LoadSources(ini.LoadOptions{Insensitive: true}, data)
	if err != nil {
		return profile, fmt.Errorf("无法解析 INI：%w", err)
	}
	value := func(section, key string) string {
		if entry, err := config.Section(section).GetKey(key); err == nil {
			return entry.String()
		}
		return config.Section("").Key(key).String()
	}
	ips := split(value("StaticIP", "IP"))
	masks := split(value("StaticIP", "Mask"))
	if len(ips) == 0 || len(masks) == 0 {
		return profile, errors.New("INI 必须包含 IP 和 Mask")
	}
	if len(masks) > len(ips) {
		return profile, errors.New("掩码数量不能多于 IP 数量")
	}
	for index, ip := range ips {
		maskIndex := min(index, len(masks)-1)
		profile.Addresses = append(profile.Addresses, network.Address{IP: ip, Mask: masks[maskIndex]})
	}
	profile.Gateways = split(value("StaticIP", "Gateway"))
	profile.DNS = append(split(value("StaticIP", "DNS1")), split(value("StaticIP", "DNS2"))...)
	profile.Targets = split(value("Check", "Target"))
	candidate := profile
	candidate.AdapterID = "1"
	if err := network.Validate(candidate); err != nil {
		return profile, fmt.Errorf("INI 网络参数无效：%w", err)
	}
	return profile, nil
}

func split(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}
