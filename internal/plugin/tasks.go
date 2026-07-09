package plugin

import (
	"context"
	"strings"
	"time"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/app"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/config"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	SyncTaskKey           = "dispatcharr-sync"
	ChannelRefreshTaskKey = "dispatcharr-refresh-channels"
	EPGRefreshTaskKey     = "dispatcharr-refresh-epg"
)

type ScheduledTaskServer struct {
	pluginv1.UnimplementedScheduledTaskServer
	coordinator      *RefreshCoordinator
	settingsProvider func() config.Settings
}

func NewScheduledTaskServer(service *app.Service, settings config.Settings) *ScheduledTaskServer {
	return &ScheduledTaskServer{coordinator: NewRefreshCoordinator(service), settingsProvider: func() config.Settings { return settings }}
}

func NewScheduledTaskServerWithProvider(service *app.Service, provider func() config.Settings) *ScheduledTaskServer {
	return &ScheduledTaskServer{coordinator: NewRefreshCoordinator(service), settingsProvider: provider}
}

func NewScheduledTaskServerWithCoordinator(coordinator *RefreshCoordinator, provider func() config.Settings) *ScheduledTaskServer {
	return &ScheduledTaskServer{coordinator: coordinator, settingsProvider: provider}
}

func (s *ScheduledTaskServer) Run(ctx context.Context, request *pluginv1.RunScheduledTaskRequest) (*pluginv1.RunScheduledTaskResponse, error) {
	taskKey := request.GetTaskKey()
	taskKind := "unknown"
	now := time.Now().Unix()
	switch {
	case isTaskKey(taskKey, SyncTaskKey):
		taskKind = "catalog"
		if err := s.coordinator.Run(ctx, RefreshCatalog, s.settingsProvider(), now); err != nil {
			return nil, err
		}
	case isTaskKey(taskKey, ChannelRefreshTaskKey):
		taskKind = "channels"
		if err := s.coordinator.Run(ctx, RefreshChannels, s.settingsProvider(), now); err != nil {
			return nil, err
		}
	case isTaskKey(taskKey, EPGRefreshTaskKey):
		taskKind = "epg"
		if err := s.coordinator.Run(ctx, RefreshGuide, s.settingsProvider(), now); err != nil {
			return nil, err
		}
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unknown scheduled task %q", taskKey)
	}

	output, err := structpb.NewStruct(map[string]any{"status": "ok", "task": taskKind})
	if err != nil {
		return nil, err
	}
	return &pluginv1.RunScheduledTaskResponse{Output: output}, nil
}

func isTaskKey(taskKey string, capabilityID string) bool {
	return taskKey == capabilityID || strings.HasSuffix(taskKey, ":"+capabilityID)
}
