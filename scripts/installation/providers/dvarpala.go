package providers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// This file holds the part of provisioning that is the same on every cloud.
//
// Creating a network and a virtual machine differs between AWS, GCP and Azure.
// What happens once there is a shell on an Ubuntu box does not, so it lives
// here once rather than three times. Each provider supplies only a way to run
// a command and a way to copy a file.

// InstallStep is one named command run over SSH.
type InstallStep struct {
	Name string
	Cmd  string
}

// SSHRunner runs a command on the target machine.
type SSHRunner func(command string) error

// InstallConfig describes what to install and where.
type InstallConfig struct {
	// SourceDir is the local checkout to install from. When empty the source
	// is cloned from GitHub instead, which is what an operator running a
	// released installer would get.
	SourceDir string

	// RepoURL and RepoRef are used when SourceDir is empty.
	RepoURL string
	RepoRef string

	// Host is the address clients will connect to. It goes into the server
	// certificate and into every issued client profile, so it must be the
	// address users can actually reach - not the VM's private address.
	Host string

	// AdminEmail, if set, is created as the first user and issued a profile.
	AdminEmail string
}

const (
	defaultRepoURL = "https://github.com/frigga-cloud/dvarpala.git"

	// The branch the cloud installers clone.
	//
	// TEMPORARY: this should be "main". It points at dev/foundation because
	// main does not yet contain scripts/install/ at all - a clone of it
	// installs nothing and fails on a missing file. Set this back to "main"
	// the day dev/foundation merges; it is the only place the branch is
	// named.
	defaultRepoRef = "install-v3"
	remoteSrcDir   = "/opt/dvarpala/src"
)

// InstallDvarpala puts the Dvarpala source on the machine and runs the
// installer, which configures PostgreSQL, Redis, OpenVPN with the Dvarpala
// hooks, the walled-garden firewall, and the application itself.
//
// Progress is reported through the same step-by-step output the rest of the
// installer uses.
func InstallDvarpala(cfg InstallConfig, run SSHRunner, upload func(local, remote string) error) error {
	if cfg.Host == "" {
		return fmt.Errorf("install: Host is required (clients connect to it and it is named in the server certificate)")
	}

	// 1. Get the source onto the machine.
	if cfg.SourceDir != "" {
		fmt.Println("📦 Uploading Dvarpala source")
		if err := uploadSource(cfg.SourceDir, run, upload); err != nil {
			return fmt.Errorf("uploading source: %w", err)
		}
	} else {
		repo := cfg.RepoURL
		if repo == "" {
			repo = defaultRepoURL
		}
		ref := cfg.RepoRef
		if ref == "" {
			ref = defaultRepoRef
		}

		fmt.Printf("📦 Cloning %s (%s)\n", repo, ref)
		// A freshly booted cloud instance is still running apt itself, and SSH
		// accepts connections before that finishes. Installing straight away
		// loses the race and dies with "Could not get lock", which reads like
		// a broken installer rather than one that arrived early.
		clone := fmt.Sprintf(
			"sudo cloud-init status --wait >/dev/null 2>&1 || true; "+
				"sudo apt-get -o DPkg::Lock::Timeout=600 update -qq && "+
				"sudo apt-get -o DPkg::Lock::Timeout=600 install -y -qq git && "+
				"sudo rm -rf %s && sudo git clone --depth 1 --branch %s %s %s",
			remoteSrcDir, shellQuote(ref), shellQuote(repo), remoteSrcDir)
		if err := run(clone); err != nil {
			return fmt.Errorf("cloning source: %w", err)
		}
	}

	// 2. Run the installer. It is idempotent, reports each step, and fails
	//    loudly rather than leaving a half-configured machine reporting
	//    success.
	fmt.Println("⚙️  Running the Dvarpala installer (this takes a few minutes)")
	install := fmt.Sprintf(
		"sudo chmod +x %s/scripts/install/install-dvarpala.sh && "+
			"sudo DVARPALA_ADMIN_EMAIL=%s %s/scripts/install/install-dvarpala.sh "+
			"--source %s --host %s",
		remoteSrcDir, shellQuote(cfg.AdminEmail), remoteSrcDir,
		remoteSrcDir, shellQuote(cfg.Host))

	if err := run(install); err != nil {
		return fmt.Errorf("running installer: %w", err)
	}

	fmt.Println("✅ Dvarpala installed")
	return nil
}

// uploadSource streams a local checkout to the machine.
//
// A tar archive is used rather than a recursive copy so the transfer is one
// stream, and .git and build output are left behind.
func uploadSource(sourceDir string, run SSHRunner, upload func(local, remote string) error) error {
	info, err := os.Stat(sourceDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("source directory not found: %s", sourceDir)
	}

	tarball := filepath.Join(os.TempDir(), "dvarpala-source.tar.gz")
	cmd := exec.Command("tar",
		"--exclude=.git", "--exclude=certs", "--exclude=bin",
		"--exclude=*.ovpn", "--exclude=node_modules",
		"-czf", tarball, "-C", sourceDir, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("creating archive: %v: %s", err, strings.TrimSpace(string(out)))
	}
	defer os.Remove(tarball)

	if err := upload(tarball, "/tmp/dvarpala-source.tar.gz"); err != nil {
		return fmt.Errorf("copying archive: %w", err)
	}

	extract := fmt.Sprintf(
		"sudo rm -rf %s && sudo mkdir -p %s && "+
			"sudo tar -xzf /tmp/dvarpala-source.tar.gz -C %s && "+
			"rm -f /tmp/dvarpala-source.tar.gz",
		remoteSrcDir, remoteSrcDir, remoteSrcDir)
	return run(extract)
}

// shellQuote makes a value safe to embed in a remote shell command.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// adminSourceRanges is where administrative access - SSH - may come from.
//
// It is the configured allowed_ips, or everywhere when nothing is configured.
// Empty must stay permissive: a deployment that never set the field would
// otherwise build a server nobody could log in to, including the person
// running the installer.
//
// The VPN port is deliberately NOT restricted by this. Employees connect from
// wherever they happen to be - a home, a phone on mobile data, an airport -
// so narrowing UDP 1194 to an office address would break the one thing the
// product exists to do, and would do it silently, days later, to somebody who
// is not the person who set the field.
func adminSourceRanges(allowedIPs []string) []string {
	ranges := make([]string, 0, len(allowedIPs))
	for _, cidr := range allowedIPs {
		if trimmed := strings.TrimSpace(cidr); trimmed != "" {
			ranges = append(ranges, trimmed)
		}
	}
	if len(ranges) == 0 {
		return []string{"0.0.0.0/0"}
	}
	return ranges
}
