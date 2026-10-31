// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

func newSettingsVM(t *testing.T, values map[string]any) (*vm.Settings, *vmtest.Backend, *vm.Worker) {
	t.Helper()
	b := &vmtest.Backend{Values: values}
	w := vmtest.SyncWorker()
	s := vm.NewSettings(b, w)
	s.Load()
	w.Wait()
	return s, b, w
}

func field(s *vm.Settings, key string) vm.SettingField {
	for _, g := range s.Groups() {
		for _, f := range s.Fields(g) {
			if f.Key.Name == key {
				return f
			}
		}
	}
	return vm.SettingField{}
}

func TestSettingsVM_groups_values_and_sources(t *testing.T) {
	s, _, _ := newSettingsVM(t, map[string]any{"timeout": "45s"})

	timeout := field(s, "timeout")
	theme := field(s, "appearance.theme")

	if !slices.Contains(s.Groups(), "appearance") || !slices.Contains(s.Groups(), "general") {
		t.Fatalf("groups %v", s.Groups())
	}
	if timeout.Value != "45s" || timeout.Source != "user (settings.yaml)" || !timeout.UserSet || theme.Value != "system" || theme.UserSet {
		t.Fatalf("timeout %+v theme %+v", timeout, theme)
	}
}

func TestSettingsVM_invalid_value_reported_not_saved(t *testing.T) {
	s, b, w := newSettingsVM(t, nil)

	s.Set("appearance.font_size", "200")
	w.Wait()

	if f := field(s, "appearance.font_size"); f.Problem == "" || b.Values["appearance.font_size"] != nil {
		t.Fatalf("field %+v values %v", f, b.Values)
	}
}

func TestSettingsVM_set_reset_and_appearance_notifies(t *testing.T) {
	s, b, w := newSettingsVM(t, nil)
	notified := 0
	s.OnAppearanceChanged(func() { notified++ })

	s.Set("appearance.theme", "dark")
	w.Wait()
	s.Reset("appearance.theme")
	w.Wait()
	s.Set("timeout", "10s")
	w.Wait()
	s.Set("ui.suggestions_max", "5")
	w.Wait()

	if notified != 3 || !b.Called("ResetUserSetting appearance.theme") || field(s, "timeout").Value != "10s" {
		t.Fatalf("notified %d calls %v", notified, b.Calls)
	}
}

func TestSettingsVM_lists_only_user_settable_keys(t *testing.T) {
	s, _, _ := newSettingsVM(t, nil)

	for _, g := range s.Groups() {
		for _, f := range s.Fields(g) {
			if f.Key.Name == "responses.save" {
				return // a user-settable key is listed
			}
		}
	}
	t.Fatal("responses.save not listed")
}

func TestSettingsVM_Set_should_refuse_colours_that_are_not_hex(t *testing.T) {
	b := &vmtest.Backend{}
	w := vmtest.SyncWorker()
	s := vm.NewSettings(b, w)
	s.Load()
	w.Wait()

	s.Set("appearance.url.host", "blue")
	s.Set("appearance.url.pair_colors", "#aabbcc, nope")
	s.Set("appearance.accent", "#12")
	badHost := fieldOf(s, "appearance.url.host").Problem
	badList := fieldOf(s, "appearance.url.pair_colors").Problem
	badAccent := fieldOf(s, "appearance.accent").Problem
	s.Set("appearance.url.host", "#89b4fa")
	s.Set("appearance.url.scheme", "") // empty is the default

	if !strings.Contains(badHost, "#rrggbb") || !strings.Contains(badList, "nope") || !strings.Contains(badAccent, "#rrggbb") {
		t.Fatalf("problems: %q / %q / %q", badHost, badList, badAccent)
	}
	if b.Values["appearance.url.host"] != "#89b4fa" || fieldOf(s, "appearance.url.host").Problem != "" || fieldOf(s, "appearance.url.scheme").Problem != "" {
		t.Fatalf("good values must be stored: %v", b.Values)
	}
	if _, bad := b.Values["appearance.accent"]; bad {
		t.Fatal("a refused colour must stay off disk")
	}
}

func TestSettingsVM_ResetGroup_should_remove_every_user_value_of_the_group(t *testing.T) {
	b := &vmtest.Backend{Values: map[string]any{"appearance.url.host": "#112233", "appearance.url.pairs": "single", "appearance.theme": "nord"}}
	w := vmtest.SyncWorker()
	s := vm.NewSettings(b, w)
	s.Load()
	w.Wait()

	s.ResetGroup("appearance.url")

	if _, ok := b.Values["appearance.url.host"]; ok || len(b.Values) != 1 || b.Values["appearance.theme"] != "nord" {
		t.Fatalf("values %v", b.Values)
	}
}

func TestSettingsVM_should_put_the_url_colours_in_their_own_group_and_keep_notes_over_reloads(t *testing.T) {
	b := &vmtest.Backend{}
	w := vmtest.SyncWorker()
	s := vm.NewSettings(b, w)
	s.Load()
	w.Wait()

	s.SetNote("appearance.palette_file", "the text colour is too close")
	s.Load()
	w.Wait()

	if len(s.Fields("appearance.url")) != 14 || fieldOf(s, "appearance.url.host").Group != "appearance.url" || fieldOf(s, "appearance.theme").Group != "appearance" {
		t.Fatalf("url group %d fields", len(s.Fields("appearance.url")))
	}
	if got := fieldOf(s, "appearance.palette_file").Problem; got != "the text colour is too close" {
		t.Fatalf("note %q", got)
	}
}

func fieldOf(s *vm.Settings, key string) vm.SettingField {
	for _, g := range s.Groups() {
		for _, f := range s.Fields(g) {
			if f.Key.Name == key {
				return f
			}
		}
	}
	return vm.SettingField{}
}

// Review finding 8: single colours are one colour, and stored trimmed.
func TestSettingsVM_Set_should_trim_colours_and_refuse_lists_for_single_colour_keys(t *testing.T) {
	b := &vmtest.Backend{}
	w := vmtest.SyncWorker()
	s := vm.NewSettings(b, w)
	s.Load()
	w.Wait()

	s.Set("appearance.url.host", "#111111,#222222")
	list := fieldOf(s, "appearance.url.host").Problem
	s.Set("appearance.url.host", "  #89b4fa ")

	if !strings.Contains(list, "one colour") || b.Values["appearance.url.host"] != "#89b4fa" {
		t.Fatalf("list problem %q stored %v", list, b.Values["appearance.url.host"])
	}
}

// Review finding 7: a failing reset said nothing.
func TestSettingsVM_ResetGroup_Given_a_failure_Then_reports_it_on_the_field(t *testing.T) {
	b := &vmtest.Backend{Values: map[string]any{"appearance.url.host": "#112233"}, ResetErr: errors.New("settings.yaml is not writable")}
	w := vmtest.SyncWorker()
	s := vm.NewSettings(b, w)
	s.Load()
	w.Wait()

	s.ResetGroup("appearance.url")

	if got := fieldOf(s, "appearance.url.host").Problem; !strings.Contains(got, "not writable") {
		t.Fatalf("problem %q", got)
	}
}
