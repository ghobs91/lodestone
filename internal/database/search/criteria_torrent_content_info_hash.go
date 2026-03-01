package search

import (
	"github.com/ghobs91/lodestone/internal/database/query"
	"github.com/ghobs91/lodestone/internal/model"
	"github.com/ghobs91/lodestone/internal/protocol"
)

func TorrentContentInfoHashCriteria(infoHashes ...protocol.ID) query.Criteria {
	return infoHashCriteria(model.TableNameTorrentContent, infoHashes...)
}
