package classifier

import (
	"context"
	"fmt"

	"github.com/ghobs91/lodestone/internal/classifier/classification"
	"github.com/ghobs91/lodestone/internal/database/query"
	"github.com/ghobs91/lodestone/internal/database/search"
	"github.com/ghobs91/lodestone/internal/model"
)

type LocalSearch interface {
	ContentByID(context.Context, model.ContentRef) (model.Content, error)
	ContentBySearch(context.Context, model.ContentType, string, model.Year) (model.Content, error)
}

type localSearch struct {
	search.Search
}

func (l localSearch) ContentByID(ctx context.Context, ref model.ContentRef) (model.Content, error) {
	options := []query.Option{
		query.Where(
			search.ContentTypeCriteria(ref.Type),
		),
		search.ContentDefaultPreload(),
		search.ContentDefaultHydrate(),
		query.Limit(1),
	}
	if ref.Source == "tmdb" {
		options = append(options, query.Where(
			search.ContentCanonicalIdentifierCriteria(model.ContentRef{
				Source: ref.Source,
				ID:     ref.ID,
			}),
		))
	} else {
		options = append(options, query.Where(
			search.ContentAlternativeIdentifierCriteria(model.ContentRef{
				Source: ref.Source,
				ID:     ref.ID,
			}),
		))
	}

	result, err := l.Content(ctx, options...)
	if err != nil {
		return model.Content{}, err
	}

	if len(result.Items) == 0 {
		return model.Content{}, classification.ErrUnmatched
	}

	return result.Items[0].Content, nil
}

func (l localSearch) ContentBySearch(
	ctx context.Context,
	ct model.ContentType,
	baseTitle string,
	year model.Year,
) (model.Content, error) {
	options := []query.Option{
		query.Where(search.ContentTypeCriteria(ct)),
		query.SearchString(fmt.Sprintf("\"%s\"", baseTitle)),
		query.OrderByQueryStringRank(),
		query.Limit(10),
		search.ContentDefaultPreload(),
		search.ContentDefaultHydrate(),
	}
	if !year.IsNil() {
		options = append(
			options,
			query.Where(search.ContentReleaseDateCriteria(model.NewDateRangeFromYear(year))),
		)
	}

	result, searchErr := l.Content(
		ctx,
		options...,
	)
	if searchErr != nil {
		return model.Content{}, searchErr
	}

	bestMatch, ok := levenshteinFindBestMatch[search.ContentResultItem](
		baseTitle,
		result.Items,
		func(item search.ContentResultItem) []string {
			candidates := []string{item.Title}
			if item.OriginalTitle.Valid {
				candidates = append(candidates, item.OriginalTitle.String)
			}

			return candidates
		},
	)
	if !ok {
		return model.Content{}, classification.ErrUnmatched
	}

	return bestMatch.Content, nil
}
