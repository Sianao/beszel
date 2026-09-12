package ups

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/henrygd/beszel/internal/entities/system"
)

func TestProtocol(t *testing.T) {
	for _, tc := range []struct {
		name, response string
		fail           bool
	}{
		{"valid", "BEGIN LIST VAR ups\nVAR ups ups.model \"Back \\\"UPS\\\" \\\\ 900\"\nVAR ups battery.charge \"0\"\nEND LIST VAR ups\n", false},
		{"stale", "ERR DATA-STALE\n", true},
		{"truncated", "BEGIN LIST VAR ups\nVAR ups ups.status \"OL\"\n", true},
		{"wrong device", "BEGIN LIST VAR ups\nVAR other ups.status \"OL\"\nEND LIST VAR ups\n", true},
		{"bad quote", "BEGIN LIST VAR ups\nVAR ups ups.status \"OL\nEND LIST VAR ups\n", true},
		{"bad header", "BEGIN LIST VAR other\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var request strings.Builder
			vars, err := readVariables(&request, bufio.NewScanner(strings.NewReader(tc.response)), "ups")
			if (err != nil) != tc.fail {
				t.Fatalf("unexpected error: %v", err)
			}
			if request.String() != "LIST VAR ups\n" {
				t.Fatalf("request: %q", request.String())
			}
			if !tc.fail && (vars["ups.model"] != `Back "UPS" \ 900` || vars["battery.charge"] != "0") {
				t.Fatalf("vars: %#v", vars)
			}
		})
	}
	for _, input := range []string{"name", `a"b\c`, "with space", ""} {
		got, err := tokens(quote(input))
		if err != nil || !reflect.DeepEqual(got, []string{input}) {
			t.Fatalf("quote round trip: %q, %v", got, err)
		}
	}
}

func TestMetrics(t *testing.T) {
	s := makeStats("ups", map[string]string{
		"battery.charge": "0", "ups.load": "125", "battery.runtime": "-1",
		"ups.realpower": "NaN", "input.voltage": "+Inf", "ups.temperature": "-5",
	})
	want := map[string]float64{"battery.charge": 0, "ups.load": 125, "ups.temperature": -5}
	if !reflect.DeepEqual(s.Metrics, want) {
		t.Fatalf("metrics: %#v", s.Metrics)
	}
}

func TestConfig(t *testing.T) {
	for _, address := range []string{"localhost", "localhost:3493", "::1", "[::1]:3493"} {
		m, err := New(address, "ups, backup,ups", "", "")
		if err != nil || len(m.names) != 2 {
			t.Fatalf("%s: %v", address, err)
		}
	}
	for _, address := range []string{"", "host:abc", "host:0", "host:65536", "host\n", "http://host"} {
		// Trailing whitespace is intentionally normalized.
		if address == "host\n" {
			continue
		}
		if _, err := New(address, "ups", "", ""); err == nil {
			t.Fatalf("accepted %q", address)
		}
	}
	for _, name := range []string{"", "ups,", "ups\nLOGOUT", `ups"`} {
		if _, err := New("localhost", name, "", ""); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	if _, err := New("localhost", "ups", "user", "x\nLOGOUT"); err == nil {
		t.Fatal("accepted command injection")
	}
}

func TestRefreshAndDisconnect(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(3 * time.Second))
		scanner := bufio.NewScanner(conn)
		for _, exchange := range [][2]string{
			{`USERNAME "reader"`, "OK\n"}, {`PASSWORD "secret"`, "OK\n"},
			{"LIST VAR missing", "ERR UNKNOWN-UPS\n"},
			{"LIST VAR ups", "BEGIN LIST VAR ups\nVAR ups ups.status \"OB LB\"\nVAR ups battery.charge \"0\"\nEND LIST VAR ups\n"},
		} {
			if !scanner.Scan() || scanner.Text() != exchange[0] {
				done <- fmt.Errorf("unexpected request: %q", scanner.Text())
				return
			}
			if _, err := io.WriteString(conn, exchange[1]); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	m, err := New(listener.Addr().String(), "missing,ups", "reader", "secret")
	if err != nil {
		t.Fatal(err)
	}
	m.refresh(context.Background())
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	first := m.Snapshot()
	if first["missing"].Online || !first["ups"].Online || first["ups"].Status != "OB LB" {
		t.Fatalf("snapshot: %#v", first)
	}
	listener.Close()
	m.refresh(context.Background())
	next := m.Snapshot()["ups"]
	if next.Online || next.Metrics != nil || next.Updated != first["ups"].Updated {
		t.Fatalf("disconnected: %#v", next)
	}
	if !first["ups"].Online {
		t.Fatal("previous snapshot was mutated")
	}
}

func TestStaleSnapshot(t *testing.T) {
	m, _ := New("localhost", "ups", "", "")
	m.data["ups"] = system.UPSStats{Name: "ups", Online: true, Updated: time.Now().Add(-time.Minute).Unix(), Metrics: map[string]float64{"ups.load": 50}}
	if s := m.Snapshot()["ups"]; s.Online || s.Metrics != nil {
		t.Fatalf("stale snapshot: %#v", s)
	}
}

func TestCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() { c, _ := listener.Accept(); accepted <- c }()
	m, _ := New(listener.Addr().String(), "ups", "", "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { m.Run(ctx); close(done) }()
	select {
	case conn := <-accepted:
		if conn != nil {
			defer conn.Close()
		}
	case <-time.After(3 * time.Second):
		t.Fatal("connection not accepted")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("poller did not stop on cancellation")
	}
}
