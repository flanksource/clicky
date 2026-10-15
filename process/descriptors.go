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

// Environment is pid's whole environment, as the kernel reports it.
func Environment(ctx context.Context, pid int) (map[string]string, error) {
	entries, err := readEnvironment(ctx, pid)
	if err != nil {
		return nil, err
	}
	return filterEnvironment(entries, EnvironmentOptions{}), nil
}

// parseLsof reads lsof -F ftPnT output: one field per line, keyed by its first
// byte, where an f field starts the next descriptor.
func parseLsof(output []byte) []Descriptor {
	var descriptors []Descriptor
	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		field, value := line[0], line[1:]
		if field == 'f' {
			descriptors = append(descriptors, Descriptor{FD: value})
			continue
		}
		if len(descriptors) == 0 {
			continue
		}
		d := &descriptors[len(descriptors)-1]
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
	for i := range descriptors {
		setEndpoints(&descriptors[i])
	}
	return descriptors
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
