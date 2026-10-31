// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func nightlySchedule() model.Schedule {
	return model.Schedule{Name: "nightly", Flow: "signup", Collection: "Shop", Cron: "0 3 * * *", Env: "local"}
}

func newSchedules(t *testing.T, b *vmtest.Backend) (*vm.Schedules, *vm.Worker) {
	t.Helper()
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	ws.Load()
	w.Wait()
	s := vm.NewSchedules(b, w, ws)
	s.Load()
	w.Wait()
	return s, w
}

func TestSchedules_Load_should_list_the_schedules_the_trigger_address_and_whether_a_token_exists(t *testing.T) {
	b := sampleBackend()
	b.ScheduleList = []model.Schedule{nightlySchedule()}
	b.TriggerSet = true
	s, _ := newSchedules(t, b)

	got := s.List()

	if len(got) != 1 || got[0].Name != "nightly" || s.TriggerAddr() != "127.0.0.1:8526" || !s.HasToken() {
		t.Fatalf("schedules %+v addr %q token %v", got, s.TriggerAddr(), s.HasToken())
	}
}

func TestSchedules_Choices_should_offer_every_flow_and_environment_of_the_collections(t *testing.T) {
	s, _ := newSchedules(t, sampleBackend())

	flows, envs := s.Flows(), s.Environments()

	if len(flows) == 0 || flows[0].Collection != "Shop" || flows[0].Path == "" || !slices.Contains(envs, "local") {
		t.Fatalf("flows %+v envs %v", flows, envs)
	}
}

func TestSchedules_Save_Given_no_old_name_Then_adds(t *testing.T) {
	b := sampleBackend()
	s, w := newSchedules(t, b)
	got := errors.New("not called")

	s.Save("", nightlySchedule(), func(err error) { got = err })
	w.Wait()

	if got != nil || !b.Called("AddSchedule nightly") || len(s.List()) != 1 {
		t.Fatalf("err %v schedules %+v", got, s.List())
	}
}

func TestSchedules_Save_Given_an_old_name_Then_replaces_that_schedule(t *testing.T) {
	b := sampleBackend()
	b.ScheduleList = []model.Schedule{nightlySchedule()}
	s, w := newSchedules(t, b)
	edited := nightlySchedule()
	edited.Name = "nightly-eu"

	s.Save("nightly", edited, func(error) {})
	w.Wait()

	if !b.Called("SaveSchedule nightly nightly-eu") || s.List()[0].Name != "nightly-eu" {
		t.Fatalf("schedules %+v", s.List())
	}
}

func TestSchedules_Save_Given_the_library_refuses_Then_reports_and_keeps_the_list(t *testing.T) {
	b := sampleBackend()
	b.ScheduleList = []model.Schedule{nightlySchedule()}
	b.ScheduleErr = errors.New("cron: bad")
	s, w := newSchedules(t, b)
	var got error

	s.Save("", model.Schedule{Name: "x"}, func(err error) { got = err })
	w.Wait()

	if got == nil || s.Err() == "" || len(s.List()) != 1 {
		t.Fatalf("err %v shown %q schedules %+v", got, s.Err(), s.List())
	}
}

func TestSchedules_Remove_should_delete_the_schedule(t *testing.T) {
	b := sampleBackend()
	b.ScheduleList = []model.Schedule{nightlySchedule()}
	s, w := newSchedules(t, b)

	s.Remove("nightly", func(error) {})
	w.Wait()

	if !b.Called("RemoveSchedule nightly") || len(s.List()) != 0 {
		t.Fatalf("schedules %+v", s.List())
	}
}

func TestSchedules_NewToken_should_hand_the_token_over_once_and_not_keep_it(t *testing.T) {
	b := sampleBackend()
	s, w := newSchedules(t, b)
	var token string
	var got error

	s.NewToken(func(tok string, err error) { token, got = tok, err })
	w.Wait()

	if got != nil || token != "tok-123" || !s.HasToken() {
		t.Fatalf("token %q err %v has %v", token, got, s.HasToken())
	}
}

func TestSchedules_NewToken_Given_the_vault_fails_Then_reports_it(t *testing.T) {
	b := sampleBackend()
	b.TokenErr = errors.New("keychain locked")
	s, w := newSchedules(t, b)
	var got error

	s.NewToken(func(_ string, err error) { got = err })
	w.Wait()

	if got == nil || s.HasToken() {
		t.Fatalf("err %v has %v", got, s.HasToken())
	}
}

func TestParseInputs_and_FormatInputs_should_round_trip_key_value_lines(t *testing.T) {
	in := []model.KeyValue{{Key: "email", Value: "a@b.c"}, {Key: "note", Value: "x=y"}}

	text := vm.FormatInputs(in)
	back, err := vm.ParseInputs(text + "\n\n")

	if err != nil || !slices.Equal(back, in) || text != "email=a@b.c\nnote=x=y" {
		t.Fatalf("text %q back %+v err %v", text, back, err)
	}
}

func TestParseInputs_Given_a_line_without_equals_Then_names_the_line(t *testing.T) {
	_, err := vm.ParseInputs("a=1\nbroken")

	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("err %v", err)
	}
}

func TestSchedules_MatchFlow_should_find_the_flow_as_the_library_does(t *testing.T) {
	b := sampleBackend()
	b.Cols[0].Flows = append(b.Cols[0].Flows, model.Flow{Meta: model.Meta{ID: "F2", Name: "Login"}, Path: "auth/login"}, model.Flow{Meta: model.Meta{ID: "F3", Name: "Login"}, Path: "login"})
	s, _ := newSchedules(t, b)
	flows := s.Flows()
	at := func(path string) int {
		for i, f := range flows {
			if f.Path == path {
				return i
			}
		}
		return -1
	}

	tests := map[string]struct {
		in   model.Schedule
		want int
	}{
		"exact path, not the suffix match": {model.Schedule{Flow: "login"}, at("login")},
		"by id":                            {model.Schedule{Flow: "F2"}, at("auth/login")},
		"flows/ form":                      {model.Schedule{Flow: "flows/auth/login.arazzo.yaml"}, at("auth/login")},
		"collection by name":               {model.Schedule{Flow: "signup", Collection: "Shop"}, at("signup")},
		"collection by id":                 {model.Schedule{Flow: "signup", Collection: string(b.Cols[0].Manifest.ID)}, at("signup")},
		"other collection":                 {model.Schedule{Flow: "signup", Collection: "Nope"}, -1},
		"unknown":                          {model.Schedule{Flow: "missing"}, -1},
	}
	for name, tc := range tests {
		if got := s.MatchFlow(tc.in); got != tc.want {
			t.Errorf("%s: got %d, want %d", name, got, tc.want)
		}
	}
}
