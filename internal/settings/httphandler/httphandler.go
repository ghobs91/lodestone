package httphandler

import (
	"net/http"

	"github.com/ghobs91/lodestone/internal/httpserver"
	"github.com/ghobs91/lodestone/internal/lazy"
	"github.com/ghobs91/lodestone/internal/settings"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

type Params struct {
	fx.In
	Store        lazy.Lazy[settings.ClassifierSettingsStore]
	TmdbStore    lazy.Lazy[settings.TmdbApiKeySettingsStore]
}

type Result struct {
	fx.Out
	Option httpserver.Option `group:"http_server_options"`
}

func New(p Params) Result {
	return Result{
		Option: &settingsOption{
			store:     p.Store,
			tmdbStore: p.TmdbStore,
		},
	}
}

type settingsOption struct {
	store     lazy.Lazy[settings.ClassifierSettingsStore]
	tmdbStore lazy.Lazy[settings.TmdbApiKeySettingsStore]
}

func (settingsOption) Key() string { return "settings" }

func (o settingsOption) Apply(e *gin.Engine) error {
	store, err := o.store.Get()
	if err != nil {
		return err
	}

	tmdbStore, err := o.tmdbStore.Get()
	if err != nil {
		return err
	}

	e.GET("/api/classifier-settings", func(c *gin.Context) {
		cs, err := store.Get(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, cs)
	})

	e.PUT("/api/classifier-settings", func(c *gin.Context) {
		var input settings.ClassifierSettings
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		saved, err := store.Save(c.Request.Context(), input)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, saved)
	})

	e.GET("/api/tmdb-api-key-settings", func(c *gin.Context) {
		ts, err := tmdbStore.Get(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, ts)
	})

	e.PUT("/api/tmdb-api-key-settings", func(c *gin.Context) {
		var input settings.TmdbApiKeySettings
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		saved, err := tmdbStore.Save(c.Request.Context(), input)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, saved)
	})

	return nil
}
