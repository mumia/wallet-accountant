package wiring

import (
	"github.com/mumia/wallet-accountant/internal/platform/eventstore"
	"github.com/mumia/wallet-accountant/internal/platform/forwarding"
)

// handlers lists every deferred event handler the worker runs. Each bounded
// context adds its Watermill handlers here.
func handlers() []forwarding.Handler {
	return nil
}

// rebuildTargets lists every aggregate whose folds the rebuild run mode
// rebuilds. Each bounded context adds its aggregates here.
func rebuildTargets() []eventstore.RebuildTarget {
	return nil
}

// ConsumerGroups names the consumer group of every deferred handler, for
// tests that drain pending events.
func ConsumerGroups() []string {
	hs := handlers()
	groups := make([]string, 0, len(hs))

	for _, h := range hs {
		groups = append(groups, h.Name)
	}

	return groups
}
