package search

import (
	"fmt"
	"strings"

	"github.com/ghobs91/lodestone/internal/database/query"
	"github.com/ghobs91/lodestone/internal/model"
)

func TorrentContentEpisodesCriteria(episodes model.Episodes) query.Criteria {
	return query.GenCriteria(func(query.DBContext) (query.Criteria, error) {
		and := make([]query.Criteria, 0, len(episodes))

		for _, s := range episodes.SeasonEntries() {
			var containment string
			if len(s.Episodes) == 0 {
				// Whole-season match: the season key maps to an empty object.
				containment = fmt.Sprintf(`{"%d":{}}`, s.Season)
			} else {
				episodeParts := make([]string, 0, len(s.Episodes))
				for _, e := range s.Episodes {
					episodeParts = append(episodeParts, fmt.Sprintf(`"%d":{}`, e))
				}

				containment = fmt.Sprintf(`{"%d":{%s}}`, s.Season, strings.Join(episodeParts, ","))
			}

			and = append(and, query.DBCriteria{
				// jsonb containment (@>) can be served by the GIN
				// (episodes jsonb_path_ops) index; the previous
				// `#>`-based predicates could not use any index.
				SQL:  "torrent_contents.episodes @> ?::jsonb",
				Args: []interface{}{containment},
			})
		}

		return query.AndCriteria{
			Criteria: and,
		}, nil
	})
}
