// Package ups reads NUT servers without requiring upsc or access to USB devices.
package ups

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/henrygd/beszel/internal/entities/system"
)

const interval = 10 * time.Second

type Manager struct {
	mu                          sync.Mutex
	address, username, password string
	names                       []string
	data                        map[string]system.UPSStats
}

// New requires explicit UPS names so a failed server is visible even before its first sample.
func New(address, names, username, password string) (*Manager, error) {
	address = strings.TrimSpace(address)
	if address == "" || strings.ContainsAny(address, " \t\r\n/") {
		return nil, errors.New("invalid NUT_HOST")
	}
	if _, _, err := net.SplitHostPort(address); err != nil {
		if strings.Contains(address, ":") && net.ParseIP(strings.Trim(address, "[]")) == nil {
			return nil, errors.New("NUT_HOST must be a hostname or host:port")
		}
		address = net.JoinHostPort(strings.Trim(address, "[]"), "3493")
	}
	host, port, _ := net.SplitHostPort(address)
	p, err := strconv.Atoi(port)
	if host == "" || err != nil || p < 1 || p > 65535 {
		return nil, errors.New("invalid NUT_HOST port or hostname")
	}
	if strings.ContainsAny(username+password, "\r\n\x00") {
		return nil, errors.New("invalid NUT credentials")
	}
	m := &Manager{address: address, username: username, password: password, data: make(map[string]system.UPSStats)}
	for _, name := range strings.Split(names, ",") {
		name = strings.TrimSpace(name)
		if name == "" || strings.ContainsAny(name, " \t\r\n\"\\\x00") || !utf8.ValidString(name) {
			return nil, errors.New("NUT_UPS must contain comma-separated UPS names")
		}
		if _, exists := m.data[name]; !exists {
			m.names = append(m.names, name)
			m.data[name] = system.UPSStats{Name: name}
		}
	}
	if len(m.names) > 32 {
		return nil, errors.New("at most 32 UPS devices are supported")
	}
	return m, nil
}

// Run polls independently of hub requests. The caller runs it once and owns its context.
func (m *Manager) Run(ctx context.Context) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		m.refresh(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Snapshot never waits for the network. Published maps are immutable.
func (m *Manager) Snapshot() map[string]system.UPSStats {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make(map[string]system.UPSStats, len(m.data))
	for name, value := range m.data {
		if time.Since(time.Unix(value.Updated, 0)) > 30*time.Second {
			value.Online = false
			value.Status = ""
			value.Metrics = nil
		}
		result[name] = value
	}
	return result
}

func (m *Manager) refresh(ctx context.Context) {
	data := make(map[string]system.UPSStats, len(m.names))
	m.mu.Lock()
	for name, old := range m.data {
		old.Online, old.Status, old.Metrics = false, "", nil
		data[name] = old
	}
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.data = data
		m.mu.Unlock()
	}()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	deadline, _ := ctx.Deadline()
	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", m.address)
	if err != nil {
		return
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	if conn.SetDeadline(deadline) != nil {
		return
	}
	scanner := bufio.NewScanner(io.LimitReader(conn, 1024*1024))
	for _, auth := range [][2]string{{"USERNAME", m.username}, {"PASSWORD", m.password}} {
		if auth[1] == "" {
			continue
		}
		if _, err := fmt.Fprintf(conn, "%s %s\n", auth[0], quote(auth[1])); err != nil {
			return
		}
		if !scanner.Scan() || scanner.Text() != "OK" {
			return
		}
	}
	for _, name := range m.names {
		vars, err := readVariables(conn, scanner, name)
		if err != nil {
			// A device-level ERR leaves the connection usable; malformed responses do not.
			if errors.Is(err, errDevice) {
				continue
			}
			return
		}
		data[name] = makeStats(name, vars)
	}
}

var errDevice = errors.New("NUT device unavailable")

func readVariables(w io.Writer, scanner *bufio.Scanner, name string) (map[string]string, error) {
	if _, err := fmt.Fprintf(w, "LIST VAR %s\n", name); err != nil {
		return nil, err
	}
	if !scanner.Scan() {
		return nil, io.ErrUnexpectedEOF
	}
	if strings.HasPrefix(scanner.Text(), "ERR ") {
		return nil, errDevice
	}
	if scanner.Text() != "BEGIN LIST VAR "+name {
		return nil, errors.New("invalid NUT list header")
	}
	vars := make(map[string]string)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "END LIST VAR "+name {
			return vars, nil
		}
		fields, err := tokens(line)
		if err != nil || len(fields) != 4 || fields[0] != "VAR" || fields[1] != name {
			return nil, errors.New("invalid NUT variable response")
		}
		vars[fields[2]] = fields[3]
	}
	return nil, io.ErrUnexpectedEOF
}

func quote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// NUT tokens only escape quotes and backslashes; Go string escapes are not equivalent.
func tokens(line string) ([]string, error) {
	var result []string
	for i := 0; i < len(line); {
		if line[i] == ' ' || line[i] == '\t' {
			i++
			continue
		}
		var token strings.Builder
		quoted := line[i] == '"'
		if quoted {
			i++
		}
		closed := !quoted
		for i < len(line) {
			c := line[i]
			i++
			if quoted && c == '"' {
				closed = true
				break
			}
			if !quoted && (c == ' ' || c == '\t') {
				break
			}
			if c == '\\' {
				if i == len(line) || (line[i] != '\\' && line[i] != '"') {
					return nil, errors.New("invalid escape")
				}
				c = line[i]
				i++
			}
			token.WriteByte(c)
		}
		if !closed || (quoted && i < len(line) && line[i] != ' ' && line[i] != '\t') {
			return nil, errors.New("invalid token")
		}
		result = append(result, strings.ToValidUTF8(token.String(), ""))
	}
	return result, nil
}

var metricKeys = []string{"battery.charge", "battery.runtime", "ups.load", "ups.realpower", "input.voltage", "output.voltage", "battery.voltage", "ups.temperature", "battery.temperature"}

func makeStats(name string, vars map[string]string) system.UPSStats {
	s := system.UPSStats{Name: name, Model: vars["ups.model"], Status: vars["ups.status"], Online: true, Updated: time.Now().Unix(), Metrics: make(map[string]float64)}
	for _, key := range metricKeys {
		value, err := strconv.ParseFloat(vars[key], 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		if !strings.HasSuffix(key, "temperature") && value < 0 {
			continue
		}
		if key == "battery.charge" && value > 100 {
			continue
		}
		s.Metrics[key] = value
	}
	return s
}
