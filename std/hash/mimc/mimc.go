// Copyright 2020-2024 Consensys Software Inc.
// Licensed under the Apache License, Version 2.0. See the LICENSE file for details.

package mimc

import (
	"errors"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/internal/utils"
)

// MiMC contains the params of the MiMC hash func and the curves on which it is implemented.
//
// NB! See the package documentation for length extension attack consideration.
type MiMC struct {
	params []big.Int           // slice containing constants for the encryption rounds
	id     ecc.ID              // id needed to know which encryption function to use
	h      frontend.Variable   // current vector in the Miyaguchi–Preneel scheme
	data   []frontend.Variable // state storage. data is updated when Write() is called. Sum sums the data.
	api    frontend.API        // underlying constraint system
}

// NewMiMC returns a MiMC instance that can be used in a gnark circuit. The
// out-circuit counterpart of this function is provided in [gnark-crypto].
//
// NB! See the package documentation for length extension attack consideration.
//
// [gnark-crypto]: https://pkg.go.dev/github.com/consensys/gnark-crypto/hash
func NewMiMC(api frontend.API) (MiMC, error) {
	// TODO @gbotrel use field
	if constructor, ok := newMimc[utils.FieldToCurve(api.Compiler().Field())]; ok {
		return constructor(api), nil
	}
	return MiMC{}, errors.New("unknown curve id")
}

// Write adds more data to the running hash.
func (h *MiMC) Write(data ...frontend.Variable) {
	h.data = append(h.data, data...)
}

// Reset resets the Hash to its initial state.
func (h *MiMC) Reset() {
	h.data = nil
	h.h = 0
}

// Sum hash using [Miyaguchi–Preneel] where the XOR operation is replaced by
// field addition.
//
// [Miyaguchi–Preneel]: https://en.wikipedia.org/wiki/One-way_compression_function
func (h *MiMC) Sum() frontend.Variable {

	//h.Write(data...)s
	for _, stream := range h.data {
		r := encryptFuncs[h.id](*h, stream)
		h.h = h.api.Add(h.h, r, stream)
	}

	h.data = nil // flush the data already hashed

	return h.h

}
