package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"github.com/tilezen/tapalcatl"
	"io"
	"net/http"
	"testing"
)

func makeTestZip(tile tapalcatl.TileCoord, content string) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	f, err := w.Create(tile.FileName())
	if err != nil {
		return nil, fmt.Errorf("Unable to create file %#v in zip: %s", tile.FileName(), err.Error())
	}
	_, err = f.Write([]byte("{}"))
	if err != nil {
		return nil, fmt.Errorf("Unable to write JSON file to zip: %s", err.Error())
	}
	err = w.Close()
	if err != nil {
		return nil, fmt.Errorf("Error while finalizing zip file: %s", err.Error())
	}
	return buf, nil
}

type fakeParser struct {
	tile tapalcatl.TileCoord
}

func (f *fakeParser) Parse(_ *http.Request) (tapalcatl.TileCoord, error) {
	return f.tile, nil
}

type fakeStorage struct {
	storage map[tapalcatl.TileCoord]*Response
}

func (f *fakeStorage) Get(t tapalcatl.TileCoord) (*Response, error) {
	resp, ok := f.storage[t]
	if ok {
		return resp, nil
	} else {
		return &Response{StatusCode: 404}, nil
	}
}

type fakeProxy struct {
	count int
}

func (f *fakeProxy) ServeHTTP(rw http.ResponseWriter, _ *http.Request) {
	f.count += 1

	rw.WriteHeader(http.StatusOK)
}

type fakeResponseWriter struct {
	header http.Header
	status int
}

func (f *fakeResponseWriter) Header() http.Header {
	return f.header
}

func (f *fakeResponseWriter) Write(buf []byte) (int, error) {
	return len(buf), nil
}

func (f *fakeResponseWriter) WriteHeader(status int) {
	f.status = status
}

func TestHandlerMiss(t *testing.T) {
	tile := tapalcatl.TileCoord{0, 0, 0, "json"}
	parser := &fakeParser{tile: tile}
	mimes := map[string]string{
		"json": "application/json",
	}
	storage := &fakeStorage{storage: make(map[tapalcatl.TileCoord]*Response)}
	proxy := &fakeProxy{}
	h := MetatileHandler(parser, 1, mimes, storage, proxy)

	rw := &fakeResponseWriter{header: make(http.Header), status: 0}
	req := &http.Request{}
	h.ServeHTTP(rw, req)

	if rw.status != 200 {
		t.Fatalf("Expected 200 OK response, but got %d", rw.status)
	}
	if proxy.count != 1 {
		t.Fatalf("Expected request to hit the proxy, but proxy hits %d != 1", proxy.count)
	}
}

type emptyReadCloser struct{}

func (_ *emptyReadCloser) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (_ *emptyReadCloser) Close() error {
	return nil
}

type bufferReadCloser struct{
	reader *bytes.Reader
}

func (b *bufferReadCloser) Read(p []byte) (int, error) {
	return b.reader.Read(p)
}

func (b *bufferReadCloser) Close() error {
	return nil
}

func TestHandlerHit(t *testing.T) {
	tile := tapalcatl.TileCoord{0, 0, 0, "json"}
	parser := &fakeParser{tile: tile}
	mimes := map[string]string{
		"json": "application/json",
	}
	storage := &fakeStorage{storage: make(map[tapalcatl.TileCoord]*Response)}

	metatile := tapalcatl.TileCoord{0, 0, 0, "zip"}
	zipfile, err := makeTestZip(tile, "{}")
	if err != nil {
		t.Fatalf("Unable to make test zip: %s", err.Error())
	}

	storage.storage[metatile] = &Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       &bufferReadCloser{reader: bytes.NewReader(zipfile.Bytes())},
	}

	proxy := &fakeProxy{}
	h := MetatileHandler(parser, 1, mimes, storage, proxy)

	rw := &fakeResponseWriter{header: make(http.Header), status: 0}
	req := &http.Request{}
	h.ServeHTTP(rw, req)

	if rw.status != 200 {
		t.Fatalf("Expected 200 OK response, but got %d", rw.status)
	}
	if proxy.count != 0 {
		t.Fatalf("Expected request not to hit the proxy, but proxy hits %d != 0", proxy.count)
	}
}