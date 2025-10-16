//go:build darwin

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
	// route add -net 10.100.0.0/16 -interface tun0
	// or
	// route add -net 10.100.0.0/16 10.200.0.1

	var cmd *exec.Cmd
	if iface != "" {
		cmd = exec.Command("route", "add", "-net", destination, "-interface", iface)
	} else {
		cmd = exec.Command("route", "add", "-net", destination, gateway)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add route: %w", err)
	}

	rm.routes = append(rm.routes, destination)
	return nil
}

// DeleteRoute removes a route from the routing table
func (rm *RouteManager) DeleteRoute(destination string) error {
	cmd := exec.Command("route", "delete", "-net", destination)
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
	var cmd *exec.Cmd
	if iface != "" {
		cmd = exec.Command("route", "add", "default", "-interface", iface)
	} else {
		cmd = exec.Command("route", "add", "default", gateway)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add default route: %w", err)
	}

	rm.routes = append(rm.routes, "default")
	return nil
}

// DeleteDefaultRoute removes the default route
func (rm *RouteManager) DeleteDefaultRoute() error {
	cmd := exec.Command("route", "delete", "default")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete default route: %w", err)
	}

	// Remove from tracked routes
	for i, route := range rm.routes {
		if route == "default" {
			rm.routes = append(rm.routes[:i], rm.routes[i+1:]...)
			break
		}
	}

	return nil
}

// Cleanup removes all installed routes
func (rm *RouteManager) Cleanup() error {
	for _, route := range rm.routes {
		var cmd *exec.Cmd
		if route == "default" {
			cmd = exec.Command("route", "delete", "default")
		} else {
			cmd = exec.Command("route", "delete", "-net", route)
		}

		if err := cmd.Run(); err != nil {
			// Log but don't fail - route might already be removed
			fmt.Printf("Warning: failed to delete route %s: %v\n", route, err)
		}
	}

	rm.routes = make([]string, 0)
	return nil
}
