package filetransfer

import (
	"dtorrent/chord"
	"dtorrent/internal"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type FileTransferServer struct {
	online atomic.Bool

	Addr string

	repo_path, torrent_path string

	activeTorrent     map[string](*map[string]interface{})
	activeTorrentLock sync.RWMutex

	listener net.Listener

	node *chord.ChordNode
}

func NewFileTransferServer(repo_path, torrent_path string, node *chord.ChordNode) *FileTransferServer {
	s := &FileTransferServer{
		repo_path:     repo_path,
		torrent_path:  torrent_path,
		activeTorrent: make(map[string]*map[string]interface{}),
		node:          node,
	}
	rpc.RegisterName(FileTransferServiceName, s)
	return s
}

func (s *FileTransferServer) Run(listen_addr, chord_addr string) error {
	var err error
	s.listener, err = net.Listen("tcp", listen_addr)
	s.Addr = listen_addr
	if err != nil {
		err = fmt.Errorf("listen error: %s", err)
		fmt.Println(err)
		return err
	}
	if err != nil {
		err = fmt.Errorf("chord dial error: %s", err)
		fmt.Println(err)
		return err
	}
	s.online.Store(true)
	go func() {
		for s.online.Load() {
			conn, err := s.listener.Accept()
			if err != nil {
				err = fmt.Errorf("accept error: %s", err)
				fmt.Println(err)
				break
			}
			go rpc.ServeConn(conn)
		}
	}()
	go func() {
		for s.online.Load() {
			s.MaintainPieceAvailInfo()
			time.Sleep(10 * time.Second)
		}
	}()
	return nil
}

func (s *FileTransferServer) Close() {
	s.online.Store(false)
	s.listener.Close()
}

func AddAddrToList(addrs, addr string) string {
	list := strings.Split(addrs, "|")
	for _, s := range list {
		if s == addr {
			return addrs
		}
	}
	return strings.Join(append(list, addr), "|")
}

func (s *FileTransferServer) MaintainPieceAvailInfo() {
	s.activeTorrentLock.RLock()
	for _, info := range s.activeTorrent {
		pieces := (*info)["PieceHash"].([]interface{})
		for _, pID := range pieces {
			id := pID.(string)
			ok, reply := s.node.Get(id)
			if ok {
				newList := AddAddrToList(reply, s.Addr)
				if newList != reply {
					s.node.Put(id, newList)
				}
			} else {
				s.node.Put(id, s.Addr)
			}
		}
	}
	s.activeTorrentLock.RUnlock()
}

func (s *FileTransferServer) FetchPiece(request FetchPieceRequest, piece *FilePiece) error {
	s.activeTorrentLock.RLock()
	info, exist := s.activeTorrent[request.TorrentName]
	s.activeTorrentLock.RUnlock()
	if !exist {
		err := fmt.Errorf("torrent named %s not active", request.TorrentName)
		fmt.Println(err)
		return err
	}
	pieceCount := (*info)["PieceCount"].(uint32)
	if request.PieceID >= pieceCount {
		err := fmt.Errorf("piece id %d exceed total piece size %d", request.PieceID, pieceCount)
		fmt.Println(err)
		return err
	}
	fn := (*info)["FileName"].(string)
	fh, err := os.Open(fn)
	if err != nil {
		err := fmt.Errorf("file %s cannot open with error: %s", fn, err)
		fmt.Println(err)
		return err
	}
	defer fh.Close()
	readSize, err := fh.ReadAt(piece.Content[:], int64(pieceCount)*FilePieceSize)
	if err != nil && err != io.EOF {
		err = fmt.Errorf("read %s at %d failed with %s", fn, int64(pieceCount)*FilePieceSize, err)
		fmt.Println(err)
		return err
	}
	piece.Size = uint32(readSize)
	pieceHash := internal.Bytes_str_sh1Abase64(piece.Content[:])
	expectedHash := (*info)["PieceHash"].([]string)[pieceCount]
	if pieceHash != expectedHash {
		err = fmt.Errorf("file %s piece %d expected hash %s but got %s", fn, pieceCount, expectedHash, pieceHash)
		fmt.Println(err)
		return err
	}
	fmt.Printf("sending %s : %d ...\n", fn, pieceCount)
	return nil
}

func (s *FileTransferServer) CheckPiece(request FetchPieceRequest, exist *bool) error {
	s.activeTorrentLock.RLock()
	info, ok := s.activeTorrent[request.TorrentName]
	s.activeTorrentLock.RUnlock()
	if !ok {
		err := fmt.Errorf("torrent named %s not active", request.TorrentName)
		fmt.Println(err)
		return err
	}
	pieceCount := (*info)["PieceCount"].(uint32)
	if request.PieceID >= pieceCount {
		err := fmt.Errorf("piece id %d exceed total piece size %d", request.PieceID, pieceCount)
		fmt.Println(err)
		return err
	}
	fn := (*info)["FileName"].(string)
	fh, err := os.Open(fn)
	if err != nil {
		err := fmt.Errorf("file %s cannot open with error: %s", fn, err)
		fmt.Println(err)
		return err
	}
	defer fh.Close()
	var buffer [FilePieceSize]byte
	_, err = fh.ReadAt(buffer[:], int64(pieceCount)*FilePieceSize)
	if err != nil && err != io.EOF {
		err = fmt.Errorf("read %s at %d failed with %s", fn, int64(pieceCount)*FilePieceSize, err)
		fmt.Println(err)
		return err
	}
	pieceHash := internal.Bytes_str_sh1Abase64(buffer[:])
	expectedHash := (*info)["PieceHash"].([]string)[pieceCount]
	if pieceHash != expectedHash {
		err = fmt.Errorf("file %s piece %d expected hash %s but got %s", fn, pieceCount, expectedHash, pieceHash)
		fmt.Println(err)
		return err
	}
	fmt.Printf("checked %s : %d OK\n", fn, pieceCount)
	return nil
}

func (s *FileTransferServer) AddTorrent(request string, reply *bool) error {
	dtpath := filepath.Join(s.torrent_path, request+DTorrentExtension)
	fh, err := os.Open(dtpath)
	if err != nil {
		err = fmt.Errorf("cannot open torrent %s with error %s", dtpath, err)
		fmt.Println(err)
		return err
	}
	defer fh.Close()
	fi, _ := fh.Stat()
	buffer := make([]byte, fi.Size())
	size, err := fh.Read(buffer)
	if err != nil {
		err = fmt.Errorf("cannot read torrent %s with error %s", dtpath, err)
		fmt.Println(err)
		return err
	}
	if size != int(fi.Size()) {
		err = fmt.Errorf("failed to readall torrent %s (expected size: %d actual size: %d)", dtpath, fi.Size(), size)
		fmt.Println(err)
		return err
	}
	torrentInfo := make(map[string]interface{})
	err = json.Unmarshal(buffer, &torrentInfo)
	if err != nil {
		err = fmt.Errorf("failed to load torrent information with error %s", err)
		fmt.Println(err)
		return err
	}
	tname := torrentInfo["TorrentName"].(string)
	fmt.Println("AddTorrent: add ", tname)
	s.activeTorrentLock.Lock()
	s.activeTorrent[tname] = &torrentInfo
	s.activeTorrentLock.Unlock()
	*reply = true
	return nil
}

func (s *FileTransferServer) RemoveTorrent(request string, reply *bool) error {
	s.activeTorrentLock.Lock()
	defer s.activeTorrentLock.Unlock()
	_, *reply = s.activeTorrent[request]
	if !*reply {
		err := fmt.Errorf("torrent %s not exist", request)
		fmt.Println(err)
		return err
	}
	delete(s.activeTorrent, request)
	return nil
}
