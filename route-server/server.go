package main

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"grpc/pb"
	"io"
	"log"
	"math"
	"net"
	"time"
)

type routeGuideServer struct {
	features []*pb.Feature
	pb.UnimplementedRouteGuideServer
}

func (rs *routeGuideServer) GetFeature(ctx context.Context, point *pb.Point) (*pb.Feature, error) {
	for _, feature := range rs.features {
		if proto.Equal(feature.Location, point) {
			return feature, nil
		}
	}
	return nil, nil
}

// check if a point is inside a rectangle
func inRange(point *pb.Point, rect *pb.Rectangle) bool {
	left := math.Min(float64(rect.Low.Longitude), float64(rect.High.Longitude))
	right := math.Max(float64(rect.Low.Longitude), float64(rect.High.Longitude))
	top := math.Max(float64(rect.Low.Latitude), float64(rect.High.Latitude))
	bottom := math.Min(float64(rect.Low.Latitude), float64(rect.High.Latitude))

	if float64(point.Longitude) >= left &&
		float64(point.Longitude) <= right &&
		float64(point.Latitude) >= bottom &&
		float64(point.Latitude) <= top {
		return true
	}
	return false
}

func (rs *routeGuideServer) ListFeatures(rectangle *pb.Rectangle, stream pb.RouteGuide_ListFeaturesServer) error {
	for _, feature := range rs.features {
		if inRange(feature.Location, rectangle) {
			err := stream.Send(feature)
			if err != nil {
				log.Fatalf("stream send err :%s", err.Error())
			}
		}
	}

	return nil
}

func toRadians(num float64) float64 {
	return num * math.Pi / float64(180)
}

// calcDistance calculates the distance between two points using the "haversine" formula.
// The formula is based on http://mathforum.org/library/drmath/view/51879.html.
func calcDistance(p1 *pb.Point, p2 *pb.Point) int32 {
	const CordFactor float64 = 1e7
	const R = float64(6371000) // earth radius in metres
	lat1 := toRadians(float64(p1.Latitude) / CordFactor)
	lat2 := toRadians(float64(p2.Latitude) / CordFactor)
	lng1 := toRadians(float64(p1.Longitude) / CordFactor)
	lng2 := toRadians(float64(p2.Longitude) / CordFactor)
	dlat := lat2 - lat1
	dlng := lng2 - lng1

	a := math.Sin(dlat/2)*math.Sin(dlat/2) +
		math.Cos(lat1)*math.Cos(lat2)*
			math.Sin(dlng/2)*math.Sin(dlng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	distance := R * c
	return int32(distance)
}
func (rs *routeGuideServer) RecordRoute(stream pb.RouteGuide_RecordRouteServer) error {
	startTime := time.Now()
	var pointCount, distance int32
	var prevPoint *pb.Point
	for {
		point, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				//conclude a route summary
				endTime := time.Now()
				return stream.SendAndClose(&pb.RouteSummary{
					PointCount:  pointCount,
					Distance:    distance,
					ElapsedTime: int32(endTime.Sub(startTime).Seconds()),
				})
			}
			log.Fatalf("server rec data fail err:%s", err.Error())
		}
		pointCount++
		if prevPoint != nil {
			distance += calcDistance(prevPoint, point)
		}
		prevPoint = point
	}

	return nil

}

//返回最远 或者最近的 point 的feature
func (rs *routeGuideServer) recommendOnce(request *pb.RecommendationRequest) (*pb.Feature, error) {
	var nearest, farthest *pb.Feature
	var nearestDistance, farthestDistance int32

	for _, feature := range rs.features {
		distance := calcDistance(feature.Location, request.Point)
		if nearest == nil || distance < nearestDistance {
			nearestDistance = distance
			nearest = feature
		}
		if farthest == nil || distance > farthestDistance {
			farthestDistance = distance
			farthest = feature
		}
	}
	if request.Mode == pb.RecommendationMode_GetFarthest {
		return farthest, nil
	} else {
		return nearest, nil
	}
}
func (rs *routeGuideServer) Recommend(stream pb.RouteGuide_RecommendServer) error {
	var (
		err         error
		request     *pb.RecommendationRequest
		recommended *pb.Feature
	)
	for {
		if request, err = stream.Recv(); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		//计算
		if recommended, err = rs.recommendOnce(request); err != nil {
			return err
		}
		//data send to client
		if err = stream.Send(recommended); err != nil {
			return err
		}
	}
}

func NewServer() *routeGuideServer {
	return &routeGuideServer{
		features: []*pb.Feature{
			{Name: "上海交通大学闵行校区 上海市闵行区东川路800号", Location: &pb.Point{
				Latitude:  310235000,
				Longitude: 121437403,
			}},
			{Name: "复旦大学 上海市杨浦区五角场邯郸路220号", Location: &pb.Point{
				Latitude:  312978870,
				Longitude: 121503457,
			}},
			{Name: "华东理工大学 上海市徐汇区梅陇路130号", Location: &pb.Point{
				Latitude:  311416130,
				Longitude: 121424904,
			}},
		},
	}
}

func main() {
	var (
		err      error
		listener net.Listener
	)
	if listener, err = net.Listen("tcp", "localhost:5000"); err != nil {
		log.Fatalln("cannot create a listener at the address")
	}

	grpcServer := grpc.NewServer()
	//register
	pb.RegisterRouteGuideServer(grpcServer, NewServer())
	if err = grpcServer.Serve(listener); err != nil {
		log.Fatalln("grpcServer start fail")
	}

}
