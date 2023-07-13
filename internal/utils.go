package internal

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
)

func Str_uint32_sha1(val string) uint32 {
	sha := sha1.Sum([]byte(val))
	var hsh uint32
	for i := 0; i < 4; i++ {
		hsh <<= 8
		hsh += uint32(sha[i])
	}
	return hsh
}

func Bytes_str_sh1Abase64(buffer []byte) string {
	var b [20]byte = sha1.Sum(buffer)
	return base64.StdEncoding.EncodeToString(b[:])
}

type ValueEntry struct {
	Key   string
	Value string
}

func LoadConfigurationFile(filename string) (map[string]string, error) {
	info := make(map[string]string)
	fh, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	fi, _ := fh.Stat()
	defer fh.Close()
	buf := make([]byte, fi.Size())
	_, err = fh.Read(buf)
	if err != nil && err != io.EOF {
		return nil, err
	}
	err = json.Unmarshal(buf, &info)
	if err != nil {
		return nil, err
	}
	return info, nil
}
