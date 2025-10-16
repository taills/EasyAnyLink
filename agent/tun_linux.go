//go:build linux

package agent

import (
	"fmt"
	"os/exec"

	"github.com/songgao/water"
)

// TUNInterface represents a TUN interface
type TUNInterface struct {
	iface *water.Interface
	name  string
	mtu   int
}

// NewTUNInterface creates a new TUN interface
func NewTUNInterface(name string, mtu int) (*TUNInterface, error) {
	config := water.Config{
		DeviceType: water.TUN,
	}

	if name != "" {
		config.Name = name
	}

	iface, err := water.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create TUN interface: %w", err)
	}

	tun := &TUNInterface{
		iface: iface,
		name:  iface.Name(),
		mtu:   mtu,
	}

	return tun, nil
}

// SetIP sets the IP address of the TUN interface
func (t *TUNInterface) SetIP(ip, netmask string) error {
	// Calculate CIDR from netmask
	cidr := netmaskToCIDR(netmask)

	// ip addr add 10.200.0.10/16 dev tun0
	cmd := exec.Command("ip", "addr", "add", fmt.Sprintf("%s/%d", ip, cidr), "dev", t.name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set IP: %w", err)
	}

	return nil
}

// SetMTU sets the MTU of the TUN interface
func (t *TUNInterface) SetMTU(mtu int) error {
	cmd := exec.Command("ip", "link", "set", "dev", t.name, "mtu", fmt.Sprintf("%d", mtu))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set MTU: %w", err)
	}

	t.mtu = mtu
	return nil
}

// Up brings the interface up
func (t *TUNInterface) Up() error {
	cmd := exec.Command("ip", "link", "set", "dev", t.name, "up")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to bring interface up: %w", err)
	}

	return nil
}

// Down brings the interface down
func (t *TUNInterface) Down() error {
	cmd := exec.Command("ip", "link", "set", "dev", t.name, "down")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to bring interface down: %w", err)
	}

	return nil
}

// Read reads a packet from the TUN interface
func (t *TUNInterface) Read(buf []byte) (int, error) {
	return t.iface.Read(buf)
}

// Write writes a packet to the TUN interface
func (t *TUNInterface) Write(buf []byte) (int, error) {
	return t.iface.Write(buf)
}

// Close closes the TUN interface
func (t *TUNInterface) Close() error {
	return t.iface.Close()
}

// Name returns the interface name
func (t *TUNInterface) Name() string {
	return t.name
}

// MTU returns the interface MTU
func (t *TUNInterface) MTU() int {
	return t.mtu
}

// netmaskToCIDR converts a netmask to CIDR notation
func netmaskToCIDR(netmask string) int {
	masks := map[string]int{
		"255.255.255.255": 32,
		"255.255.255.254": 31,
		"255.255.255.252": 30,
		"255.255.255.248": 29,
		"255.255.255.240": 28,
		"255.255.255.224": 27,
		"255.255.255.192": 26,
		"255.255.255.128": 25,
		"255.255.255.0":   24,
		"255.255.254.0":   23,
		"255.255.252.0":   22,
		"255.255.248.0":   21,
		"255.255.240.0":   20,
		"255.255.224.0":   19,
		"255.255.192.0":   18,
		"255.255.128.0":   17,
		"255.255.0.0":     16,
		"255.254.0.0":     15,
		"255.252.0.0":     14,
		"255.248.0.0":     13,
		"255.240.0.0":     12,
		"255.224.0.0":     11,
		"255.192.0.0":     10,
		"255.128.0.0":     9,
		"255.0.0.0":       8,
	}

	if cidr, ok := masks[netmask]; ok {
		return cidr
	}
	return 24 // Default
}
