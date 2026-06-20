package config

import (
	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

type ConfigSchema = pluginv1.ConfigSchema

func GlobalConfigSchema() []*ConfigSchema {
	return []*ConfigSchema{
		objectSchema("connection", "Live TV Connection", "Choose Dispatcharr Direct or a generic Xtream Codes-compatible endpoint.", `{"type":"object","properties":{"source_mode":{"type":"string","enum":["direct_login","api_key","xtream"],"default":"direct_login"},"base_url":{"type":"string","format":"uri"},"api_key":{"type":"string","writeOnly":true},"username":{"type":"string"},"password":{"type":"string","writeOnly":true},"xtream_base_url":{"type":"string","format":"uri"},"xtream_username":{"type":"string"},"xtream_password":{"type":"string","writeOnly":true},"live_tv_enabled":{"type":"boolean"},"channel_refresh_hours":{"type":"integer","minimum":1},"epg_refresh_hours":{"type":"integer","minimum":1}},"required":["source_mode"],"additionalProperties":false,"allOf":[{"if":{"properties":{"source_mode":{"const":"direct_login"}}},"then":{"required":["base_url","username","password"]}},{"if":{"properties":{"source_mode":{"const":"api_key"}}},"then":{"required":["base_url","api_key"]}},{"if":{"properties":{"source_mode":{"const":"xtream"}}},"then":{"required":["xtream_base_url","xtream_username","xtream_password"]}}]}`, true, []*pluginv1.AdminFormField{
			{Key: "source_mode", Label: "Connection Type", Description: "Use Dispatcharr Direct for Dispatcharr servers, or Xtream Codes for generic XC-compatible providers.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_SELECT, DefaultValue: structpb.NewStringValue(string(SourceModeDirectLogin)), Options: []*pluginv1.AdminFormOption{
				{Value: string(SourceModeDirectLogin), Label: "Dispatcharr Direct", Description: "Login with a Dispatcharr username and password. Uses Dispatcharr REST for catalog and guide data."},
				{Value: string(SourceModeAPIKey), Label: "Dispatcharr API Key", Description: "Use a Dispatcharr personal API key. Uses Dispatcharr REST for catalog and guide data."},
				{Value: string(SourceModeXtream), Label: "Xtream Codes", Description: "Use player_api.php, live streams, and XC EPG endpoints."},
			}},
			{Key: "base_url", Label: "Dispatcharr URL", Description: "Dispatcharr server URL.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT, Placeholder: "https://dispatcharr.example.com", Required: true},
			{Key: "api_key", Label: "Dispatcharr API Key", Description: "Personal API key for Dispatcharr API key mode.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_PASSWORD, Secret: true},
			{Key: "username", Label: "Username", Description: "Dispatcharr username for direct login.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT},
			{Key: "password", Label: "Password", Description: "Dispatcharr password for direct login.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_PASSWORD, Secret: true},
			{Key: "xtream_base_url", Label: "Xtream Base URL", Description: "Base URL for a generic Xtream Codes-compatible provider.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT, Placeholder: "https://provider.example.com"},
			{Key: "xtream_username", Label: "Xtream Username", Description: "Xtream Codes username.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT},
			{Key: "xtream_password", Label: "Xtream Password", Description: "Xtream Codes password.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_PASSWORD, Secret: true},
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
