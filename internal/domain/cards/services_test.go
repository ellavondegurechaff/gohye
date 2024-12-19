package cards

import (
	"reflect"
	"testing"

	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/bot-template/internal/domain/cards/mock"
	"go.uber.org/mock/gomock"
)

func repoMock(t *testing.T) *mock.MockRepository {
	repo := mock.NewMockRepository(gomock.NewController(t))
	repo.EXPECT().
		GetAllByUserID(gomock.Any(), "123").
		Return(mock.UserCards, nil)

	repo.EXPECT().
		GetByID(gomock.Any(), int64(1)).
		Return(mock.Cards[0], nil)

	return repo
}

var mockCards = []Card{
	{
		ID: 1,
	},
}

func Test_service_GetUserCards(t *testing.T) {
	type args struct {
		userID  string
		filters utils.FilterInfo
	}
	tests := []struct {
		name      string
		args      args
		want      []Card
		wantPages int
		wantErr   bool
	}{
		{
			name: "Success",
			args: args{
				userID:  "123",
				filters: utils.FilterInfo{},
			},
			want:      mockCards,
			wantPages: 1,
			wantErr:   false,
		},
	}

	s := &service{
		repository: repoMock(t),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := s.GetUserCards(tt.args.userID, tt.args.filters)
			if (err != nil) != tt.wantErr {
				t.Errorf("service.GetUserCards() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("service.GetUserCards() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.wantPages {
				t.Errorf("service.GetUserCards() got1 = %v, want %v", got1, tt.wantPages)
			}
		})
	}
}
