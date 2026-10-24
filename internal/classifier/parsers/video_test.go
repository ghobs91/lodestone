package parsers

import (
	"testing"

	"github.com/ghobs91/lodestone/internal/classifier/classification"
	"github.com/ghobs91/lodestone/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVideoContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		torrentName  string
		contentType  model.NullContentType
		wantType     model.NullContentType
		wantTitle    model.NullString
		wantYear     model.Year
		wantRes      model.NullVideoResolution
		wantCodec    model.NullVideoCodec
		wantGroup    model.NullString
		wantEpisodes model.Episodes
	}{
		{
			name:        "title without year keeps title",
			torrentName: "Some Movie 1080p.mkv",
			wantTitle:   model.NewNullString("Some Movie"),
			wantRes:     model.NewNullVideoResolution(model.VideoResolutionV1080p),
		},
		{
			name:        "title with year is a movie",
			torrentName: "Some.Movie.2024.1080p.WEB-DL.x264-GROUP.mkv",
			wantType:    model.NewNullContentType(model.ContentTypeMovie),
			wantTitle:   model.NewNullString("Some Movie"),
			wantYear:    2024,
			wantRes:     model.NewNullVideoResolution(model.VideoResolutionV1080p),
			wantCodec:   model.NewNullVideoCodec(model.VideoCodecX264),
			wantGroup:   model.NewNullString("GROUP"),
		},
		{
			name:         "episode pattern is a tv show",
			torrentName:  "Some.Show.S01E01.1080p.mkv",
			wantType:     model.NewNullContentType(model.ContentTypeTvShow),
			wantTitle:    model.NewNullString("Some Show"),
			wantRes:      model.NewNullVideoResolution(model.VideoResolutionV1080p),
			wantEpisodes: model.Episodes{1: map[int]struct{}{1: {}}},
		},
		{
			name:        "quality-only name has no title",
			torrentName: "1080p.mkv",
			wantRes:     model.NewNullVideoResolution(model.VideoResolutionV1080p),
		},
		{
			name:        "title with no tags keeps title",
			torrentName: "The Matrix.mkv",
			wantTitle:   model.NewNullString("The Matrix"),
		},
		{
			name:        "web in title is not a tag",
			torrentName: "Charlotte's Web 1080p.mkv",
			wantTitle:   model.NewNullString("Charlotte's Web"),
			wantRes:     model.NewNullVideoResolution(model.VideoResolutionV1080p),
		},
		{
			name:        "4k tag is stripped",
			torrentName: "Some Movie 4K HDR.mkv",
			wantTitle:   model.NewNullString("Some Movie"),
			wantRes:     model.NewNullVideoResolution(model.VideoResolutionV2160p),
		},
		{
			name:        "hinted movie without year keeps title",
			torrentName: "Some Movie 1080p.mkv",
			contentType: model.NewNullContentType(model.ContentTypeMovie),
			wantType:    model.NewNullContentType(model.ContentTypeMovie),
			wantTitle:   model.NewNullString("Some Movie"),
			wantRes:     model.NewNullVideoResolution(model.VideoResolutionV1080p),
		},
		{
			name:        "hinted xxx without year keeps old no-title behavior",
			torrentName: "Some XXX Movie 1080p.mkv",
			contentType: model.NewNullContentType(model.ContentTypeXxx),
			wantType:    model.NewNullContentType(model.ContentTypeXxx),
			wantRes:     model.NewNullVideoResolution(model.VideoResolutionV1080p),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			attrs, err := ParseVideoContent(model.Torrent{Name: tt.torrentName}, classification.Result{
				ContentAttributes: classification.ContentAttributes{ContentType: tt.contentType},
			})
			require.NoError(t, err)
			assert.Equal(t, tt.wantType, attrs.ContentType)
			assert.Equal(t, tt.wantTitle, attrs.BaseTitle)
			assert.Equal(t, tt.wantYear, attrs.Date.Year)
			assert.Equal(t, tt.wantRes, attrs.VideoResolution)
			assert.Equal(t, tt.wantCodec, attrs.VideoCodec)
			assert.Equal(t, tt.wantGroup, attrs.ReleaseGroup)
			assert.Equal(t, tt.wantEpisodes, attrs.Episodes)
		})
	}
}
