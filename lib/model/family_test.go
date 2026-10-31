// SPDX-License-Identifier: Apache-2.0

package model_test

import (
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func TestRequestVersion_Family_should_name_the_family_and_its_exchange(t *testing.T) {
	cases := []struct {
		name     string
		version  model.RequestVersion
		family   model.Family
		exchange model.Exchange
	}{
		{"http", model.RequestVersion{Method: "GET"}, model.FamilyHTTP, model.SendReceive},
		{"nothing set is http", model.RequestVersion{}, model.FamilyHTTP, model.SendReceive},
		{"grpc", model.RequestVersion{GRPC: &model.GRPC{}}, model.FamilyRPC, model.SendReceive},
		{"publish", model.RequestVersion{Async: &model.Async{Send: &model.AsyncSend{}}}, model.FamilyMessaging, model.SendOnly},
		{"subscribe", model.RequestVersion{Async: &model.Async{Receive: &model.AsyncReceive{}}}, model.FamilyMessaging, model.ReceiveOnly},
		{"request and reply", model.RequestVersion{Async: &model.Async{Send: &model.AsyncSend{}, Receive: &model.AsyncReceive{}}}, model.FamilyMessaging, model.SendReceive},
		{"dataset", model.RequestVersion{Dataset: &model.Dataset{Driver: "sqlite"}}, model.FamilyDataset, model.SendReceive},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			family := c.version.Family()
			exchange := c.version.Exchange()

			if family != c.family || exchange != c.exchange {
				t.Fatalf("got %s/%s, want %s/%s", family, exchange, c.family, c.exchange)
			}
		})
	}
}

func TestExchange_should_say_which_directions_it_has(t *testing.T) {
	if !model.SendOnly.Sends() || model.SendOnly.Receives() {
		t.Error("send only sends and does not receive")
	}
	if model.ReceiveOnly.Sends() || !model.ReceiveOnly.Receives() {
		t.Error("receive only receives and does not send")
	}
	if !model.SendReceive.Sends() || !model.SendReceive.Receives() {
		t.Error("send and receive does both")
	}
}
