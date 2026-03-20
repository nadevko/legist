package qdrant

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	qdrantgrpc "github.com/qdrant/go-client/qdrant"
)

// Client is a minimal Qdrant gRPC wrapper for batch search with payload.
type Client struct {
	grpc *qdrantgrpc.GrpcClient
}

type SearchHit struct {
	Score   float32
	Payload map[string]any
}

func NewClient(host, grpcPort string) (*Client, error) {
	portStr := strings.TrimSpace(grpcPort)
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("qdrant grpc port: %w", err)
	}

	grpcClient, err := qdrantgrpc.NewGrpcClient(&qdrantgrpc.Config{
		Host: host,
		Port: port,
		// Compatibility checks may require extra round-trips; skip in internal use.
		SkipCompatibilityCheck: true,
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant grpc: %w", err)
	}
	return &Client{grpc: grpcClient}, nil
}

func (c *Client) Close() error {
	if c == nil || c.grpc == nil {
		return nil
	}
	return c.grpc.Close()
}

// SearchBatch performs Qdrant Points.SearchBatch with payload included.
// embeddings are converted to float32 vector.
func (c *Client) SearchBatch(
	ctx context.Context,
	collection string,
	embeddings [][]float64,
	limit uint64,
	withPayloadKeys []string,
) ([][]SearchHit, error) {
	if c == nil || c.grpc == nil {
		return nil, fmt.Errorf("qdrant client is nil")
	}
	if collection == "" {
		return nil, fmt.Errorf("qdrant collection is empty")
	}

	searchPoints := make([]*qdrantgrpc.SearchPoints, 0, len(embeddings))
	for _, emb := range embeddings {
		vec := make([]float32, 0, len(emb))
		for _, x := range emb {
			vec = append(vec, float32(x))
		}

		var withPayload *qdrantgrpc.WithPayloadSelector
		if len(withPayloadKeys) > 0 {
			withPayload = qdrantgrpc.NewWithPayloadInclude(withPayloadKeys...)
		}

		searchPoints = append(searchPoints, &qdrantgrpc.SearchPoints{
			CollectionName: collection,
			Vector:         vec,
			Limit:          limit,
			WithPayload:    withPayload,
		})
	}

	resp, err := c.grpc.Points().SearchBatch(ctx, &qdrantgrpc.SearchBatchPoints{
		CollectionName: collection,
		SearchPoints:   searchPoints,
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant search batch: %w", err)
	}

	out := make([][]SearchHit, 0, len(resp.GetResult()))
	for _, batch := range resp.GetResult() {
		if batch == nil {
			out = append(out, nil)
			continue
		}
		hits := make([]SearchHit, 0, len(batch.GetResult()))
		for _, sp := range batch.GetResult() {
			if sp == nil {
				continue
			}
			hits = append(hits, SearchHit{
				Score:   sp.GetScore(),
				Payload: payloadValueMapToAny(sp.GetPayload()),
			})
		}
		out = append(out, hits)
	}
	return out, nil
}

func payloadValueMapToAny(in map[string]*qdrantgrpc.Value) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = valueToAny(v)
	}
	return out
}

func valueToAny(v *qdrantgrpc.Value) any {
	if v == nil {
		return nil
	}

	switch kind := v.GetKind().(type) {
	case *qdrantgrpc.Value_NullValue:
		return nil
	case *qdrantgrpc.Value_DoubleValue:
		return kind.DoubleValue
	case *qdrantgrpc.Value_IntegerValue:
		return kind.IntegerValue
	case *qdrantgrpc.Value_StringValue:
		return kind.StringValue
	case *qdrantgrpc.Value_BoolValue:
		return kind.BoolValue
	case *qdrantgrpc.Value_StructValue:
		sv := kind.StructValue
		if sv == nil {
			return nil
		}
		fields := sv.GetFields()
		m := make(map[string]any, len(fields))
		for fk, fv := range fields {
			m[fk] = valueToAny(fv)
		}
		return m
	case *qdrantgrpc.Value_ListValue:
		lv := kind.ListValue
		if lv == nil {
			return nil
		}
		values := lv.GetValues()
		arr := make([]any, 0, len(values))
		for _, av := range values {
			arr = append(arr, valueToAny(av))
		}
		return arr
	default:
		return nil
	}
}

