//go:build linux

package minimal

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vbatts/tar-split/archive/tar"
)

func TestGenerateAndReadFooter(t *testing.T) {
	footer := ZstdChunkedFooterData{
		ManifestType:               1,
		Offset:                     2,
		LengthCompressed:           3,
		LengthUncompressed:         4,
		OffsetTarSplit:             5,
		LengthCompressedTarSplit:   6,
		LengthUncompressedTarSplit: 7,
		ChecksumAnnotationTarSplit: "", // unused
	}
	b := footerDataToBlob(footer)
	assert.Len(t, b, FooterSizeSupported)

	footer2, err := readFooterDataFromBlob(b)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, footer, footer2)
}

// readFooterDataFromBlob reads the zstd:chunked footer from the binary buffer.
func readFooterDataFromBlob(footer []byte) (ZstdChunkedFooterData, error) {
	var footerData ZstdChunkedFooterData

	if len(footer) < FooterSizeSupported {
		return footerData, errors.New("blob too small")
	}
	footerData.Offset = binary.LittleEndian.Uint64(footer[0:8])
	footerData.LengthCompressed = binary.LittleEndian.Uint64(footer[8:16])
	footerData.LengthUncompressed = binary.LittleEndian.Uint64(footer[16:24])
	footerData.ManifestType = binary.LittleEndian.Uint64(footer[24:32])
	footerData.OffsetTarSplit = binary.LittleEndian.Uint64(footer[32:40])
	footerData.LengthCompressedTarSplit = binary.LittleEndian.Uint64(footer[40:48])
	footerData.LengthUncompressedTarSplit = binary.LittleEndian.Uint64(footer[48:56])

	// the magic number is stored in the last 8 bytes
	if !bytes.Equal(ZstdChunkedFrameMagic, footer[len(footer)-len(ZstdChunkedFrameMagic):]) {
		return footerData, errors.New("invalid magic number")
	}
	return footerData, nil
}

func TestNewFileMetadataTimesAreUTC(t *testing.T) {
	loc := time.FixedZone("test", 3*60*60)
	hdr := &tar.Header{
		Typeflag:   tar.TypeReg,
		Name:       "file",
		ModTime:    time.Date(2023, 1, 1, 3, 0, 0, 0, loc),
		AccessTime: time.Date(2023, 1, 2, 3, 0, 0, 0, loc),
		ChangeTime: time.Date(2023, 1, 3, 3, 0, 0, 0, loc),
	}
	fm, err := NewFileMetadata(hdr)
	require.NoError(t, err)
	for _, tm := range []*time.Time{fm.ModTime, fm.AccessTime, fm.ChangeTime} {
		require.NotNil(t, tm)
		assert.Equal(t, time.UTC, tm.Location())
	}
	assert.Equal(t, time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), *fm.ModTime)

	b, err := json.Marshal(fm)
	require.NoError(t, err)
	assert.Contains(t, string(b), `"modtime":"2023-01-01T00:00:00Z"`)
	assert.Contains(t, string(b), `"accesstime":"2023-01-02T00:00:00Z"`)
	assert.Contains(t, string(b), `"changetime":"2023-01-03T00:00:00Z"`)

	hdr = &tar.Header{Typeflag: tar.TypeReg, Name: "file"}
	fm, err = NewFileMetadata(hdr)
	require.NoError(t, err)
	assert.Nil(t, fm.ModTime)
	assert.Nil(t, fm.AccessTime)
	assert.Nil(t, fm.ChangeTime)
}
