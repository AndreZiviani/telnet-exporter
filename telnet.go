package main

import (
	"bufio"
	"net"
	"strings"
	"time"
)

// Telnet command bytes
const (
	IAC  = 255
	DONT = 254
	DO   = 253
	WONT = 252
	WILL = 251

	ErrReadTimeout = "read timeout"
)

// handleTelnetNegotiation responds to basic Telnet option negotiation
func handleTelnetNegotiation(conn net.Conn, b byte, reader *bufio.Reader) error {
	option, err := reader.ReadByte()
	if err != nil {
		return err
	}

	// Respond with WONT or DONT to refuse all options
	var response []byte
	if b == DO {
		response = []byte{IAC, WONT, option}
	} else if b == WILL {
		response = []byte{IAC, DONT, option}
	} else {
		return nil
	}
	_, err = conn.Write(response)
	return err
}

// readUntil reads from conn until any stopWord is found or timeout
func readUntil(conn net.Conn, stopWords []string, timeout time.Duration) (string, error) {
	conn.SetReadDeadline(time.Now().Add(timeout))
	reader := bufio.NewReader(conn)
	var output strings.Builder

	for {
		b, err := reader.ReadByte()
		if err != nil {
			return output.String(), err
		}

		if b == IAC {
			cmd, err := reader.ReadByte()
			if err != nil {
				return output.String(), err
			}
			handleTelnetNegotiation(conn, cmd, reader)
			continue
		}

		output.WriteByte(b)
		line := output.String()
		for _, stop := range stopWords {
			if strings.Contains(line, stop) {
				return line, nil
			}
		}
	}
}

// sendCommand writes a command and waits for response
func sendCommand(conn net.Conn, cmd string, prompt string) (string, error) {
	_, err := conn.Write([]byte(cmd + "\n"))
	if err != nil {
		return "", err
	}

	time.Sleep(500 * time.Millisecond) // wait for output

	if prompt == "" {
		return "", nil
	}

	// Read until the prompt appears
	return readUntil(conn, []string{prompt}, 3*time.Second)
}
