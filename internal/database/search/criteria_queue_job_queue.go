package search

import (
	"github.com/ghobs91/lodestone/internal/database/query"
	"gorm.io/gen/field"
)

func QueueJobQueueCriteria(queues ...string) query.Criteria {
	return query.DaoCriteria{
		Conditions: func(ctx query.DBContext) ([]field.Expr, error) {
			q := ctx.Query()
			return []field.Expr{
				q.QueueJob.Queue.In(queues...),
			}, nil
		},
	}
}
