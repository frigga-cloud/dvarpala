package vpn

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// ErrNoManagement means the VPN server is not listening for commands.
//
// Callers treat this as "the tunnel could not be closed early", not as a
// failure of whatever they were doing: deactivating somebody still takes
// effect at their next connection.
var ErrNoManagement = errors.New("openvpn management interface is unreachable")

// Management is a connection to OpenVPN's control channel.
//
// OpenVPN can listen on a local port where it accepts typed commands. It is
// the only way to act on a tunnel that is already up: everything else in
// Dvarpala decides at connection time and then cannot change its mind.
//
// The port grants complete control of the VPN to anything that can reach it,
// so it is bound to the loopback address and never exposed.
type Management struct {
	addr    string
	timeout time.Duration
}

// NewManagement creates a client for the address OpenVPN listens on.
func NewManagement(host string, port int) *Management {
	if host == "" {
		host = "127.0.0.1"
	}
	if port == 0 {
		port = 7505
	}
	return &Management{
		addr: net.JoinHostPort(host, strconv.Itoa(port)),
		// Short: this sits in the login path and on the disconnect hook, and
		// a hung control channel must not hold either of them open.
		timeout: 3 * time.Second,
	}
}

// ConnectedClient is one tunnel that is up right now.
//
// This is the live view. The database knows who may connect; only OpenVPN
// knows who actually is.
type ConnectedClient struct {
	CommonName     string
	RealAddress    string
	VirtualAddress string
	BytesReceived  int64
	BytesSent      int64
	ConnectedSince time.Time

	// ClientID identifies this connection to OpenVPN, and is what client-kill
	// takes. Unlike the common name it is unique per connection, so it can
	// address one tunnel when somebody has two.
	ClientID int
}

// Clients lists the tunnels currently established.
func (m *Management) Clients(ctx context.Context) ([]ConnectedClient, error) {
	// Format 3 is the tab-separated one. Formats 1 and 2 separate with commas,
	// which also appear inside the timestamps they emit.
	lines, err := m.command(ctx, "status 3", func(line string) bool {
		return line == "END"
	})
	if err != nil {
		return nil, err
	}

	var clients []ConnectedClient
	for _, line := range lines {
		f := strings.Split(line, "\t")
		if len(f) < 8 || f[0] != "CLIENT_LIST" {
			continue
		}

		// CLIENT_LIST, common name, real address, virtual address, virtual
		// IPv6, bytes received, bytes sent, connected since, ...
		c := ConnectedClient{
			CommonName:     f[1],
			RealAddress:    f[2],
			VirtualAddress: f[3],
		}
		c.BytesReceived, _ = strconv.ParseInt(f[5], 10, 64)
		c.BytesSent, _ = strconv.ParseInt(f[6], 10, 64)
		// Field 8 is the connection time as a Unix timestamp; field 7 is the
		// same moment written out for humans. Older servers stop before it.
		if len(f) > 8 {
			if secs, err := strconv.ParseInt(f[8], 10, 64); err == nil {
				c.ConnectedSince = time.Unix(secs, 0)
			}
		}
		// Field 9 is the username, 10 the client ID.
		if len(f) > 10 {
			c.ClientID, _ = strconv.Atoi(f[10])
		}
		clients = append(clients, c)
	}
	return clients, nil
}

// Kill closes the tunnels belonging to a common name, and reports how many
// it closed.
//
// Two things in Dvarpala want this. Deactivating somebody should end the
// session they are already holding, rather than only stopping the next one.
// And a client that has just signed in needs to reconnect before it can be
// given its routes - killing it makes that happen by itself, because every
// VPN client reconnects, instead of asking the person to do it by hand.
func (m *Management) Kill(ctx context.Context, commonName string) (int, error) {
	if strings.ContainsAny(commonName, "\r\n") {
		return 0, fmt.Errorf("invalid common name")
	}

	lines, err := m.command(ctx, "kill "+commonName, func(line string) bool {
		return strings.HasPrefix(line, "SUCCESS:") || strings.HasPrefix(line, "ERROR:")
	})
	if err != nil {
		return 0, err
	}
	if len(lines) == 0 {
		return 0, fmt.Errorf("no reply to kill")
	}

	reply := lines[len(lines)-1]
	if strings.HasPrefix(reply, "ERROR:") {
		// Not connected is the ordinary case, not a failure: most people being
		// deactivated are not holding a tunnel at that moment.
		return 0, nil
	}

	// "SUCCESS: common name 'sam@acme.com' found, 1 client(s) killed"
	for _, word := range strings.Fields(reply) {
		if n, err := strconv.Atoi(word); err == nil {
			return n, nil
		}
	}
	return 1, nil
}

