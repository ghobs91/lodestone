package query

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/ghobs91/lodestone/internal/database/dao"
	"github.com/ghobs91/lodestone/internal/maps"
	"github.com/ghobs91/lodestone/internal/model"
	"gorm.io/gorm/clause"
)

type FacetConfig interface {
	Key() string
	Label() string
	Logic() model.FacetLogic
	Filter() FacetFilter
	IsAggregated() bool
	AggregationOption(b OptionBuilder) (OptionBuilder, error)
	AggregationOptions() []Option
	NullValue() string
	TriggersCte() bool
}

type Facet interface {
	FacetConfig
	Values(ctx FacetContext) (map[string]string, error)
	Criteria(filter FacetFilter) []Criteria
}

// FacetGroupBy is implemented by facets whose aggregation can be computed with a
// single GROUP BY query instead of one COUNT query per distinct value. It
// returns the column to group by; an empty column (or not implementing this
// interface) falls back to per-value counting.
type FacetGroupBy interface {
	Facet
	GroupByField(ctx FacetContext) clause.Column
}

type FacetFilter map[string]struct{}

// Values allows iteration over deterministically sorted filter values, which helps with query caching.
func (f FacetFilter) Values() []string {
	values := make([]string, 0, len(f))
	for v := range f {
		values = append(values, v)
	}

	sort.Strings(values)

	return values
}

func (f FacetFilter) HasKey(key string) bool {
	_, ok := f[key]
	return ok
}

type facetConfig struct {
	key                string
	label              string
	logic              model.FacetLogic
	filter             FacetFilter
	aggregate          bool
	aggregationOptions []Option
	triggersCte        bool
	nullValue          string
}

type FacetOption func(facetConfig) facetConfig

func NewFacetConfig(options ...FacetOption) FacetConfig {
	c := facetConfig{}
	for _, option := range options {
		c = option(c)
	}

	return c
}

type AggregationItem struct {
	Label      string
	Count      uint
	IsEstimate bool
}

type AggregationItems = map[string]AggregationItem

type AggregationGroup struct {
	Label string
	Logic model.FacetLogic
	Items AggregationItems
}

type Aggregations = maps.StringMap[AggregationGroup]

type FacetContext interface {
	DBContext
	Context() context.Context
	NewAggregationQuery(options ...Option) (SubQuery, error)
}

type facetContext struct {
	optionBuilder OptionBuilder
	ctx           context.Context
}

func (ctx facetContext) Query() *dao.Query {
	return ctx.optionBuilder.Query()
}

func (ctx facetContext) TableName() string {
	return ctx.optionBuilder.TableName()
}

func (ctx facetContext) NewSubQuery(c context.Context) SubQuery {
	return ctx.optionBuilder.NewSubQuery(c)
}

func (ctx facetContext) Context() context.Context {
	return ctx.ctx
}

func (ctx facetContext) NewAggregationQuery(options ...Option) (SubQuery, error) {
	subCtx, subErr := Options(options...)(ctx.optionBuilder)
	if subErr != nil {
		return nil, subErr
	}

	sq := ctx.optionBuilder.NewSubQuery(ctx.Context())

	applyErr := subCtx.applyPre(sq, false)
	if applyErr != nil {
		return nil, applyErr
	}

	return sq, nil
}

func FacetHasKey(key string) FacetOption {
	return func(a facetConfig) facetConfig {
		a.key = key
		return a
	}
}

func FacetHasLabel(label string) FacetOption {
	return func(a facetConfig) facetConfig {
		a.label = label
		return a
	}
}

func FacetUsesLogic(logic model.FacetLogic) FacetOption {
	return func(a facetConfig) facetConfig {
		a.logic = logic
		return a
	}
}

func FacetUsesAndLogic() FacetOption {
	return FacetUsesLogic(model.FacetLogicAnd)
}

func FacetUsesOrLogic() FacetOption {
	return FacetUsesLogic(model.FacetLogicOr)
}

func FacetIsAggregated() FacetOption {
	return func(c facetConfig) facetConfig {
		c.aggregate = true
		return c
	}
}

func FacetHasFilter(filter FacetFilter) FacetOption {
	return func(c facetConfig) facetConfig {
		c.filter = filter
		return c
	}
}

func FacetHasAggregationOption(options ...Option) FacetOption {
	return func(c facetConfig) facetConfig {
		c.aggregationOptions = append(c.aggregationOptions, options...)
		return c
	}
}

