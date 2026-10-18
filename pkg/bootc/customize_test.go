package bootc

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var (
	errMissing = errors.New("no such image")
	errBoom    = errors.New("exit status 1")
)

func writeScript(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "run.sh")
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// stubTacklebox pretends the binary exists so delegation tests run anywhere.
func stubTacklebox(t *testing.T) {
	t.Helper()
	old := tackleboxLookPath
	tackleboxLookPath = func(string) (string, error) { return "/bin/tacklebox", nil }
	t.Cleanup(func() { tackleboxLookPath = old })
}

func joinArgs(args []string) string { return strings.Join(args, " ") }

// No scripts means the image passes through untouched and nothing runs.
func TestCustomizeImage_EmptyIsPassthrough(t *testing.T) {
	fake := withFake(t)
	got, err := LocalBuilder{}.customizeImage(BuildRequest{Image: "img"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "img" {
		t.Errorf("got %q, want image unchanged", got)
	}
	if len(fake.Calls()) != 0 {
		t.Errorf("ran %d commands for zero scripts", len(fake.Calls()))
	}
}

// Same inputs hash to the same tag; a byte change retags.
func TestCustomizeTag_Deterministic(t *testing.T) {
	a := writeScript(t, "echo a")
	b := writeScript(t, "echo b")
	t1, err := customizeTag("img", []string{a, b})
	if err != nil {
		t.Fatal(err)
	}
	t2, err := customizeTag("img", []string{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if t1 != t2 {
		t.Errorf("same inputs gave different tags: %q vs %q", t1, t2)
	}
	if !strings.HasPrefix(t1, "localhost/corral-bootc-customize-") {
		t.Errorf("tag %q is not a localhost ref (fetch would try to pull it)", t1)
	}
	rev, err := customizeTag("img", []string{b, a})
	if err != nil {
		t.Fatal(err)
	}
	if rev == t1 {
		t.Error("script order does not affect the tag, so a reorder would reuse a stale image")
	}
	if _, err := customizeTag("img", []string{filepath.Join(t.TempDir(), "missing.sh")}); err == nil {
		t.Error("missing script did not fail tag computation")
	}
}

// A cache hit runs nothing but the existence check.
func TestCustomizeImage_CacheHitSkipsTacklebox(t *testing.T) {
	fake := withFake(t)
	stubTacklebox(t)
	s := writeScript(t, "echo hi")
	tag, err := customizeTag("img", []string{s})
	if err != nil {
		t.Fatal(err)
	}
	fake.AddResponse("podman image exists "+tag, "", nil)
	got, err := LocalBuilder{}.customizeImage(BuildRequest{Image: "img", Customize: []string{s}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != tag {
		t.Errorf("got %q, want cached %q", got, tag)
	}
	if len(fake.Calls()) != 1 {
		t.Errorf("cache hit ran %d commands, want just the existence check", len(fake.Calls()))
	}
}

// A miss delegates one tacklebox call with image, scripts in order, and the
// content tag. corral tracks the tag it asked for, not whatever tacklebox
// prints.
func TestCustomizeImage_DelegatesToTacklebox(t *testing.T) {
	fake := withFake(t)
	stubTacklebox(t)
	s1 := writeScript(t, "echo one")
	s2 := writeScript(t, "echo two")
	fake.AddPrefixResponse("podman image exists ", "", errMissing)
	fake.AddPrefixResponse("tacklebox ", "localhost/corral-bootc-customize-x\n", nil)
	got, err := LocalBuilder{}.customizeImage(BuildRequest{Image: "img", Customize: []string{s1, s2}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "localhost/corral-bootc-customize-") {
		t.Fatalf("got %q, want content tag", got)
	}
	var tb []string
	for _, c := range fake.Calls() {
		if c.Name == "tacklebox" {
			tb = append(tb, joinArgs(c.Args))
		}
	}
	if len(tb) != 1 {
		t.Fatalf("want exactly one tacklebox call, got %v", fake.Calls())
	}
	call := tb[0]
	for _, want := range []string{"customize-live", "--image img", "--script " + s1, "--script " + s2, "--tag " + got} {
		if !strings.Contains(call, want) {
			t.Errorf("tacklebox call missing %q: %q", want, call)
		}
	}
	if strings.Index(call, s1) > strings.Index(call, s2) {
		t.Errorf("scripts out of order in tacklebox call: %q", call)
	}
}

// A tacklebox failure names the tool and runs nothing after it.
func TestCustomizeImage_TackleboxFailurePropagates(t *testing.T) {
	fake := withFake(t)
	stubTacklebox(t)
	s := writeScript(t, "echo hi")
	fake.AddPrefixResponse("podman image exists ", "", errMissing)
	fake.AddPrefixResponse("tacklebox ", "", errBoom)
	_, err := LocalBuilder{}.customizeImage(BuildRequest{Image: "img", Customize: []string{s}}, nil)
	if err == nil || !strings.Contains(err.Error(), "tacklebox") {
		t.Fatalf("want tacklebox error, got %v", err)
	}
}

// Sudo builds invoke tacklebox under sudo so the derived image lands in
// root's store, where the sudo build looks for it.
func TestCustomizeImage_SudoPrefixesTacklebox(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, where sudo is never added")
	}
	fake := withFake(t)
	stubTacklebox(t)
	s := writeScript(t, "echo hi")
	fake.AddPrefixResponse("sudo podman image exists ", "", errMissing)
	fake.AddPrefixResponse("sudo -n tacklebox ", "tag\n", nil)
	_, err := LocalBuilder{Sudo: true}.customizeImage(BuildRequest{Image: "img", Customize: []string{s}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range fake.Calls() {
		if c.Name == "tacklebox" {
			t.Errorf("tacklebox ran without sudo under a sudo build: %v", fake.Calls())
		}
	}
	found := false
	for _, c := range fake.Calls() {
		if c.Name == "sudo" && len(c.Args) > 1 && c.Args[1] == "tacklebox" {
			found = true
		}
	}
	if !found {
		t.Errorf("no sudo tacklebox call: %v", fake.Calls())
	}
}

// Without the binary, --customize fails up front with install directions,
// not mid-build.
func TestCustomizeImage_MissingBinaryFailsFast(t *testing.T) {
	fake := withFake(t)
	old := tackleboxLookPath
	tackleboxLookPath = func(string) (string, error) { return "", errors.New("not found") }
	t.Cleanup(func() { tackleboxLookPath = old })
	s := writeScript(t, "echo hi")
	_, err := LocalBuilder{}.customizeImage(BuildRequest{Image: "img", Customize: []string{s}}, nil)
	if err == nil || !strings.Contains(err.Error(), "tacklebox is not installed") {
		t.Fatalf("want install-directions error, got %v", err)
	}
	if len(fake.Calls()) != 0 {
		t.Errorf("ran %d commands without the binary", len(fake.Calls()))
	}
}

// Missing scripts fail before anything runs.
func TestValidateCustomize_RejectsMissingScript(t *testing.T) {
	fake := withFake(t)
	if err := ValidateCustomize([]string{filepath.Join(t.TempDir(), "missing.sh")}); err == nil {
		t.Fatal("missing script was accepted")
	}
	if len(fake.Calls()) != 0 {
		t.Errorf("ran %d commands during validation", len(fake.Calls()))
	}
}
