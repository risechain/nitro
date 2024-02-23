// Copyright 2022, Offchain Labs, Inc.
// For license information, see https://github.com/nitro/blob/master/LICENSE

package celestia_stub

import (
	"context"
	"encoding/base32"
	"errors"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/offchainlabs/nitro/das/dastree"
	"github.com/offchainlabs/nitro/util/pretty"
	flag "github.com/spf13/pflag"
)

type LocalFileStorageConfig struct {
	Enable                 bool   `koanf:"enable"`
	DataDir                string `koanf:"data-dir"`
	SyncFromStorageService bool   `koanf:"sync-from-storage-service"`
	SyncToStorageService   bool   `koanf:"sync-to-storage-service"`
}

var DefaultLocalFileStorageConfig = LocalFileStorageConfig{
	DataDir: "",
}

func LocalFileStorageConfigAddOptions(prefix string, f *flag.FlagSet) {
	f.Bool(prefix+".enable", DefaultLocalFileStorageConfig.Enable, "enable storage/retrieval of sequencer batch data from a directory of files, one per batch")
	f.String(prefix+".data-dir", DefaultLocalFileStorageConfig.DataDir, "local data directory")
	f.Bool(prefix+".sync-from-storage-service", DefaultLocalFileStorageConfig.SyncFromStorageService, "enable local storage to be used as a source for regular sync storage")
	f.Bool(prefix+".sync-to-storage-service", DefaultLocalFileStorageConfig.SyncToStorageService, "enable local storage to be used as a sink for regular sync storage")
}

var ErrNotFound = errors.New("not found")

type LocalFileStorageService struct {
	dataDir string
}

func NewLocalFileStorageService(dataDir string) (*LocalFileStorageService, error) {
	return &LocalFileStorageService{dataDir: dataDir}, nil
}

func EncodeStorageServiceKey(key common.Hash) string {
	return key.Hex()[2:]
}

func (s *LocalFileStorageService) GetByHash(ctx context.Context, key common.Hash) ([]byte, error) {
	log.Trace("das.LocalFileStorageService.GetByHash", "key", pretty.PrettyHash(key), "this", s)
	pathname := s.dataDir + "/" + EncodeStorageServiceKey(key)
	data, err := os.ReadFile(pathname)
	if err != nil {
		// Just for backward compatability.
		pathname = s.dataDir + "/" + base32.StdEncoding.EncodeToString(key.Bytes())
		data, err = os.ReadFile(pathname)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		return data, nil
	}
	return data, nil
}

func (s *LocalFileStorageService) Put(ctx context.Context, data []byte, timeout uint64) error {
	fileName := EncodeStorageServiceKey(dastree.Hash(data))
	finalPath := s.dataDir + "/" + fileName

	// Use a temp file and rename to achieve atomic writes.
	f, err := os.CreateTemp(s.dataDir, fileName)
	if err != nil {
		return err
	}
	err = f.Chmod(0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err != nil {
		return err
	}
	err = f.Close()
	if err != nil {
		return err
	}

	return os.Rename(f.Name(), finalPath)

}

func (s *LocalFileStorageService) putKeyValue(ctx context.Context, key common.Hash, value []byte) error {
	log.Trace("das.LocalFileStorageService.putKeyValue", "key", pretty.PrettyHash(key), "this", s)
	fileName := EncodeStorageServiceKey(key)
	finalPath := s.dataDir + "/" + fileName

	// Use a temp file and rename to achieve atomic writes.
	f, err := os.CreateTemp(s.dataDir, fileName)
	if err != nil {
		return err
	}
	err = f.Chmod(0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(value)
	if err != nil {
		return err
	}
	err = f.Close()
	if err != nil {
		return err
	}

	return os.Rename(f.Name(), finalPath)

}

func (s *LocalFileStorageService) String() string {
	return "LocalFileStorageService(" + s.dataDir + ")"
}
