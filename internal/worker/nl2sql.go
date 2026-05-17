package worker

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"

	nlv1 "github.com/dataflowagenthub/hub/internal/gen/nl2sql/v1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// NL2SQLClient 封装了 gRPC NL2SQL 工作节点客户端
type NL2SQLClient struct {
	c nlv1.NL2SQLServiceClient
}

// DialOpts 包含 gRPC 拨号选项（支持 mTLS 回退到 insecure）
type DialOpts struct {
	Addr          string
	ClientCert    string
	ClientKey     string
	CACert        string
}

func DialNL2SQL(opts DialOpts) (*grpc.ClientConn, *NL2SQLClient, error) {
	var creds credentials.TransportCredentials

	if opts.ClientCert != "" && opts.ClientKey != "" && opts.CACert != "" {
		cert, err := tls.LoadX509KeyPair(opts.ClientCert, opts.ClientKey)
		if err != nil {
			return nil, nil, err
		}
		caPEM, err := os.ReadFile(opts.CACert)
		if err != nil {
			return nil, nil, err
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caPEM) {
			return nil, nil, err
		}
		creds = credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caPool,
			MinVersion:   tls.VersionTLS12,
		})
	} else {
		creds = insecure.NewCredentials()
	}

	conn, err := grpc.Dial(opts.Addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, nil, err
	}
	return conn, &NL2SQLClient{c: nlv1.NewNL2SQLServiceClient(conn)}, nil
}

// injectTraceContext 将 W3C 追踪上下文传播到出站 gRPC 元数据中
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

// metadataCarrier 适配 metadata.MD 到 propagation.TextMapCarrier 接口
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
