package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type quickGatewayJSON struct {
	URL     string `json:"url"`
	Running bool   `json:"running"`
}

type quickInfoJSON struct {
	Route   string           `json:"route"`
	Label   string           `json:"label"`
	Routes  []quickRouteJSON `json:"routes"`
	Gateway quickGatewayJSON `json:"gateway"`
	Ready   bool             `json:"ready"`
	Reason  string           `json:"reason,omitempty"`
}

func quickInfo(ctx context.Context, cfg Config) quickInfoJSON {
	route := strings.ToLower(strings.TrimSpace(cfg.Quick.Route))
	if route == "" {
		route = "auto"
	}
	routes, routesErr := quickRoutes(ctx)
	info := quickInfoJSON{
		Route:   route,
		Label:   route,
		Routes:  routes,
		Gateway: quickGatewayJSON{URL: prowlGatewayBase()},
	}
	for _, candidate := range routes {
		if candidate.ID == route {
			info.Label = candidate.Label
			break
		}
	}
	routing, err := gatewayRouting(ctx)
	if err != nil {
		info.Reason = quickGatewayError(err).Error()
		return info
	}
	info.Gateway.Running = true
	info.Ready = routing.Routable
	if !routing.Routable {
		info.Reason = "Prowl has no provider connected; open Prowl > Providers in Rashin"
	} else if routesErr != nil {
		info.Reason = quickGatewayError(routesErr).Error()
	}
	return info
}

func cmdBackend(args []string) error {
	cfg := LoadConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if len(args) == 1 && (args[0] == "--json" || args[0] == "-j") {
		return json.NewEncoder(os.Stdout).Encode(quickInfo(ctx, cfg))
	}
	if len(args) == 0 {
		printBackend(ctx, cfg)
		return nil
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: ryoku-rashin backend [--json|<route>]")
	}
	if err := setQuickRoute(ctx, args[0]); err != nil {
		return err
	}
	fmt.Printf("fast lane route set to %s\n", strings.ToLower(strings.TrimSpace(args[0])))
	printBackend(ctx, LoadConfig())
	return nil
}

func printBackend(ctx context.Context, cfg Config) {
	info := quickInfo(ctx, cfg)
	fmt.Printf("fast lane: %s (%s)\n", info.Label, info.Route)
	fmt.Printf("  gateway: %s\n", info.Gateway.URL)
	if !info.Ready {
		fmt.Printf("  unavailable: %s\n", info.Reason)
	}
	ids := make([]string, 0, len(info.Routes))
	for _, route := range info.Routes {
		ids = append(ids, route.ID)
	}
	fmt.Printf("routes: %s\n", strings.Join(ids, ", "))
	fmt.Println("set with: ryoku-rashin backend <route>")
}
