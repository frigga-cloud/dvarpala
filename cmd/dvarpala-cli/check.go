package main

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"dvarpala/internal/auth"
	"dvarpala/internal/config"
	"dvarpala/internal/database"
	"dvarpala/internal/redis"
	"dvarpala/internal/services"
	"dvarpala/internal/vpn"

	"github.com/spf13/cobra"
)

// checkCmd answers one question: is this installation actually working?
//
// It exists because there was no way to ask. Proving an install meant a dozen
// commands - services, ports, the journal, the certificate, whether anybody
// could sign in - and the installer's own summary said "complete" while the
// server could not authenticate a single person. Anything that can be got
// wrong quietly is checked here, and every failure says what to do about it.
func checkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check whether this Dvarpala installation is working",
		Long: "Runs through everything an installation needs and reports what is\n" +
			"wrong and how to fix it. Exits non-zero if anything essential fails,\n" +
			"so it can be used as the last step of an install script.",
		RunE: func(cmd *cobra.Command, args []string) error {
			r := &report{}
			ctx := cmd.Context()

			cfg, err := config.Load(configPath)
			if err != nil {
				r.fail("Configuration", fmt.Sprintf("cannot read %s: %v", configPath, err),
					"Pass --config with the path to environment.yaml.")
				r.print()
				return errCheckFailed
			}
			r.pass("Configuration", configPath)

			checkDatabase(ctx, r, cfg)
			checkRedis(ctx, r, cfg)
			checkLogin(r, cfg)
			checkCertificates(r, cfg)
			checkOpenVPN(r, cfg)
			checkHooks(r)
			checkFirewall(r)
			checkServer(r, cfg)

			r.print()
			if r.failed > 0 {
				return errCheckFailed
			}
			return nil
		},
	}
}

// errCheckFailed is returned when something essential is wrong. main prints
// it, so the message is deliberately short - the report above it is the
// explanation.
var errCheckFailed = fmt.Errorf("this installation is not ready")

// ── the individual checks ───────────────────────────────────────────────────

func checkDatabase(ctx context.Context, r *report, cfg *config.Config) {
	db, err := database.NewConnection(cfg.Database)
	if err != nil {
		r.fail("PostgreSQL", err.Error(),
			"Is postgresql running?  systemctl status postgresql")
		return
	}

	var users int64
	if err := db.DB.WithContext(ctx).Table("users").Count(&users).Error; err != nil {
		r.fail("PostgreSQL", "connected, but cannot read the users table: "+err.Error(),
			"The schema may not have been created. Restart dvarpala, which migrates on startup.")
		return
	}
	r.pass("PostgreSQL", fmt.Sprintf("%s, %d user(s)", cfg.Database.Name, users))

	// Somebody has to be able to administer this, and only a member of the
	// administrators group can open the console.
	svc := services.New(db.DB, cfg)
	admins, err := svc.Groups.GetGroupByName(ctx, "system_admins")
	switch {
	case err != nil:
		r.warn("Administrators", "no system_admins group",
			"Create one and add somebody, or nobody can open the admin console.")
	case len(admins.Users) == 0:
		r.warn("Administrators", "system_admins group is empty",
			"dvarpala-cli group assign --user you@your-domain --group system_admins")
	default:
		r.pass("Administrators", fmt.Sprintf("%d in system_admins", len(admins.Users)))
	}
}

func checkRedis(ctx context.Context, r *report, cfg *config.Config) {
	rdb, err := redis.NewClient(cfg.Redis)
	if err != nil {
		r.fail("Redis", err.Error(), "Is redis running?  systemctl status redis-server")
		return
	}
	defer rdb.Close()

	// Sessions and sign-in codes both live here, and the login path needs a
	// command that older builds of Redis do not have.
	if err := rdb.Ping(ctx).Err(); err != nil {
		r.fail("Redis", err.Error(), "Is redis running?  systemctl status redis-server")
		return
	}
	r.pass("Redis", cfg.Redis.Addr)
}

