package main

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
)

type packet struct {
	Version   int    `json:"v"`
	Kind      string `json:"k"`
	ID        string `json:"id"`
	Name      string `json:"n"`
	Size      int    `json:"s"`
	Index     int    `json:"i"`
	Total     int    `json:"t"`
	Data      string `json:"d"`
	ChunkHash string `json:"h"`
	FileHash  string `json:"f"`
}

type setupPacket struct {
	Version          int    `json:"v"`
	Kind             string `json:"k"`
	ID               string `json:"id"`
	Name             string `json:"n"`
	Size             int    `json:"s"`
	Total            int    `json:"t"`
	FileHash         string `json:"f"`
	FrameDelayMS     int    `json:"ms"`
	Compression      string `json:"c,omitempty"`
	OriginalSize     int    `json:"us,omitempty"`
	OriginalFileHash string `json:"uf,omitempty"`
}

const (
	maximumFileSize = 100 * 1024 * 1024
	maximumChunks   = 200_000
)

func makePackets(path string, data []byte, chunkSize int, frameDelayMS int) ([]byte, [][]byte, error) {
	if chunkSize < 1 {
		return nil, nil, fmt.Errorf("chunk size must be positive")
	}
	if len(data) > maximumFileSize {
		return nil, nil, fmt.Errorf("file is too large for QRDrop: %.1f MB > %.1f MB", float64(len(data))/(1024*1024), float64(maximumFileSize)/(1024*1024))
	}
	originalData := data
	originalSum := sha256.Sum256(originalData)
	compression := ""
	if compressed, ok := gzipCompressIfSmaller(data); ok {
		data = compressed
		compression = "gzip"
	}
	idBytes := make([]byte, 6)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, nil, err
	}
	id := hex.EncodeToString(idBytes)
	fileSum := sha256.Sum256(data)
	total := (len(data) + chunkSize - 1) / chunkSize
	if total == 0 {
		total = 1
	}
	if total > maximumChunks {
		return nil, nil, fmt.Errorf("file requires too many QR frames: %d > %d (file %.1f MB, chunk size %d bytes)", total, maximumChunks, float64(len(data))/(1024*1024), chunkSize)
	}
	frames := make([][]byte, 0, total)
	for i := 0; i < total; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(data) {
			end = len(data)
		}
		chunk := data[start:end]
		chunkSum := sha256.Sum256(chunk)
		p := packet{1, "data", id, filepath.Base(path), len(data), i, total,
			base64.StdEncoding.EncodeToString(chunk), hex.EncodeToString(chunkSum[:]), hex.EncodeToString(fileSum[:])}
		encoded, err := json.Marshal(p)
		if err != nil {
			return nil, nil, err
		}
		frames = append(frames, encoded)
	}
	setupPacket := setupPacket{Version: 1, Kind: "setup", ID: id, Name: filepath.Base(path), Size: len(data), Total: total, FileHash: hex.EncodeToString(fileSum[:]), FrameDelayMS: frameDelayMS}
	if compression != "" {
		setupPacket.Compression = compression
		setupPacket.OriginalSize = len(originalData)
		setupPacket.OriginalFileHash = hex.EncodeToString(originalSum[:])
	}
	setup, err := json.Marshal(setupPacket)
	if err != nil {
		return nil, nil, err
	}
	return setup, frames, nil
}

func gzipCompressIfSmaller(data []byte) ([]byte, bool) {
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, false
	}
	if _, err := zw.Write(data); err != nil {
		_ = zw.Close()
		return nil, false
	}
	if err := zw.Close(); err != nil {
		return nil, false
	}
	compressed := buf.Bytes()
	if len(compressed) >= len(data) {
		return nil, false
	}
	out := make([]byte, len(compressed))
	copy(out, compressed)
	return out, true
}
