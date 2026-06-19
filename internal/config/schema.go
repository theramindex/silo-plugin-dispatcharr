package config

import (
	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

type ConfigSchema = pluginv1.ConfigSchema

func GlobalConfigSchema() []*ConfigSchema {
	return []*ConfigSchema{
		objectSchema("connection", "Dispatcharr Connection", "One Dispatcharr URL plus either an API key or username/password.", `{"type":"object","properties":{"base_url":{"type":"string","format":"uri"},"api_key":{"type":"string","writeOnly":true},"username":{"type":"string"},"password":{"type":"string","writeOnly":true},"live_tv_enabled":{"type":"boolean"},"channel_refresh_hours":{"type":"integer","minimum":1},"epg_refresh_hours":{"type":"integer","minimum":1},"source_mode":{"type":"string","enum":["direct_login","api_key"]}},"required":["base_url"],"additionalProperties":false}`, true, []*pluginv1.AdminFormField{
			{Key: "base_url", Label: "Dispatcharr URL", Description: "Dispatcharr server URL.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT, Placeholder: "https://dispatcharr.example.com", Required: true},
			{Key: "api_key", Label: "API Key", Description: "Use this if available; otherwise use username/password.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_PASSWORD, Secret: true},
			{Key: "username", Label: "Username", Description: "Dispatcharr username for direct login.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT},
			{Key: "password", Label: "Password", Description: "Dispatcharr password for direct login.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_PASSWORD, Secret: true},
			{Key: "live_tv_enabled", Label: "Enable Live TV", Description: "Expose the Live TV app route to Silo users.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_SWITCH, DefaultValue: structpb.NewBoolValue(true)},
			{Key: "channel_refresh_hours", Label: "Channel Refresh Hours", Description: "Refresh cadence for channels and categories.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_NUMBER, DefaultValue: structpb.NewNumberValue(DefaultChannelRefreshHours)},
			{Key: "epg_refresh_hours", Label: "EPG Refresh Hours", Description: "Refresh cadence for guide data.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_NUMBER, DefaultValue: structpb.NewNumberValue(DefaultEPGRefreshHours)},
		}, "Save Dispatcharr settings"),
	}
}

func UserConfigSchema() []*ConfigSchema {
	return []*ConfigSchema{
		objectSchema("preferences", "Preferences", "Per-user Dispatcharr plugin preferences stored by Silo.", `{"type":"object","properties":{"favorites":{"type":"object","additionalProperties":{"type":"boolean"}},"auto_favorites":{"type":"object","additionalProperties":{"type":"boolean"}},"hidden_categories":{"type":"object","additionalProperties":{"type":"boolean"}},"recent_channels":{"type":"array","items":{"type":"string"}},"continue_watching":{"type":"object","additionalProperties":true},"playback":{"type":"object","additionalProperties":true}},"additionalProperties":false}`, false, []*pluginv1.AdminFormField{}, "Save preferences"),
	}
}

func objectSchema(key, title, description, jsonSchema string, required bool, fields []*pluginv1.AdminFormField, submitLabel string) *ConfigSchema {
	return &pluginv1.ConfigSchema{
		Key:         key,
		Title:       title,
		Description: description,
		JsonSchema:  jsonSchema,
		Required:    required,
		AdminForm:   &pluginv1.AdminFormDescriptor{Fields: fields, SubmitLabel: submitLabel},
	}
}
