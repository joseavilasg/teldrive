package reader

import (
	"fmt"
	"io"
	"net/http"

	"github.com/divyam234/drive/types"
)

type linearReader struct {
	parts         []types.Part
	pos           int
	reader        io.ReadCloser
	bytesread     int64
	contentLength int64
}

func NewLinearReader(parts []types.Part, contentLength int64) (io.ReadCloser, error) {

	r := &linearReader{
		parts:         parts,
		contentLength: contentLength,
	}

	res, err := r.nextPart()

	if err != nil {
		return nil, err
	}

	r.reader = res

	return r, nil
}

func (r *linearReader) Read(p []byte) (n int, err error) {

	if r.bytesread == r.contentLength {
		return 0, io.EOF
	}

	n, err = r.reader.Read(p)

	if err == io.EOF || n == 0 {
		r.pos++
		if r.pos == len(r.parts) {
			return 0, io.EOF
		}
		r.reader, err = r.nextPart()
		if err != nil {
			return 0, err
		}
	}

	r.bytesread += int64(n)

	return n, nil
}

func (r *linearReader) Close() (err error) {
	if r.reader != nil {
		err = r.reader.Close()
		r.reader = nil
	}
	return
}

func (r *linearReader) nextPart() (io.ReadCloser, error) {

	req, err := http.NewRequest("GET", r.parts[r.pos].Url, nil)

	if err != nil {
		return nil, err
	}

	rangeHeader := fmt.Sprintf("bytes=%d-%d", r.parts[r.pos].Start, r.parts[r.pos].End)

	req.Header.Set("Range", rangeHeader)

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}
