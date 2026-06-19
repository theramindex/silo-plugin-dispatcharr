# Silo Host Navigation and Sync Notes

## User Apps navigation

Silo's user sidebar has an Apps section for installed plugins. A plugin route appears there when the route descriptor is:

```json
{
  "method": "GET",
  "path": "/dispatcharr",
  "access": "authenticated",
  "navigable": true,
  "navigation_label": "IPTV",
  "navigation_kind": "user"
}
```

The local Silo SDK v0.7.0 includes the required fields on `HttpRouteDescriptor`:

- `navigable`
- `navigation_label`
- `navigation_kind`

Use `navigation_kind: "user"` for normal user sidebar Apps entries. Use `navigation_kind: "admin"` with `access: "admin"` for admin navigation.

## Dispatcharr sync task

The plugin exposes one scheduled task capability:

```text
dispatcharr-sync
```

Silo registers that task as:

```text
plugin:<installation_id>:dispatcharr-sync
```

Current Silo host builds store task cadence outside of the plugin manifest. If no task binding trigger is configured, Silo falls back to a startup-only trigger. For Dispatcharr, configure:

```json
[
  { "type": "startup" },
  { "type": "interval", "interval_ms": 86400000 }
]
```

The startup trigger hydrates channels and EPG after install/restart. The interval trigger refreshes channels and EPG every 24 hours.
