// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"go.opentelemetry.io/otel/api/correlation"
	"go.opentelemetry.io/otel/api/key"
	"io"
	"log"
	"time"

	"github.com/shouhe_masuyama/opentelemetry-sample-go/grpc/api"
	"github.com/shouhe_masuyama/opentelemetry-sample-go/grpc/config"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/plugin/grpctrace"

	"google.golang.org/grpc"
)

func main() {
	config.Init()

	ctx := correlation.NewContext(context.Background(),
		key.String("test", "123"),
	)
	tracer := global.TraceProvider().Tracer("")

	var conn *grpc.ClientConn
	conn, err := grpc.Dial(":7777", grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(grpctrace.UnaryClientInterceptor(tracer)),
		grpc.WithStreamInterceptor(grpctrace.StreamClientInterceptor(tracer)),
	)

	if err != nil {
		log.Fatalf("did not connect: %s", err)
	}
	defer func() { _ = conn.Close() }()

	client := api.NewHelloServiceClient(conn)

	_ = tracer.WithSpan(ctx, "HelloRequest",
		func(ctx context.Context) error {
			callSayHello(client, ctx)
			tracer.WithSpan(ctx, "",
				func(ctx context.Context) error {
					callSayHelloClientStream(client, ctx)
					tracer.WithSpan(ctx, "",
						func(ctx context.Context) error {
							callSayHelloServerStream(client, ctx)
							tracer.WithSpan(ctx, "",
								func(ctx context.Context) error {
									callSayHelloBidiStream(client, ctx)
									return nil
								})
							return nil
						})
					return nil
				})
			return nil
		})
	time.Sleep(10 * time.Millisecond)
}

func callSayHello(c api.HelloServiceClient, ctx context.Context) {
	response, err := c.SayHello(ctx, &api.HelloRequest{Greeting: "World"})
	if err != nil {
		log.Fatalf("Error when calling SayHello: %s", err)
	}
	log.Printf("Response from server: %s", response.Reply)
}

func callSayHelloClientStream(c api.HelloServiceClient, ctx context.Context) {
	stream, err := c.SayHelloClientStream(ctx)
	if err != nil {
		log.Fatalf("Error when opening SayHelloClientStream: %s", err)
	}

	for i := 0; i < 5; i++ {
		err := stream.Send(&api.HelloRequest{Greeting: "World"})

		time.Sleep(time.Duration(i*50) * time.Millisecond)

		if err != nil {
			log.Fatalf("Error when sending to SayHelloClientStream: %s", err)
		}
	}

	response, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("Error when closing SayHelloClientStream: %s", err)
	}

	log.Printf("Response from server: %s", response.Reply)
}

func callSayHelloServerStream(c api.HelloServiceClient, ctx context.Context) {
	stream, err := c.SayHelloServerStream(ctx, &api.HelloRequest{Greeting: "World"})
	if err != nil {
		log.Fatalf("Error when opening SayHelloServerStream: %s", err)
	}

	for {
		response, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("Error when receiving from SayHelloServerStream: %s", err)
		}

		log.Printf("Response from server: %s", response.Reply)
		time.Sleep(50 * time.Millisecond)
	}
}

func callSayHelloBidiStream(c api.HelloServiceClient, ctx context.Context) {
	stream, err := c.SayHelloBidiStream(ctx)
	if err != nil {
		log.Fatalf("Error when opening SayHelloBidiStream: %s", err)
	}

	serverClosed := make(chan struct{})
	clientClosed := make(chan struct{})

	go func() {
		for i := 0; i < 5; i++ {
			err := stream.Send(&api.HelloRequest{Greeting: "World"})

			if err != nil {
				log.Fatalf("Error when sending to SayHelloBidiStream: %s", err)
			}

			time.Sleep(50 * time.Millisecond)
		}

		err := stream.CloseSend()
		if err != nil {
			log.Fatalf("Error when closing SayHelloBidiStream: %s", err)
		}

		clientClosed <- struct{}{}
	}()

	go func() {
		for {
			response, err := stream.Recv()
			if err == io.EOF {
				break
			} else if err != nil {
				log.Fatalf("Error when receiving from SayHelloBidiStream: %s", err)
			}

			log.Printf("Response from server: %s", response.Reply)
			time.Sleep(50 * time.Millisecond)
		}

		serverClosed <- struct{}{}
	}()

	// Wait until client and server both closed the connection.
	<-clientClosed
	<-serverClosed
}