// checkLogin is the one that matters most. An installation where nobody can
// sign in looks entirely healthy from every other angle: the tunnel forms,
// the walled garden holds, and every person who connects is stuck in it.
func checkLogin(r *report, cfg *config.Config) {
	var methods []string

	if cfg.OAuth.Google.ClientID != "" {
		methods = append(methods, "Google")

		// Google refuses plain HTTP and refuses IP addresses, so a redirect
		// URL that is either cannot work no matter what else is right.
		u := cfg.OAuth.Google.RedirectURL
		if strings.HasPrefix(u, "http://") && !strings.Contains(u, "localhost") {
			r.warn("Google login", "redirect URL is plain HTTP: "+u,
				"Google accepts only https:// and only domain names, never an IP\n"+
					"     address. This will fail with invalid_request until the portal has\n"+
					"     a domain and a certificate.")
		}
	}

	if cfg.Auth.OTP.Enabled {
		if cfg.Auth.SMTP.Host == "" && strings.ToLower(cfg.Server.Mode) != "debug" {
			r.fail("Code sign-in", "enabled but no mail server is configured",
				"Set auth.smtp, or the server will refuse to start.")
		} else if cfg.Auth.SMTP.Host == "" {
			r.warn("Code sign-in", "codes are written to the server log (debug mode)",
				"Fine for testing. Configure auth.smtp before anyone relies on it.")
			methods = append(methods, "emailed codes (to the log)")
		} else {
			checkSMTP(r, cfg)
			methods = append(methods, "emailed codes")
		}
	}

	if strings.ToLower(cfg.Server.Mode) == "debug" {
		methods = append(methods, "development login")
		r.warn("Server mode", "debug",
			"The development login accepts any identity without a password.\n"+
				"     Set server.mode: release before this is reachable by anyone else.")
	}

	if len(methods) == 0 {
		r.fail("Sign-in", "no login method is configured - nobody can authenticate",
			"Set auth.otp.enabled: true and fill in auth.smtp, then restart.\n"+
				"     Until then everyone who connects is stuck at the login page.")
		return
	}
	r.pass("Sign-in", strings.Join(methods, ", "))
}

// checkSMTP proves the mail credentials without sending anything.
//
// The password is normally set on the service rather than in the config file,
// and this command is a different process that inherits none of it - so look
// where the service keeps it before concluding that it is missing.
func checkSMTP(r *report, cfg *config.Config) {
	s := cfg.Auth.SMTP
	port := s.Port
	if port == 0 {
		port = 587
	}

	password := s.Password
	if password == "" {
		password = os.Getenv("AUTH_SMTP_PASSWORD")
	}
	if password == "" {
		password = smtpPasswordFromService()
	}
	if password == "" && s.Username != "" {
		r.warn("Mail credentials", "no password found",
			"Checked the config, AUTH_SMTP_PASSWORD, and the systemd override.\n"+
				"     Sending will fail with 'Username and Password not accepted'.")
		return
	}

	mailer := &auth.SMTPProbe{
		Host: s.Host, Port: port, Username: s.Username, Password: password,
	}
	if err := mailer.Probe(); err != nil {
		r.fail("Mail server", fmt.Sprintf("%s:%d rejected us: %v", s.Host, port, err),
			"For Google Workspace this is usually an ordinary account password\n"+
				"     where an app password is needed: myaccount.google.com/apppasswords")
		return
	}
	r.pass("Mail server", fmt.Sprintf("%s:%d accepted our credentials", s.Host, port))
}

// smtpPasswordFromService reads the password out of the systemd drop-in the
// installer's instructions tell an operator to create.
func smtpPasswordFromService() string {
	paths, _ := filepath.Glob("/etc/systemd/system/dvarpala.service.d/*.conf")
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if !strings.Contains(line, "AUTH_SMTP_PASSWORD=") {
				continue
			}
			_, value, _ := strings.Cut(line, "AUTH_SMTP_PASSWORD=")
			return strings.Trim(strings.TrimSpace(value), `"`)
		}
	}
	return ""
}

func checkCertificates(r *report, cfg *config.Config) {
	pki := cfg.OpenVPN.PKI

	ca, err := readCert(pki.CACert)
	if err != nil {
		r.fail("Certificate authority", err.Error(),
			"Without it no VPN profile can be issued and none already issued can be verified.")
		return
	}

	left := time.Until(ca.NotAfter)
	switch {
	case left <= 0:
		r.fail("Certificate authority", "expired on "+ca.NotAfter.Format("2 Jan 2006"),
			"Every certificate it signed is now untrusted.")
	case left < 90*24*time.Hour:
		r.warn("Certificate authority", fmt.Sprintf("expires in %d days", int(left.Hours()/24)),
			"Plan the replacement: every profile must be reissued.")
	default:
		r.pass("Certificate authority", fmt.Sprintf("valid until %s", ca.NotAfter.Format("2 Jan 2006")))
	}

	// The key is what signs new profiles, and the one file here that cannot
	// be recreated.
	if info, err := os.Stat(pki.CAKey); err != nil {
		r.fail("Certificate authority key", err.Error(), "No new profiles can be issued.")
	} else if info.Mode().Perm()&0o077 != 0 {
		r.warn("Certificate authority key", fmt.Sprintf("mode %o - readable by others", info.Mode().Perm()),
			"chmod 600 "+pki.CAKey+"  - anyone who reads it can issue a profile for any identity.")
	} else {
		r.pass("Certificate authority key", "present, not world readable")
	}
}

