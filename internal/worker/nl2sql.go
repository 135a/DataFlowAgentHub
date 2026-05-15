package worker

import (
	"context"

	nlv1 "github.com/dataflowagenthub/hub/internal/gen/nl2sql/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// NL2SQLClient wraps the gRPC NL2SQL worker client.
type NL2SQLClient struct {
	c nlv1.NL2SQLServiceClient
}

func DialNL2SQL(addr string) (*grpc.ClientConn, *NL2SQLClient, error) {
	// grpc.Dial is deprecated but keeps compatibility with older grpc-go installs.
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	return conn, &NL2SQLClient{c: nlv1.NewNL2SQLServiceClient(conn)}, nil
}

func (w *NL2SQLClient) GenerateSQL(ctx context.Context, traceID, sessionID, userMessage, schemaJSON, dialect string) (*nlv1.GenerateSQLResponse, error) {
	md := metadata.Pairs("x-trace-id", traceID)
	ctx = metadata.NewOutgoingContext(ctx, md)
	return w.c.GenerateSQL(ctx, &nlv1.GenerateSQLRequest{
		TraceId:     traceID,
		SessionId:   sessionID,
		UserMessage: userMessage,
		SchemaJson:  schemaJSON,
		Dialect:     dialect,
	})
}

func (w *NL2SQLClient) Health(ctx context.Context) (*nlv1.HealthResponse, error) {
	return w.c.Health(ctx, &nlv1.HealthRequest{})
}
