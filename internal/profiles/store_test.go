package profiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"netsetgo/internal/network"
)

func TestSaveLoadAndReplace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	profile := network.Profile{ID: "office", Name: "办公室", AdapterID: "12", DHCP: true}
	if err := save(path, []network.Profile{profile}); err != nil {
		t.Fatal(err)
	}
	profile.Name = "家里"
	if err := save(path, []network.Profile{profile}); err != nil {
		t.Fatal(err)
	}
	saved, err := load(path)
	if err != nil || len(saved) != 1 || saved[0].Name != "家里" {
		t.Fatalf("load: %v, %v", saved, err)
	}
	data, _ := os.ReadFile(path)
	if strings.HasPrefix(string(data), "\xef\xbb\xbf") {
		t.Fatal("unexpected BOM")
	}
}

func TestInvalidSavePreservesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	original := []byte("original")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := save(path, []network.Profile{{Name: "无编号"}}); err == nil {
		t.Fatal("expected validation failure")
	}
	actual, _ := os.ReadFile(path)
	if string(actual) != string(original) {
		t.Fatal("existing file changed")
	}
}

func TestImportINI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "office.ini")
	content := "[StaticIP]\nIP=192.168.1.10,192.168.2.10\nMask=255.255.255.0\nGateway=192.168.1.1\nDNS1=223.5.5.5\nDNS2=1.1.1.1\n[Check]\nTarget=192.168.1.1,1.1.1.1\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	profile, err := ImportINI(path)
	if err != nil {
		t.Fatal(err)
	}
	if profile.AdapterID != "" || len(profile.Addresses) != 2 || profile.Addresses[1].Mask != "255.255.255.0" || len(profile.Targets) != 2 {
		t.Fatalf("unexpected import: %+v", profile)
	}
}

func TestCorruptAndMissingStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	profiles, err := load(path)
	if err != nil || len(profiles) != 0 {
		t.Fatalf("missing: %v %v", profiles, err)
	}
	if err := os.WriteFile(path, []byte("{bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := load(path); err == nil {
		t.Fatal("expected corruption error")
	}
}
