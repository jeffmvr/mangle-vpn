package app

import (
	"fmt"
	"net/netip"
	"time"
	"unicode"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/validate"
)

// PasswordProblem says what is wrong with a new password under the
// password rules in the settings, or returns "" when it is acceptable.
func (a *App) PasswordProblem(password string) string {
	minimum := max(a.Config.Int(config.AuthPasswordMinLength, validate.MinPasswordLength), validate.MinPasswordLength)
	complex := a.Config.Bool(config.AuthPasswordComplexity, true)

	if len([]rune(password)) < minimum || (complex && !hasMixedCharacters(password)) {
		return a.PasswordRule()
	}
	return ""
}

// PasswordRule describes the password rules, for the pages that ask for a
// new password.
func (a *App) PasswordRule() string {
	minimum := max(a.Config.Int(config.AuthPasswordMinLength, validate.MinPasswordLength), validate.MinPasswordLength)
	if a.Config.Bool(config.AuthPasswordComplexity, true) {
		return fmt.Sprintf("At least %d characters, with an uppercase letter, a lowercase letter and a digit.", minimum)
	}
	return fmt.Sprintf("At least %d characters. A few words strung together make a good one.", minimum)
}

// hasMixedCharacters reports whether a password has a lowercase letter, an
// uppercase letter and a digit.
func hasMixedCharacters(password string) bool {
	var lower, upper, digit bool
	for _, r := range password {
		lower = lower || unicode.IsLower(r)
		upper = upper || unicode.IsUpper(r)
		digit = digit || unicode.IsDigit(r)
	}
	return lower && upper && digit
}

// SessionTimeouts returns how long a web session may sit idle, and how long
// it may last in all.
func (a *App) SessionTimeouts() (idle, lifetime time.Duration) {
	return time.Duration(a.Config.Int(config.AuthSessionIdleMinutes, 15)) * time.Minute,
		time.Duration(a.Config.Int(config.AuthSessionHours, 12)) * time.Hour
}

// AdminNetworks returns the networks the administration pages may be used
// from, or none when they may be used from anywhere.
func (a *App) AdminNetworks() []netip.Prefix {
	var networks []netip.Prefix
	for _, line := range config.SplitLines(a.Config.Get(config.AuthAdminNetworks)) {
		if prefix, err := ParseNetwork(line); err == nil {
			networks = append(networks, prefix)
		}
	}
	return networks
}

// AdminAllowedFrom reports whether the administration pages may be used
// from the given address.
func (a *App) AdminAllowedFrom(address string) bool {
	networks := a.AdminNetworks()
	if len(networks) == 0 {
		return true
	}
	addr, err := netip.ParseAddr(address)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, network := range networks {
		if network.Contains(addr) {
			return true
		}
	}
	return false
}

// ParseNetwork reads a network in CIDR notation, or a single address as a
// network of one.
func ParseNetwork(value string) (netip.Prefix, error) {
	if addr, err := netip.ParseAddr(value); err == nil {
		return netip.PrefixFrom(addr, addr.BitLen()), nil
	}
	prefix, err := netip.ParsePrefix(value)
	if err != nil {
		return prefix, err
	}
	return prefix.Masked(), nil
}
