package search

import (
	"github.com/ghobs91/lodestone/internal/database/query"
	"github.com/ghobs91/lodestone/internal/model"
	"github.com/ghobs91/lodestone/internal/protocol"
)

func TorrentFileInfoHashCriteria(infoHashes ...protocol.ID) query.Criteria {
	return infoHashCriteria(model.TableNameTorrentFile, infoHashes...)
}
