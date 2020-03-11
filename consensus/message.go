// Copyright (c) 2020 Sperax
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package consensus

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math/big"

	proto "github.com/golang/protobuf/proto"
	"github.com/xtaci/bdls/crypto/blake2b"
	"github.com/xtaci/bdls/crypto/secp256k1"
)

// ErrPubKey will be returned if error found while decoding message's public key
var ErrPubKey = errors.New("incorrect pubkey format")

// default elliptic curve for signing
var defaultCurve = secp256k1.S256()

const (
	// SizeAxis defines bytes size of X-axis or Y-axis in a public key
	SizeAxis = 32
	// SignaturePrefix is the prefix for signing a consensus message
	SignaturePrefix = "==BDLS CONSENSUS MESSAGE=="
)

// PubKeyAxis defines X-axis or Y-axis in a public key
type PubKeyAxis [SizeAxis]byte

// Marshal implements protobuf MarshalTo
func (t PubKeyAxis) Marshal() ([]byte, error) {
	return t[:], nil
}

// MarshalTo implements protobuf MarshalTo
func (t *PubKeyAxis) MarshalTo(data []byte) (n int, err error) {
	copy(data, (*t)[:])
	return SizeAxis, nil
}

// Unmarshal implements protobuf Unmarshal
func (t *PubKeyAxis) Unmarshal(data []byte) error {
	// mor than 32 bytes, illegal axis
	if len(data) > 32 {
		return ErrPubKey
	}

	// if data is less than 32 bytes, we MUST keep the leading 0 zeros.
	off := 32 - len(data)
	copy((*t)[off:], data)
	return nil
}

// Size implements protobuf Size
func (t *PubKeyAxis) Size() int { return SizeAxis }

// MarshalJSON implements protobuf MarshalJSON
func (t PubKeyAxis) MarshalJSON() ([]byte, error) { return json.Marshal(t) }

// UnmarshalJSON implements protobuf UnmarshalJSON
func (t *PubKeyAxis) UnmarshalJSON(data []byte) error { return json.Unmarshal(data, t) }

// coordiante encodes X-axis and Y-axis for a publickey in an array
type coordinate [2 * SizeAxis]byte

// create coordinate from public key
func newCoordFromPubKey(pubkey *ecdsa.PublicKey) (ret coordinate) {
	var X PubKeyAxis
	var Y PubKeyAxis

	err := X.Unmarshal(pubkey.X.Bytes())
	if err != nil {
		panic(err)
	}

	err = Y.Unmarshal(pubkey.Y.Bytes())
	if err != nil {
		panic(err)
	}

	copy(ret[:SizeAxis], X[:])
	copy(ret[SizeAxis:], Y[:])
	return
}

// test if X,Y axis equals to a coordinates
func (c coordinate) Equal(x1 PubKeyAxis, y1 PubKeyAxis) bool {
	if bytes.Equal(x1[:], c[:SizeAxis]) && bytes.Equal(y1[:], c[SizeAxis:]) {
		return true
	}
	return false
}

// Coordiante encodes X,Y in public key
func (sp *SignedProto) Coordiante() (ret coordinate) {
	copy(ret[:SizeAxis], sp.X[:])
	copy(ret[SizeAxis:], sp.Y[:])
	return
}

// Hash concats and hash as follows:
// blake2b(signPrefix + version + pubkey.X + pubkey.Y+len_32bit(msg) + message)
func (sp *SignedProto) Hash() []byte {
	hash, err := blake2b.New256(nil)
	if err != nil {
		panic(err)
	}
	// write prefix
	_, err = hash.Write([]byte(SignaturePrefix))
	if err != nil {
		panic(err)
	}

	// write version
	err = binary.Write(hash, binary.LittleEndian, sp.Version)
	if err != nil {
		panic(err)
	}

	// write X & Y
	_, err = hash.Write(sp.X[:])
	if err != nil {
		panic(err)
	}

	_, err = hash.Write(sp.Y[:])
	if err != nil {
		panic(err)
	}

	// write message length
	err = binary.Write(hash, binary.LittleEndian, uint32(len(sp.Message)))
	if err != nil {
		panic(err)
	}

	// write message
	_, err = hash.Write(sp.Message)
	if err != nil {
		panic(err)
	}

	return hash.Sum(nil)
}

// Sign the message into a signed consensusMessage
func (sp *SignedProto) Sign(m *Message, privateKey *ecdsa.PrivateKey) {
	bts, err := proto.Marshal(m)
	if err != nil {
		panic(err)
	}
	// hash message
	sp.Version = ProtocolVersion
	sp.Message = bts

	err = sp.X.Unmarshal(privateKey.PublicKey.X.Bytes())
	if err != nil {
		panic(err)
	}
	err = sp.Y.Unmarshal(privateKey.PublicKey.Y.Bytes())
	if err != nil {
		panic(err)
	}
	hash := sp.Hash()

	// sign the message
	r, s, err := ecdsa.Sign(rand.Reader, privateKey, hash)
	if err != nil {
		panic(err)
	}
	sp.R = r.Bytes()
	sp.S = s.Bytes()
}

// Verify the signature of this signed message
func (sp *SignedProto) Verify() bool {
	hash := sp.Hash()
	// verify against public key and r, s
	pubkey := ecdsa.PublicKey{}
	pubkey.Curve = defaultCurve
	pubkey.X = big.NewInt(0).SetBytes(sp.X[:])
	pubkey.Y = big.NewInt(0).SetBytes(sp.Y[:])
	return ecdsa.Verify(&pubkey, hash, big.NewInt(0).SetBytes(sp.R), big.NewInt(0).SetBytes(sp.S))
}