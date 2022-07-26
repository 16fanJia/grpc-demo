package main

import (
	"bufio"
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"grpc/pb"
	"io"
	"log"
	"os"
	"time"
)

func main() {
	var (
		err  error
		conn *grpc.ClientConn
	)
	if conn, err = grpc.Dial("localhost:5000", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock()); err != nil {
		log.Fatalf("grpc client dial fail:%s", err.Error())
	}

	defer conn.Close()

	client := pb.NewRouteGuideClient(conn)
	//runFirst(client)
	//runSecond(client)
	//runThird(client)
	runForth(client)
}

//run test one demo
func runFirst(client pb.RouteGuideClient) {
	in := &pb.Point{
		Latitude:  310235000,
		Longitude: 121437403,
	}
	feature, err := client.GetFeature(context.Background(), in)
	if err != nil {
		log.Fatalf("GetFeature err:%s", err.Error())
	}

	fmt.Println(feature)
}

func runSecond(client pb.RouteGuideClient) {
	in := &pb.Rectangle{
		Low: &pb.Point{
			Latitude:  313374060,
			Longitude: 121358540,
		},
		High: &pb.Point{
			Latitude:  311034130,
			Longitude: 121598790,
		},
	}
	serverStream, err := client.ListFeatures(context.Background(), in)
	if err != nil {
		log.Fatalf("ListFeatures err:%s", err.Error())
	}

	var feature *pb.Feature

	for {
		if feature, err = serverStream.Recv(); err != nil {
			if err == io.EOF {
				break
			}
			log.Fatalf("rec Feature fail err:%s", err.Error())
		}
		fmt.Println(feature)
	}
}

func runThird(client pb.RouteGuideClient) {
	//eg client input
	points := []*pb.Point{
		{Latitude: 313374060, Longitude: 121358540},
		{Latitude: 311034130, Longitude: 121598790},
		{Latitude: 310235000, Longitude: 121437403},
	}

	clientStream, err := client.RecordRoute(context.Background())
	if err != nil {
		log.Fatalf("clientStream err:%s", err.Error())
	}

	//client stream input
	for _, point := range points {
		//客户端每间隔一秒给服务端上传一个点
		if err := clientStream.Send(point); err != nil {
			log.Fatalf("client input stream err:%s", err.Error())
		}
		time.Sleep(1 * time.Second)
	}

	routeSummary, err := clientStream.CloseAndRecv()
	if err != nil {
		log.Fatalf("client rec stream err :%s", err.Error())
	}

	fmt.Println(routeSummary)
}

func runForth(client pb.RouteGuideClient) {
	twoWayStream, err := client.Recommend(context.Background())
	if err != nil {
		log.Fatalf("client twoWayStream err:%s", err.Error())
	}

	//this goroutine listen to the server stream
	go func() {
		feature, errRec := twoWayStream.Recv()
		if errRec != nil {
			log.Fatalf("client twoWayStream rec stream err:%s", err.Error())
		}
		fmt.Printf("twoWayStream recv :%s", feature)
	}()

	reader := bufio.NewReader(os.Stdin)
	//用户输入
	for {
		request := pb.RecommendationRequest{Point: new(pb.Point)}
		var mode int32
		fmt.Print("Enter Recommendation Mode (0 for farthest, 1 for the nearest)")
		readIntFromCommandLine(reader, &mode)
		fmt.Print("Enter Latitude ")
		readIntFromCommandLine(reader, &request.Point.Latitude)
		fmt.Print("Enter longitude ")
		readIntFromCommandLine(reader, &request.Point.Longitude)

		request.Mode = pb.RecommendationMode(mode)

		if err := twoWayStream.Send(&request); err != nil {
			log.Fatalf("twoWayStream send err:%s", err.Error())
		}

		time.Sleep(2 * time.Second)

	}

}

func readIntFromCommandLine(reader *bufio.Reader, target *int32) {
	_, err := fmt.Fscanf(reader, "%d\n", target)
	if err != nil {
		log.Fatalln("Cannot scan", err)
	}
}
