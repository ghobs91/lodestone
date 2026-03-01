package search

import (
	"github.com/ghobs91/lodestone/internal/database/query"
	"github.com/ghobs91/lodestone/internal/model"
)

func TorrentFileTypeCriteria(fileTypes ...model.FileType) query.Criteria {
	var extensions []string
	for _, fileType := range fileTypes {
		extensions = append(extensions, fileType.Extensions()...)
	}

	return TorrentFileExtensionCriteria(extensions...)
}
