package main

import (
	"encoding/json"
	"testing"
)

func TestPacketsRoundTripMetadata(t *testing.T) {
	data := []byte("abcdefghijklmnopqrstuvwxyz")
	setupBytes, frames, err := makePackets("/tmp/test.txt", data, 10, 333)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 3 {
		t.Fatalf("got %d frames", len(frames))
	}
	var setup setupPacket
	if err := json.Unmarshal(setupBytes, &setup); err != nil {
		t.Fatal(err)
	}
	if setup.Kind != "setup" || setup.FrameDelayMS != 333 || setup.Total != 3 {
		t.Fatalf("bad setup: %+v", setup)
	}
	var first, last packet
	if err := json.Unmarshal(frames[0], &first); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(frames[2], &last); err != nil {
		t.Fatal(err)
	}
	if first.Name != "test.txt" || first.Size != len(data) || first.Total != 3 {
		t.Fatalf("bad metadata: %+v", first)
	}
	if first.Kind != "data" {
		t.Fatalf("bad packet kind: %s", first.Kind)
	}
	if first.ID != last.ID || first.FileHash != last.FileHash || last.Index != 2 {
		t.Fatal("inconsistent transfer")
	}
}

func TestCompressesWhenSmaller(t *testing.T) {
	data := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	setupBytes, frames, err := makePackets("/tmp/repeated.txt", data, 80, 333)
	if err != nil {
		t.Fatal(err)
	}
	var setup setupPacket
	if err := json.Unmarshal(setupBytes, &setup); err != nil {
		t.Fatal(err)
	}
	if setup.Compression != "gzip" {
		t.Fatalf("expected gzip compression, got %+v", setup)
	}
	if setup.OriginalSize != len(data) || setup.OriginalFileHash == "" {
		t.Fatalf("missing original metadata: %+v", setup)
	}
	if setup.Size >= len(data) {
		t.Fatalf("compressed payload was not smaller: %+v", setup)
	}
	if len(frames) != setup.Total {
		t.Fatal("frames/setup mismatch")
	}
}
