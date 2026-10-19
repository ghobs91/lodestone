package query

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ghobs91/lodestone/internal/database/dao"
	"github.com/ghobs91/lodestone/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// fakeGroupByFacet is a minimal facet that opts into GROUP BY aggregation.
type fakeGroupByFacet struct {
	FacetConfig
	column clause.Column
}

func (fakeGroupByFacet) Values(FacetContext) (map[string]string, error) { return nil, nil }

func (fakeGroupByFacet) Criteria(FacetFilter) []Criteria { return nil }

func (f fakeGroupByFacet) GroupByField(FacetContext) clause.Column { return f.column }

// TestAggregateFacetGroupBy verifies the aggregation selects the value AND its
// count with aliases that map onto groupByResult, so the rows scan correctly.
func TestAggregateFacetGroupBy(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer mockDB.Close()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: mockDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}

	q := dao.Use(db)

	factory := func(ctx context.Context, q *dao.Query) SubQuery {
		return GenericSubQuery[dao.ITorrentContentDo]{
			SubQuery: q.TorrentContent.WithContext(ctx).ReadDB(),
		}
	}

	b, ok := newQueryContext(dbContext{
		q:         q,
		tableName: model.TableNameTorrentContent,
		factory:   factory,
	}).(optionBuilder)
	if !ok {
		t.Fatal("expected optionBuilder")
	}

	facet := fakeGroupByFacet{
		FacetConfig: NewFacetConfig(
			FacetHasKey("content_type"),
			FacetIsAggregated(),
			FacetUsesOrLogic(),
			FacetNullValue("null"),
		),
		column: clause.Column{Table: model.TableNameTorrentContent, Name: "content_type"},
	}

	// The query must select "<col> AS value" and "COUNT(*) AS count", grouped by
	// the same column.
	mock.ExpectQuery(`SELECT .*content_type.* AS value, COUNT\(\*\) AS count FROM .*torrent_contents.*GROUP BY .*content_type`).
		WillReturnRows(sqlmock.NewRows([]string{"value", "count"}).
			AddRow("movie", 5).
			AddRow(nil, 2).
			AddRow("tv_show", 3))

	items, err := b.aggregateFacetGroupBy(context.Background(), facet, facet.column)
	if err != nil {
		t.Fatalf("aggregateFacetGroupBy: %v", err)
	}

	if got := items["movie"].Count; got != 5 {
		t.Errorf("movie count = %d, want 5", got)
	}
	if got := items["null"].Count; got != 2 {
		t.Errorf("null count = %d, want 2", got)
	}
	if got := items["tv_show"].Count; got != 3 {
		t.Errorf("tv_show count = %d, want 3", got)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
