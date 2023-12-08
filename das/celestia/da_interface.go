package celestia

import (
	"context"
)

type DataAvailabilityWriter interface {
	Store(context.Context, []byte) ([]byte, bool, error)
}

// make output of read include eds or not
type DataAvailabilityReader interface {
	Read(context.Context, BlobPointer) ([]byte, *SquareData, error)
}
