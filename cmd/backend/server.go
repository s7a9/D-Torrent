package main

import (
	"dtorrent/chord"
	"dtorrent/filetransfer"
	"dtorrent/internal"
	"fmt"
	"log"
	"time"

	"flag"
)

func main() {
	var cfgfilename, chordPeerAddr string
	var newNetwork bool
	flag.StringVar(&cfgfilename, "cfg", "./dtconfig.json", "配置文件路径")
	flag.StringVar(&chordPeerAddr, "peer", "localhost:20000", "DHT同伴地址")
	flag.BoolVar(&newNetwork, "new", false, "是否新建一个DHT网络")
	flag.Parse()
	cfginfo, err := internal.LoadConfigurationFile(cfgfilename)
	if err != nil {
		log.Fatal("failed to load configuration ", err)
	}
	log.Println("starting dht service...")
	chordServer := chord.CreateChordNode(cfginfo["DHTAddr"])
	chordServer.Run()
	defer chordServer.Quit()
	if newNetwork {
		chordServer.Create()
	} else {
		if !chordServer.Join(chordPeerAddr) {
			log.Fatal("failed to join dht network, please check log")
		}
	}
	time.Sleep(400 * time.Millisecond)
	log.Println("dht service running on ", chordServer.Addr)
	fileServer := filetransfer.NewFileTransferServer(cfginfo["RepoPath"], cfginfo["TorrentPath"], chordServer)
	err = fileServer.Run(cfginfo["FileAddr"], cfginfo["DHTAddr"])
	if err != nil {
		log.Printf("failed to run file service %s\n", err)
		return
	}
	defer fileServer.Close()
	log.Println("file transfer service running on ", fileServer.Addr)
	for {
		fmt.Println("==== enter exit to shutdown server ====")
		var input string
		fmt.Scanf("%s", &input)
		if input == "exit" {
			break
		}
	}
}
