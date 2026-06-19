package plugin

import (
	"context"
	"time"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/app"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
	"google.golang.org/protobuf/types/known/structpb"
)

const SyncTaskKey = "dispatcharr-sync"

type ScheduledTaskServer struct {
	pluginv1.UnimplementedScheduledTaskServer
	service          *app.Service
	settingsProvider func() config.Settings
}

func NewScheduledTaskServer(service *app.Service, settings config.Settings) *ScheduledTaskServer {
	return &ScheduledTaskServer{service: service, settingsProvider: func() config.Settings { return settings }}
}

func NewScheduledTaskServerWithProvider(service *app.Service, provider func() config.Settings) *ScheduledTaskServer {
	return &ScheduledTaskServer{service: service, settingsProvider: provider}
}

func (s *ScheduledTaskServer) Run(ctx context.Context, request *pluginv1.RunScheduledTaskRequest) (*pluginv1.RunScheduledTaskResponse, error) {
	if request.GetTaskKey() == SyncTaskKey {
		if err := s.service.SyncNow(ctx, s.settingsProvider(), time.Now().Unix()); err != nil {
			return nil, err
		}
	}

	output, err := structpb.NewStruct(map[string]any{"status": "ok"})
	if err != nil {
		return nil, err
	}
	return &pluginv1.RunScheduledTaskResponse{Output: output}, nil
}
