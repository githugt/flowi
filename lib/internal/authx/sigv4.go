// SPDX-License-Identifier: Apache-2.0

package authx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"

	"github.com/githugt/flowi-lib/internal/httpx"
)

// sigV4 signs a request with AWS Signature Version 4, with the AWS SDK's own
// signer, for API Gateway, S3 and any other service that takes it.
type sigV4 struct {
	creds           aws.Credentials
	region, service string
	env             Env
}

func newSigV4(p params, env Env) (Signer, error) {
	if err := p.only(); err != nil {
		return nil, err
	}
	if err := p.need("access_key_id", "secret_access_key", "region", "service"); err != nil {
		return nil, err
	}
	return &sigV4{
		creds:  aws.Credentials{AccessKeyID: p.values["access_key_id"], SecretAccessKey: p.values["secret_access_key"], SessionToken: p.values["session_token"]},
		region: p.values["region"], service: p.values["service"], env: env,
	}, nil
}

func (s *sigV4) Apply(ctx context.Context, r *httpx.Request) ([]Applied, error) {
	body, err := bodyBytes(r)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	signable, err := http.NewRequestWithContext(ctx, r.Method, r.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("signing %q: %w", r.URL, err)
	}
	signable.Header = r.Header.Clone()
	signable.Header.Set("X-Amz-Content-Sha256", hash)
	if err := v4.NewSigner().SignHTTP(ctx, s.creds, signable, hash, s.service, s.region, s.env.now()); err != nil {
		return nil, fmt.Errorf("signing the request: %w", err)
	}
	var applied []Applied
	names := make([]string, 0, len(signable.Header))
	for name := range signable.Header {
		names = append(names, name)
	}
	slices.Sort(names) // a fixed order, so the record reads the same every time
	for _, name := range names {
		values := signable.Header[name]
		if old := r.Header.Values(name); len(old) == len(values) && strings.Join(old, ",") == strings.Join(values, ",") {
			continue // unchanged
		}
		r.Header[name] = values
		applied = append(applied, Applied{Where: "header", Name: name, Value: strings.Join(values, ","), Secret: name == "Authorization" || name == "X-Amz-Security-Token"})
	}
	return applied, nil
}
