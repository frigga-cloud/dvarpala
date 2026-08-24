package vpn

import (
	"bufio"
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeOpenVPN stands in for the real management interface: it greets, reads
// one command, and replies with whatever the test supplies.
func fakeOpenVPN(t *testing.T, reply func(cmd string) string) *Management {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				// Real OpenVPN greets before anything is asked of it.
				_, _ = conn.Write([]byte(">INFO:OpenVPN Management Interface Version 5\r\n"))

				cmd, err := bufio.NewReader(conn).ReadString('\n')
				if err != nil {
					return
				}
				_, _ = conn.Write([]byte(reply(strings.TrimSpace(cmd))))
			}()
		}
	}()

	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	m := NewManagement(host, 0)
	m.addr = net.JoinHostPort(host, portStr)
	return m
}

func TestClientsParsesTheLiveList(t *testing.T) {
	m := fakeOpenVPN(t, func(string) string {
		return strings.Join([]string{
			"TITLE\tOpenVPN 2.5.5",
			"TIME\tThu Aug 21 10:00:00 2026\t1755770400",
			"HEADER\tCLIENT_LIST\tCommon Name\tReal Address\tVirtual Address",
			"CLIENT_LIST\tsam@acme.com\t81.2.3.4:51820\t172.30.100.2\t\t5491\t5327\tThu Aug 21 09:53:20 2026\t1755770000\tUNDEF\t0\t0",
			"CLIENT_LIST\tmaya@acme.com\t81.2.3.9:44100\t172.30.100.3\t\t100\t200\tThu Aug 21 09:55:00 2026\t1755770100\tUNDEF\t1\t1",
			"GLOBAL_STATS\tMax bcast\t0",
			"END",
			"",
		}, "\r\n")
	})

	clients, err := m.Clients(context.Background())
	if err != nil {
		t.Fatalf("Clients() error: %v", err)
	}
	if len(clients) != 2 {
		t.Fatalf("got %d clients, want 2", len(clients))
	}

	got := clients[0]
	if got.CommonName != "sam@acme.com" {
		t.Errorf("CommonName = %q, want sam@acme.com", got.CommonName)
	}
	if got.VirtualAddress != "172.30.100.2" {
		t.Errorf("VirtualAddress = %q, want 172.30.100.2", got.VirtualAddress)
	}
	if got.RealAddress != "81.2.3.4:51820" {
		t.Errorf("RealAddress = %q, want 81.2.3.4:51820", got.RealAddress)
	}
	if got.BytesReceived != 5491 || got.BytesSent != 5327 {
		t.Errorf("bytes = %d/%d, want 5491/5327", got.BytesReceived, got.BytesSent)
	}
	if got.ConnectedSince.Unix() != 1755770000 {
		t.Errorf("ConnectedSince = %v, want unix 1755770000", got.ConnectedSince)
	}
}

// Anything OpenVPN says spontaneously must not be mistaken for the reply.
func TestAsynchronousNoticesAreIgnored(t *testing.T) {
	m := fakeOpenVPN(t, func(string) string {
		return strings.Join([]string{
			">CLIENT:CONNECT,0,0",
			"CLIENT_LIST\tsam@acme.com\t81.2.3.4:51820\t172.30.100.2\t\t1\t2\tThu Aug 21\t1755770000\tUNDEF\t0\t0",
			">BYTECOUNT_CLI:0,1,2",
			"END",
			"",
		}, "\r\n")
	})

	clients, err := m.Clients(context.Background())
	if err != nil {
		t.Fatalf("Clients() error: %v", err)
	}
	if len(clients) != 1 {
		t.Fatalf("got %d clients, want 1", len(clients))
	}
}

func TestKillReportsHowManyItClosed(t *testing.T) {
	m := fakeOpenVPN(t, func(cmd string) string {
		if cmd != "kill sam@acme.com" {
			t.Errorf("sent %q, want %q", cmd, "kill sam@acme.com")
		}
		return "SUCCESS: common name 'sam@acme.com' found, 1 client(s) killed\r\n"
	})

	n, err := m.Kill(context.Background(), "sam@acme.com")
	if err != nil {
		t.Fatalf("Kill() error: %v", err)
	}
	if n != 1 {
		t.Errorf("Kill() = %d, want 1", n)
	}
}

// Somebody who is not connected is the ordinary case when deactivating an
// account, so it must not read as a failure.
func TestKillOfSomebodyNotConnectedIsNotAnError(t *testing.T) {
	m := fakeOpenVPN(t, func(string) string {
		return "ERROR: common name 'ghost@acme.com' not found\r\n"
	})

	n, err := m.Kill(context.Background(), "ghost@acme.com")
	if err != nil {
		t.Fatalf("Kill() error: %v", err)
	}
	if n != 0 {
		t.Errorf("Kill() = %d, want 0", n)
	}
}

// A newline in a common name could smuggle a second command onto the control
// channel, which grants total control of the VPN.
func TestKillRefusesAnInjectedCommand(t *testing.T) {
	m := NewManagement("127.0.0.1", 7505)

	if _, err := m.Kill(context.Background(), "sam@acme.com\r\nkill maya@acme.com"); err == nil {
		t.Fatal("Kill() accepted a common name containing a newline")
	}
}

// A server with no management interface must fail quickly and recognisably,
// so callers can carry on rather than abort what they were doing.
func TestUnreachableInterfaceIsReported(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close() // nothing is listening there now

	host, portStr, _ := net.SplitHostPort(addr)
	m := NewManagement(host, 0)
	m.addr = net.JoinHostPort(host, portStr)
	m.timeout = 500 * time.Millisecond

	start := time.Now()
	if _, err := m.Kill(context.Background(), "sam@acme.com"); !errors.Is(err, ErrNoManagement) {
		t.Fatalf("Kill() error = %v, want ErrNoManagement", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("took %v to give up, want well under 2s", elapsed)
	}
}
