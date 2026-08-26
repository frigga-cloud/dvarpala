package main

import (
	"bufio"
	"os"
	"strings"
)

// secretsFile holds the credentials that must not live in the settings file.
//
// systemd loads it for the service, and this loads it for the CLI, so both see
// the same values. Without that they do not: a key set with "systemctl edit"
// reaches the service and nothing else, so "dvarpala-cli mail test" reported
// that no mail service was configured on a server that was sending mail
// perfectly well - the one command whose whole purpose is to check that.
const secretsFile = "/etc/dvarpala/dvarpala.env"

// loadSecrets reads the file into the environment, without overriding anything
// already set. Missing or unreadable is not an error: the file is root-owned,
// and plenty of commands here need no credentials at all.
func loadSecrets() {
	f, err := os.Open(secretsFile)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		// systemd accepts quoted values, so accept them here too rather than
		// setting a key whose value begins and ends with a quotation mark.
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		// An explicit environment variable wins, so a one-off can still be
		// tried without editing the file.
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
}
