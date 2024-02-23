package celestia_stub

import (
	"bytes"
	"context"
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/offchainlabs/nitro/das/dastree"
)

type DataAvailabilityStubWriter interface {
	Store(context.Context, []byte) ([]byte, bool, error)
}

// make output of read include eds or not
type DataAvailabilityStubReader interface {
	Read(context.Context, []byte) ([]byte, error)
}

// CelestiaMessageHeaderFlag indicates that this data is a Blob Pointer
// which will be used to retrieve data from Celestia
const CelestiaStubMessageHeaderFlag byte = 0x02

func IsCelestiaStubMessageHeaderByte(header byte) bool {
	return (CelestiaStubMessageHeaderFlag & header) > 0
}

// Add Tendermint RPC for Full node Endpoint
type CelestiaDAStub struct {
	LocalFileStorageService
}

func NewCelestiaDAStub() (*CelestiaDAStub, error) {
	return &CelestiaDAStub{
		LocalFileStorageService{
			dataDir: "/daroot",
		},
	}, nil
}

func (c *CelestiaDAStub) Store(ctx context.Context, message []byte) ([]byte, bool, error) {
	key := dastree.Hash(message)
	err := c.putKeyValue(ctx, key, message)
	if err != nil {
		log.Warn("Error write message", "err", err)
		return nil, false, err
	}

	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, CelestiaStubMessageHeaderFlag)
	if err != nil {
		log.Warn("batch type byte serialization failed", "err", err)
		return nil, false, err
	}

	err = binary.Write(buf, binary.BigEndian, key.Bytes())
	if err != nil {
		log.Warn("blob pointer data serialization failed", "err", err)
		return nil, false, err
	}

	serializedBlobPointerData := buf.Bytes()
	return serializedBlobPointerData, true, nil
}

func (c *CelestiaDAStub) Read(ctx context.Context, key []byte) ([]byte, error) {
	keyHash := common.BytesToHash(key)
	payload, err := c.GetByHash(ctx, keyHash)
	if err != nil {
		log.Warn("Error read message", "err", err)
		return nil, err
	}

	return payload, nil
}
