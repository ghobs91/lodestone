package search

import (
	"fmt"
	"strings"

	"github.com/ghobs91/lodestone/internal/database/query"
	"github.com/ghobs91/lodestone/internal/model"
	"github.com/ghobs91/lodestone/internal/protocol"
)

func TorrentInfoHashCriteria(infoHashes ...protocol.ID) query.Criteria {
	return infoHashCriteria(model.TableNameTorrent, infoHashes...)
}

func infoHashCriteria(table string, infoHashes ...protocol.ID) query.Criteria {
	if len(infoHashes) == 0 {
		return query.DBCriteria{
			SQL: "FALSE",
		}
	}

	decodes := make([]string, len(infoHashes))
	for i, infoHash := range infoHashes {
		decodes[i] = fmt.Sprintf("DECODE('%s', 'hex')", infoHash.String())
	}

	return query.DBCriteria{
		SQL: fmt.Sprintf("%s.info_hash IN (%s)", table, strings.Join(decodes, ", ")),
	}
}
