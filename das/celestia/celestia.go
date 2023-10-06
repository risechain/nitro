package celestia

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"

	"github.com/ethereum/go-ethereum/log"
	openrpc "github.com/rollkit/celestia-openrpc"
	"github.com/rollkit/celestia-openrpc/types/blob"
	"github.com/rollkit/celestia-openrpc/types/share"
)

// CelestiaMessageHeaderFlag indicates that this data is a Blob Pointer
// which will be used to retrieve data from Celestia
const CelestiaMessageHeaderFlag byte = 0x0c

func IsCelestiaMessageHeaderByte(header byte) bool {
	return (CelestiaMessageHeaderFlag & header) > 0
}

type CelestiaDA struct {
	cfg       DAConfig
	client    *openrpc.Client
	namespace share.Namespace
}

func NewCelestiaDA(cfg DAConfig) (*CelestiaDA, error) {
	daClient, err := openrpc.NewClient(context.Background(), cfg.Rpc, cfg.AuthToken)
	if err != nil {
		return nil, err
	}

	if cfg.NamespaceId == "" {
		return nil, errors.New("namespace id cannot be blank")
	}
	nsBytes, err := hex.DecodeString(cfg.NamespaceId)
	if err != nil {
		return nil, err
	}

	namespace, err := share.NewBlobNamespaceV0(nsBytes)
	if err != nil {
		return nil, err
	}

	return &CelestiaDA{
		cfg:       cfg,
		client:    daClient,
		namespace: namespace,
	}, nil
}

func (c *CelestiaDA) Store(ctx context.Context, message []byte) ([]byte, error) {
	dataBlob, err := blob.NewBlobV0(c.namespace, message)
	if err != nil {
		log.Warn("Error creating blob", "err", err)
		return nil, err
	}
	commitment, err := blob.CreateCommitment(dataBlob)
	if err != nil {
		log.Warn("Error creating commitment", "err", err)
		return nil, err
	}
	height, err := c.client.Blob.Submit(ctx, []*blob.Blob{dataBlob})
	if err != nil {
		log.Warn("Blob Submission error", "err", err)
		return nil, err
	}
	if height == 0 {
		log.Warn("Unexpected height from blob response", "height", height)
		return nil, errors.New("unexpected response code")
	}

	log.Info("Sucesfully posted data to Celestia", "height", height, "commitment", commitment)

	log.Info("Retrieving data root for height ", height)

	header, err := c.client.Header.GetByHeight(ctx, height)
	if err != nil {
		log.Warn("Header retrieval error", "err", err)
		return nil, err
	}
	blobPointer := BlobPointer{
		BlockHeight:  height,
		TxCommitment: commitment,
		DataRoot:     header.DataHash,
	}

	blobPointerData, err := blobPointer.MarshalBinary()
	if err != nil {
		log.Warn("BlobPointer MashalBinary error", "err", err)
		return nil, err
	}

	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, CelestiaMessageHeaderFlag)
	if err != nil {
		log.Warn("batch type byte serialization failed", "err", err)
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, blobPointerData)
	if err != nil {
		log.Warn("blob pointer data serialization failed", "err", err)
		return nil, err
	}

	serializedBlobPointerData := buf.Bytes()
	log.Info("Succesfully serialized Blob Pointer", "height", height, "commitment", commitment, "data root", header.DataHash)
	log.Trace("celestia.CelestiaDA.Store", "serialized_blob_pointer", serializedBlobPointerData)
	return serializedBlobPointerData, nil

}

func (c *CelestiaDA) Read(blobPointer BlobPointer) ([]byte, error) {
	log.Info("Requesting data from Celestia", "namespace", c.cfg.NamespaceId, "height", blobPointer.BlockHeight)

	blob, err := c.client.Blob.Get(context.Background(), blobPointer.BlockHeight, c.namespace, blobPointer.TxCommitment)
	if err != nil {
		return nil, err
	}

	log.Info("Succesfully fetched data from Celestia", "namespace", c.cfg.NamespaceId, "height", blobPointer.BlockHeight, "commitment", blob.Commitment)

	return blob.Data, nil
}
