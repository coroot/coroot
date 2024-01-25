package clickhouse

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func (c *Client) GetProfileTypes(ctx context.Context) (map[string][]model.ProfileType, error) {
	rows, err := c.conn.Query(ctx, qProfileTypes)
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

func (c *Client) GetProfile(ctx context.Context, from, to timeseries.Time, services []string, typ model.ProfileType, diff bool) (*model.FlameGraphNode, error) {
	avg := model.Profiles[typ].Aggregation == model.ProfileAggregationAvg
	if diff {
		return c.getDiffProfile(ctx, from, to, services, typ)
	}
	return c.getProfile(ctx, from, to, services, typ, avg)
}

func (c *Client) getProfile(ctx context.Context, from, to timeseries.Time, services []string, typ model.ProfileType, avg bool) (*model.FlameGraphNode, error) {
	query := qProfile
	if avg {
		query = qProfileAvg
	}
	rows, err := c.conn.Query(ctx, query,
		clickhouse.Named("service", services),
		clickhouse.Named("type", typ),
		clickhouse.DateNamed("from", from.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.NanoSeconds),
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

func (c *Client) getDiffProfile(ctx context.Context, from, to timeseries.Time, services []string, typ model.ProfileType) (*model.FlameGraphNode, error) {
	query := qProfileDiff
	rows, err := c.conn.Query(ctx, query,
		clickhouse.Named("service", services),
		clickhouse.Named("type", typ),
		clickhouse.DateNamed("from", from.Add(-to.Sub(from)).ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("middle", from.ToStandard(), clickhouse.NanoSeconds),
		clickhouse.DateNamed("to", to.ToStandard(), clickhouse.NanoSeconds),
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
