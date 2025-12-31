//go:build windows

package agent

import (
	"fmt"
	"os/exec"
)

// RouteManager manages routing table entries
type RouteManager struct {
	routes []string // Keep track of installed routes for cleanup
}

// NewRouteManager creates a new route manager
func NewRouteManager() *RouteManager {
	return &RouteManager{
		routes: make([]string, 0),
	}
}

// AddRoute adds a route to the routing table
func (rm *RouteManager) AddRoute(destination, gateway, iface string) error {
	// route add 10.100.0.0 mask 255.255.0.0 10.200.0.1
	// or with interface index
	// route add 10.100.0.0 mask 255.255.0.0 10.200.0.1 if <interface_index>

	var cmd *exec.Cmd
	if gateway != "" {
		cmd = exec.Command("route", "add", destination, gateway)
	} else {
		return fmt.Errorf("gateway is required for Windows routes")
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add route: %w", err)
	}

	rm.routes = append(rm.routes, destination)
	return nil
}

// DeleteRoute removes a route from the routing table
func (rm *RouteManager) DeleteRoute(destination string) error {
	cmd := exec.Command("route", "delete", destination)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete route: %w", err)
	}

	// Remove from tracked routes
	for i, route := range rm.routes {
		if route == destination {
			rm.routes = append(rm.routes[:i], rm.routes[i+1:]...)
			break
		}
	}

	return nil
}

// AddDefaultRoute adds a default route
func (rm *RouteManager) AddDefaultRoute(gateway, iface string) error {
	// route add 0.0.0.0 mask 0.0.0.0 <gateway>
	cmd := exec.Command("route", "add", "0.0.0.0", "mask", "0.0.0.0", gateway)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add default route: %w", err)
	}

	rm.routes = append(rm.routes, "0.0.0.0")
	return nil
}

// DeleteDefaultRoute removes the default route
func (rm *RouteManager) DeleteDefaultRoute() error {
	cmd := exec.Command("route", "delete", "0.0.0.0")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete default route: %w", err)
	}

	// Remove from tracked routes
	for i, route := range rm.routes {
		if route == "0.0.0.0" {
			rm.routes = append(rm.routes[:i], rm.routes[i+1:]...)
			break
		}
	}

	return nil
}

// Cleanup removes all installed routes
func (rm *RouteManager) Cleanup() error {
	var lastErr error

	// Delete routes in reverse order
	for i := len(rm.routes) - 1; i >= 0; i-- {
		route := rm.routes[i]
		var cmd *exec.Cmd
		if route == "0.0.0.0" {
			cmd = exec.Command("route", "delete", "0.0.0.0")
		} else {
			cmd = exec.Command("route", "delete", route)
		}

		if err := cmd.Run(); err != nil {
			lastErr = err
			// Continue trying to delete other routes
		}
	}

	rm.routes = make([]string, 0)
	return lastErr
}