func FacetTriggersCte() FacetOption {
	return func(c facetConfig) facetConfig {
		c.triggersCte = true
		return c
	}
}

// FacetNullValue sets the value key used for NULL values in GROUP BY
// aggregations (e.g. "null"), so they can be mapped to the facet's value map.
func FacetNullValue(value string) FacetOption {
	return func(c facetConfig) facetConfig {
		c.nullValue = value
		return c
	}
}

func (c facetConfig) Key() string {
	return c.key
}

func (c facetConfig) Label() string {
	return c.label
}

func (c facetConfig) Logic() model.FacetLogic {
	return c.logic
}

func (c facetConfig) IsAggregated() bool {
	return c.aggregate
}

func (c facetConfig) AggregationOption(b OptionBuilder) (OptionBuilder, error) {
	return Options(c.aggregationOptions...)(b)
}

func (c facetConfig) AggregationOptions() []Option {
	return c.aggregationOptions
}

func (c facetConfig) NullValue() string {
	return c.nullValue
}

func (c facetConfig) Filter() FacetFilter {
	return c.filter
}

func (c facetConfig) TriggersCte() bool {
	return c.triggersCte
}

func (b optionBuilder) createFacetsFilterCriteria() (c Criteria, err error) {
	cs := make([]Criteria, 0, len(b.facets))

	for _, facet := range b.facets {
		cr := facet.Criteria(facet.Filter())

		switch facet.Logic() {
		case model.FacetLogicAnd:
			cs = append(cs, AndCriteria{cr})
		case model.FacetLogicOr:
			if b.currentFacet != facet.Key() {
				cs = append(cs, OrCriteria{cr})
			}
		}
	}

	return AndCriteria{cs}, nil
}

// when aggregating with or logic the current facet's filter should be ignored
func withCurrentFacet(facetKey string) Option {
	return func(b OptionBuilder) (OptionBuilder, error) {
		return b.withCurrentFacet(facetKey), nil
	}
}

func (b optionBuilder) calculateAggregations(ctx context.Context) (Aggregations, error) {
	aggregations := make(Aggregations, len(b.facets))
	wgOuter := sync.WaitGroup{}

	mtx := sync.Mutex{}

	var errs []error

	addErr := func(err error) {
		mtx.Lock()
		defer mtx.Unlock()

		errs = append(errs, err)
	}
	addAggregation := func(key string, aggregation AggregationGroup) {
		mtx.Lock()
		defer mtx.Unlock()

		aggregations[key] = aggregation
	}

	for _, facet := range b.facets {
		if !facet.IsAggregated() {
			continue
		}

		wgOuter.Add(1)

		go func(facet Facet) {
			defer wgOuter.Done()

			fctx := facetContext{
				optionBuilder: b,
				ctx:           ctx,
			}

			values, valuesErr := facet.Values(fctx)
			if valuesErr != nil {
				addErr(fmt.Errorf("failed to get values for key '%s': %w",
					facet.Key(),
					valuesErr))
				return
			}

			var items AggregationItems

			if gb, ok := facet.(FacetGroupBy); ok {
				if col := gb.GroupByField(fctx); col.Name != "" {
					grouped, groupedErr := b.aggregateFacetGroupBy(ctx, gb, col)
					if groupedErr != nil {
						addErr(fmt.Errorf("failed to aggregate facet '%s': %w", facet.Key(), groupedErr))
						return
					}

					items = make(AggregationItems, len(grouped))
					for key, item := range grouped {
						label, ok := values[key]
						if !ok {
							continue
						}

						item.Label = label

						items[key] = item
					}
				}
			}

			if items == nil {
				items = b.aggregateFacetPerValue(ctx, facet, values, addErr)
			}

			addAggregation(facet.Key(), AggregationGroup{
				Label: facet.Label(),
				Logic: facet.Logic(),
				Items: items,
			})
		}(facet)
	}

	wgOuter.Wait()

	return aggregations, errors.Join(errs...)
}

