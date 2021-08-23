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

package plonk

import (
	"crypto/sha256"
	"errors"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr"

	"github.com/consensys/gnark-crypto/ecc/bn254/fr/kzg"

	curve "github.com/consensys/gnark-crypto/ecc/bn254"

	bn254witness "github.com/consensys/gnark/internal/backend/bn254/witness"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/fiat-shamir"
)

var (
	errWrongClaimedQuotient = errors.New("claimed quotient is not as expected")
)

func Verify(proof *Proof, vk *VerifyingKey, publicWitness bn254witness.Witness) error {

	// pick a hash function to derive the challenge (the same as in the prover)
	hFunc := sha256.New()

	// transcript to derive the challenge
	fs := fiatshamir.NewTranscript(hFunc, "gamma", "alpha", "zeta")

	// derive gamma from Comm(l), Comm(r), Comm(o)
	gamma, err := deriveRandomness(&fs, "gamma", &proof.LRO[0], &proof.LRO[1], &proof.LRO[2])
	if err != nil {
		return err
	}

	// derive alpha from Comm(l), Comm(r), Comm(o), Com(Z)
	alpha, err := deriveRandomness(&fs, "alpha", &proof.Z)
	if err != nil {
		return err
	}

	// derive zeta, the point of evaluation
	zeta, err := deriveRandomness(&fs, "zeta", &proof.H[0], &proof.H[1], &proof.H[2])
	if err != nil {
		return err
	}

	// evaluation of Z=X**m-1 at zeta
	var zetaPowerM, zzeta fr.Element
	var bExpo big.Int
	one := fr.One()
	bExpo.SetUint64(vk.Size)
	zetaPowerM.Exp(zeta, &bExpo)
	zzeta.Sub(&zetaPowerM, &one)

	// ccompute PI = Sum_i<n L_i*w_i
	// TODO use batch inversion
	var pi, den, lagrangeOne, xiLi fr.Element
	lagrange := zzeta // zeta**m-1
	acc := fr.One()
	den.Sub(&zeta, &acc)
	lagrange.Div(&lagrange, &den).Mul(&lagrange, &vk.SizeInv) // 1/n*(zeta**n-1)/(zeta-1)
	lagrangeOne.Set(&lagrange)                                // save it for later
	for i := 0; i < len(publicWitness); i++ {

		xiLi.Mul(&lagrange, &publicWitness[i])
		pi.Add(&pi, &xiLi)

		// use L_i+1 = w*Li*(X-z**i)/(X-z**i+1)
		lagrange.Mul(&lagrange, &vk.Generator).
			Mul(&lagrange, &den)
		acc.Mul(&acc, &vk.Generator)
		den.Sub(&zeta, &acc)
		lagrange.Div(&lagrange, &den)
	}

	// linearizedpolynomial + pi(zeta) + (Z(u*zeta))*(a+s1+gamma)*(b+s2+gamma)*(c+gamma)*alpha - alpha**2*L1(zeta)
	var _s1, _s2, _o, alphaSquareLagrange fr.Element

	zu := proof.ZShiftedOpening.ClaimedValue

	claimedQuotient := proof.BatchedProof.ClaimedValues[0]
	linearizedPolynomialZeta := proof.BatchedProof.ClaimedValues[1]
	l := proof.BatchedProof.ClaimedValues[2]
	r := proof.BatchedProof.ClaimedValues[3]
	o := proof.BatchedProof.ClaimedValues[4]
	s1 := proof.BatchedProof.ClaimedValues[5]
	s2 := proof.BatchedProof.ClaimedValues[6]

	_s1.Add(&l, &s1).Add(&_s1, &gamma) // (a+s1+gamma)
	_s2.Add(&r, &s2).Add(&_s2, &gamma) // (b+s2+gamma)
	_o.Add(&o, &gamma)                 // (c+gamma)

	_s1.Mul(&_s1, &_s2).
		Mul(&_s1, &_o).
		Mul(&_s1, &alpha).
		Mul(&_s1, &zu) // alpha*Z(u*zeta)*(a+s1+gamma)*(b+s2+gamma)*(c+gamma)

	alphaSquareLagrange.Mul(&lagrangeOne, &alpha).
		Mul(&alphaSquareLagrange, &alpha) // alpha**2*L1(zeta)
	linearizedPolynomialZeta.Add(&linearizedPolynomialZeta, &pi). // linearizedpolynomial + pi(zeta)
									Add(&linearizedPolynomialZeta, &_s1).                // linearizedpolynomial+pi(zeta)+alpha*Z(u*zeta)*(a+s1+gamma)*(b+s2+gamma)*(c+gamma)
									Sub(&linearizedPolynomialZeta, &alphaSquareLagrange) // linearizedpolynomial+pi(zeta)+(Z(u*zeta))*(a+s1+gamma)*(b+s2+gamma)*(c+gamma)*alpha-alpha**2*L1(zeta)

	// Compute H(zeta) using the previous result: H(zeta) = prev_result/(zeta**n-1)
	var zetaPowerMMinusOne fr.Element
	zetaPowerMMinusOne.Sub(&zetaPowerM, &one)
	linearizedPolynomialZeta.Div(&linearizedPolynomialZeta, &zetaPowerMMinusOne)

	// check that H(zeta) is as claimed
	if !claimedQuotient.Equal(&linearizedPolynomialZeta) {
		return errWrongClaimedQuotient
	}

	// compute the folded commitment to H: Comm(h1) + zeta**m*Comm(h2) + zeta**2m*Comm(h3)
	mPlusTwo := big.NewInt(int64(vk.Size) + 2)
	var zetaMPlusTwo fr.Element
	zetaMPlusTwo.Exp(zeta, mPlusTwo)
	var zetaMPlusTwoBigInt big.Int
	zetaMPlusTwo.ToBigIntRegular(&zetaMPlusTwoBigInt)
	foldedH := proof.H[2]
	foldedH.ScalarMultiplication(&foldedH, &zetaMPlusTwoBigInt)
	foldedH.Add(&foldedH, &proof.H[1])
	foldedH.ScalarMultiplication(&foldedH, &zetaMPlusTwoBigInt)
	foldedH.Add(&foldedH, &proof.H[0])

	// Compute the commitment to the linearized polynomial
	// linearizedPolynomialDigest =
	// 		l*ql+r*qr+rl*qm+o*qo+qk +
	// 		alpha*( Z(uzeta)(a+s1+gamma)*(b+s2+gamma)*s3(X)-Z(X)(a+zeta+gamma)*(b+uzeta+gamma)*(c+u**2*zeta+gamma) ) +
	// 		alpha**2*L1(zeta)*Z
	// first part: individual constraints
	var rl fr.Element
	rl.Mul(&l, &r)

	var linearizedPolynomialDigest curve.G1Affine

	// second part: alpha*( Z(uzeta)(a+s1+gamma)*(b+s2+gamma)*s3(X)-Z(X)(a+zeta+gamma)*(b+uzeta+gamma)*(c+u**2*zeta+gamma) )
	var t fr.Element
	_s1.Add(&l, &s1).Add(&_s1, &gamma)
	t.Add(&r, &s2).Add(&t, &gamma)
	_s1.Mul(&_s1, &t).
		Mul(&_s1, &zu).
		Mul(&_s1, &alpha) // alpha*(Z(uzeta)(a+s1+gamma)*(b+s2+gamma))
	_s2.Add(&l, &zeta).Add(&_s2, &gamma)
	t.Mul(&zeta, &vk.Shifter[0]).Add(&t, &r).Add(&t, &gamma)
	_s2.Mul(&t, &_s2)
	t.Mul(&zeta, &vk.Shifter[1]).Add(&t, &o).Add(&t, &gamma)
	_s2.Mul(&t, &_s2).
		Mul(&_s2, &alpha) // alpha*(a+zeta+gamma)*(b+uzeta+gamma)*(c+u**2*zeta+gamma)
	_s2.Sub(&alphaSquareLagrange, &_s2)
	// note since third part =  alpha**2*L1(zeta)*Z
	// we add alphaSquareLagrange to _s2

	points := []curve.G1Affine{
		vk.Ql, vk.Qr, vk.Qm, vk.Qo, vk.Qk, // first part
		vk.S[2], proof.Z, // second & third part
	}

	scalars := []fr.Element{
		l, r, rl, o, one, // first part
		_s1, _s2, // second & third part
	}
	if _, err := linearizedPolynomialDigest.MultiExp(points, scalars, ecc.MultiExpConfig{ScalarsMont: true}); err != nil {
		return err
	}

	// Fold the first proof
	foldedProof, foldedDigest, err := kzg.FoldProof([]kzg.Digest{
		foldedH,
		linearizedPolynomialDigest,
		proof.LRO[0],
		proof.LRO[1],
		proof.LRO[2],
		vk.S[0],
		vk.S[1],
	},
		&proof.BatchedProof,
		hFunc,
	)
	if err != nil {
		return err
	}

	// Batch verify
	return kzg.BatchVerifyMultiPoints([]kzg.Digest{
		foldedDigest,
		proof.Z,
	},
		[]kzg.OpeningProof{
			foldedProof,
			proof.ZShiftedOpening,
		},
		vk.KZGSRS,
	)
}

func deriveRandomness(fs *fiatshamir.Transcript, challenge string, points ...*curve.G1Affine) (fr.Element, error) {

	var buf [curve.SizeOfG1AffineUncompressed]byte
	var r fr.Element

	for _, p := range points {
		buf = p.RawBytes()
		if err := fs.Bind(challenge, buf[:]); err != nil {
			return r, err
		}
	}

	b, err := fs.ComputeChallenge(challenge)
	if err != nil {
		return r, err
	}
	r.SetBytes(b)
	return r, nil
}
