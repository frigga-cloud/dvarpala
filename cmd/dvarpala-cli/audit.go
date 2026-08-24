package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"dvarpala/internal/services"

	"github.com/spf13/cobra"
)

// auditCmd reads the trail.
//
// Every record in it was already being written; until now the only way to see
// one was a database password and a hand-written query, which puts the most
// useful data in the system out of reach of the person most likely to need it.
//
// Reading only, deliberately. There is no command here that alters or removes
// a record, because a trail the operator can edit is not evidence of anything.
func auditCmd() *cobra.Command {
	var (
		email  string
		action string
		since  string
		limit  int
		full   bool
	)

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Read the audit trail",
		Long: "Shows what happened, newest first.\n\n" +
			"With no options, the most recent activity of any kind. The filters\n" +
			"combine, so --user with --action narrows to both.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}

			window, err := parseWindow(since)
			if err != nil {
				return err
			}

			records, err := svc.Audit.List(context.Background(), services.AuditQuery{
				Email: email, Action: action, Since: window, Limit: limit,
			})
			if err != nil {
				return err
			}

			if len(records) == 0 {
				fmt.Println(noRecordsMessage(email, action, since))
				return nil
			}

			printRecords(records, full)
			return nil
		},
	}

	cmd.Flags().StringVar(&email, "user", "", "only this person's activity")
	cmd.Flags().StringVar(&action, "action", "", "only this kind of event (see: audit actions)")
	cmd.Flags().StringVar(&since, "since", "", "only recent activity, e.g. 30m, 2h, 7d")
	cmd.Flags().IntVar(&limit, "limit", 50, "how many records to show")
	cmd.Flags().BoolVar(&full, "full", false, "show the details of each record in full")

	cmd.AddCommand(auditActionsCmd())
	return cmd
}

// auditActionsCmd answers "what can I filter on?" without reading the source.
func auditActionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "actions",
		Short: "List the kinds of event in the trail, and how many of each",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := openServices()
			if err != nil {
				return err
			}

			counts, err := svc.Audit.Actions(context.Background())
			if err != nil {
				return err
			}
			if len(counts) == 0 {
				fmt.Println("The audit trail is empty.")
				return nil
			}

			names := make([]string, 0, len(counts))
			width := 0
			for name := range counts {
				names = append(names, name)
				if len(name) > width {
					width = len(name)
				}
			}
			sort.Slice(names, func(i, j int) bool {
				if counts[names[i]] != counts[names[j]] {
					return counts[names[i]] > counts[names[j]]
				}
				return names[i] < names[j]
			})

			for _, name := range names {
				fmt.Printf("%-*s  %d\n", width, name, counts[name])
			}
			return nil
		},
	}
}

func printRecords(records []services.AuditRecord, full bool) {
	fmt.Printf("%-19s  %-24s  %-26s  %-15s  %s\n",
		"WHEN", "ACTION", "WHO", "FROM", "DETAILS")

	for _, r := range records {
		who := r.Email
		if who == "" {
			// No attributed user. Either the action happened before anyone was
			// identified, or it names the person only in its details.
			who = detailString(r.Details, "email")
			if who == "" {
				who = "-"
			} else {
				// Marked, because knowing an address was typed is not the same
				// as knowing whose action this was.
				who = who + " *"
			}
		}

		from := r.IPAddress
		if from == "" {
			from = "-" // a CLI action, run on the server itself
		}

		fmt.Printf("%-19s  %-24s  %-26s  %-15s  %s\n",
			r.CreatedAt.Format("2006-01-02 15:04:05"),
			r.Action, truncate(who, 26), truncate(from, 15),
			formatDetails(r.Details, full))
	}

	fmt.Printf("\n%d record(s). * means the address was supplied rather than authenticated.\n",
		len(records))
}

// formatDetails renders the jsonb column for a terminal.
//
// Unless asked for in full, the email is dropped: it already has its own
// column, and repeating it crowds out the part that says what happened.
func formatDetails(raw string, full bool) string {
	if raw == "" || raw == "{}" {
		return "-"
	}

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return raw // not valid JSON; show it as stored rather than hiding it
	}

	if !full {
		delete(m, "email")
	}
	if len(m) == 0 {
		return "-"
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v := fmt.Sprintf("%v", m[k])
		if !full {
			v = truncate(v, 60)
		}
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, " ")
}

func detailString(raw, key string) string {
	var m map[string]interface{}
	if json.Unmarshal([]byte(raw), &m) != nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

// parseWindow accepts the usual durations plus days, which time.ParseDuration
// does not - and which is the unit anybody investigating actually reaches for.
func parseWindow(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}

	if strings.HasSuffix(s, "d") {
		var days float64
		if _, err := fmt.Sscanf(strings.TrimSuffix(s, "d"), "%f", &days); err != nil {
			return 0, fmt.Errorf("--since %q: expected something like 30m, 2h or 7d", s)
		}
		return time.Duration(days * float64(24*time.Hour)), nil
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("--since %q: expected something like 30m, 2h or 7d", s)
	}
	return d, nil
}

// noRecordsMessage distinguishes an empty trail from filters that matched
// nothing, so "no results" is never mistaken for "nothing happened".
func noRecordsMessage(email, action, since string) string {
	var applied []string
	if email != "" {
		applied = append(applied, "user "+email)
	}
	if action != "" {
		applied = append(applied, "action "+action)
	}
	if since != "" {
		applied = append(applied, "the last "+since)
	}

	if len(applied) == 0 {
		return "The audit trail is empty."
	}
	return "No records for " + strings.Join(applied, ", ") + ".\n" +
		"Run 'dvarpala-cli audit actions' to see what kinds of event exist."
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
