package plugin

import (
	"context"
	"strings"
	"time"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/app"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	SyncTaskKey           = "dispatcharr-sync"
	ChannelRefreshTaskKey = "dispatcharr-refresh-channels"
	EPGRefreshTaskKey     = "dispatcharr-refresh-epg"
)

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
	taskKey := request.GetTaskKey()
	taskKind := "unknown"
	now := time.Now().Unix()
	switch {
	case isTaskKey(taskKey, SyncTaskKey), isTaskKey(taskKey, ChannelRefreshTaskKey):
		taskKind = "catalog"
		if err := s.service.SyncNow(ctx, s.settingsProvider(), now); err != nil {
			return nil, err
		}
	case isTaskKey(taskKey, EPGRefreshTaskKey):
		taskKind = "epg"
		if err := s.service.RefreshEPGNow(ctx, s.settingsProvider(), now); err != nil {
			return nil, err
		}
	}

	output, err := structpb.NewStruct(map[string]any{"status": "ok", "task": taskKind})
	if err != nil {
		return nil, err
	}
	return &pluginv1.RunScheduledTaskResponse{Output: output}, nil
}

func isSyncTaskKey(taskKey string) bool {
	return isTaskKey(taskKey, SyncTaskKey)
}

func isTaskKey(taskKey string, capabilityID string) bool {
	return taskKey == capabilityID || strings.HasSuffix(taskKey, ":"+capabilityID)
}
