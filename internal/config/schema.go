package config

import (
	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

type ConfigSchema = pluginv1.ConfigSchema

const connectionJSONSchema = `{
  "type": "object",
  "properties": {
    "source_mode": {
      "type": "string",
      "enum": ["direct_login", "api_key", "xtream", "m3u_xmltv"],
      "default": "direct_login"
    },
    "base_url": {
      "type": "string",
      "format": "uri"
    },
    "api_key": {
      "type": "string",
      "writeOnly": true
    },
    "username": {
      "type": "string"
    },
    "password": {
      "type": "string",
      "writeOnly": true
    },
    "m3u_url": {
      "type": "string",
      "format": "uri"
    },
    "epg_xml_url": {
      "type": "string",
      "format": "uri"
    },
    "live_tv_enabled": {
      "type": "boolean"
    },
    "channel_refresh_hours": {
      "type": "integer",
      "minimum": 1
    },
    "epg_refresh_hours": {
      "type": "integer",
      "minimum": 1
    }
  },
  "required": ["source_mode"],
  "additionalProperties": false,
  "allOf": [
    {
      "if": {
        "properties": {
          "source_mode": {
            "const": "direct_login"
          }
        }
      },
      "then": {
        "required": ["base_url", "username", "password"]
      }
    },
    {
      "if": {
        "properties": {
          "source_mode": {
            "const": "api_key"
          }
        }
      },
      "then": {
        "required": ["base_url", "api_key"]
      }
    },
    {
      "if": {
        "properties": {
          "source_mode": {
            "const": "xtream"
          }
        }
      },
      "then": {
        "required": ["base_url", "username", "password"]
      }
    },
    {
      "if": {
        "properties": {
          "source_mode": {
            "const": "m3u_xmltv"
          }
        }
      },
      "then": {
        "required": ["m3u_url", "epg_xml_url"]
      }
    }
  ]
}`

func GlobalConfigSchema() []*ConfigSchema {
	return []*ConfigSchema{
		objectSchema("connection", "Live TV Connection", "Choose how Silo should connect to Dispatcharr or another IPTV source.", connectionJSONSchema, true, []*pluginv1.AdminFormField{
			{Key: "source_mode", Label: "Source Type", Description: "Dispatcharr Direct is recommended. Xtream Codes can be used with Dispatcharr's API & XC credentials.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_SELECT, DefaultValue: structpb.NewStringValue(string(SourceModeDirectLogin)), Options: []*pluginv1.AdminFormOption{
				{Value: string(SourceModeDirectLogin), Label: "Dispatcharr Direct Connect", Description: "Use Dispatcharr REST with a username and password. Silo keeps the session refreshed for sync and playback."},
				{Value: string(SourceModeAPIKey), Label: "Dispatcharr Direct: API Key", Description: "Use a Dispatcharr Admin API key from System > Users > Edit User > API & XC."},
				{Value: string(SourceModeXtream), Label: "Xtream Codes", Description: "Use player_api.php, live streams, VOD, series, and XC EPG endpoints."},
				{Value: string(SourceModeM3UXMLTV), Label: "M3U + EPG", Description: "Use a playlist URL plus XMLTV guide data. Live TV and guide only."},
			}},
			{Key: "base_url", Label: "Server URL", Description: "Dispatcharr or Xtream Codes server URL.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT, Placeholder: "https://dispatcharr.example.com", Required: true},
			{Key: "api_key", Label: "Admin API Key", Description: "Dispatcharr Admin API key from System > Users > Edit User > API & XC.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_PASSWORD, Secret: true},
			{Key: "username", Label: "Username", Description: "Dispatcharr dashboard username, or Xtream Codes username from Dispatcharr's User settings.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT},
			{Key: "password", Label: "Password", Description: "Dispatcharr dashboard password, or Xtream Codes password from Dispatcharr's User settings.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_PASSWORD, Secret: true},
			{Key: "m3u_url", Label: "M3U Playlist URL", Description: "Playlist URL for M3U + EPG mode.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT, Placeholder: "https://provider.example.com/playlist.m3u"},
			{Key: "epg_xml_url", Label: "Custom XMLTV URL", Description: "Optional guide XML URL for Xtream Codes, required for M3U + EPG.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT, Placeholder: "https://provider.example.com/guide.xml"},
			{Key: "live_tv_enabled", Label: "Enable Live TV", Description: "Expose the Live TV app route to Silo users.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_SWITCH, DefaultValue: structpb.NewBoolValue(true)},
			{Key: "channel_refresh_hours", Label: "Channel Refresh Hours", Description: "Refresh cadence for channels and categories.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_NUMBER, DefaultValue: structpb.NewNumberValue(DefaultChannelRefreshHours)},
			{Key: "epg_refresh_hours", Label: "EPG Refresh Hours", Description: "Refresh cadence for guide data.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_NUMBER, DefaultValue: structpb.NewNumberValue(DefaultEPGRefreshHours)},
		}, "Save Live TV connection"),
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
