// SPDX-License-Identifier: Apache-2.0

package pubsub

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"google.golang.org/api/option"
)

// explicitCredentials builds the option for the credentials_json binding: the
// JSON itself, or the path to a file holding it. The credential type is
// checked (Google's guidance for credentials that come from outside the
// program) and the content is never put in an error.
func explicitCredentials(value string) (option.ClientOption, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("auth is explicit but binding credentials_json is empty")
	}
	data := []byte(value)
	if !strings.HasPrefix(value, "{") {
		var err error
		if data, err = os.ReadFile(value); err != nil {
			return nil, errors.New("binding credentials_json is neither JSON nor a readable file")
		}
	}
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, errors.New("binding credentials_json is not valid JSON")
	}
	switch head.Type {
	case "service_account":
		return option.WithAuthCredentialsJSON(option.ServiceAccount, data), nil
	case "authorized_user":
		return option.WithAuthCredentialsJSON(option.AuthorizedUser, data), nil
	}
	return nil, fmt.Errorf("binding credentials_json must be a service_account or authorized_user key")
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
