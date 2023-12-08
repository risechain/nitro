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
	"github.com/tendermint/tendermint/rpc/client/http"
)

// CelestiaMessageHeaderFlag indicates that this data is a Blob Pointer
// which will be used to retrieve data from Celestia
const CelestiaMessageHeaderFlag byte = 0x0c

func IsCelestiaMessageHeaderByte(header byte) bool {
	return (CelestiaMessageHeaderFlag & header) > 0
}

// Add Tendermint RPC for Full node Endpoint
type CelestiaDA struct {
	cfg       DAConfig
	client    *openrpc.Client
	trpc      *http.HTTP
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

	trpc, err := http.New(cfg.TendermintRPC, "/websocket")
	if err != nil {
		log.Error("Unable to establish connection with celestia-core tendermint rpc")
		return nil, err
	}
	err = trpc.Start()
	if err != nil {
		return nil, err
	}

	return &CelestiaDA{
		cfg:       cfg,
		client:    daClient,
		trpc:      trpc,
		namespace: namespace,
	}, nil
}

func (c *CelestiaDA) Store(ctx context.Context, message []byte) ([]byte, bool, error) {

	dataBlob, err := blob.NewBlobV0(c.namespace, message)
	if err != nil {
		log.Warn("Error creating blob", "err", err)
		return nil, false, err
	}
	commitment, err := blob.CreateCommitment(dataBlob)
	if err != nil {
		log.Warn("Error creating commitment", "err", err)
		return nil, false, err
	}
	log.Info("Blob to be submitted: ", "blob", []*blob.Blob{dataBlob})
	height, err := c.client.Blob.Submit(ctx, []*blob.Blob{dataBlob}, openrpc.DefaultSubmitOptions())
	if err != nil {
		log.Warn("Blob Submission error", "err", err)
		return nil, false, err
	}
	if height == 0 {
		log.Warn("Unexpected height from blob response", "height", height)
		return nil, false, errors.New("unexpected response code")
	}

	// how long do we have to wait to retrieve a proof?
	log.Info("Retrieving Proof from Celestia", "height", height, "commitment", commitment)
	proofs, err := c.client.Blob.GetProof(ctx, height, c.namespace, commitment)
	if err != nil {
		log.Warn("Error retrieving proof", "err", err)
		return nil, false, err
	}

	log.Info("Checking for inclusion", "height", height, "commitment", commitment)
	included, err := c.client.Blob.Included(ctx, height, c.namespace, proofs, commitment)
	if err != nil {
		log.Warn("Error checking for inclusion", "err", err, "proof", proofs)
		return nil, included, err
	}

	log.Info("Sucesfully posted data to Celestia", "height", height, "commitment", commitment)

	log.Info("Retrieving data root for height ", "height", height)

	header, err := c.client.Header.GetByHeight(ctx, height)
	if err != nil {
		log.Warn("Header retrieval error", "err", err)
		return nil, included, err
	}

	var startIndex uint64
	sharesLength := uint64(0)
	for i, proof := range *proofs {
		if i == 0 {
			startIndex = uint64(proof.Start())
		}
		sharesLength += uint64(proof.End()) - uint64(proof.Start())
	}

	// 2. Get tRPC interface and query /data_root_inclusion_proof
	proof, err := c.trpc.DataRootInclusionProof(ctx, height, height, height+1)
	if err != nil {
		log.Warn("DataRootInclusionProof error", "err", err)
		return nil, included, err
	}

	blobPointer := BlobPointer{
		BlockHeight:  height,
		Start:        startIndex,
		SharesLength: sharesLength,
		Key:          uint64(proof.Proof.Index),
		NumLeaves:    uint64(proof.Proof.Total),
		TxCommitment: commitment,
		DataRoot:     header.DAH.Hash(),
		SideNodes:    proof.Proof.Aunts,
	}

	blobPointerData, err := blobPointer.MarshalBinary()
	if err != nil {
		log.Warn("BlobPointer MashalBinary error", "err", err)
		return nil, included, err
	}

	buf := new(bytes.Buffer)
	err = binary.Write(buf, binary.BigEndian, CelestiaMessageHeaderFlag)
	if err != nil {
		log.Warn("batch type byte serialization failed", "err", err)
		return nil, included, err
	}

	err = binary.Write(buf, binary.BigEndian, blobPointerData)
	if err != nil {
		log.Warn("blob pointer data serialization failed", "err", err)
		return nil, included, err
	}

	serializedBlobPointerData := buf.Bytes()
	log.Info("Succesfully serialized Blob Pointer", "height", height, "commitment", commitment, "data root", header.DataHash)
	log.Trace("celestia.CelestiaDA.Store", "serialized_blob_pointer", serializedBlobPointerData)
	return serializedBlobPointerData, included, nil

}

type SquareData struct {
	RowRoots    [][]byte
	ColumnRoots [][]byte
	Rows        [][][]byte
	// Refers to the square size of the extended data square
	SquareSize uint64
	StartRow   uint64
	EndRow     uint64
}

func (c *CelestiaDA) Read(ctx context.Context, blobPointer BlobPointer) ([]byte, *SquareData, error) {
	log.Info("Requesting data from Celestia", "namespace", c.cfg.NamespaceId, "height", blobPointer.BlockHeight)

	blob, err := c.client.Blob.Get(ctx, blobPointer.BlockHeight, c.namespace, blobPointer.TxCommitment)
	if err != nil {
		return nil, nil, err
	}

	header, err := c.client.Header.GetByHeight(ctx, blobPointer.BlockHeight)
	if err != nil {
		return nil, nil, err
	}

	eds, err := c.client.Share.GetEDS(ctx, header)
	if err != nil {
		return nil, nil, err
	}

	squareSize := uint64(eds.Width())
	startRow := blobPointer.Start / squareSize
	log.Info("End Row", "blobPointer.Start", blobPointer.Start, "shares length", blobPointer.SharesLength, "squareSize", squareSize)
	endRow := (blobPointer.Start + blobPointer.SharesLength) / squareSize

	rows := [][][]byte{}
	for i := startRow; i <= endRow; i++ {
		rows = append(rows, eds.Row(uint(i)))
	}

	squareData := SquareData{
		RowRoots:    header.DAH.RowRoots,
		ColumnRoots: header.DAH.ColumnRoots,
		Rows:        rows,
		SquareSize:  squareSize,
		StartRow:    startRow,
		EndRow:      endRow,
	}

	log.Info("Succesfully fetched data from Celestia", "namespace", c.cfg.NamespaceId, "height", blobPointer.BlockHeight, "commitment", blob.Commitment)

	return blob.Data, &squareData, nil
}
