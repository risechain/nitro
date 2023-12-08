package tree

import (
	"bytes"
	"errors"

	"github.com/celestiaorg/rsmt2d"
	"github.com/ethereum/go-ethereum/common"
)

// need to pass square size and axis index
func ComputeNmtRoot(createTreeFn rsmt2d.TreeConstructorFn, index uint, shares [][]byte) ([]byte, error) {
	// create NMT with custom Hasher
	// use create tree function, pass it to the ComputeNmtRoot function
	tree := createTreeFn(rsmt2d.Row, index)
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
	flagLen := NamespaceSize * 2
	sha256Len := 32
	leftChild = hash[1 : flagLen+sha256Len]
	rightChild = hash[flagLen+sha256Len+1:]
	return leftChild, rightChild
}

// walkMerkleTree recursively walks down the Merkle tree and collects leaf node data.
func NmtContent(oracle func(bytes32) ([]byte, error), rootHash []byte) ([][]byte, error) {
	preimage, err := oracle(common.BytesToHash(rootHash[NamespaceSize*2:]))
	if err != nil {
		return nil, err
	}

	minNid := rootHash[:NamespaceSize]
	maxNid := rootHash[NamespaceSize : NamespaceSize*2]
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
