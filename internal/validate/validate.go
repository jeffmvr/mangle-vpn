// Package validate holds the application's shared value validators.
package validate

import (
	"net/netip"
	"regexp"
	"strconv"
	"strings"
)

// domainRE matches a lowercase DNS name made of one or more hyphen-separated
// labels followed by an alphabetic top level domain.
var domainRE = regexp.MustCompile(`^([a-z0-9]+(-[a-z0-9]+)*\.)+[a-z]{2,}$`)

// emailLocalRE matches the unquoted local part of an e-mail address.
var emailLocalRE = regexp.MustCompile(`^[-!#$%&'*+/=?^_` + "`" + `{}|~0-9A-Za-z]+(\.[-!#$%&'*+/=?^_` + "`" + `{}|~0-9A-Za-z]+)*$`)

// emailDomainRE matches the domain part of an e-mail address.
var emailDomainRE = regexp.MustCompile(`^(?i)([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}\.?$`)

// IsIP reports whether value is an IP address.
//
// An IPv6 zone ("fe80::1%eth0") is refused: the zone may hold any text at
// all, newlines and spaces included, and these values are written into
// OpenVPN configuration and iptables arguments, where that text would become
// directives of its own.
func IsIP(value string) bool {
	addr, err := netip.ParseAddr(value)
	return err == nil && addr.Zone() == ""
}

// IsCIDR reports whether value is an IP network or a bare IP address. Host
// bits beyond the prefix length are tolerated.
func IsCIDR(value string) bool {
	if IsIP(value) {
		return true
	}
	_, err := netip.ParsePrefix(value)
	return err == nil
}

// IsPort reports whether value is a usable TCP or UDP port number.
func IsPort(value string) bool {
	port, err := strconv.Atoi(value)
	return err == nil && port > 1 && port < 65535
}

// IsDomain reports whether value is a lowercase DNS name.
func IsDomain(value string) bool {
	return domainRE.MatchString(value)
}

// IsHostname reports whether value is usable as a hostname: either a DNS name
// or a literal IP address.
func IsHostname(value string) bool {
	return IsDomain(value) || IsIP(value)
}

// IsEmail reports whether value is an e-mail address.
func IsEmail(value string) bool {
	local, domain, ok := strings.Cut(value, "@")
	if !ok || local == "" || len(value) > 320 {
		return false
	}
	if !emailLocalRE.MatchString(local) {
		return false
	}
	// A bracketed address literal stands in for a domain name.
	if lit, found := strings.CutPrefix(domain, "["); found {
		lit, found = strings.CutSuffix(lit, "]")
		return found && IsIP(strings.TrimPrefix(lit, "IPv6:"))
	}
	return emailDomainRE.MatchString(domain)
}

// MinPasswordLength is the shortest password the application accepts,
// whatever the settings ask for.
const MinPasswordLength = 8
