package worker

import (
	"context"

	nlv1 "github.com/dataflowagenthub/hub/internal/gen/nl2sql/v1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// NL2SQLClient wraps the gRPC NL2SQL worker client.
type NL2SQLClient struct {
	c nlv1.NL2SQLServiceClient
}

func DialNL2SQL(addr string) (*grpc.ClientConn, *NL2SQLClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	return conn, &NL2SQLClient{c: nlv1.NewNL2SQLServiceClient(conn)}, nil
}

// injectTraceContext propagates W3C trace context into outgoing gRPC metadata.
func injectTraceContext(ctx context.Context) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}
	otel.GetTextMapPropagator().Inject(ctx, &metadataCarrier{md: &md})
	return metadata.NewOutgoingContext(ctx, md)
}

func (w *NL2SQLClient) GenerateSQL(ctx context.Context, traceID, sessionID, userMessage, schemaJSON, dialect string) (*nlv1.GenerateSQLResponse, error) {
	md := metadata.Pairs("x-trace-id", traceID)
	ctx = metadata.NewOutgoingContext(ctx, md)
	ctx = injectTraceContext(ctx)
	return w.c.GenerateSQL(ctx, &nlv1.GenerateSQLRequest{
		TraceId:     traceID,
		SessionId:   sessionID,
		UserMessage: userMessage,
		SchemaJson:  schemaJSON,
		Dialect:     dialect,
	})
}

func (w *NL2SQLClient) RunAgentPipeline(ctx context.Context, traceID, sessionID, runID, userMessage, schemaJSON string) (*nlv1.RunAgentPipelineResponse, error) {
	md := metadata.Pairs("x-trace-id", traceID)
	ctx = metadata.NewOutgoingContext(ctx, md)
	ctx = injectTraceContext(ctx)
	return w.c.RunAgentPipeline(ctx, &nlv1.RunAgentPipelineRequest{
		TraceId:     traceID,
		SessionId:   sessionID,
		RunId:       runID,
		UserMessage: userMessage,
		SchemaJson:  schemaJSON,
	})
}

func (w *NL2SQLClient) Health(ctx context.Context) (*nlv1.HealthResponse, error) {
	return w.c.Health(ctx, &nlv1.HealthRequest{})
}

// metadataCarrier adapts metadata.MD to the propagation.TextMapCarrier interface.
var _ propagation.TextMapCarrier = (*metadataCarrier)(nil)

type metadataCarrier struct {
	md *metadata.MD
}

func (c *metadataCarrier) Get(key string) string {
	vs := (*c.md).Get(key)
	if len(vs) == 0 {
		return ""
	}
	return vs[0]
}

func (c *metadataCarrier) Set(key, value string) {
	(*c.md).Set(key, value)
}

func (c *metadataCarrier) Keys() []string {
	keys := make([]string, 0, len(*c.md))
	for k := range *c.md {
		keys = append(keys, k)
	}
	return keys
}
