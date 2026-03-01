package httpserver

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ghobs91/lodestone/internal/httpserver"
	"github.com/ghobs91/lodestone/internal/importer"
	"github.com/ghobs91/lodestone/internal/lazy"
	"github.com/ghobs91/lodestone/internal/model"
	"github.com/ghobs91/lodestone/internal/protocol"
	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type SqliteImportParams struct {
	fx.In
	Importer lazy.Lazy[importer.Importer]
	Logger   *zap.SugaredLogger
}

type SqliteImportResult struct {
	fx.Out
	Option httpserver.Option `group:"http_server_options"`
}

func NewSqliteImport(p SqliteImportParams) SqliteImportResult {
	return SqliteImportResult{
		Option: &sqliteImportOption{
			importer: p.Importer,
			logger:   p.Logger.Named("sqlite_import"),
		},
	}
}

type sqliteImportOption struct {
	importer lazy.Lazy[importer.Importer]
	logger   *zap.SugaredLogger
}

func (sqliteImportOption) Key() string { return "sqlite_import" }

func (o sqliteImportOption) Apply(e *gin.Engine) error {
	i, err := o.importer.Get()
	if err != nil {
		return err
	}

	e.POST("/api/import-sqlite", func(ctx *gin.Context) {
		o.handle(ctx, i)
	})

	return nil
}

const rarbgQuery = `
SELECT
  hash AS infoHash,
  title AS name,
  size,
  CASE
    WHEN cat LIKE 'ebooks%' THEN 'ebook'
    WHEN cat LIKE 'games%' THEN 'software'
    WHEN cat LIKE 'movies%' THEN 'movie'
    WHEN cat LIKE 'tv%' THEN 'tv_show'
    WHEN cat LIKE 'music%' THEN 'music'
    WHEN cat LIKE 'software%' THEN 'software'
    WHEN cat = 'xxx' THEN 'xxx'
  END AS contentType,
  CASE
    WHEN cat LIKE '%_4k' THEN 'V2160p'
    WHEN cat LIKE '%_720' THEN 'V720p'
    WHEN cat LIKE '%_SD' THEN 'V480p'
  END AS videoResolution,
  CASE
    WHEN cat LIKE '%_bd_%' THEN 'BluRay'
  END AS videoSource,
  CASE
    WHEN cat LIKE '%_bd_full' THEN 'BRDISK'
    WHEN cat LIKE '%_bd_remux' THEN 'REMUX'
  END AS videoModifier,
  CASE
    WHEN cat LIKE '%_x264%' THEN 'x264'
    WHEN cat LIKE '%_x265%' THEN 'x265'
    WHEN cat LIKE '%_xvid%' THEN 'XviD'
  END AS videoCodec,
  CASE
    WHEN cat LIKE '%_3D' THEN 'V3D'
  END AS video3D,
  imdb,
  (SUBSTR(dt, 0, 11) || 'T' || SUBSTR(dt, 12) || '.000Z') AS publishedAt
FROM items
ORDER BY hash
`

