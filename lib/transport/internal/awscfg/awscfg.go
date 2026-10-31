// SPDX-License-Identifier: Apache-2.0

// Package awscfg builds the AWS configuration the SQS and SNS transports
// share: the provider's default credential lookup, or the explicit
// credentials of the request.
package awscfg

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/transport"
)

const (
	// defaultRegion stands in when a custom endpoint (an emulator) names no
	// region; real AWS endpoints always carry one.
	defaultRegion = "us-east-1"
	maxAttempts   = 2
)

// Load returns the configuration for the operation. Binding auth is default
// (the SDK's own lookup: environment, shared profile, roles; the credential
// bindings are never read) or explicit (access_key_id, secret_access_key and
// optionally session_token). Binding region and profile apply as in the AWS
// CLI. The operation's TLS settings reach the HTTP client.
func Load(ctx context.Context, op transport.Operation) (aws.Config, error) {
	opts := []func(*config.LoadOptions) error{config.WithRetryMaxAttempts(maxAttempts)}
	if region := regionOf(op); region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	if op.TLS != nil {
		opts = append(opts, config.WithHTTPClient(awshttp.NewBuildableClient().WithTransportOptions(func(t *http.Transport) {
			t.TLSClientConfig = op.TLS.Clone()
		})))
	}
	authOpts, err := authOptions(op.Binding)
	if err != nil {
		return aws.Config{}, err
	}
	cfg, err := config.LoadDefaultConfig(ctx, append(opts, authOpts...)...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("loading the AWS configuration: %w", err)
	}
	return cfg, nil
}

func regionOf(op transport.Operation) string {
	if r := op.Binding["region"]; r != "" {
		return r
	}
	if op.Server != "" {
		return defaultRegion
	}
	return ""
}

func authOptions(binding map[string]string) ([]func(*config.LoadOptions) error, error) {
	switch mode := binding["auth"]; mode {
	case "", model.AuthDefault:
		if profile := binding["profile"]; profile != "" {
			return []func(*config.LoadOptions) error{config.WithSharedConfigProfile(profile)}, nil
		}
		return nil, nil
	case model.AuthExplicit:
		provider, err := staticCredentials(binding)
		if err != nil {
			return nil, err
		}
		return []func(*config.LoadOptions) error{config.WithCredentialsProvider(provider)}, nil
	default:
		return nil, fmt.Errorf("binding auth %q must be %s or %s", mode, model.AuthDefault, model.AuthExplicit)
	}
}

// staticCredentials builds the provider from the credential bindings; the
// errors name the field and never hold a value.
func staticCredentials(binding map[string]string) (aws.CredentialsProvider, error) {
	var missing []error
	for _, name := range []string{"access_key_id", "secret_access_key"} {
		if binding[name] == "" {
			missing = append(missing, fmt.Errorf("auth is explicit but binding %s is empty", name))
		}
	}
	if err := errors.Join(missing...); err != nil {
		return nil, err
	}
	return credentials.NewStaticCredentialsProvider(binding["access_key_id"], binding["secret_access_key"], binding["session_token"]), nil
}
