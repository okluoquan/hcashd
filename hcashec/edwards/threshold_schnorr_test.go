// Copyright (c) 2017-2018 The Hcash developers @sammietocat
package edwards

import (
	"bytes"
	crand "crypto/rand"
	"encoding/hex"
	"math/big"
	"math/rand"
	"testing"
)

// Functions in test
// * TestStdSchnorrThresholdSig
// * TestStdSchnorrThresholdSigImpl
// * TestSchnorrThresholdSigOnBadPk
// * TestSchnorrThresholdSigOnBadSecNonce
// * TestSchnorrThresholdSigOnBadSk
// * TestSchnorrThresholdSigOnBadSecNonce

// TestStdSchnorrThresholdSig test Schnorr threshold signature
func TestStdSchnorrThresholdSig(t *testing.T) {
	const MAX_SIGNATORIES = 10
	const NUM_TEST = 5

	tRand := rand.New(rand.NewSource(543212345))

	curve := new(TwistedEdwardsCurve)
	curve.InitParam25519()

	msg, _ := hex.DecodeString(
		"d04b98f48e8f8bcc15c6ae5ac050801cd6dcfd428fb5f9e65c4e16e7807340fa")

	for i := 0; i < NUM_TEST; i++ {
		numKeysForTest := tRand.Intn(MAX_SIGNATORIES-2) + 2

		schnorrKeyVec := mockUpSchnorrKeyVec(curve, numKeysForTest, msg)

		partialSignatures := make([]*Signature, numKeysForTest, numKeysForTest)
		// Partial signature generation.
		for j := range schnorrKeyVec.skVec {
			r, s, err := schnorrPartialSign(curve, msg,
				schnorrKeyVec.skVec[j].Serialize(),
				schnorrKeyVec.pkVecSum.Serialize(),
				schnorrKeyVec.secNonceVec[j].Serialize(),
				schnorrKeyVec.pubNonceVecSum.Serialize())

			if err != nil {
				t.Fatalf("unexpected error %s, ", err)
			}

			localSig := NewSignature(r, s)
			partialSignatures[j] = localSig
		}

		// Combine signatures.
		combinedSignature, err := SchnorrCombineSigs(curve, partialSignatures)
		if err != nil {
			t.Fatalf("unexpected error %s, ", err)
		}

		// Verify the combined signature and public keys.
		if !Verify(schnorrKeyVec.pkVecSum, msg, combinedSignature.GetR(),
			combinedSignature.GetS()) {
			t.Fatalf("failed to verify the combined signature")
		}
	}
}

// TestStdSchnorrThresholdSigImpl test detailed implementation of
// Schnorr threshold signature
func TestStdSchnorrThresholdSigImpl(t *testing.T) {
	const MAX_SIGNATORIES = 10
	const NUM_TEST = 5

	tRand := rand.New(rand.NewSource(543212345))

	curve := new(TwistedEdwardsCurve)
	curve.InitParam25519()

	msg, _ := hex.DecodeString(
		"d04b98f48e8f8bcc15c6ae5ac050801cd6dcfd428fb5f9e65c4e16e7807340fa")

	for i := 0; i < NUM_TEST; i++ {
		numKeysForTest := tRand.Intn(MAX_SIGNATORIES-2) + 2

		schnorrKeyVec := mockUpSchnorrKeyVec(curve, numKeysForTest, msg)
		combinedSignature, err := mockUpSchnorrMultiSign(curve, msg, schnorrKeyVec)
		if nil != err {
			t.Fatal(err)
		}

		// Make sure the combined signatures are the same as the
		// signatures that would be generated by simply adding
		// the private keys and private nonces.
		combinedPrivkeysD := new(big.Int).SetInt64(0)
		for _, priv := range schnorrKeyVec.skVec {
			combinedPrivkeysD = ScalarAdd(combinedPrivkeysD, priv.GetD())
			combinedPrivkeysD = combinedPrivkeysD.Mod(combinedPrivkeysD, curve.N)
		}

		combinedNonceD := new(big.Int).SetInt64(0)
		for _, priv := range schnorrKeyVec.secNonceVec {
			combinedNonceD.Add(combinedNonceD, priv.GetD())
			combinedNonceD.Mod(combinedNonceD, curve.N)
		}

		// convert the scalar to a valid secret key for curve
		combinedPrivkey, _, err := PrivKeyFromScalar(curve,
			copyBytes(combinedPrivkeysD.Bytes())[:])
		if err != nil {
			t.Fatalf("unexpected error %s", err)
		}
		// convert the scalar to a valid nonce for curve
		combinedNonce, _, err := PrivKeyFromScalar(curve,
			copyBytes(combinedNonceD.Bytes())[:])
		if err != nil {
			t.Fatalf("unexpected error %s", err)
		}

		// sign with the combined secret key and nonce
		cSigR, cSigS, err := SignFromScalar(curve, combinedPrivkey,
			combinedNonce.Serialize(), msg)
		sumSig := NewSignature(cSigR, cSigS)
		if err != nil {
			t.Fatalf("unexpected error %s", err)
		}
		if !bytes.Equal(sumSig.Serialize(), combinedSignature.Serialize()) {
			t.Fatalf("want %s, got %s",
				hex.EncodeToString(combinedSignature.Serialize()),
				hex.EncodeToString(sumSig.Serialize()))
		}
	}
}

