package manager

import (
	"github.com/ghobs91/lodestone/internal/database/dao"
	"gorm.io/gorm"
)

type manager struct {
	dao *dao.Query
	db  *gorm.DB
}
