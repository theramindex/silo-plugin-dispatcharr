package config

import (
	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

type ConfigSchema = pluginv1.ConfigSchema

func GlobalConfigSchema() []*ConfigSchema {
	return []*ConfigSchema{
		objectSchema("general", "General", "General Dispatcharr plugin settings.", `{"type":"object","properties":{"source_mode":{"type":"string","enum":["direct_login","api_key","xtream","m3u_xmltv"]},"live_tv_enabled":{"type":"boolean"},"channel_refresh_hours":{"type":"integer","minimum":1},"epg_refresh_hours":{"type":"integer","minimum":1}},"required":["source_mode"],"additionalProperties":false}`, true, []*pluginv1.AdminFormField{
			{Key: "source_mode", Label: "Source Mode", Description: "Use Dispatcharr direct auth first; Xtream and M3U/XMLTV remain fallbacks.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_SELECT, Required: true, DefaultValue: structpb.NewStringValue(string(SourceModeDirectLogin)), Options: []*pluginv1.AdminFormOption{{Value: string(SourceModeDirectLogin), Label: "Dispatcharr login"}, {Value: string(SourceModeAPIKey), Label: "Dispatcharr API key"}, {Value: string(SourceModeXtream), Label: "Xtream"}, {Value: string(SourceModeM3UXMLTV), Label: "M3U/XMLTV"}}},
			{Key: "live_tv_enabled", Label: "Enable Live TV", Description: "Expose the Live TV app route to Silo users.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_SWITCH, DefaultValue: structpb.NewBoolValue(true)},
			{Key: "channel_refresh_hours", Label: "Channel Refresh Hours", Description: "Refresh cadence for channels and categories.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_NUMBER, DefaultValue: structpb.NewNumberValue(DefaultChannelRefreshHours)},
			{Key: "epg_refresh_hours", Label: "EPG Refresh Hours", Description: "Refresh cadence for guide data.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_NUMBER, DefaultValue: structpb.NewNumberValue(DefaultEPGRefreshHours)},
		}, "Save general settings"),
		objectSchema("dispatcharr", "Dispatcharr", "Dispatcharr direct connection settings.", `{"type":"object","properties":{"base_url":{"type":"string","format":"uri"},"username":{"type":"string","minLength":1},"password":{"type":"string","minLength":1,"writeOnly":true},"api_key":{"type":"string","writeOnly":true},"user_agent":{"type":"string"}},"required":["base_url"],"additionalProperties":false}`, true, []*pluginv1.AdminFormField{
			{Key: "base_url", Label: "Dispatcharr URL", Description: "Dispatcharr server URL.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT, Placeholder: "https://dispatcharr.example.com", Required: true},
			{Key: "username", Label: "Username", Description: "Dispatcharr username for direct login.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT},
			{Key: "password", Label: "Password", Description: "Dispatcharr password for direct login.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_PASSWORD, Secret: true},
			{Key: "api_key", Label: "API Key", Description: "Optional Dispatcharr API key.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_PASSWORD, Secret: true},
			{Key: "user_agent", Label: "User Agent", Description: "Optional client user agent for Dispatcharr requests.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT, Placeholder: "Silo Dispatcharr Plugin"},
		}, "Save Dispatcharr settings"),
		objectSchema("xtream", "Xtream", "Xtream fallback connection settings for Dispatcharr-compatible providers.", `{"type":"object","properties":{"base_url":{"type":"string","format":"uri"},"username":{"type":"string","minLength":1},"password":{"type":"string","minLength":1,"writeOnly":true}},"required":["base_url","username","password"],"additionalProperties":false}`, false, []*pluginv1.AdminFormField{
			{Key: "base_url", Label: "Xtream Base URL", Description: "Dispatcharr Xtream endpoint base URL.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT, Placeholder: "https://dispatcharr.example.com", Required: true},
			{Key: "username", Label: "Xtream Username", Description: "Xtream username for Dispatcharr.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT, Required: true},
			{Key: "password", Label: "Xtream Password", Description: "Xtream password for Dispatcharr.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_PASSWORD, Required: true, Secret: true},
		}, "Save Xtream settings"),
		objectSchema("m3u_xmltv", "M3U/XMLTV", "Fallback playlist and XMLTV settings.", `{"type":"object","properties":{"m3u_url":{"type":"string","format":"uri"},"epg_xml_url":{"type":"string","format":"uri"}},"required":["m3u_url","epg_xml_url"],"additionalProperties":false}`, false, []*pluginv1.AdminFormField{
			{Key: "m3u_url", Label: "M3U URL", Description: "Playlist URL for fallback mode.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT, Placeholder: "https://dispatcharr.example.com/playlist.m3u", Required: true},
			{Key: "epg_xml_url", Label: "EPG XML URL", Description: "XMLTV URL for fallback mode.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT, Placeholder: "https://dispatcharr.example.com/guide.xml", Required: true},
		}, "Save M3U/XMLTV settings"),
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