func checkOpenVPN(r *report, cfg *config.Config) {
	// The port clients actually dial.
	if out, err := exec.Command("ss", "-lun").Output(); err == nil {
		if strings.Contains(string(out), ":1194") {
			r.pass("VPN", "listening on udp/1194")
		} else {
			r.fail("VPN", "nothing is listening on udp/1194",
				"systemctl status openvpn-server@server")
		}
	}

	// The control channel, which is how a session is ended at once and how a
	// client is told to reconnect after signing in.
	m := vpn.NewManagement(cfg.OpenVPN.Management.Host, cfg.OpenVPN.Management.Port)
	clients, err := m.Clients(context.Background())
	if err != nil {
		r.warn("VPN control channel", "unreachable",
			"Add 'management 127.0.0.1 7505' to /etc/openvpn/server/server.conf.\n"+
				"     Without it, deactivating somebody leaves their tunnel up, and people\n"+
				"     must reconnect by hand after signing in.")
		return
	}
	r.pass("VPN control channel", fmt.Sprintf("responding, %d client(s) connected", len(clients)))
}

func checkHooks(r *report) {
	for _, path := range []string{
		"/opt/dvarpala/scripts/client-connect.sh",
		"/opt/dvarpala/scripts/client-disconnect.sh",
		"/opt/dvarpala/scripts/dvarpala-firewall.sh",
	} {
		info, err := os.Stat(path)
		if err != nil {
			r.fail("VPN hooks", path+" is missing",
				"Without it every client is granted the portal and nothing else, forever.")
			return
		}
		if info.Mode().Perm()&0o111 == 0 {
			r.fail("VPN hooks", path+" is not executable", "chmod +x "+path)
			return
		}
	}
	r.pass("VPN hooks", "connect, disconnect and firewall scripts present")
}

func checkFirewall(r *report) {
	out, err := exec.Command("iptables", "-L", "DVARPALA", "-n").Output()
	if err != nil {
		r.fail("Firewall", "the DVARPALA chain does not exist",
			"Routes alone do not confine anybody. Run:\n"+
				"     /opt/dvarpala/scripts/dvarpala-firewall.sh setup")
		return
	}
	if !strings.Contains(string(out), "DROP") {
		r.fail("Firewall", "the DVARPALA chain has no DROP rule",
			"Unpermitted traffic is not being blocked.")
		return
	}
	r.pass("Firewall", "DVARPALA chain present and dropping by default")
}

func checkServer(r *report, cfg *config.Config) {
	port := cfg.Server.Port
	if port == 0 {
		port = 8080
	}

	client := http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	if err != nil {
		r.fail("Portal", err.Error(),
			"The login page is unreachable, so nobody can sign in.\n"+
				"     systemctl status dvarpala")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		r.fail("Portal", fmt.Sprintf("returned %d", resp.StatusCode),
			"journalctl -u dvarpala -n 50")
		return
	}
	r.pass("Portal", fmt.Sprintf("answering on port %d", port))
}

// ── reporting ───────────────────────────────────────────────────────────────

type result struct {
	kind   string // pass, warn, fail
	name   string
	detail string
	advice string
}

type report struct {
	results []result
	failed  int
	warned  int
}

func (r *report) pass(name, detail string) {
	r.results = append(r.results, result{"pass", name, detail, ""})
}

func (r *report) warn(name, detail, advice string) {
	r.results = append(r.results, result{"warn", name, detail, advice})
	r.warned++
}

func (r *report) fail(name, detail, advice string) {
	r.results = append(r.results, result{"fail", name, detail, advice})
	r.failed++
}

func (r *report) print() {
	fmt.Println()
	for _, res := range r.results {
		mark := " ok "
		switch res.kind {
		case "warn":
			mark = " !! "
		case "fail":
			mark = "FAIL"
		}

		fmt.Printf("  [%s]  %-24s %s\n", mark, res.name, res.detail)
		if res.advice != "" {
			for _, line := range strings.Split(res.advice, "\n") {
				fmt.Printf("           %s\n", line)
			}
		}
	}

	fmt.Println()
	switch {
	case r.failed > 0:
		fmt.Printf("  %d problem(s) must be fixed before this installation can be used.\n\n", r.failed)
	case r.warned > 0:
		fmt.Printf("  Working, with %d thing(s) worth attention.\n\n", r.warned)
	default:
		fmt.Print("  Everything checks out.\n\n")
	}
}

// ── small helpers ───────────────────────────────────────────────────────────

func readCert(path string) (*x509.Certificate, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("%s is not a certificate", path)
	}
	return x509.ParseCertificate(block.Bytes)
}
