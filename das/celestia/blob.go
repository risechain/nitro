package celestia

import (
	"encoding/binary"
)

// BlobPointer contains the reference to the data blob on Celestia
type BlobPointer struct {
	BlockHeight  uint64
	TxCommitment []byte
	DataRoot     []byte
}

// MarshalBinary encodes the BlobPointer to binary
// serialization format: height + commitment
//
//	-------------------------------------------------------------
//
// | 8 byte uint64  |  32 byte commitment   | 32 byte data root |
//
//	-------------------------------------------------------------
//
// | <-- height --> | <-- commitment -->    | <-- data root --> |
//
//	-------------------------------------------------------------
func (b *BlobPointer) MarshalBinary() ([]byte, error) {
	blob := make([]byte, 8+len(b.TxCommitment))

	binary.LittleEndian.PutUint64(blob, b.BlockHeight)
	copy(blob[8:], b.TxCommitment)

	return blob, nil
}

// UnmarshalBinary decodes the binary to BlobPointer
// serialization format: height + commitment
//
//	----------------------------------------
//
// | 8 byte uint64  |  32 byte commitment   |
//
//	----------------------------------------
//
// | <-- height --> | <-- commitment -->    |
//
//	----------------------------------------
func (b *BlobPointer) UnmarshalBinary(ref []byte) error {
	b.BlockHeight = binary.LittleEndian.Uint64(ref[:8])
	b.TxCommitment = ref[8:33]
	b.DataRoot = ref[33:]
	return nil
}
