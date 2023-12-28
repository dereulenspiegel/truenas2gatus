package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/dereulenspiegel/truenas2gatos/gatus"
)

type fileStore struct {
	dataPath      string
	resultsToKeep int
	results       []*gatus.Result

	lock *sync.Mutex
}

func NewFileStore(dataPath string, resultsToKeep int) (*fileStore, error) {
	dataFile, err := os.OpenFile(dataPath, os.O_CREATE|os.O_RDWR, 0664)
	if err != nil {
		return nil, fmt.Errorf("failed to open data file: %w", err)
	}
	defer dataFile.Close()
	results := make([]*gatus.Result, 0)
	dataFileStat, err := dataFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat data file: %w", err)
	}
	if dataFileStat.Size() > 0 {
		if err := json.NewDecoder(dataFile).Decode(&results); err != nil {
			return nil, fmt.Errorf("failed to decode existing results: %w", err)
		}
	}
	f := &fileStore{
		dataPath:      dataPath,
		resultsToKeep: resultsToKeep,
		results:       results,
		lock:          &sync.Mutex{},
	}
	return f, nil
}

func (f *fileStore) SaveResult(result *gatus.Result) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.results = append(f.results, result)
	if len(f.results) > f.resultsToKeep {
		f.results = f.results[1:]
	}
	dataFile, err := os.OpenFile(f.dataPath, os.O_RDWR, 0664)
	if err != nil {
		return fmt.Errorf("failed to open data file: %w", err)
	}
	defer func() {
		dataFile.Sync()
		dataFile.Close()
	}()
	if err := json.NewEncoder(dataFile).Encode(f.results); err != nil {
		return fmt.Errorf("failed to encode results: %w", err)
	}
	return nil
}

func (f *fileStore) GetResults() ([]*gatus.Result, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.results, nil
}
