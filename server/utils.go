package server

import (
	"log"
	"time"

	pbkv "github.com/streamingfast/substreams-sink-kv/pb/substreams/sink/kv/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

func ConnectToSinkServer(sinkServerAddress string) pbkv.KvClient {
	conn, err := grpc.Dial(
		sinkServerAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(1024*1024*1024),
			grpc.WaitForReady(true),
		),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                5 * time.Minute, // send pings every ... when there is no activity
			Timeout:             5 * time.Second, // wait that amount of time for ping ack before considering the connection dead
			PermitWithoutStream: false,
		}),
	)
	if err != nil {
		log.Fatalf("failed to connect to the sink server: %v", err)
	}

	return pbkv.NewKvClient(conn)
}
