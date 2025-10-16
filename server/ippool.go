package server

import (
	"fmt"
	"net"
	"sync"
)

// IPPool manages IP address allocation for the overlay network
type IPPool struct {
	cidr      *net.IPNet
	allocated map[string]net.IP // agentID -> IP
	available []net.IP
	mu        sync.RWMutex
}

// NewIPPool creates a new IP pool from CIDR notation
func NewIPPool(cidr string) (*IPPool, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR: %w", err)
	}

	pool := &IPPool{
		cidr:      ipNet,
		allocated: make(map[string]net.IP),
		available: make([]net.IP, 0),
	}

	// Generate available IPs from CIDR
	// Reserve .0 (network), .1 (gateway), and .255 (broadcast)
	ip := ipNet.IP.Mask(ipNet.Mask)
	for {
		ip = nextIP(ip)
		if !ipNet.Contains(ip) {
			break
		}

		// Skip network address, gateway, and broadcast
		if isReserved(ip, ipNet) {
			continue
		}

		pool.available = append(pool.available, copyIP(ip))
	}

	if len(pool.available) == 0 {
		return nil, fmt.Errorf("no available IPs in CIDR range")
	}

	return pool, nil
}

// Allocate assigns an IP address to an agent
func (p *IPPool) Allocate(agentID string) (net.IP, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if agent already has an IP
	if ip, exists := p.allocated[agentID]; exists {
		return ip, nil
	}

	// Assign next available
	if len(p.available) == 0 {
		return nil, fmt.Errorf("IP pool exhausted")
	}

	ip := p.available[0]
	p.available = p.available[1:]
	p.allocated[agentID] = ip

	return ip, nil
}

// Release frees an IP address
func (p *IPPool) Release(agentID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	ip, exists := p.allocated[agentID]
	if !exists {
		return fmt.Errorf("agent does not have allocated IP")
	}

	delete(p.allocated, agentID)
	p.available = append(p.available, ip)

	return nil
}

// GetAllocated returns the IP address allocated to an agent
func (p *IPPool) GetAllocated(agentID string) (net.IP, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ip, exists := p.allocated[agentID]
	if !exists {
		return nil, fmt.Errorf("agent does not have allocated IP")
	}

	return ip, nil
}

// IsAllocated checks if an IP is already allocated
func (p *IPPool) IsAllocated(ip net.IP) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, allocatedIP := range p.allocated {
		if allocatedIP.Equal(ip) {
			return true
		}
	}

	return false
}

// AllocateSpecific allocates a specific IP address
func (p *IPPool) AllocateSpecific(agentID string, ip net.IP) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if IP is in CIDR range
	if !p.cidr.Contains(ip) {
		return fmt.Errorf("IP not in CIDR range")
	}

	// Check if IP is reserved
	if isReserved(ip, p.cidr) {
		return fmt.Errorf("IP is reserved")
	}

	// Check if IP is already allocated
	for _, allocatedIP := range p.allocated {
		if allocatedIP.Equal(ip) {
			return fmt.Errorf("IP already allocated")
		}
	}

	// Remove from available list
	for i, availableIP := range p.available {
		if availableIP.Equal(ip) {
			p.available = append(p.available[:i], p.available[i+1:]...)
			break
		}
	}

	p.allocated[agentID] = ip
	return nil
}

// AvailableCount returns the number of available IPs
func (p *IPPool) AvailableCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.available)
}

// AllocatedCount returns the number of allocated IPs
func (p *IPPool) AllocatedCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.allocated)
}

// nextIP returns the next IP address
func nextIP(ip net.IP) net.IP {
	next := make(net.IP, len(ip))
	copy(next, ip)
	for i := len(next) - 1; i >= 0; i-- {
		next[i]++
		if next[i] > 0 {
			break
		}
	}
	return next
}

// copyIP creates a copy of an IP address
func copyIP(ip net.IP) net.IP {
	dup := make(net.IP, len(ip))
	copy(dup, ip)
	return dup
}

// isReserved checks if an IP is reserved (network, gateway, broadcast)
func isReserved(ip net.IP, ipNet *net.IPNet) bool {
	// Network address (first IP)
	networkIP := ipNet.IP.Mask(ipNet.Mask)
	if ip.Equal(networkIP) {
		return true
	}

	// Gateway (second IP, e.g., .0.1)
	gatewayIP := nextIP(networkIP)
	if ip.Equal(gatewayIP) {
		return true
	}

	// Broadcast address (last IP for IPv4)
	if ip.To4() != nil {
		ones, bits := ipNet.Mask.Size()
		if ones == 0 {
			return false
		}
		// Calculate broadcast address
		broadcast := make(net.IP, len(ip))
		copy(broadcast, ipNet.IP)
		for i := 0; i < len(broadcast); i++ {
			if i*8 < bits-ones {
				broadcast[i] |= ^ipNet.Mask[i]
			}
		}
		if ip.Equal(broadcast) {
			return true
		}
	}

	return false
}