// KillClient closes one connection by its id.
//
// Kill works by common name and closes every tunnel that name holds. This is
// for the case where that is wrong: ending a tunnel somebody left sitting
// unidentified must not also end the one they are working over.
//
// The message tells the client what to do about it. HALT means stop and stay
// stopped, which is what an abandoned tunnel deserves - RESTART would bring it
// straight back and the reap would achieve nothing.
func (m *Management) KillClient(ctx context.Context, clientID int, message string) error {
	if message == "" {
		message = "HALT"
	}
	if strings.ContainsAny(message, "\r\n") {
		return fmt.Errorf("invalid message")
	}

	lines, err := m.command(ctx, fmt.Sprintf("client-kill %d %s", clientID, message),
		func(line string) bool {
			return strings.HasPrefix(line, "SUCCESS:") || strings.HasPrefix(line, "ERROR:")
		})
	if err != nil {
		return err
	}
	if len(lines) > 0 && strings.HasPrefix(lines[len(lines)-1], "ERROR:") {
		// Already gone between listing and killing. Not a failure: the tunnel
		// is closed either way, which is what was wanted.
		return nil
	}
	return nil
}

// command sends one command and collects the reply.
//
// done reports whether a line ends the reply. OpenVPN also emits unsolicited
// lines beginning with ">" whenever something happens on the server; those are
// dropped, so a client connecting mid-command cannot corrupt the answer.
func (m *Management) command(ctx context.Context, cmd string, done func(string) bool) ([]string, error) {
	dialer := net.Dialer{Timeout: m.timeout}

	conn, err := dialer.DialContext(ctx, "tcp", m.addr)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoManagement, err)
	}
	defer conn.Close()

	deadline := time.Now().Add(m.timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)

	if _, err := fmt.Fprintf(conn, "%s\r\n", cmd); err != nil {
		return nil, fmt.Errorf("sending %q: %w", cmd, err)
	}

	var lines []string
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")

		if strings.HasPrefix(line, ">") {
			continue // asynchronous notice, not part of this reply
		}
		lines = append(lines, line)

		if done(line) {
			return lines, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading reply to %q: %w", cmd, err)
	}
	return lines, nil
}

// Reconnect asks a person's client to re-establish its tunnel, and reports how
// many it asked.
//
// This is how a completed login applies itself. Routes are chosen when a
// tunnel is established and cannot be changed afterwards, so somebody who
// signs in is still holding the walled-garden tunnel they arrived on. Telling
// their client to restart gets them a second connection, and that one is
// granted their routes.
//
// It differs from Kill in the one way that matters here: Kill removes the
// session and leaves the client to notice on its own, which takes until the
// keepalive expires - up to two minutes of appearing to be connected while
// reaching nothing. client-kill sends the client a RESTART, so it comes back
// immediately.
func (m *Management) Reconnect(ctx context.Context, commonName string) (int, error) {
	clients, err := m.Clients(ctx)
	if err != nil {
		return 0, err
	}

	asked := 0
	for _, c := range clients {
		if c.CommonName != commonName {
			continue
		}

		// RESTART is client-kill's default message, but say it explicitly:
		// the difference between reconnecting and being dropped is the whole
		// point of using this instead of Kill.
		_, err := m.command(ctx, fmt.Sprintf("client-kill %d RESTART", c.ClientID),
			func(line string) bool {
				return strings.HasPrefix(line, "SUCCESS:") || strings.HasPrefix(line, "ERROR:")
			})
		if err != nil {
			return asked, err
		}
		asked++
	}
	return asked, nil
}
