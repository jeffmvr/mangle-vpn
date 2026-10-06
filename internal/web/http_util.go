package web

import (
	"fmt"
	"net/url"
	"strings"
)

// contentDisposition builds an attachment header for a filename.
//
// The name is given twice: once stripped to ASCII for older clients, and
// once percent-encoded so that clients which understand RFC 5987 get the
// original.
func contentDisposition(filename string) string {
	var ascii strings.Builder
	for _, r := range filename {
		switch {
		case r == '"' || r == '\\' || r < 0x20:
			ascii.WriteByte('_')
		case r < 128:
			ascii.WriteRune(r)
		default:
			ascii.WriteByte('_')
		}
	}

	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		ascii.String(), url.PathEscape(filename))
}
