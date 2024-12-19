package mock

import (
	context "context"
	reflect "reflect"

	models "github.com/disgoorg/bot-template/internal/gateways/database/models"
	gomock "go.uber.org/mock/gomock"
)

// MockRepository is a mock of Repository interface.
type MockRepository struct {
	ctrl     *gomock.Controller
	recorder *MockRepositoryMockRecorder
	isgomock struct{}
}

// MockRepositoryMockRecorder is the mock recorder for MockRepository.
type MockRepositoryMockRecorder struct {
	mock *MockRepository
}

// NewMockRepository creates a new mock instance.
func NewMockRepository(ctrl *gomock.Controller) *MockRepository {
	mock := &MockRepository{ctrl: ctrl}
	mock.recorder = &MockRepositoryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRepository) EXPECT() *MockRepositoryMockRecorder {
	return m.recorder
}

// BulkCreate mocks base method.
func (m *MockRepository) BulkCreate(ctx context.Context, cards []*models.Card) (int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BulkCreate", ctx, cards)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// BulkCreate indicates an expected call of BulkCreate.
func (mr *MockRepositoryMockRecorder) BulkCreate(ctx, cards any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BulkCreate", reflect.TypeOf((*MockRepository)(nil).BulkCreate), ctx, cards)
}

// Create mocks base method.
func (m *MockRepository) Create(ctx context.Context, card *models.Card) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Create", ctx, card)
	ret0, _ := ret[0].(error)
	return ret0
}

// Create indicates an expected call of Create.
func (mr *MockRepositoryMockRecorder) Create(ctx, card any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockRepository)(nil).Create), ctx, card)
}

// Delete mocks base method.
func (m *MockRepository) Delete(ctx context.Context, id int64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", ctx, id)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockRepositoryMockRecorder) Delete(ctx, id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockRepository)(nil).Delete), ctx, id)
}