// TestSchnorrThresholdSigOnBadPk test Schnorr threshold signature
// being verified by wrong public keys
// !!!TBU: if we change some pk_i, and recalculate the sum of all
//	pk_i, the sum(pk_i) isn't changed sometimes
func TestSchnorrThresholdSigOnBadPk(t *testing.T) {
	const MAX_SIGNATORIES = 10
	const NUM_TEST = 5

	tRand := rand.New(rand.NewSource(543212345))

	curve := new(TwistedEdwardsCurve)
	curve.InitParam25519()

	msg, _ := hex.DecodeString(
		"d04b98f48e8f8bcc15c6ae5ac050801cd6dcfd428fb5f9e65c4e16e7807340fa")

	for i := 0; i < NUM_TEST; i++ {
		numKeysForTest := tRand.Intn(MAX_SIGNATORIES-2) + 2
		schnorrKeyVec := mockUpSchnorrKeyVec(curve, numKeysForTest, msg)

		// generates the signature
		combinedSignature, err := mockUpSchnorrMultiSign(curve, msg, schnorrKeyVec)
		if nil != err {
			t.Fatal(err)
		}

		// simulate corruption causing the final sum(pk_i) to change
		_, xDelta, yDelta, err := GenerateKey(curve, crand.Reader)
		if nil != err {
			t.Fatalf("unexpected error: %s", err)
		}
		xBad, yBad := curve.Add(schnorrKeyVec.pkVecSum.GetX(),
			schnorrKeyVec.pkVecSum.GetY(), xDelta, yDelta)
		schnorrKeyVec.pkVecSum = NewPublicKey(curve, xBad, yBad)

		// Verify the combined signature and public keys.
		if Verify(schnorrKeyVec.pkVecSum, msg, combinedSignature.GetR(),
			combinedSignature.GetS()) {
			t.Fatalf("verify the combined signature should fail")
		}
	}
}

// TestSchnorrThresholdSigOnBadSecNonce test Schnorr threshold signature
// generated by mismatched public nonces
func TestSchnorrThresholdSigOnBadPubNonce(t *testing.T) {
	const MAX_SIGNATORIES = 10
	const NUM_TEST = 5

	tRand := rand.New(rand.NewSource(543212345))

	curve := new(TwistedEdwardsCurve)
	curve.InitParam25519()

	msg, _ := hex.DecodeString(
		"d04b98f48e8f8bcc15c6ae5ac050801cd6dcfd428fb5f9e65c4e16e7807340fa")

	for i := 0; i < NUM_TEST; i++ {
		numKeysForTest := tRand.Intn(MAX_SIGNATORIES-2) + 2
		schnorrKeyVec := mockUpSchnorrKeyVec(curve, numKeysForTest, msg)

		// simulate corruption causing the final sum(pubNonce_i) to change
		_, xDelta, yDelta, err := GenerateKey(curve, crand.Reader)
		if nil != err {
			t.Fatalf("unexpected error: %s", err)
		}
		xBad, yBad := curve.Add(schnorrKeyVec.pubNonceVecSum.GetX(),
			schnorrKeyVec.pubNonceVecSum.GetY(), xDelta, yDelta)
		schnorrKeyVec.pubNonceVecSum = NewPublicKey(curve, xBad, yBad)

		// generates the signature
		combinedSignature, err := mockUpSchnorrMultiSign(curve, msg, schnorrKeyVec)
		if nil != err {
			t.Fatal(err)
		}

		// Verify the combined signature and public keys.
		if Verify(schnorrKeyVec.pkVecSum, msg, combinedSignature.GetR(),
			combinedSignature.GetS()) {
			t.Fatalf("verify the combined signature should fail")
		}
	}
}

