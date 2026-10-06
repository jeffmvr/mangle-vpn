package openvpn

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// Traffic is how much data a connected client has moved, from the server's
// side: received from the client, and sent to it.
type Traffic struct {
	BytesReceived int64
	BytesSent     int64
}

// ReadStatus reads the traffic of each connected client, by common name,
// from the status file the server rewrites every few seconds in
// status-version 2 format. A missing or unreadable file reads as no
// clients: the server may simply not be running.
func ReadStatus(path string) map[string]Traffic {
	traffic := map[string]Traffic{}

	f, err := os.Open(path)
	if err != nil {
		return traffic
	}
	defer f.Close()

	// The HEADER line names the CLIENT_LIST columns, which have grown over
	// OpenVPN releases, so they are found by name.
	columns := map[string]int{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ",")
		switch {
		case len(fields) > 2 && fields[0] == "HEADER" && fields[1] == "CLIENT_LIST":
			for i, name := range fields[2:] {
				columns[name] = i + 1
			}
		case fields[0] == "CLIENT_LIST" && len(columns) > 0:
			name := column(fields, columns, "Common Name")
			if name == "" {
				continue
			}
			received, _ := strconv.ParseInt(column(fields, columns, "Bytes Received"), 10, 64)
			sent, _ := strconv.ParseInt(column(fields, columns, "Bytes Sent"), 10, 64)
			traffic[name] = Traffic{BytesReceived: received, BytesSent: sent}
		}
	}
	return traffic
}

// column returns the named field of a status row, or "".
func column(fields []string, columns map[string]int, name string) string {
	if i, ok := columns[name]; ok && i < len(fields) {
		return fields[i]
	}
	return ""
}
