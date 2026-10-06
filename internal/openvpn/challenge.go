package openvpn

import (
	"encoding/base64"
	"strings"
)

// ParseStaticChallenge reads the password a client sends when its profile
// carries static-challenge: "SCRV1:<base64 password>:<base64 response>".
// ok is false for a password in any other form, which is what a profile
// without static-challenge sends.
func ParseStaticChallenge(value string) (password, response string, ok bool) {
	rest, found := strings.CutPrefix(value, "SCRV1:")
	if !found {
		return "", "", false
	}
	encodedPassword, encodedResponse, found := strings.Cut(rest, ":")
	if !found {
		return "", "", false
	}

	decodedPassword, err := base64.StdEncoding.DecodeString(encodedPassword)
	if err != nil {
		return "", "", false
	}
	decodedResponse, err := base64.StdEncoding.DecodeString(encodedResponse)
	if err != nil {
		return "", "", false
	}
	return string(decodedPassword), string(decodedResponse), true
}
