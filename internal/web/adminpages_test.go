package web

import (
	"html/template"
	"strings"
	"testing"
	"time"
)

// consoleTemplate parses the console the way the application does.
//
// A template error is invisible until somebody opens the page: gin renders
// what it managed to produce and logs the rest. The console is where an
// administrator goes during an incident, so a page that half-renders there is
// worse than one that fails loudly in a test.
func consoleTemplate(t *testing.T) *template.Template {
	t.Helper()
	tmpl, err := template.ParseFiles("../../web/templates/admin.html")
	if err != nil {
		t.Fatalf("parsing the console template: %v", err)
	}
	return tmpl
}

func render(t *testing.T, v adminView) string {
	t.Helper()
	var out strings.Builder
	if err := consoleTemplate(t).Execute(&out, v); err != nil {
		t.Fatalf("rendering the %q page: %v", v.Page, err)
	}
	return out.String()
}

// The activity page has to distinguish a session that opens the network from
// one that only signed a browser in. Confusing the two during an incident
// means either chasing somebody who has no access, or missing somebody who
// does.
func TestActivityPageSeparatesTunnelFromBrowserOnly(t *testing.T) {
	html := render(t, adminView{
		Page: "activity",
		Live: []liveRow{
			{Email: "sam@acme.com", Provider: "otp", ClientIP: "172.30.100.7",
				Groups: "engineering", Since: "2026-08-25 10:14", Remaining: "7h 12m", OnTunnel: true},
			{Email: "maya@acme.com", Provider: "otp", Since: "2026-08-25 09:02",
				Remaining: "22m", OnTunnel: false},
		},
		SignIns: []signInRow{
			{Email: "sam@acme.com", Status: "active", When: "2026-08-25 10:14", Ago: "3h ago"},
		},
	})

	for _, want := range []string{"tunnel open", "browser only", "172.30.100.7", "7h 12m", "3h ago"} {
		if !strings.Contains(html, want) {
			t.Errorf("the activity page never shows %q", want)
		}
	}
}

// Both tables say "nobody" rather than rendering an empty frame, because an
// empty table reads as a page that failed to load.
func TestActivityPageSaysSoWhenNobodyIsOn(t *testing.T) {
	html := render(t, adminView{Page: "activity"})

	if !strings.Contains(html, "Nobody is signed in.") {
		t.Error("an empty network does not say so")
	}
	if !strings.Contains(html, "Nobody has ever signed in.") {
		t.Error("an empty sign-in history does not say so")
	}
}

// An entry written before anyone was identified is the most interesting kind
// in the trail - a refused sign-in. It must not render as a blank cell that
// reads like missing data.
func TestAuditPageNamesUnidentifiedActors(t *testing.T) {
	html := render(t, adminView{
		Page: "audit",
		Trail: []auditRow{
			{ID: 41, When: "2026-08-25 10:14:02", Email: "", Action: "authentication_failed",
				IP: "172.30.100.9", Details: `{"email":"nobody@example.com"}`},
			{ID: 40, When: "2026-08-25 10:13:40", Email: "sam@acme.com", Action: "user_login",
				IP: "172.30.100.7"},
		},
		Kinds: []kindRow{{Action: "user_login", Count: 12}},
	})

	if !strings.Contains(html, "not identified") {
		t.Error("an unattributed record renders as an empty cell")
	}
	if !strings.Contains(html, "authentication_failed") {
		t.Error("the action is missing from the trail")
	}
	if !strings.Contains(html, "user_login (12)") {
		t.Error("the action filter does not offer the kinds present in the trail")
	}
}

// The filter has to survive a search, or narrowing the trail and then
// narrowing it again silently starts from scratch.
func TestAuditFilterIsRememberedAcrossASearch(t *testing.T) {
	html := render(t, adminView{
		Page:        "audit",
		FilterEmail: "sam@acme.com",
		FilterKind:  "user_login",
		Kinds:       []kindRow{{Action: "user_login", Count: 12}},
	})

	if !strings.Contains(html, `value="sam@acme.com"`) {
		t.Error("the email filter is cleared after searching")
	}
	if !strings.Contains(html, "selected") {
		t.Error("the action filter is cleared after searching")
	}
}

// Coarse enough to read at a glance, and never claiming a session is alive
// when Redis says it is not.
func TestHumanDuration(t *testing.T) {
	for _, c := range []struct {
		in   time.Duration
		want string
	}{
		{-time.Second, "expired"},
		{0, "expired"},
		{30 * time.Second, "under a minute"},
		{22 * time.Minute, "22m"},
		{2 * time.Hour, "2h"},
		{2*time.Hour + 12*time.Minute, "2h 12m"},
		{50 * time.Hour, "2d 2h"},
	} {
		if got := humanDuration(c.in); got != c.want {
			t.Errorf("humanDuration(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// A details column that silently cut its value would misrepresent the record.
func TestTruncateMarksWhatItCut(t *testing.T) {
	if got := truncate("short", 90); got != "short" {
		t.Errorf("a short value was altered: %q", got)
	}
	long := strings.Repeat("x", 200)
	got := truncate(long, 90)
	if !strings.HasSuffix(got, "…") {
		t.Error("a truncated value does not show that it was cut")
	}
	if len([]rune(got)) != 91 {
		t.Errorf("truncated to %d runes, want 91", len([]rune(got)))
	}
}
