package process

import (
	"bufio"
	"bytes"
	"context"
	"strconv"
	"strings"
)

// Descriptor is one file a process holds open, as lsof reports it: a regular
// file, directory, device, pipe or socket. Internet sockets carry their
// protocol, TCP state and endpoints.
type Descriptor struct {
	FD         string `json:"fd"`
	Type       string `json:"type"`
	Name       string `json:"name"`
	Protocol   string `json:"protocol,omitempty"`
	State      string `json:"state,omitempty"`
	LocalAddr  string `json:"localAddr,omitempty"`
	RemoteAddr string `json:"remoteAddr,omitempty"`
	Port       int    `json:"port,omitempty"`
}

// IsSocket reports whether d is an internet or unix-domain socket.
func (d Descriptor) IsSocket() bool {
	switch d.Type {
	case "IPv4", "IPv6", "unix", "sock":
		return true
	}
	return false
}

// IsListening reports whether d accepts traffic on its port: a listening TCP
// socket, or a UDP socket bound without a peer.
func (d Descriptor) IsListening() bool {
	switch d.Protocol {
	case "TCP":
		return d.State == "LISTEN"
	case "UDP":
		return d.RemoteAddr == "" && d.Port != 0
	}
	return false
}

// IsConnected reports whether d is a TCP or UDP socket with a peer.
func (d Descriptor) IsConnected() bool {
	return (d.Protocol == "TCP" || d.Protocol == "UDP") && d.RemoteAddr != ""
}

// Environment is pid's environment as the kernel reports it, limited to the
// keys and prefixes options name; no keys and no prefixes is all of it.
func Environment(ctx context.Context, pid int, options EnvironmentOptions) (map[string]string, error) {
	entries, err := readEnvironment(ctx, pid)
	if err != nil {
		return nil, err
	}
	return filterEnvironment(entries, options), nil
}

// parseLsof reads lsof -F pftPnT output, by pid: one field per line, keyed by
// its first byte, where a p field starts the next process and an f field the
// next descriptor of it.
func parseLsof(output []byte) map[int][]Descriptor {
	byPID := map[int][]Descriptor{}
	pid := 0
	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		field, value := line[0], line[1:]
		switch {
		case field == 'p':
			pid, _ = strconv.Atoi(value)
			continue
		case pid == 0:
			continue
		case field == 'f':
			byPID[pid] = append(byPID[pid], Descriptor{FD: value})
			continue
		case len(byPID[pid]) == 0:
			continue
		}
		setField(&byPID[pid][len(byPID[pid])-1], field, value)
	}
	for _, descriptors := range byPID {
		for i := range descriptors {
			setEndpoints(&descriptors[i])
		}
	}
	return byPID
}

func setField(d *Descriptor, field byte, value string) {
	switch field {
	case 't':
		d.Type = value
	case 'P':
		d.Protocol = value
	case 'n':
		d.Name = value
	case 'T':
		if state, ok := strings.CutPrefix(value, "ST="); ok {
			d.State = state
		}
	}
}

func setEndpoints(d *Descriptor) {
	if d.Type != "IPv4" && d.Type != "IPv6" {
		return
	}
	d.LocalAddr, d.RemoteAddr, _ = strings.Cut(d.Name, "->")
	if i := strings.LastIndexByte(d.LocalAddr, ':'); i >= 0 {
		d.Port, _ = strconv.Atoi(d.LocalAddr[i+1:])
	}
}
