package filetransfer

import "net/rpc"

type FileTransferClient struct {
	*rpc.Client
}

func DialFileTransferService(address string) (*FileTransferClient, error) {
	c, err := rpc.Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	return &FileTransferClient{c}, nil
}

func (c *FileTransferClient) RemoteCall(method string, arg interface{}, reply interface{}) error {
	return c.Client.Call(FileTransferServiceName+method, arg, reply)
}

func (c *FileTransferClient) FetchPiece(torrent string, pieceID uint32, piece *FilePiece) error {
	return c.RemoteCall(".FetchPiece", FetchPieceRequest{torrent, pieceID}, piece)
}

func (c *FileTransferClient) CheckPiece(torrent string, pieceID uint32, exist *bool) error {
	return c.RemoteCall(".CheckPiece", FetchPieceRequest{torrent, pieceID}, exist)
}

func (c *FileTransferClient) AddTorrent(request string, reply *bool) error {
	return c.RemoteCall(".AddTorrent", request, reply)
}

func (c *FileTransferClient) RemoveTorrent(request string, reply *bool) error {
	return c.RemoteCall(".RemoveTorrent", request, reply)
}
