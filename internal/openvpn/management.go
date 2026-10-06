package openvpn

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// managementTimeout bounds every read and write on the management socket, so
// that an unresponsive server cannot wedge the caller.
const managementTimeout = 5 * time.Second

// Management is a connection to the OpenVPN server's management socket.
type Management struct {
	conn   net.Conn
	reader *bufio.Reader
}

// Connect opens the management socket at path and reads the server's
// greeting. The caller must close the result.
func Connect(ctx context.Context, path string) (*Management, error) {
	var dialer net.Dialer

	conn, err := dialer.DialContext(ctx, "unix", path)
	if err != nil {
		return nil, fmt.Errorf("openvpn: open management socket %s: %w", path, err)
	}

	m := &Management{conn: conn, reader: bufio.NewReader(conn)}

	// The server announces itself with a single ">INFO" line as soon as the
	// socket is open. Read past it so the first command sees its own reply.
	if _, err := m.readLine(); err != nil {
		m.Close()
		return nil, fmt.Errorf("openvpn: read management greeting: %w", err)
	}
	return m, nil
}

// Run sends a command and returns the server's reply.
func (m *Management) Run(args ...string) (string, error) {
	if err := m.send(strings.Join(args, " ")); err != nil {
		return "", err
	}
	return m.readReply()
}

// Close says goodbye and closes the connection.
func (m *Management) Close() error {
	// A failure to send "quit" is not worth reporting; the close below ends
	// the session either way.
	_ = m.send("quit")
	return m.conn.Close()
}

// send writes one command line to the socket.
func (m *Management) send(command string) error {
	if err := m.conn.SetWriteDeadline(time.Now().Add(managementTimeout)); err != nil {
		return err
	}
	if _, err := m.conn.Write([]byte(command + "\r\n")); err != nil {
		return fmt.Errorf("openvpn: send %q: %w", command, err)
	}
	return nil
}

// readLine reads one line from the socket, including its terminator.
func (m *Management) readLine() (string, error) {
	if err := m.conn.SetReadDeadline(time.Now().Add(managementTimeout)); err != nil {
		return "", err
	}

	line, err := m.reader.ReadString('\n')
	if err != nil {
		return line, fmt.Errorf("openvpn: read from management socket: %w", err)
	}
	return line, nil
}

// readReply reads lines until one of them completes a reply.
//
// The management protocol has no length prefix. Real-time notifications
// begin with ">" and may be interleaved with a reply at any point, so they
// are skipped; everything else is part of the answer. The application only
// issues single-line commands, whose replies end at the first "SUCCESS:" or
// "ERROR:" line, so any other line is taken as a complete reply rather than
// risking a wait for a terminator that never comes.
func (m *Management) readReply() (string, error) {
	var reply strings.Builder
	for {
		line, err := m.readLine()
		if err != nil {
			// Whatever arrived before the socket went quiet is still the
			// server's answer, so hand it back rather than discarding it.
			if reply.Len() > 0 && (errors.Is(err, net.ErrClosed) || isTimeout(err)) {
				return reply.String(), nil
			}
			return reply.String(), err
		}

		// Skip an asynchronous notification without treating it as the
		// answer to the command just sent.
		if strings.HasPrefix(line, ">") {
			continue
		}

		reply.WriteString(line)

		trimmed := strings.TrimRight(line, "\r\n")
		if !strings.HasPrefix(trimmed, "END") || trimmed == "END" {
			return reply.String(), nil
		}
	}
}

// isTimeout reports whether err is a deadline expiry.
func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// KillClient closes the OpenVPN connection identified by name, which may be
// a common name or an address and port pair.
func KillClient(ctx context.Context, socket, name string) error {
	m, err := Connect(ctx, socket)
	if err != nil {
		return err
	}
	defer m.Close()

	reply, err := m.Run("kill", name)
	if err != nil {
		return err
	}
	if strings.HasPrefix(reply, "ERROR:") {
		return fmt.Errorf("openvpn: kill %s: %s", name, strings.TrimSpace(reply))
	}
	return nil
}
