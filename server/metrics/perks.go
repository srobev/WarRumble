package metrics

// Stub implementations for perks metrics - wire to Prometheus when available
// TODO: Replace with github.com/prometheus/client_golang/prometheus when wired

import (
	"log"
)

// Stub implementations that log metrics until Prometheus is wired
type StubCounterVec struct{ name string }
type StubGaugeVec struct{ name string }

type StubInc struct{}
type StubSet struct{}

var PerkPurchases = StubCounterVec{name: "perk_purchases_total"}
var PerkActivations = StubCounterVec{name: "perk_activations_total"}
var GridRerollDenied = StubCounterVec{name: "grid_reroll_denied_total"}
var GridPerkOffers = StubGaugeVec{name: "grid_perk_offers"}

func (s StubCounterVec) WithLabelValues(values ...string) StubInc {
	log.Printf("METRIC %s: %v", s.name, values)
	return StubInc{}
}

func (s StubGaugeVec) WithLabelValues(values ...string) StubSet {
	log.Printf("METRIC %s set: %v", s.name, values)
	return StubSet{}
}

func (s StubInc) Inc() {}

func (s StubSet) Set(v float64) {}
