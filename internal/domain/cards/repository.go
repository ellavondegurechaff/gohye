package cards

import (
	"context"

	"github.com/disgoorg/bot-template/internal/gateways/database/models"
)

//domanis should have their own model, not use the same as database
//using the database one for now

type Repository interface {
	Create(ctx context.Context, card *models.Card) error
	GetByID(ctx context.Context, id int64) (*models.Card, error)
	GetByName(ctx context.Context, name string) ([]*models.Card, error)
	GetAll(ctx context.Context) ([]*models.Card, error)
	GetByCollectionID(ctx context.Context, colID string) ([]*models.Card, error)
	Update(ctx context.Context, card *models.Card) error
	Delete(ctx context.Context, id int64) error
	GetByTag(ctx context.Context, tag string) ([]*models.Card, error)
	BulkCreate(ctx context.Context, cards []*models.Card) (int, error)
	GetByLevel(ctx context.Context, level int) ([]*models.Card, error)
	GetAnimated(ctx context.Context) ([]*models.Card, error)
	SafeDelete(ctx context.Context, cardID int64) (*models.DeletionReport, error)
	Search(ctx context.Context, filters models.SearchFilters, offset, limit int) ([]*models.Card, int, error)
	UpdateUserCard(ctx context.Context, userCard *models.UserCard) error
	DeleteUserCard(ctx context.Context, id int64) error
	GetUserCard(ctx context.Context, userID string, cardID int64) (*models.UserCard, error)
	GetAllByUserID(ctx context.Context, userID string) ([]*models.UserCard, error)
	GetByIDs(ctx context.Context, ids []int64) ([]*models.Card, error)
	GetByQuery(ctx context.Context, query string) (*models.Card, error)
}
