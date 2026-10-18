// Customize delegates image customization to tacklebox before the disk
// build.
//
// corral does not reimplement this: `tacklebox customize-live` runs its
// embedded live baseline (live user, DM autologin, live networking, sleep
// masking) followed by each --customize script in order, inside a container
// of the image, and prints the derived image ref. That baseline is part of
// the deal — a customized image boots to a live session, not to the stock
// image plus your script.
//
// The derived image is content-tagged (image plus every script's bytes, in
// order), so re-running the same customization reuses the tag instead of
// re-executing anything. tacklebox runs podman as its invoking user, so
// corral invokes tacklebox itself under sudo exactly when the build uses
// sudo podman — otherwise the derived image would land in a store the
// build never looks at.

package bootc

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// customizeTagBase names the derived images. localhost/ keeps podman from
// treating the ref as a registry and trying to pull it (see fetch).
const customizeTagBase = "localhost/corral-bootc-customize"

// customizeTag derives a stable tag from the image plus every script's bytes
// in order. Same inputs, same tag: a cache hit skips tacklebox entirely.
func customizeTag(image string, scripts []string) (string, error) {
	h := sha256.New()
	io.WriteString(h, image)
	h.Write([]byte{0})
	for _, s := range scripts {
		body, err := os.ReadFile(s)
		if err != nil {
			return "", fmt.Errorf("customize script %s: %w", s, err)
		}
		h.Write(body)
		h.Write([]byte{0})
	}
	return fmt.Sprintf("%s-%x", customizeTagBase, h.Sum(nil)[:8]), nil
}

// tackleboxLookPath is exec.LookPath as a var so tests can stub the
// binary's presence without installing anything.
var tackleboxLookPath = exec.LookPath

// ValidateCustomize checks flag values before any backend runs: every script
// must exist and be readable.
func ValidateCustomize(scripts []string) error {
	for _, s := range scripts {
		f, err := os.Open(s)
		if err != nil {
			return fmt.Errorf("customize script %s: %w", s, err)
		}
		_ = f.Close()
	}
	return nil
}

// tacklebox runs the binary, honouring Sudo for the same reason podman
// does: tacklebox shells to podman as its invoking user, so a sudo build
// needs a sudo tacklebox or the derived image lands in the wrong store.
func (b LocalBuilder) tacklebox(args ...string) (string, []string) {
	if b.Sudo && os.Geteuid() != 0 {
		return "sudo", append([]string{"-n", "tacklebox"}, args...)
	}
	return "tacklebox", args
}

// customizeImage applies req.Customize through tacklebox and returns the
// image the build should install: the derived tag, or req.Image when no
// scripts were given. It reports through report, like the rest of the build.
func (b LocalBuilder) customizeImage(req BuildRequest, report func(string)) (string, error) {
	say := func(msg string) {
		if report != nil {
			report(msg)
		}
	}
	if len(req.Customize) == 0 {
		return req.Image, nil
	}
	if _, err := tackleboxLookPath("tacklebox"); err != nil {
		return "", fmt.Errorf("tacklebox is not installed, and --customize delegates to it — build it from https://github.com/tuna-os/tacklebox (tagged releases with binaries are tracked there) or drop --customize")
	}
	tag, err := customizeTag(req.Image, req.Customize)
	if err != nil {
		return "", err
	}
	name, args := b.podman("image", "exists", tag)
	if _, err := runner.Run(name, args...); err == nil {
		say("reusing customized image " + tag)
		return tag, nil
	}

	targs := []string{"customize-live", "--image", req.Image}
	for _, s := range req.Customize {
		targs = append(targs, "--script", s)
	}
	targs = append(targs, "--tag", tag)
	say(fmt.Sprintf("customizing %s with %d script(s) via tacklebox", req.Image, len(req.Customize)))
	tname, targs := b.tacklebox(targs...)
	out, err := runner.Run(tname, targs...)
	if err != nil {
		return "", fmt.Errorf("tacklebox customize-live failed: %s", commandError(out, err))
	}
	return tag, nil
}
