package adapter

import (
	"context"

	"github.com/ghobs91/lodestone/internal/database/query"
	"github.com/ghobs91/lodestone/internal/database/search"
	"github.com/ghobs91/lodestone/internal/torznab"
)

func New(search search.Search) Adapter {
	return Adapter{
		search: search,
	}
}

type Adapter struct {
	search search.Search
}

func (a Adapter) Search(ctx context.Context, req torznab.SearchRequest) (torznab.SearchResult, error) {
	options := []query.Option{search.TorrentContentDefaultOption(), query.WithTotalCount(false)}

	reqOptions, reqErr := searchRequestToQueryOptions(req)
	if reqErr != nil {
		return torznab.SearchResult{}, reqErr
	}

	options = append(options, reqOptions...)

	searchResult, searchErr := a.search.TorrentContent(ctx, options...)
	if searchErr != nil {
		return torznab.SearchResult{}, searchErr
	}

	return torrentContentResultToTorznabResult(req, searchResult), nil
}
