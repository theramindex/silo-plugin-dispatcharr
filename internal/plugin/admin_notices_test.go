package plugin

import (
	"strings"
	"testing"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/theramindex/silo-plugin-dispatcharr/internal/cache"
)

func TestGuideLicenseNoticeOnlyInAdmin(t *testing.T) {
	server := NewHTTPRoutesServer(cache.NewStore())
	for _, path := range []string{"/dispatcharr", "/dispatcharr/player", "/dispatcharr/admin"} {
		t.Run(path, func(t *testing.T) {
			body := server.playerPageHTML(&pluginv1.HandleHTTPRequest{Path: path})
			admin := path == "/dispatcharr/admin"
			for _, marker := range []string{"IPTVnator", "Copyright 2020-2021", "admin-license-notices", "Permission is hereby granted"} {
				if strings.Contains(body, marker) != admin {
					t.Errorf("notice %q presence did not match admin route", marker)
				}
			}
		})
	}
}