// DeleteUserCard mocks base method.
func (m *MockRepository) DeleteUserCard(ctx context.Context, id int64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteUserCard", ctx, id)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteUserCard indicates an expected call of DeleteUserCard.
func (mr *MockRepositoryMockRecorder) DeleteUserCard(ctx, id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteUserCard", reflect.TypeOf((*MockRepository)(nil).DeleteUserCard), ctx, id)
}

// GetAll mocks base method.
func (m *MockRepository) GetAll(ctx context.Context) ([]*models.Card, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAll", ctx)
	ret0, _ := ret[0].([]*models.Card)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAll indicates an expected call of GetAll.
func (mr *MockRepositoryMockRecorder) GetAll(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAll", reflect.TypeOf((*MockRepository)(nil).GetAll), ctx)
}

// GetAllByUserID mocks base method.
func (m *MockRepository) GetAllByUserID(ctx context.Context, userID string) ([]*models.UserCard, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAllByUserID", ctx, userID)
	ret0, _ := ret[0].([]*models.UserCard)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAllByUserID indicates an expected call of GetAllByUserID.
func (mr *MockRepositoryMockRecorder) GetAllByUserID(ctx, userID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAllByUserID", reflect.TypeOf((*MockRepository)(nil).GetAllByUserID), ctx, userID)
}

// GetAnimated mocks base method.
func (m *MockRepository) GetAnimated(ctx context.Context) ([]*models.Card, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAnimated", ctx)
	ret0, _ := ret[0].([]*models.Card)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAnimated indicates an expected call of GetAnimated.
func (mr *MockRepositoryMockRecorder) GetAnimated(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAnimated", reflect.TypeOf((*MockRepository)(nil).GetAnimated), ctx)
}

// GetByCollectionID mocks base method.
func (m *MockRepository) GetByCollectionID(ctx context.Context, colID string) ([]*models.Card, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetByCollectionID", ctx, colID)
	ret0, _ := ret[0].([]*models.Card)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetByCollectionID indicates an expected call of GetByCollectionID.
func (mr *MockRepositoryMockRecorder) GetByCollectionID(ctx, colID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetByCollectionID", reflect.TypeOf((*MockRepository)(nil).GetByCollectionID), ctx, colID)
}

// GetByID mocks base method.
func (m *MockRepository) GetByID(ctx context.Context, id int64) (*models.Card, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetByID", ctx, id)
	ret0, _ := ret[0].(*models.Card)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetByID indicates an expected call of GetByID.
func (mr *MockRepositoryMockRecorder) GetByID(ctx, id any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetByID", reflect.TypeOf((*MockRepository)(nil).GetByID), ctx, id)
}

// GetByIDs mocks base method.
func (m *MockRepository) GetByIDs(ctx context.Context, ids []int64) ([]*models.Card, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetByIDs", ctx, ids)
	ret0, _ := ret[0].([]*models.Card)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetByIDs indicates an expected call of GetByIDs.
func (mr *MockRepositoryMockRecorder) GetByIDs(ctx, ids any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetByIDs", reflect.TypeOf((*MockRepository)(nil).GetByIDs), ctx, ids)
}

// GetByLevel mocks base method.
func (m *MockRepository) GetByLevel(ctx context.Context, level int) ([]*models.Card, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetByLevel", ctx, level)
	ret0, _ := ret[0].([]*models.Card)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetByLevel indicates an expected call of GetByLevel.
func (mr *MockRepositoryMockRecorder) GetByLevel(ctx, level any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetByLevel", reflect.TypeOf((*MockRepository)(nil).GetByLevel), ctx, level)
}

// GetByName mocks base method.
func (m *MockRepository) GetByName(ctx context.Context, name string) ([]*models.Card, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetByName", ctx, name)
	ret0, _ := ret[0].([]*models.Card)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetByName indicates an expected call of GetByName.
func (mr *MockRepositoryMockRecorder) GetByName(ctx, name any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetByName", reflect.TypeOf((*MockRepository)(nil).GetByName), ctx, name)
}

// GetByQuery mocks base method.
func (m *MockRepository) GetByQuery(ctx context.Context, query string) (*models.Card, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetByQuery", ctx, query)
	ret0, _ := ret[0].(*models.Card)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetByQuery indicates an expected call of GetByQuery.
func (mr *MockRepositoryMockRecorder) GetByQuery(ctx, query any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetByQuery", reflect.TypeOf((*MockRepository)(nil).GetByQuery), ctx, query)
}

// GetByTag mocks base method.
func (m *MockRepository) GetByTag(ctx context.Context, tag string) ([]*models.Card, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetByTag", ctx, tag)
	ret0, _ := ret[0].([]*models.Card)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetByTag indicates an expected call of GetByTag.
func (mr *MockRepositoryMockRecorder) GetByTag(ctx, tag any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetByTag", reflect.TypeOf((*MockRepository)(nil).GetByTag), ctx, tag)
}

// GetUserCard mocks base method.
func (m *MockRepository) GetUserCard(ctx context.Context, userID string, cardID int64) (*models.UserCard, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetUserCard", ctx, userID, cardID)
	ret0, _ := ret[0].(*models.UserCard)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetUserCard indicates an expected call of GetUserCard.
func (mr *MockRepositoryMockRecorder) GetUserCard(ctx, userID, cardID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUserCard", reflect.TypeOf((*MockRepository)(nil).GetUserCard), ctx, userID, cardID)
}

// SafeDelete mocks base method.
func (m *MockRepository) SafeDelete(ctx context.Context, cardID int64) (*models.DeletionReport, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SafeDelete", ctx, cardID)
	ret0, _ := ret[0].(*models.DeletionReport)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SafeDelete indicates an expected call of SafeDelete.
func (mr *MockRepositoryMockRecorder) SafeDelete(ctx, cardID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SafeDelete", reflect.TypeOf((*MockRepository)(nil).SafeDelete), ctx, cardID)
}

// Search mocks base method.
func (m *MockRepository) Search(ctx context.Context, filters models.SearchFilters, offset, limit int) ([]*models.Card, int, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Search", ctx, filters, offset, limit)
	ret0, _ := ret[0].([]*models.Card)
	ret1, _ := ret[1].(int)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Search indicates an expected call of Search.
func (mr *MockRepositoryMockRecorder) Search(ctx, filters, offset, limit any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Search", reflect.TypeOf((*MockRepository)(nil).Search), ctx, filters, offset, limit)
}

// Update mocks base method.
func (m *MockRepository) Update(ctx context.Context, card *models.Card) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Update", ctx, card)
	ret0, _ := ret[0].(error)
	return ret0
}

// Update indicates an expected call of Update.
func (mr *MockRepositoryMockRecorder) Update(ctx, card any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockRepository)(nil).Update), ctx, card)
}

// UpdateUserCard mocks base method.
func (m *MockRepository) UpdateUserCard(ctx context.Context, userCard *models.UserCard) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateUserCard", ctx, userCard)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateUserCard indicates an expected call of UpdateUserCard.
func (mr *MockRepositoryMockRecorder) UpdateUserCard(ctx, userCard any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateUserCard", reflect.TypeOf((*MockRepository)(nil).UpdateUserCard), ctx, userCard)
}
