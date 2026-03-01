package classifier

import (
	"github.com/ghobs91/lodestone/internal/tmdb"
)

type dependencies struct {
	search     LocalSearch
	tmdbClient tmdb.Client
}
