package web

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"github.com/jeffmvr/mangle-vpn/internal/config"
)

// The organization's logo, uploaded on the settings page. It is stored as a
// data URL in the settings and served from /logo, to the sign in page and
// the interface.

// maxLogoBytes is the largest logo accepted.
const maxLogoBytes = 256 << 10

// logoTypes are the image types a logo may be.
var logoTypes = []string{"image/png", "image/jpeg", "image/webp", "image/svg+xml"}

// decodeLogo reads a logo's data URL, returning its type and bytes.
func decodeLogo(dataURL string) (string, []byte, error) {
	header, encoded, ok := strings.Cut(strings.TrimPrefix(dataURL, "data:"), ",")
	contentType, isBase64 := strings.CutSuffix(header, ";base64")
	if !strings.HasPrefix(dataURL, "data:") || !ok || !isBase64 {
		return "", nil, errors.New("The logo could not be read. Upload a PNG, JPEG, WebP or SVG image.")
	}

	known := false
	for _, t := range logoTypes {
		known = known || t == contentType
	}
	if !known {
		return "", nil, errors.New("Upload a PNG, JPEG, WebP or SVG image.")
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	switch {
	case err != nil:
		return "", nil, errors.New("The logo could not be read. Upload a PNG, JPEG, WebP or SVG image.")
	case len(data) > maxLogoBytes:
		return "", nil, errors.New("The logo is too large. Keep it under 256 KB.")
	}
	return contentType, data, nil
}

// logoURL returns the address of the logo, which changes with the logo so
// a browser never shows an old one, or "" when there is none.
func (s *Server) logoURL() string {
	logo := s.app.Config.Get(config.AppLogo)
	if logo == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(logo))
	return "/logo?v=" + hex.EncodeToString(sum[:6])
}

// serveLogo sends the organization's logo. It needs no sign in, since the
// sign in page shows it.
func (s *Server) serveLogo(w http.ResponseWriter, r *http.Request) {
	contentType, data, err := decodeLogo(s.app.Config.Get(config.AppLogo))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// An SVG is a document that could carry script; opened on its own it is
	// kept from running any.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(data)
}
