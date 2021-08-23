// Copyright 2020 ConsenSys Software Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by gnark DO NOT EDIT

package plonk_test

import (
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"

	curve "github.com/consensys/gnark-crypto/ecc/bn254"

	"github.com/consensys/gnark/internal/backend/bn254/cs"

	bn254witness "github.com/consensys/gnark/internal/backend/bn254/witness"

	bn254plonk "github.com/consensys/gnark/internal/backend/bn254/plonk"

	"bytes"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr/kzg"
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/backend/plonk"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/internal/backend/circuits"
)

func TestCircuits(t *testing.T) {
	for name, circuit := range circuits.Circuits {
		t.Run(name, func(t *testing.T) {
			assert := plonk.NewAssert(t)
			pcs, err := frontend.Compile(curve.ID, backend.PLONK, circuit.Circuit)
			assert.NoError(err)
			assert.ProverSucceeded(pcs, circuit.Good)
			assert.ProverFailed(pcs, circuit.Bad)
		})
	}
}

//--------------------//
//     benches		  //
//--------------------//

type refCircuit struct {
	nbConstraints int
	X             frontend.Variable
	Y             frontend.Variable `gnark:",public"`
}

func (circuit *refCircuit) Define(curveID ecc.ID, cs *frontend.ConstraintSystem) error {
	for i := 0; i < circuit.nbConstraints; i++ {
		circuit.X = cs.Mul(circuit.X, circuit.X)
	}
	cs.AssertIsEqual(circuit.X, circuit.Y)
	return nil
}

func referenceCircuit() (frontend.CompiledConstraintSystem, frontend.Circuit, *kzg.SRS) {
	const nbConstraints = 40000
	circuit := refCircuit{
		nbConstraints: nbConstraints,
	}
	ccs, err := frontend.Compile(curve.ID, backend.PLONK, &circuit)
	if err != nil {
		panic(err)
	}

	var good refCircuit
	good.X.Assign(2)

	// compute expected Y
	var expectedY fr.Element
	expectedY.SetUint64(2)

	for i := 0; i < nbConstraints; i++ {
		expectedY.Mul(&expectedY, &expectedY)
	}

	good.Y.Assign(expectedY)
	srs, err := kzg.NewSRS(ecc.NextPowerOfTwo(nbConstraints)+3, new(big.Int).SetUint64(42))
	if err != nil {
		panic(err)
	}

	return ccs, &good, srs
}

func TestReferenceCircuit(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	assert := plonk.NewAssert(t)
	ccs, witness, _ := referenceCircuit()
	assert.ProverSucceeded(ccs, witness)
}

func BenchmarkSetup(b *testing.B) {
	ccs, _, srs := referenceCircuit()

	b.ResetTimer()

	b.Run("setup", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _, _ = bn254plonk.Setup(ccs.(*cs.SparseR1CS), srs)
		}
	})
}

func BenchmarkProver(b *testing.B) {
	ccs, _solution, srs := referenceCircuit()
	fullWitness := bn254witness.Witness{}
	err := fullWitness.FromFullAssignment(_solution)
	if err != nil {
		b.Fatal(err)
	}

	pk, _, err := bn254plonk.Setup(ccs.(*cs.SparseR1CS), srs)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = bn254plonk.Prove(ccs.(*cs.SparseR1CS), pk, fullWitness)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVerifier(b *testing.B) {
	ccs, _solution, srs := referenceCircuit()
	fullWitness := bn254witness.Witness{}
	err := fullWitness.FromFullAssignment(_solution)
	if err != nil {
		b.Fatal(err)
	}
	publicWitness := bn254witness.Witness{}
	err = publicWitness.FromPublicAssignment(_solution)
	if err != nil {
		b.Fatal(err)
	}

	pk, vk, err := bn254plonk.Setup(ccs.(*cs.SparseR1CS), srs)
	if err != nil {
		b.Fatal(err)
	}

	proof, err := bn254plonk.Prove(ccs.(*cs.SparseR1CS), pk, fullWitness)
	if err != nil {
		panic(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bn254plonk.Verify(proof, vk, publicWitness)
	}
}

func BenchmarkSerialization(b *testing.B) {
	ccs, _solution, srs := referenceCircuit()
	fullWitness := bn254witness.Witness{}
	err := fullWitness.FromFullAssignment(_solution)
	if err != nil {
		b.Fatal(err)
	}

	pk, _, err := bn254plonk.Setup(ccs.(*cs.SparseR1CS), srs)
	if err != nil {
		b.Fatal(err)
	}

	proof, err := bn254plonk.Prove(ccs.(*cs.SparseR1CS), pk, fullWitness)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()

	// ---------------------------------------------------------------------------------------------
	// bn254plonk.ProvingKey binary serialization
	b.Run("pk: binary serialization (bn254plonk.ProvingKey)", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			_, _ = pk.WriteTo(&buf)
		}
	})
	b.Run("pk: binary deserialization (bn254plonk.ProvingKey)", func(b *testing.B) {
		var buf bytes.Buffer
		_, _ = pk.WriteTo(&buf)
		var pkReconstructed bn254plonk.ProvingKey
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := bytes.NewBuffer(buf.Bytes())
			_, _ = pkReconstructed.ReadFrom(buf)
		}
	})
	{
		var buf bytes.Buffer
		_, _ = pk.WriteTo(&buf)
	}

	// ---------------------------------------------------------------------------------------------
	// bn254plonk.Proof binary serialization
	b.Run("proof: binary serialization (bn254plonk.Proof)", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var buf bytes.Buffer
			_, _ = proof.WriteTo(&buf)
		}
	})
	b.Run("proof: binary deserialization (bn254plonk.Proof)", func(b *testing.B) {
		var buf bytes.Buffer
		_, _ = proof.WriteTo(&buf)
		var proofReconstructed bn254plonk.Proof
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := bytes.NewBuffer(buf.Bytes())
			_, _ = proofReconstructed.ReadFrom(buf)
		}
	})
	{
		var buf bytes.Buffer
		_, _ = proof.WriteTo(&buf)
	}

}
