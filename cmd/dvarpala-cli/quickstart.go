package main

import (
	"context"
	"fmt"

	"dvarpala/internal/auth"
	"dvarpala/internal/config"

	"github.com/spf13/cobra"
)

// quickstartCmd prints the order things have to be done in.
//
// Every individual command explains itself, and "check" reports what is
// broken. Neither answers the question somebody actually has on their first
// morning, which is what to do first: that a permission cannot be granted
// before the resource exists, that a group has to exist before anybody joins
// it, and that a VPN profile is worthless until somebody can sign in.
//
// The installer prints this once, at the end, where it scrolls past. This is
// the same thing, available whenever it is wanted, and it reports how far
// along the installation actually is rather than describing a fresh one.
func quickstartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "quickstart",
		Short: "What to do, in the order it has to be done",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Best effort. The point is the order, and that is worth printing
			// even on a machine whose database is not up yet.
			state := surveyInstallation()

			fmt.Println("Setting up Dvarpala")
			fmt.Println("===================")
			fmt.Println()
			fmt.Println("Each step depends on the one before it. Skipping ahead")
			fmt.Println("produces things that look right and do nothing.")
			fmt.Println()

			step(1, state.canSignIn,
				"Make it possible to sign in",
				[]string{
					"Nobody can sign in on a fresh server. Until this is done,",
					"anyone who connects reaches the sign-in page and nothing else.",
					"",
					"  sudo nano /opt/dvarpala/config/environment.yaml   # auth.otp.enabled: true",
					"                                                    # auth.brevo or auth.smtp",
					"  sudo nano /etc/dvarpala/dvarpala.env              # the key or password",
					"  sudo systemctl restart dvarpala",
					"  dvarpala-cli mail test you@your-domain",
					"",
					"Do not go further until a real message arrives.",
				})

			step(2, state.hasResource,
				"Register what people should be able to reach",
				[]string{
					"A resource is anything with an address this server can reach.",
					"Check it can, before granting it: if the server cannot reach it,",
					"no permission will help.",
					"",
					"  dvarpala-cli resource create --name wiki --type service --ip 10.0.5.20",
					"",
					"A resource with no address can be created and granted, and will",
					"produce no route and no access at all.",
				})

			step(3, state.hasGroup,
				"Make a group, and grant it that resource",
				[]string{
					"Permissions attach to groups, never to a person. People inherit",
					"what their groups have.",
					"",
					"  dvarpala-cli group create --name engineering --description \"Engineering\"",
					"  dvarpala-cli permission grant --group engineering --resource wiki --type read",
				})

			step(4, state.hasNonAdminUser,
				"Add people, and put them in a group",
				[]string{
					"  dvarpala-cli user create --email sam@your-domain --name \"Sam Patel\"",
					"  dvarpala-cli group assign --user sam@your-domain --group engineering",
					"",
					"Check it came out as intended:",
					"",
					"  dvarpala-cli user access sam@your-domain",
				})

			step(5, state.hasProfile,
				"Issue each person a VPN profile",
				[]string{
					"  dvarpala-cli vpn issue --user sam@your-domain --output sam.ovpn",
					"",
					"The file contains a private key. Hand it over directly rather than",
					"by email, and delete your copy afterwards.",
					"",
					"Tell them: import it into an OpenVPN client, connect, then open",
					"http://signin - no operating system announces a captive portal",
					"when a VPN comes up, so that address has to be passed on.",
				})

			step(6, false,
				"Keep a copy of the backups somewhere else",
				[]string{
					"Written nightly to /var/backups/dvarpala, which is no protection",
					"if the machine is lost.",
					"",
					"  scp -i <key> ubuntu@<this-server>:/var/backups/dvarpala/\\* .",
					"",
					"They hold the certificate authority. Without it, every profile ever",
					"issued stops working and everybody needs a new one.",
				})

			fmt.Println("Afterwards")
			fmt.Println("----------")
			fmt.Println("  dvarpala-cli check            is this installation working")
			fmt.Println("  dvarpala-cli session list     who is connected now")
			fmt.Println("  dvarpala-cli audit --limit 20 what has happened")
			fmt.Println("  dvarpala-cli --help           everything else")
			fmt.Println()
			return nil
		},
	}
}

// installationState is how far along this machine is, so the steps can say
// which are already done rather than describing a fresh install to somebody
// half way through one.
type installationState struct {
	canSignIn       bool
	hasResource     bool
	hasGroup        bool
	hasNonAdminUser bool
	hasProfile      bool
}

func surveyInstallation() installationState {
	var state installationState

	// Whether anybody could sign in, asked the same way the server asks it:
	// codes switched on, and something able to send them that is not the
	// debug-only log.
	if cfg, err := config.Load(configPath); err == nil && cfg.Auth.OTP.Enabled {
		mailer, _, err := auth.NewMailer(auth.MailerSettings{
			BrevoAPIKey: cfg.Auth.Brevo.APIKey,
			BrevoFrom:   cfg.Auth.Brevo.From,
			SMTPHost:    cfg.Auth.SMTP.Host,
			SMTPFrom:    cfg.Auth.SMTP.From,
			ServerMode:  cfg.Server.Mode,
		})
		if err == nil {
			_, viaLog := mailer.(auth.LogMailer)
			state.canSignIn = !viaLog
		}
	}

	svc, err := openServices()
	if err != nil {
		return state // no database yet; the steps still stand
	}
	ctx := context.Background()

	if users, err := svc.Users.ListUsers(ctx); err == nil {
		// The installer creates the first administrator, so one user alone says
		// nothing about progress. A second one does.
		state.hasNonAdminUser = len(users) > 1
	}

	if resources, err := svc.Resources.ListResources(ctx); err == nil {
		for _, r := range resources {
			// The two the installer creates have no address and are not
			// evidence that anybody has registered anything real.
			if r.IPAddress != "" {
				state.hasResource = true
				break
			}
		}
	}

	if groups, err := svc.Groups.ListGroups(ctx); err == nil {
		for _, g := range groups {
			if g.Name != "system_admins" && g.Name != "vpn_users" {
				state.hasGroup = true
				break
			}
		}
	}

	if configs, err := svc.VPNConfigs.ListAll(ctx); err == nil && len(configs) > 0 {
		state.hasProfile = true
	}

	return state
}

func step(n int, done bool, title string, lines []string) {
	mark := " "
	if done {
		mark = "✓"
	}
	fmt.Printf("[%s] STEP %d - %s\n", mark, n, title)
	if done {
		fmt.Println("        (looks done already)")
	}
	fmt.Println()
	for _, l := range lines {
		if l == "" {
			fmt.Println()
			continue
		}
		fmt.Println("    " + l)
	}
	fmt.Println()
}