func (o sqliteImportOption) handle(ctx *gin.Context, imp importer.Importer) {
	file, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	// Save uploaded file to a temporary location
	tmpFile, err := os.CreateTemp("", "rarbg-import-*.sqlite")
	if err != nil {
		o.logger.Errorw("failed to create temp file", "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process upload"})
		return
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	if err := ctx.SaveUploadedFile(file, tmpPath); err != nil {
		o.logger.Errorw("failed to save uploaded file", "error", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save upload"})
		return
	}

	// Open the SQLite database read-only
	db, err := sql.Open("sqlite", tmpPath+"?mode=ro")
	if err != nil {
		o.logger.Errorw("failed to open sqlite db", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "failed to open SQLite database: " + err.Error()})
		return
	}
	defer db.Close()

	// Verify it has an "items" table
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='items'").Scan(&tableName)
	if err != nil {
		o.logger.Errorw("sqlite db missing items table", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "SQLite database does not contain an 'items' table. Is this a valid RARBG backup?"})
		return
	}

	rows, err := db.Query(rarbgQuery)
	if err != nil {
		o.logger.Errorw("failed to query sqlite db", "error", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "failed to query database: " + err.Error()})
		return
	}
	defer rows.Close()

	ai := imp.New(ctx, importer.Info{
		ID: fmt.Sprintf("rarbg-%d", time.Now().Unix()),
	})

	count := 0
	errCount := 0

	// Stream results using SSE for progress reporting
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer.Header().Set("Cache-Control", "no-cache")
	ctx.Writer.Header().Set("Connection", "keep-alive")

	for rows.Next() {
		var (
			infoHashStr     string
			name            string
			size            sql.NullInt64
			contentType     sql.NullString
			videoResolution sql.NullString
			videoSource     sql.NullString
			videoModifier   sql.NullString
			videoCodec      sql.NullString
			video3D         sql.NullString
			imdbID          sql.NullString
			publishedAtStr  sql.NullString
		)

		if err := rows.Scan(
			&infoHashStr, &name, &size,
			&contentType, &videoResolution, &videoSource,
			&videoModifier, &videoCodec, &video3D,
			&imdbID, &publishedAtStr,
		); err != nil {
			o.logger.Errorw("failed to scan row", "error", err)
			errCount++
			continue
		}

		// Parse info hash
		infoHashStr = strings.ToLower(strings.TrimSpace(infoHashStr))
		hashBytes, err := hex.DecodeString(infoHashStr)
		if err != nil || len(hashBytes) != 20 {
			errCount++
			continue
		}
		var id protocol.ID
		copy(id[:], hashBytes)

		// Parse published time
		var publishedAt time.Time
		if publishedAtStr.Valid && publishedAtStr.String != "" {
			publishedAt, _ = time.Parse(time.RFC3339Nano, publishedAtStr.String)
		}

		item := importer.Item{
			Source:   "rarbg",
			InfoHash: id,
			Name:     name,
			Size:     uint(size.Int64),
		}

		if publishedAt.After(time.Time{}) {
			item.PublishedAt = publishedAt
		}

		if contentType.Valid && contentType.String != "" {
			item.ContentType = model.NullContentType{Valid: true, ContentType: model.ContentType(contentType.String)}
		}

		if videoResolution.Valid && videoResolution.String != "" {
			item.VideoResolution = model.NullVideoResolution{Valid: true, VideoResolution: model.VideoResolution(videoResolution.String)}
		}

		if videoSource.Valid && videoSource.String != "" {
			item.VideoSource = model.NullVideoSource{Valid: true, VideoSource: model.VideoSource(videoSource.String)}
		}

		if videoCodec.Valid && videoCodec.String != "" {
			item.VideoCodec = model.NullVideoCodec{Valid: true, VideoCodec: model.VideoCodec(videoCodec.String)}
		}

		if video3D.Valid && video3D.String != "" {
			item.Video3D = model.NullVideo3D{Valid: true, Video3D: model.Video3D(video3D.String)}
		}

		if videoModifier.Valid && videoModifier.String != "" {
			item.VideoModifier = model.NullVideoModifier{Valid: true, VideoModifier: model.VideoModifier(videoModifier.String)}
		}

		// Map IMDB ID to content source/id
		if imdbID.Valid && imdbID.String != "" {
			item.ContentSource = model.NullString{Valid: true, String: "imdb"}
			item.ContentID = model.NullString{Valid: true, String: imdbID.String}
		}

		if err := ai.Import(item); err != nil {
			o.logger.Errorw("failed to import item", "error", err)
			errCount++
			continue
		}

		count++
		if count%1000 == 0 {
			fmt.Fprintf(ctx.Writer, "data: {\"imported\": %d, \"errors\": %d}\n\n", count, errCount)
			ctx.Writer.Flush()
		}
	}

	ai.Drain()

	if err := ai.Close(); err != nil {
		o.logger.Errorw("error closing import", "error", err)
		fmt.Fprintf(ctx.Writer, "data: {\"error\": \"import close error: %s\"}\n\n", err.Error())
		ctx.Writer.Flush()
		return
	}

	fmt.Fprintf(ctx.Writer, "data: {\"imported\": %d, \"errors\": %d, \"done\": true}\n\n", count, errCount)
	ctx.Writer.Flush()
}
