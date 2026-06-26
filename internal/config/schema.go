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

const categorySettingsJSONSchema = `{
  "type": "object",
  "properties": {
    "mode": {
      "type": "string",
      "enum": ["normal", "delimiter"],
      "default": "normal"
    },
    "delimiter": {
      "type": "string",
      "enum": ["pipe", "dash"],
      "default": "pipe"
    },
    "ecmEnabled": {
      "type": "boolean",
      "default": false
    },
    "ecmURL": {
      "type": "string",
      "default": ""
    },
    "categoryAliases": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "sourcePath": {
            "type": "string",
            "minLength": 1
          },
          "aliasPath": {
            "type": "string",
            "minLength": 1
          }
        },
        "required": ["sourcePath", "aliasPath"],
        "additionalProperties": false
      },
      "default": []
    }
  },
  "additionalProperties": false
}`

func GlobalConfigSchema() []*ConfigSchema {
	return []*ConfigSchema{
		objectSchema("connection", "Dispatcharr for Silo", "Pick one source type. Silo shows every field in this form, so fill only the fields named by the selected source type.", connectionJSONSchema, true, []*pluginv1.AdminFormField{
			{Key: "source_mode", Label: "Source Type", Description: "This controls which fields are required. Direct and Xtream both use Server URL, Username, and Password.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_SELECT, DefaultValue: structpb.NewStringValue(string(SourceModeDirectLogin)), Options: []*pluginv1.AdminFormOption{
				{Value: string(SourceModeDirectLogin), Label: "Dispatcharr Direct Connect", Description: "Use Dispatcharr REST. Fill Server URL, Username, and Password with your Dispatcharr dashboard login."},
				{Value: string(SourceModeAPIKey), Label: "Dispatcharr Direct: API Key", Description: "Use Dispatcharr REST without saving a password. Fill Server URL and Admin API Key."},
				{Value: string(SourceModeXtream), Label: "Xtream Codes", Description: "Use Dispatcharr's Xtream-compatible API. Fill Server URL, Username, and Password with the API & XC credentials from Dispatcharr user settings."},
				{Value: string(SourceModeM3UXMLTV), Label: "M3U + XMLTV", Description: "Use playlist files only. Fill M3U Playlist URL and Custom XMLTV URL; ignore Server URL, Username, Password, and API Key."},
			}},
			{Key: "base_url", Label: "Server URL", Description: "Required for Direct Connect, API Key, and Xtream Codes. For Dispatcharr Xtream, use the Dispatcharr base URL, not the full player_api.php URL.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT, Placeholder: "https://dispatcharr.example.com", Required: true},
			{Key: "api_key", Label: "Admin API Key", Description: "Only used by Dispatcharr Direct: API Key. Find it in Dispatcharr under System > Users > Edit User > API & XC.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_PASSWORD, Secret: true},
			{Key: "username", Label: "Username", Description: "Required for Direct Connect and Xtream Codes. Use your Dispatcharr dashboard username for Direct Connect, or the XC username from Dispatcharr user settings for Xtream.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT},
			{Key: "password", Label: "Password", Description: "Required for Direct Connect and Xtream Codes. Use your Dispatcharr dashboard password for Direct Connect, or the XC password from Dispatcharr user settings for Xtream.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_PASSWORD, Secret: true},
			{Key: "m3u_url", Label: "M3U Playlist URL", Description: "Only used by M3U + XMLTV. Leave this blank for Direct Connect, API Key, and Xtream Codes.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT, Placeholder: "https://provider.example.com/playlist.m3u"},
			{Key: "epg_xml_url", Label: "Custom XMLTV URL", Description: "Required for M3U + XMLTV. Optional for Xtream Codes if you want to override the provider's built-in EPG.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_TEXT, Placeholder: "https://provider.example.com/guide.xml"},
			{Key: "live_tv_enabled", Label: "Enable Live TV", Description: "Expose the Live TV app route to Silo users.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_SWITCH, DefaultValue: structpb.NewBoolValue(true)},
			{Key: "channel_refresh_hours", Label: "Channel Refresh Hours", Description: "Refresh cadence for channels and categories.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_NUMBER, DefaultValue: structpb.NewNumberValue(DefaultChannelRefreshHours)},
			{Key: "epg_refresh_hours", Label: "EPG Refresh Hours", Description: "Refresh cadence for guide data.", Control: pluginv1.AdminFormControl_ADMIN_FORM_CONTROL_NUMBER, DefaultValue: structpb.NewNumberValue(DefaultEPGRefreshHours)},
		}, "Save Dispatcharr for Silo settings"),
		objectSchema("category_settings", "Live TV Category Settings", "Admin-managed Live TV category presentation mode and delimiter.", categorySettingsJSONSchema, false, []*pluginv1.AdminFormField{}, "Save category settings"),
	}
}

func UserConfigSchema() []*ConfigSchema {
	return []*ConfigSchema{
		objectSchema("preferences", "Preferences", "Per-user Dispatcharr plugin preferences stored by Silo.", `{"type":"object","properties":{"favorites":{"type":"object","additionalProperties":{"type":"boolean"}},"favoriteOrder":{"type":"array","items":{"type":"string"}},"autoFavorites":{"type":"object","additionalProperties":{"type":"boolean"}},"hiddenCategories":{"type":"object","additionalProperties":{"type":"boolean"}},"sportsFavoriteTeams":{"type":"object","additionalProperties":{"type":"boolean"}},"recentChannels":{"type":"array","items":{"type":"string"}},"continueWatching":{"type":"object","additionalProperties":true},"playback":{"type":"object","additionalProperties":true},"categoryParsing":{"type":"object","properties":{"enabled":{"type":"boolean"},"mode":{"type":"string","enum":["off","delimiter","regex"]},"delimiter":{"type":"string","enum":["dash","pipe"]},"regex":{"type":"string"},"output":{"type":"string"}},"additionalProperties":false},"customGroups":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"},"order":{"type":"integer"}},"required":["id","name"],"additionalProperties":false}},"customGroupMemberships":{"type":"object","additionalProperties":{"type":"array","items":{"type":"string"}}}},"additionalProperties":false}`, false, []*pluginv1.AdminFormField{}, "Save preferences"),
		objectSchema("adminCategorySettings", "Admin Category Settings", "Admin-managed Live TV category mode saved through Silo plugin settings.", categorySettingsJSONSchema, false, []*pluginv1.AdminFormField{}, "Save category settings"),
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
