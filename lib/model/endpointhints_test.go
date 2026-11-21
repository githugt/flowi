// SPDX-License-Identifier: Apache-2.0

package model

import (
	"slices"
	"testing"
)

// Before: every protocol showed {{env.broker}} and "topic, subject, queue…".
func TestAsyncEndpointHint_Given_each_protocol_Then_it_names_its_own_channel(t *testing.T) {
	want := map[string]string{
		ProtocolWS: "Path", ProtocolSSE: "Event", ProtocolMQTT: "Topic", ProtocolKafka: "Topic", ProtocolAMQP: "Queue",
		ProtocolNATS: "Subject", ProtocolRedis: "Channel", ProtocolSQS: "Queue", ProtocolSNS: "Topic",
		ProtocolServiceBus: "Queue or topic", ProtocolPubSub: "Topic", ProtocolAMQP10: "Address",
	}
	for p, channel := range want {
		h := AsyncEndpointHint(p)
		if h.Channel != channel || h.Server == "" || h.ChannelHint == "" {
			t.Errorf("%s: %+v, want channel %q with a server and a hint", p, h, channel)
		}
	}
}

func TestAsyncEndpointHint_Given_every_listed_protocol_Then_none_falls_back_to_the_general_hint(t *testing.T) {
	general := AsyncEndpointHint("")
	for _, p := range AsyncProtocols() {
		if AsyncEndpointHint(p) == general {
			t.Errorf("%s has no hint of its own", p)
		}
	}
}

func TestAsyncEndpointHint_Given_an_unknown_protocol_Then_the_general_hint(t *testing.T) {
	if got := AsyncEndpointHint("x"); got.Channel != "Channel" || got.Server != "{{env.broker}}" {
		t.Fatalf("got %+v", got)
	}
}

func TestDatasetServerExample_Given_a_database_Then_it_writes_that_drivers_url(t *testing.T) {
	for _, d := range []string{DriverPostgres, DriverMySQL, DriverMariaDB} {
		if got := DatasetServerExample(d); !slices.Contains([]string{"postgres://user:password@host:5432/database", "mysql://user:password@host:3306/database", "mariadb://user:password@host:3306/database"}, got) {
			t.Errorf("%s: %q", d, got)
		}
	}
	if DatasetServerExample(DriverSQLite) != "{{env.database_url}}" {
		t.Error("sqlite has no URL example")
	}
}
