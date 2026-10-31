// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// Replaced answers with the contracts in By where it has one and with Base for everything else. It is how a
// changed API description is tried on the flows that use it before the change is imported: By holds what the
// new description says, and a contract with Removed set stands for a request version the description dropped.
type Replaced struct {
	Base Contracts
	By   map[string]Contract
}

// ContractKey is the key of a request version in Replaced.By: the request's path and the version label.
func ContractKey(requestPath, version string) string { return requestPath + "\x00" + version }

// Contract returns the replacement for the request version, or the base contract.
func (r Replaced) Contract(c model.Collection, req model.Request, version string) (Contract, diag.List) {
	if own, ok := r.By[ContractKey(req.Path, version)]; ok {
		return own, nil
	}
	return r.Base.Contract(c, req, version)
}
