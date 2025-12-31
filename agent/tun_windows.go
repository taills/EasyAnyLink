//go:build windows

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

	// Note: Windows version of water library may not support Name field
	// The interface name will be auto-generated

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
	// netsh interface ip set address name="tun0" static 10.200.0.10 255.255.0.0
	cmd := exec.Command("netsh", "interface", "ip", "set", "address",
		fmt.Sprintf("name=%s", t.name), "static", ip, netmask)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set IP: %w", err)
	}

	return nil
}

// SetMTU sets the MTU of the TUN interface
func (t *TUNInterface) SetMTU(mtu int) error {
	// netsh interface ipv4 set subinterface "tun0" mtu=1400
	cmd := exec.Command("netsh", "interface", "ipv4", "set", "subinterface",
		t.name, fmt.Sprintf("mtu=%d", mtu))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set MTU: %w", err)
	}

	t.mtu = mtu
	return nil
}

// Up brings the interface up
func (t *TUNInterface) Up() error {
	// netsh interface set interface name="tun0" admin=enabled
	cmd := exec.Command("netsh", "interface", "set", "interface",
		fmt.Sprintf("name=%s", t.name), "admin=enabled")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to bring interface up: %w", err)
	}

	return nil
}

// Down brings the interface down
func (t *TUNInterface) Down() error {
	// netsh interface set interface name="tun0" admin=disabled
	cmd := exec.Command("netsh", "interface", "set", "interface",
		fmt.Sprintf("name=%s", t.name), "admin=disabled")
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

// MTU returns the MTU
func (t *TUNInterface) MTU() int {
	return t.mtu
}