// aggregateFacetGroupBy computes all of a facet's value counts with a single
// GROUP BY query, instead of issuing one COUNT query per distinct value. This
// is significantly cheaper on the database for high-cardinality facets.
func (b optionBuilder) aggregateFacetGroupBy(
	ctx context.Context,
	facet FacetGroupBy,
	column clause.Column,
) (AggregationItems, error) {
	criterias := facet.Criteria(facet.Filter())

	var criteria Criteria
	switch facet.Logic() {
	case model.FacetLogicAnd:
		criteria = AndCriteria{criterias}
	case model.FacetLogicOr:
		criteria = OrCriteria{criterias}
	}

	builder, err := facadeApplyOptions(b, facet, criteria)
	if err != nil {
		return nil, err
	}

	agg, ok := builder.(optionBuilder)
	if !ok {
		return nil, errors.New("unexpected option builder type")
	}

	// Select the grouped value alongside its count. The aliases must match
	// groupByResult's fields so GORM can scan the rows.
	agg.selections = []clause.Expr{
		{SQL: "? AS value", Vars: []interface{}{column}},
		{SQL: "COUNT(*) AS count"},
	}
	agg.groupBy = []clause.Column{column}

	sq := agg.NewSubQuery(ctx)

	if selectErr := agg.applySelect(sq.UnderlyingDB(), false); selectErr != nil {
		return nil, selectErr
	}

	if preErr := agg.applyPre(sq, false); preErr != nil {
		return nil, preErr
	}

	var rows []groupByResult

	if findErr := sq.UnderlyingDB().Find(&rows).Error; findErr != nil {
		return nil, findErr
	}

	items := make(AggregationItems, len(rows))
	nullValue := facet.NullValue()

	for _, row := range rows {
		if row.Count <= 0 {
			continue
		}

		key := row.Value.String
		if !row.Value.Valid {
			key = nullValue
		}

		items[key] = AggregationItem{
			Count: uint(row.Count),
		}
	}

	return items, nil
}

// aggregateFacetPerValue is the fallback aggregation path: one budgeted count
// query per facet value.
func (b optionBuilder) aggregateFacetPerValue(
	ctx context.Context,
	facet Facet,
	values map[string]string,
	addErr func(error),
) AggregationItems {
	var items AggregationItems

	filter := facet.Filter()
	items = make(AggregationItems, len(values))
	mtx := sync.Mutex{}

	addItem := func(key string, item AggregationItem) {
		mtx.Lock()
		defer mtx.Unlock()
		items[key] = item
	}

	wgInner := sync.WaitGroup{}
	wgInner.Add(len(values))

	for key, label := range values {
		go func(key, label string) {
			defer wgInner.Done()
			criterias := facet.Criteria(FacetFilter{key: struct{}{}})
			var criteria Criteria
			switch facet.Logic() {
			case model.FacetLogicAnd:
				criteria = AndCriteria{criterias}
			case model.FacetLogicOr:
				criteria = OrCriteria{criterias}
			}
			aggBuilder, aggBuilderErr := Options(
				facet.AggregationOption,
				withCurrentFacet(facet.Key()),
				Where(criteria),
			)(b)
			if aggBuilderErr != nil {
				addErr(
					fmt.Errorf(
						"failed to create aggregation option for key '%s': %w", facet.Key(), aggBuilderErr),
				)
				return
			}
			q := aggBuilder.NewSubQuery(ctx)
			if preErr := aggBuilder.applyPre(q, false); preErr != nil {
				addErr(fmt.Errorf("failed to apply pre for key '%s': %w", facet.Key(), preErr))
				return
			}
			countResult, countErr := dao.BudgetedCount(q.UnderlyingDB(), b.aggregationBudget)
			if countErr != nil {
				addErr(fmt.Errorf("failed to get count for key '%s': %w", facet.Key(), countErr))
				return
			}
			if countResult.Count > 0 || countResult.BudgetExceeded || filter.HasKey(key) {
				addItem(key, AggregationItem{
					Label:      label,
					Count:      uint(countResult.Count),
					IsEstimate: countResult.BudgetExceeded,
				})
			}
		}(key, label)
	}
	wgInner.Wait()

	return items
}

// facadeApplyOptions builds the base aggregation query for a facet, applying
// the facet's aggregation options, ignoring its own (already applied) filter,
// and restricting the result to rows matching the facet's current filter.
func facadeApplyOptions(b optionBuilder, facet Facet, criteria Criteria) (OptionBuilder, error) {
	options := append([]Option{}, facet.AggregationOptions()...)
	options = append(options, withCurrentFacet(facet.Key()))
	if criteria != nil {
		options = append(options, Where(criteria))
	}

	return Options(options...)(b)
}

type groupByResult struct {
	Value sql.NullString
	Count int64
}