// TestSchnorrThresholdSigOnBadSk test Schnorr threshold signature
// being verified by wrong secret keys
func TestSchnorrThresholdSigOnBadSk(t *testing.T) {
	const MAX_SIGNATORIES = 10
	const NUM_TEST = 5

	tRand := rand.New(rand.NewSource(543212345))

	curve := new(TwistedEdwardsCurve)
	curve.InitParam25519()

	msg, _ := hex.DecodeString(
		"d04b98f48e8f8bcc15c6ae5ac050801cd6dcfd428fb5f9e65c4e16e7807340fa")

	for i := 0; i < NUM_TEST; i++ {
		numKeysForTest := tRand.Intn(MAX_SIGNATORIES-2) + 2
		schnorrKeyVec := mockUpSchnorrKeyVec(curve, numKeysForTest, msg)

		// Corrupt private key.
		randItem := tRand.Intn(numKeysForTest)
		skBad := schnorrKeyVec.skVec[randItem].Serialize()
		pos := tRand.Intn(31)
		bitPos := tRand.Intn(7)
		skBad[pos] ^= 1 << uint8(bitPos)
		schnorrKeyVec.skVec[randItem].ecPk.D.SetBytes(skBad)

		// generates the signature
		combinedSignature, err := mockUpSchnorrMultiSign(curve, msg, schnorrKeyVec)
		if nil != err {
			t.Fatal(err)
		}

		// Verify the combined signature and public keys.
		if Verify(schnorrKeyVec.pkVecSum, msg, combinedSignature.GetR(),
			combinedSignature.GetS()) {
			t.Fatalf("verify the combined signature should fail")
		}
	}
}

// TestSchnorrThresholdSigOnBadSecNonce test Schnorr threshold signature
// being verified by wrong secret nonces
func TestSchnorrThresholdSigOnBadSecNonce(t *testing.T) {
	const MAX_SIGNATORIES = 10
	const NUM_TEST = 5

	tRand := rand.New(rand.NewSource(543212345))

	curve := new(TwistedEdwardsCurve)
	curve.InitParam25519()

	msg, _ := hex.DecodeString(
		"d04b98f48e8f8bcc15c6ae5ac050801cd6dcfd428fb5f9e65c4e16e7807340fa")

	for i := 0; i < NUM_TEST; i++ {
		numKeysForTest := tRand.Intn(MAX_SIGNATORIES-2) + 2
		schnorrKeyVec := mockUpSchnorrKeyVec(curve, numKeysForTest, msg)

		randItem := tRand.Intn(numKeysForTest - 1)
		secNonceBytes := schnorrKeyVec.secNonceVec[randItem].Serialize()
		pos := tRand.Intn(31)
		bitPos := tRand.Intn(7)
		secNonceBytes[pos] ^= 1 << uint8(bitPos)
		schnorrKeyVec.secNonceVec[randItem].ecPk.D.SetBytes(secNonceBytes)

		// generates the signature
		combinedSignature, err := mockUpSchnorrMultiSign(curve, msg, schnorrKeyVec)
		if nil != err {
			t.Fatal(err)
		}

		// Verify the combined signature and public keys.
		if Verify(schnorrKeyVec.pkVecSum, msg, combinedSignature.GetR(),
			combinedSignature.GetS()) {
			t.Fatalf("verify the combined signature should fail")
		}
	}
}
