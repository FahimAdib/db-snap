package policy

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"

	"db-snap/internal/model"
)

var (
	ErrDeniedHost            = errors.New("host denied by policy")
	ErrHostNotLocalOrAllowed = errors.New("host is not localhost or allowed private network")
)

type Result struct {
	Allowed            bool
	RequiresWarningAck bool
	ResolvedIPs        []netip.Addr
	Reason             string
}

func ValidateTarget(host string, p model.SafetyPolicy) (Result, error) {
	h := strings.ToLower(strings.TrimSpace(host))
	for _, kw := range p.DenyHostKeywords {
		if kw == "" {
			continue
		}
		if strings.Contains(h, strings.ToLower(kw)) {
			return Result{}, fmt.Errorf("%w: host contains denied keyword %q", ErrDeniedHost, kw)
		}
	}

	ips, err := resolveHost(h)
	if err != nil {
		return Result{}, err
	}
	if len(ips) == 0 {
		return Result{}, fmt.Errorf("unable to resolve host: %s", host)
	}
	if h != "localhost" {
		if _, err := netip.ParseAddr(h); err != nil {
			if err := confirmForwardReverse(h, ips); err != nil {
				return Result{}, err
			}
		}
	}

	if allLoopback(ips) {
		return Result{Allowed: true, ResolvedIPs: ips, RequiresWarningAck: false, Reason: "loopback target"}, nil
	}

	allowNets, err := parseCIDRs(p.AllowCIDRs)
	if err != nil {
		return Result{}, err
	}
	for _, ip := range ips {
		if !ip.IsPrivate() {
			return Result{}, fmt.Errorf("%w: resolved ip %s is public", ErrHostNotLocalOrAllowed, ip.String())
		}
		if !inAllowList(ip, allowNets) {
			return Result{}, fmt.Errorf("%w: %s not in allowlist", ErrHostNotLocalOrAllowed, ip.String())
		}
	}

	return Result{Allowed: true, ResolvedIPs: ips, RequiresWarningAck: true, Reason: "private allowlisted target"}, nil
}

func resolveHost(host string) ([]netip.Addr, error) {
	if host == "localhost" {
		return []netip.Addr{netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("::1")}, nil
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{ip}, nil
	}
	raw, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("lookup host %s: %w", host, err)
	}
	out := make([]netip.Addr, 0, len(raw))
	for _, ip := range raw {
		if addr, ok := netip.AddrFromSlice(ip); ok {
			out = append(out, addr)
		}
	}
	return out, nil
}

func parseCIDRs(cidrs []string) ([]netip.Prefix, error) {
	out := make([]netip.Prefix, 0, len(cidrs))
	for _, c := range cidrs {
		if strings.TrimSpace(c) == "" {
			continue
		}
		p, err := netip.ParsePrefix(c)
		if err != nil {
			return nil, fmt.Errorf("parse cidr %s: %w", c, err)
		}
		out = append(out, p)
	}
	return out, nil
}

func allLoopback(ips []netip.Addr) bool {
	for _, ip := range ips {
		if !ip.IsLoopback() {
			return false
		}
	}
	return true
}

func inAllowList(ip netip.Addr, allow []netip.Prefix) bool {
	for _, p := range allow {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

func confirmForwardReverse(host string, ips []netip.Addr) error {
	for _, ip := range ips {
		names, err := net.LookupAddr(ip.String())
		if err != nil {
			continue
		}
		for _, name := range names {
			name = strings.TrimSuffix(name, ".")
			forward, err := net.LookupIP(name)
			if err != nil {
				continue
			}
			for _, fwd := range forward {
				if addr, ok := netip.AddrFromSlice(fwd); ok && addr == ip {
					return nil
				}
			}
		}
	}
	return fmt.Errorf("dns safety check failed for host %s", host)
}
