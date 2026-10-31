// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"

	"github.com/githugt/flowi/lib/model"
)

// An async request's credentials are entries of its binding list: the
// Authentication section edits the credential entries and the `auth` mode,
// the Binding section the rest. Both write the same list, so the file format
// has no separate place for credentials.

// authName is the binding entry that holds the credentials mode of a cloud
// protocol (default or explicit).
const authName = "auth"

func isCredential(a model.Async, name string) bool {
	return slices.Contains(model.AsyncCredentials(a.Protocol), name)
}

// optionsOf are the binding entries that are neither credentials nor the mode.
func optionsOf(a model.Async) []model.KeyValue {
	var out []model.KeyValue
	for _, kv := range a.Binding {
		if kv.Key != authName && !isCredential(a, kv.Key) {
			out = append(out, kv)
		}
	}
	return out
}

func credentialsOf(a model.Async) []model.KeyValue {
	var out []model.KeyValue
	for _, kv := range a.Binding {
		if isCredential(a, kv.Key) {
			out = append(out, kv)
		}
	}
	return out
}

func authModeOf(a model.Async) string {
	for _, kv := range a.Binding {
		if kv.Key == authName && kv.Value != "" {
			return kv.Value
		}
	}
	return model.AuthDefault
}

// BindingOptions are the selected async version's binding entries that the
// Binding section shows: everything except credentials and the mode.
func (r *Request) BindingOptions() []model.KeyValue { return optionsOf(r.Async()) }

// SetBindingOptions replaces the options, leaving the credentials and the
// mode as they are.
func (r *Request) SetBindingOptions(options []model.KeyValue) {
	r.EditAsync(func(a *model.Async) {
		kept := slices.DeleteFunc(slices.Clone(a.Binding), func(kv model.KeyValue) bool {
			return kv.Key != authName && !isCredential(*a, kv.Key)
		})
		a.Binding = append(slices.Clone(options), kept...)
	})
}

// CredentialNames are the credentials the selected version's protocol takes,
// in the order the Authentication section lists them.
func (r *Request) CredentialNames() []string { return model.AsyncCredentials(r.Async().Protocol) }

// Credential is the value of a credential ("" when not set).
func (r *Request) Credential(name string) string {
	for _, kv := range r.Async().Binding {
		if kv.Key == name {
			return kv.Value
		}
	}
	return ""
}

// SetCredential writes a credential, or removes it when value is empty. A
// name the protocol does not take as a credential is ignored.
func (r *Request) SetCredential(name, value string) {
	r.EditAsync(func(a *model.Async) {
		if !isCredential(*a, name) {
			return
		}
		a.Binding = slices.DeleteFunc(a.Binding, func(kv model.KeyValue) bool { return kv.Key == name })
		if value != "" {
			a.Binding = append(a.Binding, model.KeyValue{Key: name, Value: value})
		}
	})
}

// AuthMode is how a cloud protocol signs in: default (the provider's own
// lookup) or explicit (the credentials of the request). Protocols without the
// mode report default.
func (r *Request) AuthMode() string { return authModeOf(r.Async()) }

// SetAuthMode stores the mode; the default is stored as no entry at all.
func (r *Request) SetAuthMode(mode string) {
	r.EditAsync(func(a *model.Async) {
		a.Binding = slices.DeleteFunc(a.Binding, func(kv model.KeyValue) bool { return kv.Key == authName })
		if mode != "" && mode != model.AuthDefault {
			a.Binding = append(a.Binding, model.KeyValue{Key: authName, Value: mode})
		}
	})
}

// CloudProtocol reports whether the selected version's protocol has the mode.
func (r *Request) CloudProtocol() bool { return model.CloudProtocol(r.Async().Protocol) }
