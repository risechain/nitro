package tree

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"hash"

	"github.com/celestiaorg/nmt"
	"github.com/ethereum/go-ethereum/common"
)

// customHasher embeds hash.Hash and includes a map for the hash-to-preimage mapping
type NmtPreimageHasher struct {
	hash.Hash
	record func(bytes32, []byte)
	data   []byte
}

// Need to make sure this is writting relevant data into the tree
// Override the Sum method to capture the preimage
func (h *NmtPreimageHasher) Sum(b []byte) []byte {
	hashed := h.Hash.Sum(nil)
	hashKey := common.BytesToHash(hashed)
	h.record(hashKey, append([]byte(nil), h.data...))
	return h.Hash.Sum(b)
}

func (h *NmtPreimageHasher) Write(p []byte) (n int, err error) {
	h.data = append(h.data[:0], p...) // Update the data slice with the new data
	return h.Hash.Write(p)
}

// Override the Reset method to clean the hash state and the data slice
func (h *NmtPreimageHasher) Reset() {
	h.Hash.Reset()
	h.data = h.data[:0] // Reset the data slice to be empty, but keep the underlying array
}

func newNmtPreimageHasher(record func(bytes32, []byte)) hash.Hash {
	return &NmtPreimageHasher{
		Hash:   sha256.New(),
		record: record,
	}
}

func ComputeNmtRoot(record func(bytes32, []byte), shares [][]byte) ([]byte, error) {
	// create NMT with custom Hasher
	tree := nmt.New(newNmtPreimageHasher(record), nmt.NamespaceIDSize(29), nmt.IgnoreMaxNamespace(true))
	if !isComplete(shares) {
		return nil, errors.New("can not compute root of incomplete row")
	}
	for _, d := range shares {

		err := tree.Push(d)
		if err != nil {
			return nil, err
		}
	}

	return tree.Root()
}

// isComplete returns true if all the shares are non-nil.
func isComplete(shares [][]byte) bool {
	for _, share := range shares {
		if share == nil {
			return false
		}
	}
	return true
}

// getNmtChildrenHashes splits the preimage into the hashes of the left and right children of the NMT
// note that a leaf has the format minNID || maxNID || hash, here hash is the hash of the left and right
// (NodePrefix) || (leftMinNID || leftMaxNID || leftHash) || (rightMinNID || rightMaxNID || rightHash)
func getNmtChildrenHashes(hash []byte) (leftChild, rightChild []byte) {
	flagLen := 29 * 2
	sha256Len := 32
	leftChild = hash[1 : flagLen+sha256Len]
	rightChild = hash[flagLen+sha256Len+1:]
	return leftChild, rightChild
}

// walkMerkleTree recursively walks down the Merkle tree and collects leaf node data.
func NmtContent(oracle func(bytes32) ([]byte, error), rootHash []byte) ([][]byte, error) {
	preimage, err := oracle(common.BytesToHash(rootHash[29*2:]))
	if err != nil {
		return nil, err
	}

	minNid := rootHash[:29]
	maxNid := rootHash[29 : 29*2]
	// check if the hash corresponds to a leaf
	if bytes.Equal(minNid, maxNid) {
		// returns the data with the namespace ID prepended
		return [][]byte{preimage[1:]}, nil
	}

	leftChildHash, rightChildHash := getNmtChildrenHashes(preimage)
	leftData, err := NmtContent(oracle, leftChildHash)
	if err != nil {
		return nil, err
	}
	rightData, err := NmtContent(oracle, rightChildHash)
	if err != nil {
		return nil, err
	}

	// Combine the data from the left and right subtrees.
	return append(leftData, rightData...), nil
}
