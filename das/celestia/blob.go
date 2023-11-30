package celestia

import (
	"encoding/binary"
)

// BlobPointer contains the reference to the data blob on Celestia
type BlobPointer struct {
	BlockHeight  uint64
	Start        uint64
	SharesLength uint64
	Key          uint64
	NumLeaves    uint64
	TxCommitment []byte
	DataRoot     []byte
	SideNodes    [][]byte
}

// MarshalBinary encodes the BlobPointer to binary
// serialization format: height + start + end + commitment + data root
func (b *BlobPointer) MarshalBinary() ([]byte, error) {
	blob := make([]byte, 8*3+len(b.TxCommitment)+len(b.DataRoot))

	binary.LittleEndian.PutUint64(blob, b.BlockHeight)
	binary.LittleEndian.PutUint64(blob[8:16], b.Start)
	binary.LittleEndian.PutUint64(blob[16:24], b.SharesLength)
	binary.LittleEndian.PutUint64(blob[24:32], b.Key)
	binary.LittleEndian.PutUint64(blob[32:40], b.NumLeaves)
	copy(blob[40:72], b.TxCommitment)
	copy(blob[72:104], b.DataRoot)

	// each side node is 32 bytes (sha256 merkle tree hash)
	for i, node := range b.SideNodes {
		index := 104 + (i * 32)
		copy(blob[index:index+32], node)
	}
	return blob, nil
}

// UnmarshalBinary decodes the binary to BlobPointer
// serialization format: height + start + end + commitment + data root
func (b *BlobPointer) UnmarshalBinary(ref []byte) error {
	b.BlockHeight = binary.LittleEndian.Uint64(ref[:8])
	b.Start = binary.LittleEndian.Uint64(ref[8:16])
	b.SharesLength = binary.LittleEndian.Uint64(ref[16:24])
	b.Key = binary.LittleEndian.Uint64(ref[24:32])
	b.NumLeaves = binary.LittleEndian.Uint64(ref[32:40])
	b.TxCommitment = ref[40:72]
	b.DataRoot = ref[72:104]
	sideNodesLength := len(ref[104:])
	b.SideNodes = make([][]byte, sideNodesLength)

	for i := range b.SideNodes {
		index := 104 + (i * 32)
		b.SideNodes[i] = ref[index : index+32]
	}
	return nil
}
