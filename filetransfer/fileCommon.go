package filetransfer

import (
	"dtorrent/internal"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const DTorrentExtension = ".json"
const FileTransferServiceName = "filetransfer.IFileTransferService"

type FetchPieceRequest struct {
	TorrentName string
	PieceID     uint32
}

const FilePieceSize = 2 * 1024

type FilePiece struct {
	Size    uint32
	Content [FilePieceSize]byte
}

type IFileTransferService interface {
	FetchPiece(request FetchPieceRequest, piece *FilePiece) error

	CheckPiece(request FetchPieceRequest, exist *bool) error

	AddTorrent(request string, reply *bool) error

	RemoveTorrent(request string, reply *bool) error
}

func MakeTorrentSeed(path, filename, outputdir string) error {
	full_filename := filepath.Join(path, filename)
	info := make(map[string]interface{})
	fh, err := os.Open(full_filename)
	if err != nil {
		return fmt.Errorf("cannot open file %s with error %s", full_filename, err)
	}
	info["FileName"] = filename
	pieceCnt := 0
	var buffer [FilePieceSize]byte
	var pieceHash []string
	for {
		_, err := fh.Read(buffer[:])
		if err != nil {
			fh.Close()
			if err == io.EOF {
				break
			}
			return err
		}
		pieceCnt++
		pieceHash = append(pieceHash, internal.Bytes_str_sh1Abase64(buffer[:]))
	}
	info["PieceCount"] = pieceCnt
	info["PieceHash"] = pieceHash
	info["TorrentName"] = internal.Bytes_str_sh1Abase64([]byte(filename + time.Now().Format(time.ANSIC)))
	outputfilename := filepath.Join(outputdir, filename+DTorrentExtension)
	fh, err = os.OpenFile(outputfilename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("cannot open outpput file %s with error %s", outputfilename, err)
	}
	outputContent, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("cannot serialize torrent info with error %s", err)
	}
	_, err = fh.Write(outputContent)
	fh.Close()
	return err
}
