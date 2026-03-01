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
	Store lazy.Lazy[settings.ClassifierSettingsStore]
}

type Result struct {
	fx.Out
	Option httpserver.Option `group:"http_server_options"`
}

func New(p Params) Result {
	return Result{
		Option: &settingsOption{store: p.Store},
	}
}

type settingsOption struct {
	store lazy.Lazy[settings.ClassifierSettingsStore]
}

func (settingsOption) Key() string { return "settings" }

func (o settingsOption) Apply(e *gin.Engine) error {
	store, err := o.store.Get()
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

	return nil
}
