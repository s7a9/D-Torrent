package main

import (
	"dtorrent/chord"
	"dtorrent/filetransfer"
	"dtorrent/internal"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const HelpDocument = `D-Torrent User Manual
download filename
add      filename
remove   filename
exit
help`

var torrentPath, downloadPath string
var link chord.ChordLink
var client *filetransfer.FileTransferClient

func downloadPiece(filename, pieceHash string, pieceID uint32) error {
	fmt.Println("begin download ", filename, " piece ", pieceID, " hash ", pieceHash)
	var piece filetransfer.FilePiece
	link.GetDataByKey()
}

func download(filename string) {
	dtpath := filepath.Join(torrentPath, filename+filetransfer.DTorrentExtension)
	fh, err := os.Open(dtpath)
	if err != nil {
		err = fmt.Errorf("cannot open torrent %s with error %s", dtpath, err)
		fmt.Println(err)
		return
	}
	defer fh.Close()
	fi, _ := fh.Stat()
	buffer := make([]byte, fi.Size())
	size, err := fh.Read(buffer)
	if err != nil {
		err = fmt.Errorf("cannot read torrent %s with error %s", dtpath, err)
		fmt.Println(err)
		return
	}
	if size != int(fi.Size()) {
		err = fmt.Errorf("failed to readall torrent %s (expected size: %d actual size: %d)", dtpath, fi.Size(), size)
		fmt.Println(err)
		return
	}
	torrentInfo := make(map[string]interface{})
	err = json.Unmarshal(buffer, &torrentInfo)
	if err != nil {
		err = fmt.Errorf("failed to load torrent information with error %s", err)
		fmt.Println(err)
		return
	}
	ch := make(chan int, 5)
	pceLst := torrentInfo["PieceHash"].([]interface{})
	for i, vv := range pceLst {
		ch <- i
		go downloadPiece(filename, vv.(string), uint32(i))
	}
}

func main() {
	var cfgfilename, newTorrentName string
	flag.StringVar(&cfgfilename, "cfg", "./dtconfig.json", "配置文件路径")
	flag.StringVar(&newTorrentName, "new", "", "新建Torrent文件")
	flag.Parse()
	cfginfo, err := internal.LoadConfigurationFile(cfgfilename)
	if err != nil {
		log.Fatal("failed to load configuration ", err)
	}
	if newTorrentName != "" {
		err := filetransfer.MakeTorrentSeed(cfginfo["RepoPath"], newTorrentName, cfginfo["TorrentPath"])
		if err != nil {
			log.Println("failed to make torrent: ", err)
		}
		fmt.Println("torrent created")
		return
	}
	var ope, arg string
	if err := link.Dial(cfginfo["DHTAddr"]); err != nil {
		log.Fatal("connect to local dht server ", cfginfo["DHTAddr"], " failed with ", err)
	}
	defer link.Close()
	client, err = filetransfer.DialFileTransferService(cfginfo["FileAddr"])
	if err != nil {
		log.Println("dial file service failed with ", err)
		return
	}
	torrentPath = cfginfo["TorrentPath"]
	downloadPath = cfginfo["RepoPath"]
	defer client.Close()
	var call_reply bool
	for {
		fmt.Print(">>> ")
		fmt.Scan(&ope)
		ope = strings.ToLower(ope)
		if ope == "download" {
			fmt.Scan(&arg)
			download(arg)
		} else if ope == "add" {
			fmt.Scan(&arg)
			if err := client.AddTorrent(arg, &call_reply); err != nil {
				log.Println(err)
				continue
			}
			log.Println("added torrent ", arg)
		} else if ope == "remove" {
			fmt.Scan(&arg)
			if err := client.RemoveTorrent(arg, &call_reply); err != nil {
				log.Println(err)
				continue
			}
		} else if ope == "exit" {
			break
		} else if ope == "help" {
			fmt.Println(HelpDocument)
		} else {
			fmt.Println("unknown command: ", ope)
		}
	}
}
