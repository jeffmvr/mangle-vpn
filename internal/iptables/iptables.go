// Package iptables drives the iptables command line.
//
// Rule arguments may be passed either as individual tokens or as whole
// pre-formatted fragments such as "-j DROP"; both are split on whitespace and
// handed to iptables directly, never through a shell.
package iptables

import (
	"context"
	"log/slog"
	"os/exec"
	"strings"
)

// Filter is the iptables table the application builds its chains in.
const Filter = "filter"

// Run reports whether the iptables invocation succeeded.
func Run(ctx context.Context, args ...string) bool {
	_, ok := Output(ctx, args...)
	return ok
}

// Output runs iptables and returns its standard output along with whether
// the command succeeded.
func Output(ctx context.Context, args ...string) (string, bool) {
	flat := fields(args)

	// An empty leading argument means there is nothing to do. Rule sets are
	// stored as newline separated text and splitting them yields blank lines.
	if len(args) > 0 && strings.TrimSpace(args[0]) == "" {
		return "", true
	}
	if len(flat) == 0 {
		return "", true
	}

	cmd := exec.CommandContext(ctx, "iptables", append([]string{"--wait"}, flat...)...)

	out, err := cmd.Output()
	if err != nil {
		slog.Debug("iptables failed", "args", flat, "err", err)
		return string(out), false
	}
	return string(out), true
}

// CreateChain creates the named chain.
func CreateChain(ctx context.Context, table, chain string) bool {
	return Run(ctx, "-t", table, "-N", chain)
}

// DeleteChain flushes and then removes the named chain.
func DeleteChain(ctx context.Context, table, chain string) bool {
	Flush(ctx, table, chain)
	return Run(ctx, "-t", table, "-X", chain)
}

// Flush removes every rule from the named chain, or from the whole table when
// chain is empty.
func Flush(ctx context.Context, table, chain string) bool {
	return Run(ctx, "-t", table, "-F", chain)
}

// AppendRule adds the rule to the end of a chain.
func AppendRule(ctx context.Context, table, chain string, rule ...string) bool {
	return Run(ctx, append([]string{"-t", table, "-A", chain}, rule...)...)
}

// AppendUniqueRule adds the rule to the end of a chain unless it is already
// present.
func AppendUniqueRule(ctx context.Context, table, chain string, rule ...string) bool {
	if RuleExists(ctx, table, chain, rule...) {
		return true
	}
	return AppendRule(ctx, table, chain, rule...)
}

// DeleteRule removes the first matching rule from a chain.
func DeleteRule(ctx context.Context, table, chain string, rule ...string) bool {
	return Run(ctx, append([]string{"-t", table, "-D", chain}, rule...)...)
}

// RuleExists reports whether the rule is present in the chain.
func RuleExists(ctx context.Context, table, chain string, rule ...string) bool {
	return Run(ctx, append([]string{"-t", table, "-C", chain}, rule...)...)
}

// fields splits every argument on whitespace so that callers may pass either
// single tokens or whole rule fragments.
func fields(args []string) []string {
	var out []string
	for _, arg := range args {
		out = append(out, strings.Fields(arg)...)
	}
	return out
}
