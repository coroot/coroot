package clickhouse

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

type ProfileQuery struct {
	Type       model.ProfileType
	From       timeseries.Time
	To         timeseries.Time
	Diff       bool
	Services   []string
	Containers []string
	Namespace  string
	Pod        string
}

func (c *Client) GetProfileTypes(ctx context.Context, from timeseries.Time) (map[string][]model.ProfileType, error) {
	rows, err := c.Query(ctx, qProfileTypes,
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.NanoSeconds),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var serviceName, typ string
	res := map[string][]model.ProfileType{}
	for rows.Next() {
		err = rows.Scan(&serviceName, &typ)
		if err != nil {
			return nil, err
		}
		res[serviceName] = append(res[serviceName], model.ProfileType(typ))
	}
	return res, nil
}

func (c *Client) GetProfile(ctx context.Context, q ProfileQuery) (*model.FlameGraphNode, error) {
	if q.Diff {
		return c.getDiffProfile(ctx, q)
	}
	return c.getProfile(ctx, q)
}

func (c *Client) getProfile(ctx context.Context, q ProfileQuery) (*model.FlameGraphNode, error) {
	query := qProfile
	if model.Profiles[q.Type].Aggregation == model.ProfileAggregationAvg {
		query = qProfileAvg
	}
	rows, err := c.Query(ctx, query,
		clickhouse.Named("type", q.Type),
		clickhouse.DateNamed("from", q.From.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", q.To.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.Named("services", q.Services),
		clickhouse.Named("containers", q.Containers),
		clickhouse.Named("namespace", q.Namespace),
		clickhouse.Named("pod", q.Pod),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var value int64
	var stack []string
	root := &model.FlameGraphNode{Name: "total"}
	for rows.Next() {
		err = rows.Scan(&value, &stack)
		if err != nil {
			return nil, err
		}
		root.InsertStack(stack, value, nil)
	}

	if root.Total == 0 {
		return nil, nil
	}
	return root, nil
}

func (c *Client) getDiffProfile(ctx context.Context, q ProfileQuery) (*model.FlameGraphNode, error) {
	query := qProfileDiff
	rows, err := c.Query(ctx, query,
		clickhouse.Named("type", q.Type),
		clickhouse.DateNamed("from", q.From.Add(-q.To.Sub(q.From)).ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("middle", q.From.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", q.To.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.Named("services", q.Services),
		clickhouse.Named("containers", q.Containers),
		clickhouse.Named("namespace", q.Namespace),
		clickhouse.Named("pod", q.Pod),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var base, comp int64
	var stack []string
	root := &model.FlameGraphNode{Name: "total"}
	for rows.Next() {
		err = rows.Scan(&base, &comp, &stack)
		if err != nil {
			return nil, err
		}
		root.InsertStack(stack, base+comp, &comp)
	}

	if root.Total == 0 || root.Total == root.Comp || root.Comp == 0 {
		return nil, nil
	}

	return root, nil
}
