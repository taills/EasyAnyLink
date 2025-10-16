//go:build darwin

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
	// ifconfig tun0 10.200.0.10 10.200.0.1 netmask 255.255.0.0
	cmd := exec.Command("ifconfig", t.name, ip, "netmask", netmask)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set IP: %w", err)
	}

	return nil
}

// SetMTU sets the MTU of the TUN interface
func (t *TUNInterface) SetMTU(mtu int) error {
	cmd := exec.Command("ifconfig", t.name, "mtu", fmt.Sprintf("%d", mtu))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set MTU: %w", err)
	}

	t.mtu = mtu
	return nil
}

// Up brings the interface up
func (t *TUNInterface) Up() error {
	cmd := exec.Command("ifconfig", t.name, "up")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to bring interface up: %w", err)
	}

	return nil
}

// Down brings the interface down
func (t *TUNInterface) Down() error {
	cmd := exec.Command("ifconfig", t.name, "down")
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
