// Part of the composed app.js bundle (order: ui_assets.go playerAppScriptParts).
// Dispatcharr Admin: settings save, admin field updates, admin tab rendering.
function saveAdminCategorySettings() {
  state.adminCategorySettings = Object.assign(defaultAdminCategorySettings(), state.adminCategorySettings || {});
  normalizeAdminCategorySettings();
  state.adminSaveStatus = "saving";
  state.adminSaveMessage = "Saving...";
  if (state.view === "admin") renderAdminPage();
  postJSON(adminSettingsURL(), state.adminCategorySettings).then(function(saved) {
    state.adminCategorySettings = readAdminSettingsValue(saved);
    normalizeAdminCategorySettings();
    return persistAdminCategorySettingsInSilo(state.adminCategorySettings);
  }).then(function() {
    state.savedAdminCategorySettings = cloneAdminCategorySettings(state.adminCategorySettings);
    state.sportsReplayKey = "";
    state.sportsReplayItems = [];
    state.sportsReplayMatches = {};
    state.sportsReplayStandaloneEvents = [];
    state.adminSaveStatus = "saved";
    state.adminSaveMessage = "Saved plugin settings.";
    if (state.view === "admin") renderAdminPage();
  }).catch(function(error) {
    state.adminSaveStatus = "error";
    state.adminSaveMessage = "Could not save plugin settings: " + readableError(error);
    if (state.view === "admin") renderAdminPage();
    try { console.warn("Dispatcharr admin plugin settings save failed", error); } catch (_) {}
  });
}
function discardAdminCategorySettings() {
  state.adminCategorySettings = cloneAdminCategorySettings(state.savedAdminCategorySettings);
  state.adminSaveStatus = "idle";
  state.adminSaveMessage = "";
  if (state.category.indexOf("virtual:") === 0 && !categoryName(state.category)) state.category = "";
  renderAdminPage();
}
async function saveAdminOrganizationSettings() {
  if (state.adminSaveStatus === "saving" || state.adminConnectionStatus === "saving") return;
  const saveRules = adminSettingsDirty();
  const saveLineup = adminLineupDirty();
  if (!saveRules && !saveLineup) return;
  state.adminSaveStatus = "saving";
  state.adminSaveMessage = "Saving...";
  if (saveLineup) state.adminConnectionStatus = "saving";
  renderAdminPage();
  const failures = [];
  if (saveRules) {
    try {
      const saved = await postJSON(adminSettingsURL(), state.adminCategorySettings);
      state.adminCategorySettings = readAdminSettingsValue(saved);
      normalizeAdminCategorySettings();
      await persistAdminCategorySettingsInSilo(state.adminCategorySettings);
      state.savedAdminCategorySettings = cloneAdminCategorySettings(state.adminCategorySettings);
      state.sportsReplayKey = "";
      state.sportsReplayItems = [];
      state.sportsReplayMatches = {};
      state.sportsReplayStandaloneEvents = [];
    } catch (error) {
      failures.push("Folder rules could not be saved: " + readableError(error));
    }
  }
  if (saveLineup) {
    try {
      const result = await postJSON("/dispatcharr/api/admin-connection", adminConnectionPayload("save"));
      const returned = result && result.connection ? normalizeAdminConnection(result.connection) : normalizeAdminConnection(state.adminConnection);
      state.adminConnection = returned;
      state.adminConnection.password = "";
      state.adminConnection.apiKey = "";
      state.savedAdminConnection = cloneAdminConnection(state.adminConnection);
      state.adminConnectionStatus = "saved";
      state.adminConnectionMessage = "Lineup saved. Catalog refresh queued.";
      refreshAdminStatus();
    } catch (error) {
      state.adminConnectionStatus = "error";
      state.adminConnectionMessage = "Lineup could not be saved: " + readableError(error);
      failures.push(state.adminConnectionMessage);
    }
  }
  if (failures.length) {
    state.adminSaveStatus = "error";
    state.adminSaveMessage = failures.join(" ");
    showAppToast("Some organization changes could not be saved.");
  } else {
    state.adminSaveStatus = "saved";
    state.adminSaveMessage = "Organization saved.";
    showAppToast("Profiles and groups saved.");
  }
  renderAdminPage();
}
function discardAdminOrganizationSettings() {
  if (state.adminSaveStatus === "saving" || state.adminConnectionStatus === "saving") return;
  state.adminCategorySettings = cloneAdminCategorySettings(state.savedAdminCategorySettings);
  state.adminConnection = cloneAdminConnection(state.savedAdminConnection);
  state.adminSaveStatus = "idle";
  state.adminSaveMessage = "";
  state.adminConnectionStatus = "idle";
  state.adminConnectionMessage = "";
  renderAdminPage();
}
function updateAdminECMField(field, target) {
  const settings = state.adminCategorySettings || defaultAdminCategorySettings();
  if (field === "url") settings.ecmURL = target.value;
  state.adminCategorySettings = settings;
  normalizeAdminCategorySettings();
  if (!adminECMEnabled() && state.adminTab === "manager") state.adminTab = "integrations";
  markAdminSettingsDraft();
  renderAdminPage();
}
function updateAdminRecordingField(field, target) {
  const settings = state.adminCategorySettings || defaultAdminCategorySettings();
  if (field === "default") settings.allowRecordingsByDefault = !!target.checked;
  if (field === "prepad") settings.recordingPrePadMinutes = Math.max(0, Math.min(60, Math.round(Number(target.value || 0))));
  if (field === "postpad") settings.recordingPostPadMinutes = Math.max(0, Math.min(60, Math.round(Number(target.value || 0))));
  if (field === "comskip") settings.recordingComskipEnabled = !!target.checked;
  state.adminCategorySettings = settings;
  normalizeAdminCategorySettings();
  markAdminSettingsDraft();
  renderAdminPage();
}
function updateAdminIdentityField(field, target) {
  const settings = state.adminCategorySettings || defaultAdminCategorySettings();
  if (field === "name") settings.appDisplayName = target.value;
  if (field === "side-menu") settings.sideMenuMode = target.value;
  state.adminCategorySettings = settings;
  normalizeAdminCategorySettings();
  markAdminSettingsDraft();
  renderAdminPage();
}
function updateAdminPlayerField(field, target, preserveInput) {
  const settings = state.adminCategorySettings || defaultAdminCategorySettings();
  if (field === "sports") settings.sportsFirstPlayerEnabled = !!target.checked;
  if (field === "hls-buffer") settings.hlsBufferSeconds = Number(target.value || 12);
  state.adminCategorySettings = settings;
  normalizeAdminCategorySettings();
  markAdminSettingsDraft();
  if (preserveInput) renderAdminTopbarActions();
  else renderAdminPage();
}
function updateAdminSportsField(field, target) {
  const settings = state.adminCategorySettings || defaultAdminCategorySettings();
  if (field === "enabled") settings.sportsEnabled = !!target.checked;
  if (field === "guide") {
    settings.sportsGuide = adminSportsGuidePolicy(settings);
    settings.sportsGuide.enabled = !!target.checked;
  }
  if (field === "separate") settings.separateSportsApp = !!target.checked;
  if (field === "library") {
    const libraryID = Number(target.getAttribute("data-admin-sports-library-id"));
    let libraryIDs = items(settings.sportsLibraryIds).map(Number).filter(function(id) { return Number.isInteger(id) && id > 0; });
    if (target.checked && libraryIDs.indexOf(libraryID) === -1) libraryIDs.push(libraryID);
    if (!target.checked) libraryIDs = libraryIDs.filter(function(id) { return id !== libraryID; });
    settings.sportsLibraryIds = libraryIDs;
  }
  state.adminCategorySettings = settings;
  normalizeAdminCategorySettings();
  state.sportsReplayKey = "";
  markAdminSettingsDraft();
  renderAdminPage();
}
function updateAdminTimeShiftField(field, target, preserveInput) {
  const settings = state.adminCategorySettings || defaultAdminCategorySettings();
  if (field === "enabled") settings.liveRewindEnabled = !!target.checked;
  if (field === "cache") settings.liveRewindCacheGB = Number(target.value || 5);
  if (field === "window") settings.liveRewindWindowMinutes = Number(target.value || 30);
  if (field === "free") settings.liveRewindMinFreeGB = Number(target.value || 2);
  if (field === "channels") settings.liveRewindMaxChannels = Number(target.value || 20);
  state.adminCategorySettings = settings;
  normalizeAdminCategorySettings();
  markAdminSettingsDraft();
  if (preserveInput) renderAdminTopbarActions();
  else renderAdminPage();
}
function defaultAdminConnection() {
  return { sourceMode: "direct_login", baseUrl: "", username: "", password: "", apiKey: "", channelProfile: "", channelProfiles: [], channelGroups: [], m3uUrl: "", epgXmlUrl: "", secretConfigured: false, m3uConfigured: false, epgXmlConfigured: false, configured: false, origin: "empty" };
}
function normalizeAdminConnection(value) {
  const connection = Object.assign(defaultAdminConnection(), value || {});
  if (["direct_login", "api_key", "xtream", "m3u_xmltv"].indexOf(connection.sourceMode) < 0) connection.sourceMode = "direct_login";
  ["baseUrl", "username", "password", "apiKey", "channelProfile", "m3uUrl", "epgXmlUrl", "origin"].forEach(function(key) { connection[key] = String(connection[key] || ""); });
  connection.channelProfiles = uniqueIDs(items(connection.channelProfiles).map(function(value) { return String(value || "").trim(); }).filter(Boolean));
  connection.channelGroups = uniqueIDs(items(connection.channelGroups).map(function(value) { return String(value || "").trim(); }).filter(Boolean));
  if (!connection.channelProfiles.length && connection.channelProfile) connection.channelProfiles = [connection.channelProfile];
  connection.channelProfile = connection.channelProfiles.length === 1 ? connection.channelProfiles[0] : "";
  connection.secretConfigured = !!connection.secretConfigured;
  connection.m3uConfigured = !!connection.m3uConfigured;
  connection.epgXmlConfigured = !!connection.epgXmlConfigured;
  connection.configured = !!connection.configured;
  return connection;
}
function cloneAdminConnection(value) {
  return normalizeAdminConnection(JSON.parse(JSON.stringify(value || defaultAdminConnection())));
}
function updateAdminConnectionField(field, target) {
  const connection = normalizeAdminConnection(state.adminConnection);
  if (field === "sourceMode" && connection.sourceMode !== target.value) {
    connection.secretConfigured = false;
    connection.m3uConfigured = false;
    connection.epgXmlConfigured = false;
    connection.password = "";
    connection.apiKey = "";
    connection.m3uUrl = "";
    connection.epgXmlUrl = "";
  }
  connection[field] = target.value || "";
  state.adminConnection = connection;
  state.adminConnectionStatus = "dirty";
  state.adminConnectionMessage = "Unsaved source changes.";
  if (field === "sourceMode") {
    renderAdminPage();
    return;
  }
  const message = byId("admin-connection-message");
  if (message) {
    message.classList.remove("settings-warning", "hide");
    message.textContent = state.adminConnectionMessage;
  }
  renderAdminTopbarActions();
}
function adminLineupInventory(kind) {
  if (kind === "profiles") return availableChannelProfiles().map(function(profile) {
    return { id: String(profile.id || profile.name || ""), name: profile.name || profile.id || "Unnamed profile", count: Number(profile.channelCount || 0) };
  }).filter(function(item) { return !!item.id; });
  const counts = {};
  effectiveChannels(false).forEach(function(channel) {
    if (channel.categoryId) counts[channel.categoryId] = (counts[channel.categoryId] || 0) + 1;
  });
  return items(state.app && state.app.categories).map(function(group) {
    return { id: String(group.id || ""), name: group.name || group.id || "Unnamed group", count: Number(counts[group.id] || 0) };
  }).filter(function(item) { return !!item.id; });
}
function updateAdminLineupSelection(kind, id, checked) {
  const connection = normalizeAdminConnection(state.adminConnection);
  const field = kind === "profiles" ? "channelProfiles" : "channelGroups";
  const inventory = adminLineupInventory(kind);
  const allIDs = inventory.map(function(item) { return item.id; });
  let selected = connection[field].length ? connection[field].slice() : allIDs.slice();
  selected = selected.filter(function(value) { return value !== id; });
  if (checked) selected.push(id);
  selected = uniqueIDs(selected);
  if (!selected.length && allIDs.length) {
    showAppToast("Keep at least one " + (kind === "profiles" ? "profile" : "group") + " selected.");
    renderAdminPage();
    return;
  }
  connection[field] = selected.length === allIDs.length && allIDs.every(function(value) { return selected.indexOf(value) >= 0; }) ? [] : selected;
  connection.channelProfile = connection.channelProfiles.length === 1 ? connection.channelProfiles[0] : "";
  state.adminConnection = connection;
  state.adminConnectionStatus = "dirty";
  state.adminConnectionMessage = "Unsaved lineup changes.";
  renderAdminPage();
}
function useAllAdminLineup(kind) {
  const connection = normalizeAdminConnection(state.adminConnection);
  if (kind === "profiles") {
    connection.channelProfiles = [];
    connection.channelProfile = "";
  } else connection.channelGroups = [];
  state.adminConnection = connection;
  state.adminConnectionStatus = "dirty";
  state.adminConnectionMessage = "Unsaved lineup changes.";
  renderAdminPage();
}
function adminConnectionPayload(action) {
  const connection = normalizeAdminConnection(state.adminConnection);
  return {
    action: action || "save",
    sourceMode: connection.sourceMode,
    baseUrl: connection.baseUrl,
    username: connection.username,
    password: connection.password,
    apiKey: connection.apiKey,
    channelProfile: connection.channelProfile,
    channelProfiles: connection.channelProfiles,
    channelGroups: connection.channelGroups,
    m3uUrl: connection.m3uUrl,
    epgXmlUrl: connection.epgXmlUrl
  };
}
async function submitAdminConnection(action) {
  if (state.adminConnectionStatus === "saving" || state.adminConnectionStatus === "testing") return;
  state.adminConnectionStatus = action === "test" ? "testing" : "saving";
  state.adminConnectionMessage = action === "test" ? "Testing connection..." : "Saving source...";
  renderAdminPage();
  try {
    const result = await postJSON("/dispatcharr/api/admin-connection", adminConnectionPayload(action));
    const returned = result && result.connection ? normalizeAdminConnection(result.connection) : null;
    if (action === "test") {
      if (returned) {
        state.adminConnection.secretConfigured = returned.secretConfigured;
        state.adminConnection.m3uConfigured = returned.m3uConfigured;
        state.adminConnection.epgXmlConfigured = returned.epgXmlConfigured;
      }
      state.adminConnectionStatus = "tested";
      state.adminConnectionMessage = "Connection successful.";
    } else {
      state.adminConnection = returned || normalizeAdminConnection(state.adminConnection);
      state.adminConnection.password = "";
      state.adminConnection.apiKey = "";
      state.savedAdminConnection = cloneAdminConnection(state.adminConnection);
      if (state.app && state.app.source) state.app.source.mode = state.adminConnection.sourceMode;
      state.adminConnectionStatus = "saved";
      state.adminConnectionMessage = "Source saved. Catalog refresh queued.";
      state.adminConnectionEditorOpen = false;
      showAppToast("Dispatcharr connection saved.");
      refreshAdminStatus();
    }
  } catch (error) {
    state.adminConnectionStatus = "error";
    state.adminConnectionMessage = (action === "test" ? "Connection test failed: " : "Could not save source: ") + readableError(error);
  }
  renderAdminPage();
}
function discardAdminConnection() {
  state.adminConnection = cloneAdminConnection(state.savedAdminConnection);
  if (state.app && state.app.source) state.app.source.mode = state.adminConnection.sourceMode;
  state.adminConnectionEditorOpen = false;
  state.adminConnectionEditorStep = "connection";
  state.adminConnectionStatus = "idle";
  state.adminConnectionMessage = "";
  renderAdminPage();
}
function openAdminConnectionEditor(create) {
  state.adminConnection = cloneAdminConnection(create ? defaultAdminConnection() : state.savedAdminConnection);
  state.adminConnectionEditorOpen = true;
  state.adminConnectionEditorStep = "connection";
  state.adminConnectionStatus = "idle";
  state.adminConnectionMessage = "";
  renderAdminPage();
  requestAnimationFrame(function() {
    const close = document.querySelector(".source-dialog .source-close");
    if (close) close.focus({ preventScroll: true });
  });
}
function adminConnectionModeDefinitions() {
  return [
    ["direct_login", "Dispatcharr Direct", "Admin Username and Password"],
    ["api_key", "Dispatcharr API Key", "Admin API Key"],
    ["xtream", "Xtream Codes", "Xtream credentials from Dispatcharr"]
  ];
}
function adminConnectionModeOptions(connection) {
  return adminConnectionModeDefinitions().map(function(option) {
    return "<button type=\"button\" data-admin-source-mode=\"" + option[0] + "\" class=\"" + (connection.sourceMode === option[0] ? "active" : "") + "\"><strong>" + option[1] + "</strong><span>" + option[2] + "</span></button>";
  }).join("");
}
function adminConnectionSecretField(connection, key, label) {
  const configured = connection.secretConfigured && !connection[key];
  return "<label><span>" + label + (configured ? " <small>· leave blank to keep current</small>" : "") + "</span><input type=\"password\" autocomplete=\"new-password\" data-admin-connection-field=\"" + key + "\" value=\"" + escapeHTML(connection[key] || "") + "\" placeholder=\"" + (configured ? "Saved securely" : "Required") + "\"></label>";
}
function renderAdminConnectionFields(connection) {
  if (connection.sourceMode === "m3u_xmltv") {
    return "<label class=\"source-field-wide\"><span>M3U playlist URL" + (connection.m3uConfigured && !connection.m3uUrl ? " <small>· leave blank to keep current</small>" : "") + "</span><input type=\"url\" autocomplete=\"off\" data-admin-connection-field=\"m3uUrl\" value=\"" + escapeHTML(connection.m3uUrl) + "\" placeholder=\"" + (connection.m3uConfigured ? "Saved securely" : "https://provider.example.com/playlist.m3u") + "\"></label>"
      + "<label class=\"source-field-wide\"><span>XMLTV guide URL" + (connection.epgXmlConfigured && !connection.epgXmlUrl ? " <small>· leave blank to keep current</small>" : "") + "</span><input type=\"url\" autocomplete=\"off\" data-admin-connection-field=\"epgXmlUrl\" value=\"" + escapeHTML(connection.epgXmlUrl) + "\" placeholder=\"" + (connection.epgXmlConfigured ? "Saved securely" : "https://provider.example.com/guide.xml") + "\"></label>";
  }
  let fields = "<label class=\"source-field-wide\"><span>Server URL</span><input type=\"url\" data-admin-connection-field=\"baseUrl\" value=\"" + escapeHTML(connection.baseUrl) + "\" placeholder=\"https://dispatcharr.example.com\"></label>";
  if (connection.sourceMode !== "api_key") fields += "<label><span>Username</span><input data-admin-connection-field=\"username\" value=\"" + escapeHTML(connection.username) + "\" autocomplete=\"off\"></label>" + adminConnectionSecretField(connection, "password", "Password");
  if (connection.sourceMode === "api_key") fields += adminConnectionSecretField(connection, "apiKey", "Admin API key");
  return fields;
}
function renderAdminLineupFields(connection) {
  if (connection.sourceMode === "direct_login" || connection.sourceMode === "api_key") {
    return "<div class=\"admin-lineup-grid\">" + renderAdminLineupSelector("profiles", "Profiles", "Choose the Dispatcharr profiles available in Live TV.", connection.channelProfiles) + renderAdminLineupSelector("groups", "Channel groups", "Optionally narrow the lineup to specific groups.", connection.channelGroups) + "</div>";
  }
  const copy = connection.sourceMode === "xtream"
    ? "Channels and groups are imported from the Xtream account returned by Dispatcharr. Profile-based organization is not available in this mode."
    : "Channel groups come from the M3U playlist and program data comes from XMLTV. Profile-based organization is not available in this mode.";
  return "<div class=\"source-lineup-summary\">" + icon("guide") + "<div><strong>Lineup follows the configured source</strong><span>" + escapeHTML(copy) + "</span></div></div>";
}
function renderAdminLineupSelector(kind, title, description, selection) {
  const inventory = adminLineupInventory(kind);
  const selected = items(selection);
  const allSelected = selected.length === 0;
  const summary = allSelected ? "All " + kind : selected.length + " selected";
  const rows = inventory.map(function(item) {
    const checked = allSelected || selected.indexOf(item.id) >= 0;
    const detail = item.count ? String(item.count) + " channels" : "Available " + (kind === "profiles" ? "profile" : "group");
    return "<label class=\"admin-lineup-option\" data-admin-lineup-search-value=\"" + escapeHTML(lower(item.name + " " + item.id)) + "\"><span><strong>" + escapeHTML(item.name) + "</strong><small>" + escapeHTML(detail) + "</small></span><input type=\"checkbox\" data-admin-lineup-kind=\"" + kind + "\" data-admin-lineup-id=\"" + escapeHTML(item.id) + "\"" + (checked ? " checked" : "") + "></label>";
  }).join("");
  const body = inventory.length ? rows : "<div class=\"admin-lineup-empty\">Save the connection and refresh the catalog to load " + kind + ".</div>";
  return "<section class=\"admin-lineup-selector\"><header><div><h4>" + title + "</h4><p>" + description + "</p></div><button type=\"button\" data-admin-lineup-all=\"" + kind + "\">" + escapeHTML(summary) + "</button></header><label class=\"admin-lineup-search\">" + icon("search") + "<input data-admin-lineup-filter=\"" + kind + "\" placeholder=\"Filter " + kind + "\" aria-label=\"Filter " + kind + "\"></label><div class=\"admin-lineup-options\" data-admin-lineup-options=\"" + kind + "\">" + body + "</div></section>";
}
function adminLineupSummary(connection) {
  const profileCopy = connection.channelProfiles.length ? connection.channelProfiles.length + " profile" + (connection.channelProfiles.length === 1 ? "" : "s") : "All profiles";
  const groupCopy = connection.channelGroups.length ? connection.channelGroups.length + " group" + (connection.channelGroups.length === 1 ? "" : "s") : "all groups";
  return profileCopy + " · " + groupCopy;
}
function adminConnectionEndpoint(connection) {
  if (connection.sourceMode === "m3u_xmltv") return connection.m3uConfigured ? "Playlist and guide saved securely" : "Playlist URL required";
  return connection.baseUrl || "Server URL required";
}
function adminConnectionAccount(connection) {
  if (connection.sourceMode === "api_key") return "API key";
  if (connection.sourceMode === "m3u_xmltv") return "Managed URLs";
  return connection.username || "Account required";
}
function renderAdminConnectionEditor() {
  if (!state.adminConnectionEditorOpen) return "";
  const connection = normalizeAdminConnection(state.adminConnection);
  const warning = state.adminConnectionStatus === "error";
  const message = state.adminConnectionMessage ? "<div id=\"admin-connection-message\" class=\"settings-note" + (warning ? " settings-warning" : "") + "\" role=\"status\">" + escapeHTML(state.adminConnectionMessage) + "</div>" : "<div id=\"admin-connection-message\" class=\"settings-note hide\" role=\"status\"></div>";
  const step = state.adminConnectionEditorStep;
  let content = "";
  if (step === "connection") content = "<div class=\"source-step-copy\"><h3>Connection</h3><p>Choose a login type, then enter its connection details.</p></div><div class=\"source-mode-options\">" + adminConnectionModeOptions(connection) + "</div><div class=\"source-form source-connection-fields\">" + renderAdminConnectionFields(connection) + "</div>";
  else content = "<div class=\"source-step-copy\"><h3>Lineup</h3><p>Control which Dispatcharr channels become available in Live TV.</p></div>" + renderAdminLineupFields(connection);
  const busy = state.adminConnectionStatus === "saving" || state.adminConnectionStatus === "testing";
  return "<div class=\"source-dialog-backdrop\" data-admin-connection-backdrop=\"true\"><section class=\"source-dialog\" role=\"dialog\" aria-modal=\"true\" aria-labelledby=\"source-dialog-title\">"
    + "<header><span class=\"source-dialog-icon\">" + icon("guide") + "</span><div><h2 id=\"source-dialog-title\">" + (state.savedAdminConnection && state.savedAdminConnection.configured ? "Edit Dispatcharr" : "Add Dispatcharr") + "</h2><p>Connect this Silo installation to its Live TV source.</p></div><button type=\"button\" class=\"source-close\" data-admin-connection-action=\"cancel\" aria-label=\"Close connection setup\">" + icon("x") + "</button></header>"
    + "<div class=\"source-dialog-body\"><nav class=\"source-dialog-nav\" aria-label=\"Connection setup steps\">"
    + "<button type=\"button\" data-admin-connection-step=\"connection\" class=\"" + (step === "connection" ? "active" : "") + "\">" + icon("integrations") + "<span>Connection</span></button>"
    + "<button type=\"button\" data-admin-connection-step=\"lineup\" class=\"" + (step === "lineup" ? "active" : "") + "\">" + icon("guide") + "<span>Lineup</span></button></nav>"
    + "<div class=\"source-dialog-content\">" + message + content + "</div></div>"
    + "<footer><button type=\"button\" data-admin-connection-action=\"cancel\"" + (busy ? " disabled" : "") + ">Cancel</button><button type=\"button\" class=\"admin-outline-action\" data-admin-connection-action=\"test\"" + (busy ? " disabled" : "") + ">" + icon("loader") + "<span>" + (state.adminConnectionStatus === "testing" ? "Testing" : "Test Connection") + "</span></button><button type=\"button\" class=\"admin-save\" data-admin-connection-action=\"save\"" + (busy ? " disabled" : "") + ">" + (state.adminConnectionStatus === "saving" ? "Saving" : "Save Connection") + "</button></footer></section></div>";
}
function renderAdminConnectionTab() {
  const connection = normalizeAdminConnection(state.savedAdminConnection);
  const status = state.app && state.app.status ? state.app.status : {};
  const message = state.adminConnectionMessage && !state.adminConnectionEditorOpen ? "<div class=\"settings-note" + (state.adminConnectionStatus === "error" ? " settings-warning" : "") + "\" role=\"status\">" + escapeHTML(state.adminConnectionMessage) + "</div>" : "";
  const loadError = state.adminConnectionLoadError ? "<div class=\"settings-note settings-warning settings-note-action\" role=\"alert\"><span>Connection details could not be loaded: " + escapeHTML(state.adminConnectionLoadError) + "</span><button type=\"button\" data-admin-connection-action=\"retry-load\">Retry</button></div>" : "";
  let inventory = "<div class=\"source-empty\"><strong>No Dispatcharr connection</strong><span>Add a connection to import channels, guide data, and profiles into Silo.</span><button type=\"button\" class=\"source-add\" data-admin-connection-action=\"add\">Add Connection</button></div>";
  if (state.adminConnectionLoading && !connection.configured) inventory = "<div class=\"source-empty\" role=\"status\"><strong>Loading Dispatcharr connection</strong><span>Your other admin settings remain available while the connection is loaded.</span></div>";
  else if (state.adminConnectionLoadError && !connection.configured) inventory = "<div class=\"source-empty\"><strong>Connection status unavailable</strong><span>The saved connection was not changed. Retry when Silo or Dispatcharr is reachable.</span><button type=\"button\" class=\"source-add\" data-admin-connection-action=\"retry-load\">Retry</button></div>";
  if (connection.configured) {
    inventory = "<div class=\"source-table\" role=\"table\" aria-label=\"Dispatcharr connections\"><div class=\"source-table-head\" role=\"row\"><span>Connection</span><span>Account</span><span>Channels</span><span>Mode</span><span>Status</span><span>Actions</span></div>"
      + "<div class=\"source-table-row\" role=\"row\"><div class=\"source-primary\"><strong>Dispatcharr</strong><small>" + escapeHTML(adminConnectionEndpoint(connection)) + "</small></div>"
      + "<div class=\"source-user\"><span>" + escapeHTML(adminConnectionAccount(connection)) + "</span><small>" + escapeHTML(adminLineupSummary(connection)) + "</small></div>"
      + "<div class=\"source-count\"><strong>" + escapeHTML(String(status.channelCount || items(state.app && state.app.channels).length || 0)) + "</strong><small>channels</small></div>"
      + "<div class=\"source-format\"><span>" + escapeHTML(sourceModeLabel(connection.sourceMode)) + "</span><small>source</small></div>"
      + "<span class=\"source-status enabled\">Configured</span><div class=\"source-actions\"><button type=\"button\" data-admin-connection-action=\"test-saved\">" + icon("loader") + "<span>Test</span></button><button type=\"button\" data-admin-connection-action=\"edit\">" + icon("settings") + "<span>Edit</span></button></div></div></div>";
  }
  return adminStatusPanel() + loadError + message + "<div class=\"settings-card source-manager-card\"><div class=\"settings-card-head\"><div><h2>Connections</h2><p>Manage the source that powers Live TV, guide data, profiles, and playback.</p></div><span class=\"profile-selection-summary\">" + (connection.configured ? "1 connection" : (state.adminConnectionLoadError ? "Unavailable" : "Not configured")) + "</span></div>" + inventory + "</div>" + renderAdminConnectionEditor();
}
function renderAdminPage() {
  normalizeAdminCategorySettings();
  if (!adminECMEnabled() && state.adminTab === "manager") state.adminTab = "integrations";
  renderAdminTopbarTabs();
  renderAdminTopbarActions();
  const shell = document.querySelector(".shell");
  if (shell) shell.classList.toggle("is-admin-manager", state.adminTab === "manager");
  const content = byId("view");
  if (content) {
    content.setAttribute("role", "tabpanel");
    content.setAttribute("aria-labelledby", "admin-tab-" + state.adminTab);
    const stackClass = "settings-stack" + (state.adminTab === "organization" ? " organization-settings" : "");
    content.innerHTML = state.adminTab === "manager" ? renderExternalChannelManager() : "<div class=\"" + stackClass + "\">" + (state.adminTab === "source" ? renderAdminConnectionTab() : (state.adminTab === "integrations" ? renderAdminIntegrationsTab() : renderAdminSettingsTab(state.adminTab))) + "</div>";
  }
  if (state.adminTab === "general") {
    renderAdminIdentitySettings();
    renderAdminRecordingSettings();
  }
  if (state.adminTab === "playback") {
    renderAdminPlayerSettings();
    renderAdminTimeShiftSettings();
  }
  if (state.adminTab === "sports") {
    renderAdminSportsSettings();
  }
  if (state.adminTab === "organization") {
    renderAdminCategorySettings();
    renderAdminCategoryAliasSettings();
  }
  if (state.adminTab === "events") {
    renderAdminFeaturedEventSettings();
    renderAdminEventKeywordSettings();
  }
  if (state.adminTab === "integrations") {
    renderAdminECMSettings();
  }
}
function renderAdminTopbarTabs() {
  const root = byId("admin-tabs");
  if (!root) return;
  root.setAttribute("role", "tablist");
  root.setAttribute("aria-label", "Dispatcharr Admin sections");
  root.innerHTML = "<button id=\"admin-tab-source\" role=\"tab\" type=\"button\" data-admin-tab=\"source\" aria-selected=\"" + (state.adminTab === "source" ? "true" : "false") + "\" aria-controls=\"view\" class=\"" + (state.adminTab === "source" ? "active" : "") + "\">" + icon("guide") + "<span>Sources</span><small class=\"admin-tab-count\">" + (state.savedAdminConnection && state.savedAdminConnection.configured ? "1" : "0") + "</small></button>"
    + "<button id=\"admin-tab-general\" role=\"tab\" type=\"button\" data-admin-tab=\"general\" aria-selected=\"" + (state.adminTab === "general" ? "true" : "false") + "\" aria-controls=\"view\" class=\"" + (state.adminTab === "general" ? "active" : "") + "\">" + icon("settings") + "<span>General</span></button>"
    + "<button id=\"admin-tab-playback\" role=\"tab\" type=\"button\" data-admin-tab=\"playback\" aria-selected=\"" + (state.adminTab === "playback" ? "true" : "false") + "\" aria-controls=\"view\" class=\"" + (state.adminTab === "playback" ? "active" : "") + "\">" + icon("play") + "<span>Playback</span></button>"
    + "<button id=\"admin-tab-sports\" role=\"tab\" type=\"button\" data-admin-tab=\"sports\" aria-selected=\"" + (state.adminTab === "sports" ? "true" : "false") + "\" aria-controls=\"view\" class=\"" + (state.adminTab === "sports" ? "active" : "") + "\">" + icon("trophy") + "<span>Sports</span></button>"
    + "<button id=\"admin-tab-organization\" role=\"tab\" type=\"button\" data-admin-tab=\"organization\" aria-selected=\"" + (state.adminTab === "organization" ? "true" : "false") + "\" aria-controls=\"view\" class=\"" + (state.adminTab === "organization" ? "active" : "") + "\">" + icon("guide") + "<span>Profiles &amp; Groups</span></button>"
    + "<button id=\"admin-tab-events\" role=\"tab\" type=\"button\" data-admin-tab=\"events\" aria-selected=\"" + (state.adminTab === "events" ? "true" : "false") + "\" aria-controls=\"view\" class=\"" + (state.adminTab === "events" ? "active" : "") + "\">" + icon("clock") + "<span>Events</span></button>"
    + "<button id=\"admin-tab-integrations\" role=\"tab\" type=\"button\" data-admin-tab=\"integrations\" aria-selected=\"" + (state.adminTab === "integrations" ? "true" : "false") + "\" aria-controls=\"view\" class=\"" + (state.adminTab === "integrations" ? "active" : "") + "\">" + icon("integrations") + "<span>Integrations</span></button>"
    + (adminECMEnabled() ? "<button id=\"admin-tab-manager\" role=\"tab\" type=\"button\" data-admin-tab=\"manager\" aria-selected=\"" + (state.adminTab === "manager" ? "true" : "false") + "\" aria-controls=\"view\" class=\"" + (state.adminTab === "manager" ? "active" : "") + "\">" + icon("external") + "<span>Channel Manager</span></button>" : "");
}
function renderAdminTopbarActions() {
  const root = byId("admin-actions");
  if (!root) return;
  if (state.adminTab === "manager") {
    root.innerHTML = "";
    return;
  }
  if (state.adminTab === "source") {
    if (state.adminConnectionLoading || state.adminConnectionLoadError) {
      root.innerHTML = "<button class=\"admin-outline-action\" data-admin-connection-action=\"retry-load\"" + (state.adminConnectionLoading ? " disabled aria-busy=\"true\"" : "") + ">" + icon("loader") + "<span>" + (state.adminConnectionLoading ? "Loading Connection" : "Retry Connection") + "</span></button>";
      return;
    }
    const configured = !!(state.savedAdminConnection && state.savedAdminConnection.configured);
    root.innerHTML = "<button class=\"admin-outline-action\" data-admin-connection-action=\"refresh\"" + (state.adminStatusRefreshing ? " disabled aria-busy=\"true\"" : "") + ">" + icon("loader") + "<span>" + (state.adminStatusRefreshing ? "Refreshing" : "Refresh Catalog") + "</span></button>"
      + "<button class=\"admin-primary-action\" data-admin-connection-action=\"" + (configured ? "edit" : "add") + "\">" + icon(configured ? "settings" : "guide") + "<span>" + (configured ? "Edit Connection" : "Add Connection") + "</span></button>";
    return;
  }
  const dirty = state.adminTab === "organization" ? adminOrganizationDirty() : adminSettingsDirty();
  const saving = state.adminSaveStatus === "saving" || (state.adminTab === "organization" && state.adminConnectionStatus === "saving");
  const failed = state.adminSaveStatus === "error" || (state.adminTab === "organization" && state.adminConnectionStatus === "error");
  const failureMessage = state.adminSaveMessage || state.adminConnectionMessage;
  const status = saving ? "Saving..." : (failed ? (failureMessage || "Could not save changes.") : (dirty ? "Unsaved changes." : (state.adminSaveMessage || "All changes saved.")));
  root.setAttribute("aria-busy", saving ? "true" : "false");
  root.innerHTML = "<span class=\"admin-save-status\" role=\"status\" aria-live=\"polite\">" + escapeHTML(status) + "</span><button class=\"admin-save\" data-admin-settings-action=\"save\"" + ((!dirty || saving) ? " disabled" : "") + ">" + (saving ? "Saving" : "Save") + "</button><button class=\"admin-discard\" data-admin-settings-action=\"discard\"" + ((!dirty || saving) ? " disabled" : "") + ">Discard</button>";
}
function setAdminTab(tab, options) {
  options = options || {};
  if (tab === "manager" && adminECMEnabled()) state.adminTab = "manager";
  else if (tab === "integrations") state.adminTab = "integrations";
  else if (tab === "source") state.adminTab = "source";
  else if (tab === "playback" || tab === "sports" || tab === "organization" || tab === "events") state.adminTab = tab;
  else state.adminTab = "general";
  renderAdminPage();
  if (options.historyMode !== "none") commitAppRoute("push");
  if (state.adminTab === "organization") loadAdminSourceGroups(false);
  requestAnimationFrame(function() {
    const content = byId("view");
    if (!content) return;
    content.tabIndex = -1;
    content.focus({ preventScroll: true });
  });
}
function renderAdminSettingsTab(tab) {
  if (tab === "playback") {
    return "<div class=\"settings-card playback-streaming-card\"><div class=\"settings-card-head\"><div><h2>Streaming</h2><p>Balance playback stability against live-stream delay.</p></div></div><div id=\"admin-player-settings\"></div></div>"
      + "<div class=\"settings-card playback-rewind-card\"><div class=\"settings-card-head playback-rewind-head\"><div><h2>Live Rewind</h2><p>Pause and rewind supported channels with a bounded shared disk cache.</p></div><div id=\"admin-timeshift-toggle\"></div></div><div id=\"admin-timeshift-settings\"></div></div>";
  }
  if (tab === "organization") {
    return "<div id=\"admin-category-settings\" class=\"organization-settings-root\"></div>";
  }
  if (tab === "sports") {
    return "<div class=\"settings-card\"><div class=\"settings-card-head\"><div><h2>Sports</h2><p>Combine live Dispatcharr events with replays from selected Silo Sports libraries.</p></div></div><div id=\"admin-sports-settings\" class=\"settings-list\"></div></div>";
  }
  if (tab === "events") {
    return "<div class=\"settings-card\"><div class=\"settings-card-head\"><div><h2>Featured events</h2><p>Pin important guide events above automatic recommendations for everyone.</p></div></div><div id=\"admin-featured-event-settings\" class=\"settings-list admin-featured-event-list\"></div></div>"
      + "<div class=\"settings-card\"><div class=\"settings-card-head\"><div><h2>Event Keywords</h2><p>Events are detected from the Dispatcharr guide. One keyword per line or comma-separated.</p></div></div><div id=\"admin-event-keyword-settings\" class=\"settings-list event-keyword-list\"></div></div>";
  }
  return "<div class=\"settings-card settings-card-compact\"><h2>App identity</h2><div id=\"admin-identity-settings\" class=\"settings-list\"></div></div>"
    + "<div class=\"settings-card settings-card-compact\"><h2>Recordings</h2><div id=\"admin-recording-settings\" class=\"settings-list\"></div></div>"
    + (isAdminRoute && byId("admin-license-notices") ? byId("admin-license-notices").innerHTML : "");
}
function renderAdminIdentitySettings() {
  const root = byId("admin-identity-settings");
  if (!root) return;
  const settings = adminSettings();
  root.innerHTML = adminSaveStatusHTML()
    + "<label class=\"settings-row settings-form-row\"><span class=\"settings-field-copy\"><strong>App name</strong><small>Shown in the app header and browser title. The Silo sidebar uses the installed plugin label.</small></span><input type=\"text\" maxlength=\"80\" data-admin-identity-field=\"name\" value=\"" + escapeHTML(settings.appDisplayName) + "\"></label>"
    + "<label class=\"settings-row settings-form-row\"><span class=\"settings-field-copy\"><strong>User side menu</strong><small>Keep the TV Guide in the menu, or replace it with Channels and move the Home channel groups there.</small></span><select data-admin-identity-field=\"side-menu\"><option value=\"guide\"" + (settings.sideMenuMode === "guide" ? " selected" : "") + ">Guide</option><option value=\"channels\"" + (settings.sideMenuMode === "channels" ? " selected" : "") + ">Channels</option></select></label>";
}
function renderAdminRecordingSettings() {
  const root = byId("admin-recording-settings");
  if (!root) return;
  const settings = adminSettings();
  const available = !!(state.app && state.app.capabilities && state.app.capabilities.recordings && isDispatcharrDirectSource());
  const canSchedule = recordingSchedulingEnabled();
  const description = !available ? "Recordings require Dispatcharr Direct Connect." : (canSchedule ? "Show recording controls for Dispatcharr Direct users." : recordingScheduleReason());
  root.innerHTML = "<label class=\"settings-row compact-row\"><span><strong>Allow recordings by default</strong><small>" + escapeHTML(description) + "</small></span><input type=\"checkbox\" data-admin-recording-field=\"default\"" + (settings.allowRecordingsByDefault !== false ? " checked" : "") + (canSchedule ? "" : " disabled") + "></label>"
    + "<label class=\"settings-row compact-row\"><span><strong>Start padding</strong><small>Minutes to begin recording before the guide start time.</small></span><input type=\"number\" min=\"0\" max=\"60\" data-admin-recording-field=\"prepad\" value=\"" + Number(settings.recordingPrePadMinutes || 0) + "\"" + (canSchedule ? "" : " disabled") + "></label>"
    + "<label class=\"settings-row compact-row\"><span><strong>End padding</strong><small>Minutes to keep recording after the guide end time.</small></span><input type=\"number\" min=\"0\" max=\"60\" data-admin-recording-field=\"postpad\" value=\"" + Number(settings.recordingPostPadMinutes || 0) + "\"" + (canSchedule ? "" : " disabled") + "></label>"
    + "<label class=\"settings-row compact-row\"><span><strong>Comskip</strong><small>Ask Dispatcharr to mark commercials when the server supports it.</small></span><input type=\"checkbox\" data-admin-recording-field=\"comskip\"" + (settings.recordingComskipEnabled ? " checked" : "") + (canSchedule ? "" : " disabled") + "></label>";
}
function renderAdminPlayerSettings() {
  const root = byId("admin-player-settings");
  if (!root) return;
  const settings = adminSettings();
  root.innerHTML = "<label class=\"playback-setting\"><span class=\"settings-field-copy\"><strong>HLS buffer</strong><small>Browser memory used to absorb short connection drops.</small></span><span class=\"settings-unit-field\"><input type=\"number\" min=\"5\" max=\"60\" step=\"1\" value=\"" + escapeHTML(boundedHLSBufferSeconds(settings.hlsBufferSeconds)) + "\" data-admin-player-field=\"hls-buffer\"><em>sec</em></span></label>";
  const stations=document.createElement("div");stations.className="station-mappings";root.append(stations);
  loadPlayerLibrary("media.js","SiloLiveMedia").then(function(){if(stations.isConnected===false)return;SiloLiveMedia.stationEditor(stations,settings,items(state.app&&state.app.channels),function(rows){state.adminCategorySettings.stationMappings=rows;markAdminSettingsDraft();renderAdminTopbarActions();});}).catch(function(){stations.textContent="Station mapping controls could not be loaded. Reload to try again.";});

}
function renderAdminSportsSettings() {
  const root = byId("admin-sports-settings");
  if (!root) return;
  const settings = adminSettings();
  const selected = {};
  configuredSportsLibraryIDs().forEach(function(id) { selected[id] = true; });
  let libraryHTML = "";
  if (state.sportsLibrariesLoading) {
    libraryHTML = "<div class=\"settings-note\">Loading accessible Silo libraries...</div>";
  } else if (state.sportsLibrariesError) {
    libraryHTML = "<div class=\"settings-note settings-warning settings-note-action\"><span>Could not load Silo libraries: " + escapeHTML(state.sportsLibrariesError) + "</span><button type=\"button\" data-admin-sports-action=\"refresh-libraries\">Retry</button></div>";
  } else if (!items(state.sportsLibraries).length) {
    libraryHTML = "<div class=\"settings-note\">No accessible Silo libraries are available.</div>";
  } else {
    const visible = {};
    items(state.sportsLibraries).forEach(function(library) { visible[Number(library.id)] = true; });
    const unavailable = configuredSportsLibraryIDs().filter(function(id) { return !visible[id]; }).length;
    libraryHTML = (unavailable ? "<div class=\"settings-note settings-warning\">" + unavailable + " configured " + (unavailable === 1 ? "library is" : "libraries are") + " not accessible to this admin account and will remain configured.</div>" : "")
      + "<div class=\"profile-selection-list sports-library-list\">" + items(state.sportsLibraries).map(function(library) {
        const id = Number(library.id);
        return "<label class=\"profile-selection-row\"><span><strong>" + escapeHTML(library.name || ("Library " + id)) + "</strong><small>" + escapeHTML(library.type || "Silo library") + "</small></span><input type=\"checkbox\" data-admin-sports-field=\"library\" data-admin-sports-library-id=\"" + id + "\"" + (selected[id] ? " checked" : "") + (settings.sportsEnabled ? "" : " disabled") + "></label>";
      }).join("") + "</div>";
  }
  root.innerHTML = adminSaveStatusHTML()
    + "<label class=\"settings-row compact-row\"><span><strong>Enable Sports</strong><small>Show live scores and channels. Replays appear only when they match items in the selected libraries.</small></span><input type=\"checkbox\" data-admin-sports-field=\"enabled\"" + (settings.sportsEnabled ? " checked" : "") + "></label>"
    + "<label class=\"settings-row compact-row\"><span><strong>Separate sports Silo app</strong><small>Show Sports as its own Silo Apps entry and hide it from Live TV. Turn this off to keep Sports inside Live TV.</small></span><input type=\"checkbox\" data-admin-sports-field=\"separate\"" + (settings.separateSportsApp ? " checked" : "") + (settings.sportsEnabled ? "" : " disabled") + "></label>"
    + "<label class=\"settings-row compact-row\"><span><strong>Sports-first player</strong><small>Use the score and matched-channel player when a channel is launched from Sports.</small></span><input type=\"checkbox\" data-admin-player-field=\"sports\"" + (settings.sportsFirstPlayerEnabled ? " checked" : "") + (settings.sportsEnabled ? "" : " disabled") + "></label>"
    + "<div class=\"settings-card-head sports-library-head\"><div><h3>Replay libraries</h3><p>Select only dedicated Sports libraries. Each user still sees only libraries their Silo profile can access.</p></div><span class=\"profile-selection-summary\">" + configuredSportsLibraryIDs().length + " selected</span></div>"
    + libraryHTML
    + renderAdminSportsTeamChannels(settings)
    + renderAdminSportsGuide(settings);
  if (!state.sportsLibraries && !state.sportsLibrariesLoading) loadSportsLibraries(false);
  if (!state.sports && !state.sportsLoading && settings.sportsEnabled) loadSports(false);
}
function adminSportsTeamOptions() {
  const teams = sportsHubTeams(state.sports || {});
  return Object.keys(teams).map(function(key) { return teams[key]; }).sort(function(left, right) {
    return String(left.leagueName).localeCompare(String(right.leagueName)) || sportsTeamName(left.team).localeCompare(sportsTeamName(right.team));
  });
}
function ensureAdminChannelsLoaded() {
  if (!isAdminRoute || state.adminChannelsRequested) return;
  state.adminChannelsRequested = true;
  getJSON("/dispatcharr/api/channels").then(function(payload) {
    if (state.app) state.app.channels = items(payload && payload.channels);
  }).catch(function() {}).then(function() {
    if (state.view === "admin" && state.adminTab === "sports") renderAdminSportsSettings();
  });
}
function renderAdminSportsTeamChannels(settings) {
  ensureAdminChannelsLoaded();
  const pins = items(settings.sportsTeamChannels);
  const teams = adminSportsTeamOptions();
  const channels = items(state.app && state.app.channels).slice().sort(function(left, right) { return String(left.name || "").localeCompare(String(right.name || "")); });
  const leagueName = function(id) {
    const league = sportsLeagueByID(state.sports, id);
    return (league && league.name) || id;
  };
  const list = pins.length ? "<div class=\"profile-selection-list sports-team-channel-list\">" + pins.map(function(pin, index) {
    const channel = channelByID(pin.channelId);
    return "<div class=\"profile-selection-row\"><span><strong>" + escapeHTML(pin.teamName) + "</strong><small>" + escapeHTML(leagueName(pin.leagueId) + " → " + ((channel && channel.name) || pin.channelName || pin.channelId) + (channel ? "" : " (channel not found)")) + "</small></span><button type=\"button\" data-admin-sports-action=\"remove-team-channel\" data-admin-team-channel-index=\"" + index + "\" aria-label=\"Remove channel for " + escapeHTML(pin.teamName) + "\">" + icon("x") + "</button></div>";
  }).join("") + "</div>" : "<div class=\"settings-note\">No team channels yet. Pin a team to its regional or team network so its games always get a Watch button.</div>";
  const teamOptions = "<option value=\"\">Choose a team</option>" + teams.map(function(entry) {
    return "<option value=\"" + escapeHTML(entry.leagueID + "|" + sportsTeamName(entry.team)) + "\">" + escapeHTML(sportsTeamName(entry.team) + " · " + entry.leagueName) + "</option>";
  }).join("");
  const channelOptions = "<option value=\"\">Choose a channel</option>" + channels.map(function(channel) {
    return "<option value=\"" + escapeHTML(channel.id) + "\">" + escapeHTML(channel.name || channel.id) + "</option>";
  }).join("");
  return "<div class=\"settings-card-head sports-library-head\"><div><h3>Team channels</h3><p>Pin a team to one channel, such as a regional sports network. Its live and upcoming games list that channel first.</p></div><span class=\"profile-selection-summary\">" + pins.length + " pinned</span></div>"
    + list
    + "<div class=\"sports-team-channel-form\"><label><span>Team</span><select id=\"admin-team-channel-team\"" + (teams.length ? "" : " disabled") + ">" + teamOptions + "</select></label><label><span>Channel</span><select id=\"admin-team-channel-channel\">" + channelOptions + "</select></label><button type=\"button\" data-admin-sports-action=\"add-team-channel\">Pin channel</button></div>"
    + (teams.length ? "" : "<div class=\"settings-note\">Teams appear here once Sports data has loaded.</div>");
}
function updateAdminSportsTeamChannels(action, button) {
  const settings = state.adminCategorySettings || defaultAdminCategorySettings();
  const pins = items(settings.sportsTeamChannels).slice();
  if (action === "remove-team-channel") {
    pins.splice(Number(button.getAttribute("data-admin-team-channel-index")), 1);
  } else {
    const team = String((byId("admin-team-channel-team") || {}).value || "");
    const channelID = String((byId("admin-team-channel-channel") || {}).value || "");
    const split = team.indexOf("|");
    if (split < 1 || !channelID) {
      showAppToast("Choose a team and a channel.");
      return;
    }
    const leagueId = team.slice(0, split);
    const teamName = team.slice(split + 1);
    const channel = channelByID(channelID);
    const existing = pins.findIndex(function(pin) { return pin.leagueId === leagueId && lower(pin.teamName) === lower(teamName); });
    const pin = { leagueId: leagueId, teamName: teamName, channelId: channelID, channelName: (channel && channel.name) || "" };
    if (existing >= 0) pins[existing] = pin;
    else pins.push(pin);
  }
  settings.sportsTeamChannels = pins;
  state.adminCategorySettings = settings;
  normalizeAdminCategorySettings();
  markAdminSettingsDraft();
  renderAdminPage();
}
function byteSizeLabel(value) {
  const bytes = Number(value || 0);
  if (bytes >= 1073741824) return (bytes / 1073741824).toFixed(bytes >= 10737418240 ? 0 : 1) + " GB";
  if (bytes >= 1048576) return (bytes / 1048576).toFixed(1) + " MB";
  return Math.max(0, Math.round(bytes / 1024)) + " KB";
}
function renderAdminTimeShiftSettings() {
  const root = byId("admin-timeshift-settings");
  const toggleRoot = byId("admin-timeshift-toggle");
  if (!root) return;
  const settings = adminSettings();
  const status = state.timeShiftAdminStatus || {};
  const direct = isDispatcharrDirectSource();
  const enabled = direct && settings.liveRewindEnabled;
  const disabled = enabled ? "" : " disabled";
  const usage = state.timeShiftAdminLoading ? "Loading cache usage..." : (status.unavailable ? "Cache status is unavailable." : (state.timeShiftAdminStatus ? byteSizeLabel(status.bytes) + " of " + byteSizeLabel(status.maxBytes) + " · " + String(status.activeBuffers || 0) + " buffered channels · " + String(status.activeLeases || 0) + " viewers" : "Usage has not been checked yet."));
  const windowOptions = [
    { value: 15, label: "15 min" },
    { value: 30, label: "30 min" },
    { value: 60, label: "1 hr" },
    { value: 90, label: "90 min" },
    { value: 120, label: "2 hr" }
  ].map(function(option) {
    const selected = Number(settings.liveRewindWindowMinutes) === option.value;
    return "<button type=\"button\" role=\"radio\" aria-checked=\"" + (selected ? "true" : "false") + "\" class=\"" + (selected ? "active" : "") + "\" data-admin-rewind-window=\"" + option.value + "\"" + disabled + ">" + option.label + "</button>";
  }).join("");
  if (toggleRoot) {
    toggleRoot.innerHTML = "<label class=\"playback-master-toggle\"><span>" + (settings.liveRewindEnabled ? "On" : "Off") + "</span><input type=\"checkbox\" data-admin-timeshift-field=\"enabled\"" + (settings.liveRewindEnabled ? " checked" : "") + (direct ? "" : " disabled") + "></label>";
  }
  root.innerHTML = adminSaveStatusHTML()
    + (!direct ? "<div class=\"settings-note settings-warning\">Live Rewind requires Dispatcharr Direct MPEG-TS playback.</div>" : "")
    + "<div class=\"timeshift-controls" + (enabled ? "" : " is-disabled") + "\">"
    + "<div class=\"timeshift-window\"><span class=\"settings-field-copy\"><strong>Rewind window</strong><small>Older segments are removed first.</small></span><div class=\"settings-segmented\" role=\"radiogroup\" aria-label=\"Maximum rewind window\">" + windowOptions + "</div></div>"
    + "<div class=\"timeshift-metrics\">"
    + "<label class=\"timeshift-metric\"><span><strong>Storage budget</strong><small>Shared cache</small></span><span class=\"settings-number-unit\"><input type=\"number\" min=\"1\" max=\"500\" step=\"1\" data-admin-timeshift-field=\"cache\" value=\"" + escapeHTML(String(settings.liveRewindCacheGB)) + "\"" + disabled + "><em>GB</em></span></label>"
    + "<label class=\"timeshift-metric\"><span><strong>Keep free</strong><small>Disk reserve</small></span><span class=\"settings-number-unit\"><input type=\"number\" min=\"1\" max=\"100\" step=\"1\" data-admin-timeshift-field=\"free\" value=\"" + escapeHTML(String(settings.liveRewindMinFreeGB)) + "\"" + disabled + "><em>GB</em></span></label>"
    + "<label class=\"timeshift-metric\"><span><strong>Channel limit</strong><small>Concurrent buffers</small></span><input type=\"number\" min=\"1\" max=\"100\" step=\"1\" data-admin-timeshift-field=\"channels\" value=\"" + escapeHTML(String(settings.liveRewindMaxChannels)) + "\"" + disabled + "></label>"
    + "</div>"
    + "<div class=\"timeshift-usage-row\"><span class=\"settings-field-copy\"><strong>Cache usage</strong><small>" + escapeHTML(usage) + "</small></span><div class=\"settings-inline-actions\"><button type=\"button\" data-timeshift-admin-action=\"refresh\">Refresh</button><button type=\"button\" data-timeshift-admin-action=\"clear\" class=\"danger\">Clear cache</button></div></div>"
    + "</div>";
  if (!state.timeShiftAdminStatus && !state.timeShiftAdminLoading) refreshAdminTimeShiftStatus(true);
}
async function refreshAdminTimeShiftStatus(quiet) {
  if (state.timeShiftAdminLoading) return;
  state.timeShiftAdminLoading = true;
  if (!quiet) renderAdminPage();
  try {
    state.timeShiftAdminStatus = await getJSON("/dispatcharr/api/media/admin-status");
  } catch (_) {
    state.timeShiftAdminStatus = { unavailable: true };
    if (!quiet) showAppToast("Live Rewind cache status is unavailable.");
  } finally {
    state.timeShiftAdminLoading = false;
    if (state.view === "admin" && state.adminTab === "playback") renderAdminPage();
  }
}
async function clearAdminTimeShiftCache() {
  try {
    await postJSON("/dispatcharr/api/media/clear", {});
    state.timeShiftAdminStatus = null;
    showAppToast("Live Rewind cache cleared.");
    refreshAdminTimeShiftStatus(true);
  } catch (_) {
    showAppToast("Live Rewind cache could not be cleared.");
  }
}
function renderAdminIntegrationsTab() {
  return ""
    + "<div class=\"settings-card integrations-card\"><div class=\"settings-card-head\"><div><h2>ECM</h2><p>Embed Enhanced Channel Manager for Dispatcharr channel work.</p></div></div><div id=\"admin-ecm-settings\" class=\"settings-list\"></div></div>"
    + "";
}
function adminStatusPill(status) {
  status = String(status || "ok").toLowerCase();
  const label = status === "failed" ? "Error" : (status === "loading" ? "Updating" : (status === "error" ? "Error" : (status === "unavailable" ? "Unavailable" : (status === "all_access" ? "All access" : (status === "empty" ? "Not assigned" : (status === "available" ? "Available" : "Healthy"))))));
  const cls = status === "loading" ? " loading" : ((status === "error" || status === "failed" || status === "unavailable") ? " error" : ((status === "empty" || status === "all_access") ? " warning" : ""));
  return "<span class=\"admin-status-pill" + cls + "\">" + escapeHTML(label) + "</span>";
}
function plainTextFromHTML(value) {
  return String(value || "").replace(/<[^>]*>/g, "").replace(/\s+/g, " ").trim();
}
function adminStatusItem(label, value, detail) {
  return "<div class=\"admin-status-item\" title=\"" + escapeHTML((label ? label + ": " : "") + (detail || plainTextFromHTML(value))) + "\"><span>" + escapeHTML(label) + "</span><strong>" + value + "</strong>" + (detail ? "<small>" + escapeHTML(detail) + "</small>" : "") + "</div>";
}
function adminStatusPanel() {
  const status = state.app && state.app.status ? state.app.status : {};
  const source = state.app && state.app.source ? state.app.source : {};
  const profileAccess = source.profileAccess || {};
  const guideStatus = String(status.epgStatus || (status.epgProgramCount ? "ok" : "unknown"));
  const error = status.lastError || status.epgLastError || "";
  const profileStatus = String(profileAccess.status || (items(source.profiles).length ? "available" : "empty"));
  const profileCount = Number(profileAccess.profileCount || items(source.profiles).length || 0);
  const membershipCount = Number(profileAccess.channelMembershipCount || 0);
  const profileValue = profileStatus === "available" ? escapeHTML(String(profileCount)) : adminStatusPill(profileStatus);
  const profileDetail = profileStatus === "available" ? String(membershipCount) + " channel memberships" : (profileAccess.message || "No profile information returned by Dispatcharr.");
  const refreshClass = "admin-status-refresh" + (state.adminStatusRefreshing ? " is-loading" : "");
  const refreshLabel = state.adminStatusRefreshing ? "Refreshing..." : "Refresh";
  return "<div class=\"admin-status-strip\" aria-label=\"Connection Status\"><div class=\"settings-card-head\"><div><h2>Connection Status</h2></div><button type=\"button\" class=\"" + refreshClass + "\" data-admin-status-refresh=\"true\" aria-label=\"Refresh connection status\"" + (state.adminStatusRefreshing ? " disabled aria-busy=\"true\"" : "") + ">" + icon("loader") + "<span>" + refreshLabel + "</span></button></div><div class=\"admin-status-grid\">"
    + adminStatusItem("Connection", adminStatusPill(status.status || "ok"), sourceModeLabel(source.mode))
    + adminStatusItem("Channels", escapeHTML(String(status.channelCount || items(state.app.channels).length || 0)), "Last catalog sync: " + dateTimeLabel(status.lastSuccessUnix))
    + adminStatusItem("Guide", adminStatusPill(guideStatus), String(status.epgProgramCount || items(state.app.programs).length || 0) + " programs · " + dateTimeLabel(status.epgLastSuccessUnix))
    + (isDispatcharrDirectSource() ? adminStatusItem("Profiles", profileValue, profileDetail) : "")
    + "</div>"
    + (error ? "<div class=\"settings-note settings-warning admin-status-note\">" + escapeHTML(error) + "</div>" : "")
    + (isDispatcharrDirectSource() && profileStatus !== "available" ? "<div class=\"settings-note settings-warning admin-status-note is-actionable\"><span>" + escapeHTML(profileDetail) + "</span><button type=\"button\" data-admin-profile-refresh=\"true\"" + (state.adminProfileRefreshing ? " disabled" : "") + ">" + (state.adminProfileRefreshing ? "Refreshing..." : "Retry profiles") + "</button></div>" : "")
    + "</div>";
}

async function refreshAdminStatus() {
  if (state.adminStatusRefreshing) return;
  state.adminStatusRefreshing = true;
  renderAdminPage();
  try {
    await refreshStatusData();
    showAppToast("Connection status refreshed.");
  } catch (error) {
    showAppToast("Could not refresh connection status.");
    try { console.warn("Dispatcharr admin status refresh failed", error); } catch (_) {}
  } finally {
    state.adminStatusRefreshing = false;
    renderAdminPage();
  }
}

async function refreshAdminCatalog() {
  if (state.adminStatusRefreshing) return;
  state.adminStatusRefreshing = true;
  state.adminConnectionStatus = "refreshing";
  state.adminConnectionMessage = "Catalog refresh started.";
  renderAdminPage();
  try {
    await hydrateApp(await postJSON("/dispatcharr/api/refresh-channels", {}), { reuseSettings: true });
    await refreshStatusData();
    state.adminConnectionStatus = "saved";
    state.adminConnectionMessage = "Catalog refresh queued. Status will update as channels are imported.";
    showAppToast("Dispatcharr catalog refresh started.");
  } catch (error) {
    state.adminConnectionStatus = "error";
    state.adminConnectionMessage = "Could not refresh catalog: " + readableError(error);
    showAppToast("Could not refresh the Dispatcharr catalog.");
  } finally {
    state.adminStatusRefreshing = false;
    renderAdminPage();
  }
}

async function refreshAdminProfiles() {
  if (state.adminProfileRefreshing) return;
  state.adminProfileRefreshing = true;
  renderAdminPage();
  try {
    await hydrateApp(await postJSON("/dispatcharr/api/refresh-channels", {}), { reuseSettings: true });
    for (let attempt = 0; attempt < 150; attempt++) {
      const profileAccess = state.app && state.app.source ? state.app.source.profileAccess || {} : {};
      const refresh = state.app && state.app.status ? state.app.status.refresh || {} : {};
      const refreshState = String(refresh.state || "").toLowerCase();
      if (profileAccess.status === "available") {
        showAppToast("Channel profiles refreshed.");
        return;
      }
      if (refreshState === "failed" || refreshState === "canceled") throw new Error(refresh.error || "profile refresh did not complete");
      await new Promise(function(resolve) { setTimeout(resolve, 2000); });
      await refreshStatusData();
    }
    throw new Error("profile refresh timed out");
  } catch (error) {
    showAppToast("Dispatcharr profile refresh failed.");
  } finally {
    state.adminProfileRefreshing = false;
    renderAdminPage();
  }
}
function renderExternalChannelManager() {
  const managerURL = adminECMURL();
  return "<div class=\"external-manager-surface\"><iframe class=\"external-manager-frame\" src=\"" + escapeHTML(managerURL) + "\" title=\"Channel Manager\"></iframe></div>";
}
function renderAdminECMSettings() {
  const settings = adminSettings();
  const root = byId("admin-ecm-settings");
  if (!root) return;
  root.innerHTML = adminSaveStatusHTML() + "<div class=\"settings-row ecm-url-row compact-row\"><span><strong>ECM URL</strong><small>Leave blank to hide Channel Manager.</small></span><input type=\"url\" data-admin-ecm-field=\"url\" value=\"" + escapeHTML(settings.ecmURL || "") + "\"></div>";
}
function renderAdminCategorySettings() {
  const settings = adminSettings();
  const root = byId("admin-category-settings");
  const profileAccess = state.app && state.app.source && state.app.source.profileAccess ? state.app.source.profileAccess : {};
  const layers = organizationLayerState();
  const activeTab = state.adminOrganizationTab === "groups" || state.adminOrganizationTab === "rules" ? state.adminOrganizationTab : "profiles";
  const kind = activeTab === "groups" ? "groups" : "profiles";
  const profileCount = adminLineupInventory("profiles").length;
  const groupCount = adminLineupInventory("groups").length;
  const tabs = "<div class=\"organization-tabs\" role=\"tablist\" aria-label=\"Dispatcharr organization settings\"><button type=\"button\" role=\"tab\" data-admin-organization-tab=\"profiles\" aria-selected=\"" + (activeTab === "profiles" ? "true" : "false") + "\" class=\"" + (activeTab === "profiles" ? "active" : "") + "\"><span>Profiles</span><small>" + profileCount + "</small></button><button type=\"button\" role=\"tab\" data-admin-organization-tab=\"groups\" aria-selected=\"" + (activeTab === "groups" ? "true" : "false") + "\" class=\"" + (activeTab === "groups" ? "active" : "") + "\"><span>Groups</span><small>" + groupCount + "</small></button><button type=\"button\" role=\"tab\" data-admin-organization-tab=\"rules\" aria-selected=\"" + (activeTab === "rules" ? "true" : "false") + "\" class=\"" + (activeTab === "rules" ? "active" : "") + "\"><span>Rules</span></button></div>";
  const heading = "<div class=\"organization-browser-heading\"><div><h2>Profiles &amp; Groups</h2><p>Choose what appears in Live TV, then control how those names become folders.</p></div></div>";
  const nested = settings.mode !== "normal" ? "<div class=\"organization-parsing-grid\">"
    + "<label class=\"organization-field\"><span><strong>Delimiter</strong><small>Split names into nested folders.</small></span><select data-admin-category-field=\"delimiter\"><option value=\"pipe\"" + (settings.delimiter === "pipe" ? " selected" : "") + ">Pipe · Sports | NHL Teams</option><option value=\"dash\"" + (settings.delimiter === "dash" ? " selected" : "") + ">Dash · Sports - NHL Teams</option></select></label>"
    + "<label class=\"organization-field\"><span><strong>Library label</strong><small>Name the generated browse collection.</small></span><div class=\"virtual-label-control\"><span>Virtual</span><input data-admin-category-field=\"virtualGroupLabel\" value=\"" + escapeHTML(virtualGroupLabelSuffix(settings.virtualGroupLabel)) + "\" placeholder=\"Groups\"></div></label>"
    + "</div>" : "";
  if (activeTab === "rules") {
    root.innerHTML = adminSaveStatusHTML() + "<section class=\"organization-browser organization-rules-view\">" + heading + "<div class=\"organization-control-row is-rules\">" + tabs + "</div>"
      + "<section class=\"settings-card organization-rules-card\"><div class=\"settings-card-head\"><div><h2>Folder rules</h2><p>Control how enabled profile and group names become nested browse paths.</p></div></div>"
      + "<div class=\"organization-section\"><div class=\"organization-section-heading\"><div><strong>Hierarchy layers</strong><span>Choose whether profiles and groups contribute folders to every channel path.</span></div><span class=\"organization-layer-count\">" + (Number(layers.profiles) + Number(layers.groups)) + " active</span></div><div class=\"organization-layer-grid\">"
      + "<label class=\"organization-layer-toggle\"><span class=\"organization-layer-icon\">" + icon("guide") + "</span><span><strong>Channel Profiles</strong><small>Use enabled profiles as top-level libraries.</small></span><input type=\"checkbox\" data-admin-organization-layer=\"profiles\"" + (layers.profiles ? " checked" : "") + "></label>"
      + "<label class=\"organization-layer-toggle\"><span class=\"organization-layer-icon\">" + icon("settings") + "</span><span><strong>Channel Groups</strong><small>Use enabled groups as folders within a profile or as the top-level hierarchy.</small></span><input type=\"checkbox\" data-admin-organization-layer=\"groups\"" + (layers.groups ? " checked" : "") + "></label></div></div>"
      + "<div class=\"organization-section organization-parsing\"><div class=\"organization-parsing-grid\"><label class=\"organization-field\"><span><strong>Folder mode</strong><small>Keep source names whole or split them into paths.</small></span><select data-admin-category-field=\"mode\"><option value=\"normal\"" + (settings.mode === "normal" ? " selected" : "") + ">Keep names as provided</option><option value=\"delimiter\"" + (settings.mode === "delimiter" ? " selected" : "") + ">Split into nested folders</option></select></label></div>"
      + nested
      + "<div class=\"organization-options\"><label><span><strong>Channel-name folders</strong><small>Use channel names as a fallback hierarchy. Required when Profiles and Groups are both off.</small></span><input type=\"checkbox\" data-admin-organization-layer=\"channels\"" + (layers.channels ? " checked" : "") + ((!layers.profiles && !layers.groups) || layers.profiles ? " disabled" : "") + "></label><label><span><strong>Collapse duplicate names</strong><small>Remove repeated labels where profile, group, and channel paths overlap.</small></span><input type=\"checkbox\" data-admin-category-field=\"collapseDuplicateVirtualGroups\"" + (settings.collapseDuplicateVirtualGroups !== false ? " checked" : "") + "></label><label><span><strong>Flatten redundant wrapper folders</strong><small>Skip a shared intermediate group when every group under a profile starts with the same folder.</small></span><input type=\"checkbox\" data-admin-category-field=\"flattenRedundantGroupWrappers\"" + (settings.flattenRedundantGroupWrappers !== false ? " checked" : "") + "></label></div></div>"
      + renderOrganizationPreview(settings)
      + (useChannelProfileVirtualPaths() && profileAccess.status !== "available" ? "<div class=\"settings-note settings-warning\">" + escapeHTML(profileAccess.message || "No Channel Profiles are available to the configured Dispatcharr account. Assign profiles in Dispatcharr, then refresh Live TV.") + "</div>" : "")
      + (settings.mode === "normal" ? "<div class=\"settings-note\">Channel groups are shown as provided, without remapping or resorting.</div>" : "") + "</section>"
      + "<section class=\"settings-card organization-overrides-card\"><div class=\"settings-card-head\"><div><h2>Presentation overrides</h2><p>Publish a source group at an additional browse path without changing Dispatcharr.</p></div><span class=\"profile-selection-summary\">" + categoryAliases().length + " configured</span></div><div id=\"admin-category-alias-settings\" class=\"settings-list\"></div></section></section>";
    return;
  }
  const inventory = adminLineupInventory(kind);
  const connection = normalizeAdminConnection(state.adminConnection || state.savedAdminConnection);
  const field = kind === "profiles" ? "channelProfiles" : "channelGroups";
  const selected = connection[field].length ? connection[field] : inventory.map(function(item) { return item.id; });
  const selectedMap = {};
  selected.forEach(function(id) { selectedMap[id] = true; });
  const query = lower(state.adminOrganizationQuery || "").trim();
  let matchingCount = 0;
  const cards = inventory.map(function(item) {
    const enabled = !!selectedMap[item.id];
    const detail = item.count + " channel" + (item.count === 1 ? "" : "s");
    const matches = !query || lower(item.name + " " + item.id).indexOf(query) >= 0;
    if (matches) matchingCount++;
    return "<label class=\"organization-item-card" + (enabled ? " is-enabled" : "") + "\" data-admin-organization-search=\"" + escapeHTML(lower(item.name + " " + item.id)) + "\"" + (matches ? "" : " hidden") + "><span class=\"organization-item-icon\">" + icon(kind === "profiles" ? "guide" : "settings") + "</span><span class=\"organization-item-copy\"><strong>" + escapeHTML(item.name) + "</strong><small>" + escapeHTML(detail) + "</small></span><span class=\"organization-item-status\">" + (enabled ? "Enabled" : "Excluded") + "</span><input type=\"checkbox\" data-admin-lineup-kind=\"" + kind + "\" data-admin-lineup-id=\"" + escapeHTML(item.id) + "\"" + (enabled ? " checked" : "") + " aria-label=\"Include " + escapeHTML(item.name) + "\"></label>";
  }).join("");
  const empty = state.adminSourceGroupsLoading ? "Loading " + kind + "..." : (state.adminSourceGroupsError ? "Could not load " + kind + ". Refresh the catalog and try again." : "No " + kind + " are available for this connection.");
  const activeCount = selected.filter(function(id) { return inventory.some(function(item) { return item.id === id; }); }).length;
  const summary = activeCount === inventory.length && inventory.length ? "All " + inventory.length + " enabled" : activeCount + " of " + inventory.length + " enabled";
  root.innerHTML = adminSaveStatusHTML()
    + "<section class=\"organization-browser\"><div class=\"organization-browser-heading\"><div><h2>Profiles &amp; Groups</h2><p>Choose what appears in Live TV, then control how those names become folders.</p></div><span class=\"profile-selection-summary\">" + escapeHTML(summary) + "</span></div>"
    + "<div class=\"organization-control-row\">" + tabs + "<label class=\"organization-search\">" + icon("search") + "<input data-admin-organization-filter placeholder=\"Filter " + kind + "\" value=\"" + escapeHTML(state.adminOrganizationQuery || "") + "\" aria-label=\"Filter " + kind + "\"></label><details class=\"organization-bulk-menu\"><summary title=\"" + (kind === "profiles" ? "Profile" : "Group") + " actions\" aria-label=\"" + (kind === "profiles" ? "Profile" : "Group") + " actions\">" + icon("ellipsis") + "</summary><div><button type=\"button\" data-admin-lineup-all=\"" + kind + "\">" + icon("check") + "<span>Enable all " + kind + "</span></button></div></details></div>"
    + "<div class=\"organization-item-grid\">" + cards + "<div class=\"organization-empty organization-filter-empty\"" + (inventory.length && matchingCount ? " hidden" : "") + ">" + escapeHTML(inventory.length ? "No " + kind + " match this filter." : empty) + "</div></div></section>";
}
function organizationPreviewPath(settings) {
  const channel = effectiveChannels(false)[0] || {};
  const separator = settings.delimiter === "dash" ? " - " : " | ";
  const split = function(value, fallback) {
    const parts = String(value || fallback || "").split(separator).map(function(part) { return part.trim(); }).filter(Boolean);
    return parts.length ? parts : [fallback || "Unassigned"];
  };
  const profile = profilePathsForChannel(channel)[0] || "Profile";
  const group = sourceCategoryOriginalLabel(channel) || sourceCategoryLabel(channel) || "Group";
  const channelPath = channel.name || "Channel";
  const source = normalizeVirtualGroupSource(settings.virtualGroupSource, settings.inferChannelNameGroups === true);
  const stages = [];
  if (source === "profile" || source === "profile_group") stages.push({ label: "Profile path", value: split(profile, "Profile").join(" / ") });
  if (source === "group" || source === "group_channel" || source === "profile_group") stages.push({ label: "Group path", value: split(group, "Group").join(" / ") });
  if (source === "channel" || source === "group_channel") stages.push({ label: "Channel path", value: split(channelPath, "Channel").join(" / ") });
  return stages;
}
function renderOrganizationPreview(settings) {
  const stages = organizationPreviewPath(settings);
  const finalPath = stages.map(function(stage) { return stage.value; }).join(" / ") || organizationRootLabel();
  const inputs = stages.map(function(stage) {
    return '<div class="organization-stage"><strong>' + escapeHTML(stage.label) + "</strong><span>" + escapeHTML(stage.value) + "</span></div>";
  }).join('<span class="organization-arrow" aria-hidden="true">→</span>');
  return '<div id="organization-preview" class="organization-preview-panel" aria-live="polite"><div class="organization-preview-copy"><strong>Browse path preview</strong><span>This is how one channel will appear in Silo.</span></div><div class="organization-preview">' + inputs + (inputs ? '<span class="organization-arrow" aria-hidden="true">→</span>' : "") + '<div class="organization-result"><strong>Browse path</strong><span>' + escapeHTML(finalPath) + "</span></div></div></div>";
}
function adminSourceGroups() {
  const groups = {};
  effectiveChannels(false).forEach(function(channel) {
    const sourcePath = sourceCategoryOriginalLabel(channel);
    if (!sourcePath) return;
    const parts = parsedCategoryPath(sourcePath).map(normalizedNameToken).filter(Boolean);
    const separator = adminSettings().delimiter === "dash" ? " - " : " | ";
    const paths = parts.length ? parts.map(function(_, index) { return parts.slice(0, index + 1).join(separator); }) : [sourcePath];
    uniqueIDs(paths).forEach(function(path) {
      groups[path] = groups[path] || { sourcePath: path, count: 0 };
      groups[path].count++;
    });
  });
  return Object.keys(groups).sort().map(function(sourcePath) { return groups[sourcePath]; });
}
function adminSourceGroupCount(sourcePath) {
  const path = configuredCategoryPath(sourcePath);
  const group = adminSourceGroups().find(function(item) {
    return item.sourcePath === sourcePath || configuredCategoryPath(item.sourcePath) === path;
  });
  return group ? group.count : 0;
}
function addAdminCategoryAlias() {
  const source = byId("admin-alias-source");
  const alias = byId("admin-alias-path");
  const sourcePath = source ? String(source.value || "").trim() : "";
  const aliasPath = alias ? String(alias.value || "").trim() : "";
  if (!sourcePath || !aliasPath) return;
  const settings = state.adminCategorySettings || defaultAdminCategorySettings();
  settings.categoryAliases = normalizeCategoryAliases(items(settings.categoryAliases).concat([{ sourcePath: sourcePath, aliasPath: aliasPath }]));
  state.adminCategorySettings = settings;
  markAdminSettingsDraft();
  renderAdminPage();
}
function removeAdminCategoryAlias(index) {
  const settings = state.adminCategorySettings || defaultAdminCategorySettings();
  settings.categoryAliases = items(settings.categoryAliases).filter(function(_, rowIndex) { return rowIndex !== index; });
  state.adminCategorySettings = settings;
  normalizeAdminCategorySettings();
  markAdminSettingsDraft();
  renderAdminPage();
}
function updateAdminCategoryAlias(index, field, value) {
  const settings = state.adminCategorySettings || defaultAdminCategorySettings();
  const aliases = items(settings.categoryAliases).slice();
  aliases[index] = Object.assign({}, aliases[index] || {});
  aliases[index][field] = value;
  settings.categoryAliases = aliases;
  state.adminCategorySettings = settings;
  markAdminSettingsDraft();
}
function renderAdminCategoryAliasSettings() {
  const root = byId("admin-category-alias-settings");
  if (!root) return;
  const settings = adminSettings();
  const sourceGroups = adminSourceGroups();
  const aliases = categoryAliases();
  const sourceOptions = sourceGroups.map(function(group) {
    return "<option value=\"" + escapeHTML(group.sourcePath) + "\">" + escapeHTML(group.sourcePath) + " (" + escapeHTML(String(group.count)) + ")</option>";
  }).join("");
  let sourceState = "Select a source group";
  if (state.adminSourceGroupsLoading) sourceState = "Loading source groups...";
  else if (state.adminSourceGroupsError) sourceState = "Could not load source groups";
  else if (state.adminSourceGroupsLoaded && !sourceGroups.length) sourceState = "No source groups found";
  const sourceSelectOptions = "<option value=\"\">" + escapeHTML(sourceState) + "</option>" + sourceOptions;
  const sourceReady = state.adminSourceGroupsLoaded && sourceGroups.length > 0;
  const loadState = state.adminSourceGroupsError
    ? "<div class=\"settings-note settings-warning settings-note-action alias-source-state\"><span>Could not load source groups: " + escapeHTML(state.adminSourceGroupsError) + "</span><button type=\"button\" data-admin-alias-action=\"retry\">Retry</button></div>"
    : (!state.adminSourceGroupsLoaded && !state.adminSourceGroupsLoading ? "<div class=\"settings-note alias-source-state\">Open Organization to load available Dispatcharr groups.</div>" : "");
  const addRow = "<div class=\"alias-builder\"><div class=\"alias-builder-step\"><span class=\"alias-step-number\">1</span><label><span>Source group</span><select id=\"admin-alias-source\"" + (!sourceReady ? " disabled" : "") + ">" + sourceSelectOptions + "</select></label></div><span class=\"alias-builder-arrow\" aria-hidden=\"true\">→</span><div class=\"alias-builder-step\"><span class=\"alias-step-number\">2</span><label><span>Additional browse path</span><input id=\"admin-alias-path\" placeholder=\"Category | Subcategory\"" + (!sourceReady ? " disabled" : "") + "></label></div><button data-admin-alias-action=\"add\"" + (!sourceReady ? " disabled" : "") + ">Add path</button></div>";
  const rows = aliases.map(function(alias, index) {
    const count = adminSourceGroupCount(alias.sourcePath);
    return "<div class=\"alias-table-row" + (!count ? " stale" : "") + "\"><div class=\"alias-table-source\"><strong title=\"" + escapeHTML(alias.sourcePath) + "\">" + escapeHTML(alias.sourcePath) + "</strong>" + (!count ? "<small>Source not found</small>" : "") + "</div><span class=\"alias-table-arrow\">&rarr;</span><label><input data-admin-alias-index=\"" + index + "\" data-admin-alias-field=\"aliasPath\" value=\"" + escapeHTML(alias.aliasPath) + "\" title=\"" + escapeHTML(alias.aliasPath) + "\" aria-label=\"Alternative group name\"></label><span class=\"alias-table-count\">" + escapeHTML(String(count)) + "</span><span class=\"alias-table-actions\"><button data-admin-alias-action=\"remove\" data-admin-alias-index=\"" + index + "\">Remove</button></span></div>";
  }).join("");
  root.innerHTML = (settings.mode !== "delimiter" ? "<div class=\"settings-note settings-warning\">Alternative group names apply when category mode is By delimiter.</div>" : "")
    + loadState
    + addRow
    + "<div class=\"alias-table\"><div class=\"alias-table-head\"><span>Source group</span><span></span><span>Also show as</span><span>Channels</span><span>Actions</span></div>" + (rows || "<div class=\"empty\">No alternative group names yet.</div>") + "</div>";
}
function renderAdminEventKeywordSettings() {
  const root = byId("admin-event-keyword-settings");
  if (!root) return;
  const rows = normalizeEventKeywordRows(adminSettings().eventKeywords);
  root.innerHTML = rows.map(function(row, index) {
    const label = row.categoryName || eventCategoryName(row.categoryId);
    const series = row.eventSeries ? "<div class=\"event-keyword-options\"><label><span>Exclude</span><textarea data-admin-event-keyword-index=\"" + index + "\" data-admin-event-keyword-field=\"excludeKeywords\" aria-label=\"" + escapeHTML(label + " exclusion keywords") + "\">" + escapeHTML(row.excludeKeywords.join("\n")) + "</textarea></label><label class=\"event-window-field\"><span>Coverage window</span><span><input type=\"number\" min=\"15\" max=\"360\" step=\"15\" data-admin-event-keyword-index=\"" + index + "\" data-admin-event-keyword-field=\"groupWindowMinutes\" value=\"" + escapeHTML(String(row.groupWindowMinutes || 60)) + "\"><small>minutes</small></span></label></div>" : "";
    return "<div class=\"settings-row event-keyword-row" + (row.eventSeries ? " event-series-rule" : "") + "\"><span class=\"event-keyword-label\">" + escapeHTML(label) + (row.eventSeries ? "<small>Event series</small>" : "") + "</span><div class=\"event-keyword-fields\"><label><span>Match</span><textarea data-admin-event-keyword-index=\"" + index + "\" data-admin-event-keyword-field=\"keywords\" aria-label=\"" + escapeHTML(label + " event keywords") + "\">" + escapeHTML(row.keywords.join("\n")) + "</textarea></label>" + series + "</div></div>";
  }).join("");
}
function renderAdminFeaturedEventSettings() {
  const root = byId("admin-featured-event-settings");
  if (!root) return;
  if (!state.events && !state.eventsLoading) {
    state.eventsLoading = true;
    getJSON("/dispatcharr/api/events").then(function(payload) {
      state.events = payload || { events: [], categories: [] };
    }).catch(function(error) {
      state.events = { events: [], categories: [], error: readableError(error) };
    }).finally(function() {
      state.eventsLoading = false;
      if (state.view === "admin" && state.adminTab === "events") renderAdminPage();
    });
  }
  const selected = adminFeaturedEventMap();
  const events = items(state.events && state.events.events).filter(function(event) {
    return !event.completed || selected[event.id];
  }).sort(function(left, right) {
    if (!!selected[left.id] !== !!selected[right.id]) return selected[left.id] ? -1 : 1;
    if (sportsEventIsLive(left) !== sportsEventIsLive(right)) return sportsEventIsLive(left) ? -1 : 1;
    return Number(left.startUnix || Number.MAX_SAFE_INTEGER) - Number(right.startUnix || Number.MAX_SAFE_INTEGER);
  }).slice(0, 40);
  if (state.eventsLoading && !events.length) {
    root.innerHTML = "<div class=\"settings-note\">Loading detected events...</div>";
    return;
  }
  if (!events.length) {
    root.innerHTML = "<div class=\"settings-note" + (state.events && state.events.error ? " settings-warning" : "") + "\">" + escapeHTML((state.events && state.events.error) || "No upcoming guide events are currently detected.") + "</div>";
    return;
  }
  root.innerHTML = "<div class=\"admin-featured-event-grid\">" + events.map(function(event) {
    const checked = !!selected[event.id];
    const status = sportsEventIsLive(event) ? "Live now" : eventStatusLabel(event);
    return "<label class=\"admin-featured-event-row\"><span><strong>" + escapeHTML(event.shortName || event.name || "Event") + "</strong><small>" + escapeHTML([event.categoryName, status].filter(Boolean).join(" · ")) + "</small></span><input type=\"checkbox\" data-admin-featured-event-id=\"" + escapeHTML(event.id || "") + "\"" + (checked ? " checked" : "") + "></label>";
  }).join("") + "</div>";
}
function updateAdminEventKeywords(index, field, value) {
  const settings = state.adminCategorySettings || defaultAdminCategorySettings();
  const rows = normalizeEventKeywordRows(settings.eventKeywords);
  if (!rows[index]) return;
  const update = {};
  if (field === "groupWindowMinutes") update.groupWindowMinutes = Math.max(15, Math.min(360, Number(value) || 60));
  else update[field === "excludeKeywords" ? "excludeKeywords" : "keywords"] = normalizeKeywordList(value);
  rows[index] = Object.assign({}, rows[index], update);
  settings.eventKeywords = rows;
  state.adminCategorySettings = settings;
  state.events = null;
  markAdminSettingsDraft();
}
function ensureSelectedCustomGroup() {
  const groups = customGroups();
  if (groups.some(function(group) { return group.id === state.selectedCustomGroup; })) return;
  state.selectedCustomGroup = groups.length ? groups[0].id : "";
}
function selectedCustomGroup() {
  ensureSelectedCustomGroup();
  return customGroups().find(function(group) { return group.id === state.selectedCustomGroup; }) || null;
}
function renderCustomGroupSettings() {
  const root = byId("custom-group-settings");
  if (!root) return;
  const groups = customGroups();
  const selected = selectedCustomGroup();
  const memberships = selected ? customMemberships(selected.id) : [];
  const query = lower(state.customGroupQuery);
  const availableChannels = effectiveChannels(false).filter(function(channel) {
    if (selected && memberships.indexOf(channel.id) !== -1) return false;
    if (!query) return true;
    return lower(channel.name || channel.id).indexOf(query) !== -1 || lower(sourceCategoryLabel(channel)).indexOf(query) !== -1;
  });
  if (!availableChannels.some(function(channel) { return channel.id === state.customGroupChannelID; })) state.customGroupChannelID = availableChannels.length ? availableChannels[0].id : "";
  const pickerChannels = availableChannels.slice(0, 24);
  const createControl = "<div class=\"custom-group-control\"><label for=\"custom-group-name\">New group</label><div class=\"custom-group-field\"><input id=\"custom-group-name\" placeholder=\"Spanish\"><button data-custom-group-action=\"create\">Create</button></div></div>";
  const manageControl = groups.length
    ? "<div class=\"custom-group-control\"><label for=\"custom-group-select\">Edit group</label><div class=\"custom-group-field\"><select id=\"custom-group-select\">" + groups.map(function(group) { return "<option value=\"" + escapeHTML(group.id) + "\"" + (selected && selected.id === group.id ? " selected" : "") + ">" + escapeHTML(group.name) + "</option>"; }).join("") + "</select><button data-custom-group-action=\"delete\">Delete</button></div></div>"
    : "";
  const searchStatus = availableChannels.length
    ? "Showing " + pickerChannels.length + " of " + availableChannels.length + " matching channels."
    : "No matching channels.";
  const resultRows = pickerChannels.length ? pickerChannels.map(function(channel) {
    const active = channel.id === state.customGroupChannelID;
    return "<div class=\"custom-channel-result" + (active ? " selected" : "") + "\">"
      + "<button class=\"custom-channel-option\" type=\"button\" role=\"option\" aria-selected=\"" + (active ? "true" : "false") + "\" data-custom-group-channel-option=\"" + escapeHTML(channel.id) + "\"><strong>" + escapeHTML(channel.name || channel.id) + "</strong><small>" + escapeHTML(sourceCategoryLabel(channel) || "Live TV") + "</small></button>"
      + "<button class=\"custom-channel-add\" type=\"button\" data-custom-group-add-channel=\"" + escapeHTML(channel.id) + "\">Add</button>"
      + "</div>";
  }).join("") : "<div class=\"custom-group-empty\">No channels match that search.</div>";
  const memberRows = memberships.length ? memberships.map(function(id) {
    const channel = channelByID(id);
    return "<div class=\"custom-member-row\"><div><strong>" + escapeHTML((channel && channel.name) || id) + "</strong><small>" + escapeHTML((channel && sourceCategoryLabel(channel)) || "Missing from current lineup") + "</small></div><button data-custom-group-action=\"remove-channel\" data-channel-id=\"" + escapeHTML(id) + "\">Remove</button></div>";
  }).join("") : "<div class=\"custom-group-empty\">No channels in this group yet.</div>";
  root.innerHTML = "<div class=\"custom-groups-panel\">"
    + "<div class=\"custom-groups-controls\">" + createControl + manageControl + "</div>"
    + (selected ? "<div class=\"custom-group-meta\"><strong>" + escapeHTML(selected.name) + "</strong><span>" + memberships.length + " channels</span></div>"
      + "<div class=\"custom-group-workspace\">"
      + "<section class=\"custom-group-browser\"><div class=\"custom-group-section-head\"><strong>Add channels</strong><span>" + escapeHTML(searchStatus) + "</span></div><input class=\"custom-channel-search\" id=\"custom-group-channel-search\" role=\"combobox\" aria-controls=\"custom-group-channel-options\" aria-expanded=\"true\" aria-autocomplete=\"list\" placeholder=\"Search channels or groups\" value=\"" + escapeHTML(state.customGroupQuery) + "\"><div id=\"custom-group-channel-options\" class=\"custom-channel-options\" role=\"listbox\">" + resultRows + "</div></section>"
      + "<section class=\"custom-group-members\"><div class=\"custom-group-section-head\"><strong>Group channels</strong><span>" + memberships.length + " saved</span></div><div class=\"custom-member-list\">" + memberRows + "</div></section>"
      + "</div>"
      : "<div class=\"custom-group-empty\"><strong>Create a group</strong><span>Build a personal channel lineup by adding channels from the current Dispatcharr catalog.</span></div>")
    + "</div>";
}
function selectCustomGroupChannel(channelID) {
  state.customGroupChannelID = channelID || "";
  renderSettings();
}
function slug(value) {
  return String(value || "").toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "").slice(0, 48) || "group";
}
function createCustomGroup(name) {
  name = String(name || "").trim();
  if (!name) return;
  const base = "group:" + slug(name);
  let id = base;
  let index = 2;
  while (customGroups().some(function(group) { return group.id === id; })) id = base + "-" + index++;
  state.app.preferences.customGroups.push({ id: id, name: name, order: customGroups().length + 1 });
  state.app.preferences.customGroupMemberships[id] = [];
  state.selectedCustomGroup = id;
  savePrefs();
  render();
}
function deleteSelectedCustomGroup() {
  const selected = selectedCustomGroup();
  if (!selected) return;
  state.app.preferences.customGroups = customGroups().filter(function(group) { return group.id !== selected.id; });
  delete state.app.preferences.customGroupMemberships[selected.id];
  if (state.category === customCategoryID(selected.id)) state.category = "";
  state.selectedCustomGroup = "";
  savePrefs();
  render();
}
function addChannelToSelectedGroup(channelID) {
  const selected = selectedCustomGroup();
  if (!selected || !channelID) return;
  state.app.preferences.customGroupMemberships[selected.id] = uniqueIDs(customMemberships(selected.id).concat([channelID]));
  savePrefs();
  render();
}
function removeChannelFromSelectedGroup(channelID) {
  const selected = selectedCustomGroup();
  if (!selected || !channelID) return;
  state.app.preferences.customGroupMemberships[selected.id] = customMemberships(selected.id).filter(function(id) { return id !== channelID; });
  savePrefs();
  render();
}

function adminSportsGuidePolicy(settings) {
  const policy = settings.sportsGuide || {};
  return { enabled: policy.enabled !== false, groups: items(policy.groups).slice(), overrides: items(policy.overrides).slice() };
}
function renderAdminSportsGuide(settings) {
  const policy = adminSportsGuidePolicy(settings);
  const channels = items(state.app && state.app.channels);
  const groups = {};
  channels.forEach(function(ch) { if (ch.categoryId) groups[ch.categoryId] = ch.categoryName || ch.categoryId; });
  const leagueMap = {};
  items(state.sports && state.sports.leagues).forEach(function(league) { leagueMap[league.id] = league; });
  const events = items(state.sports && (state.sports.guideOptions || state.sports.events)).filter(function(event) { return String(event.id || "").indexOf("epg:") !== 0; });
  events.forEach(function(event) { if (event.leagueId && !leagueMap[event.leagueId]) leagueMap[event.leagueId] = { id: event.leagueId, name: event.leagueName || event.leagueId }; });
  const leagues = Object.keys(leagueMap).map(function(id) { return leagueMap[id]; });
  const option = function(value, label) { return "<option value=\"" + escapeHTML(value) + "\">" + escapeHTML(label) + "</option>"; };
  const channelOptions = option("", "Choose a channel") + channels.map(function(ch) { return option(ch.id, ch.name || ch.id); }).join("");
  const rules = policy.groups.map(function(rule, i) {
    return "<div class=\"profile-selection-row\"><span><strong>" + escapeHTML(groups[rule.groupId] || rule.groupId) + "</strong><small>" + escapeHTML((rule.leagueId || "Automatic league") + " · " + (rule.mode || "auto")) + "</small></span><button type=\"button\" data-admin-sports-action=\"guide-remove-group\" data-guide-index=\"" + i + "\" aria-label=\"Remove group hint\">" + icon("x") + "</button></div>";
  }).join("");
  const links = policy.overrides.map(function(link, i) {
    const channel = channels.find(function(ch) { return ch.id === link.channelId; });
    const event = events.find(function(event) { return event.id === link.eventId; });
    return "<div class=\"profile-selection-row\"><span><strong>" + escapeHTML((channel && channel.name) || link.channelId) + "</strong><small>" + escapeHTML(link.disabled ? "Automatic fill disabled" : ((event && event.name) || link.eventId)) + "</small></span><button type=\"button\" data-admin-sports-action=\"guide-remove-link\" data-guide-index=\"" + i + "\" aria-label=\"Remove event link\">" + icon("x") + "</button></div>";
  }).join("");
  return "<div class=\"settings-card-head sports-library-head\"><div><h3>Sports guide</h3><p>Fill gaps in the guide from confidently matched sports schedules. Provider listings stay in place. Generated windows are estimates.</p></div></div>"
    + "<label class=\"settings-row compact-row\"><span><strong>Fill missing sports listings</strong><small>Pregame, game, and postgame entries are cached for all guide views.</small></span><input type=\"checkbox\" data-admin-sports-field=\"guide\"" + (policy.enabled ? " checked" : "") + "></label>"
    + "<h4>Group hints</h4><p class=\"settings-note\">Narrow a group to one league, use tournament/session matching, or turn filling off for that group.</p>"
    + (rules ? "<div class=\"profile-selection-list\">" + rules + "</div>" : "")
    + "<div class=\"sports-team-channel-form\"><label><span>Group</span><select id=\"admin-guide-group\">" + option("", "Choose a group") + Object.keys(groups).sort().map(function(id) { return option(id, groups[id]); }).join("") + "</select></label>"
    + "<label><span>League</span><select id=\"admin-guide-league\">" + option("", "Automatic league") + leagues.map(function(league) { return option(league.id, league.name || league.id); }).join("") + "</select></label>"
    + "<label><span>Matching</span><select id=\"admin-guide-mode\">" + option("auto", "Automatic") + option("matchup", "Two competitors") + option("session", "Tournament or session") + option("off", "Do not fill") + "</select></label><button type=\"button\" data-admin-sports-action=\"guide-add-group\">Set hint</button></div>"
    + "<h4>Manual event links</h4><p class=\"settings-note\">Link an ambiguous channel to a scheduled event. The link expires with that event; it never rolls forward to another game.</p>"
    + (links ? "<div class=\"profile-selection-list\">" + links + "</div>" : "")
    + "<div class=\"sports-team-channel-form\"><label><span>Channel</span><select id=\"admin-guide-channel\">" + channelOptions + "</select></label>"
    + "<label><span>Event</span><select id=\"admin-guide-event\">" + option("", "Choose an event") + option("__off", "Do not fill this channel") + events.map(function(event) { return option(event.id, (event.name || event.id) + " · " + new Date(Number(event.startUnix) * 1000).toLocaleString()); }).join("") + "</select></label><button type=\"button\" data-admin-sports-action=\"guide-add-link\">Set link</button></div>";
}
function updateAdminSportsGuide(action, target) {
  const settings = state.adminCategorySettings || defaultAdminCategorySettings();
  const policy = adminSportsGuidePolicy(settings);
  const value = function(id) { const input = byId(id); return input ? String(input.value || "") : ""; };
  if (action === "guide-add-group") {
    const groupId = value("admin-guide-group");
    if (!groupId) return;
    policy.groups = policy.groups.filter(function(rule) { return rule.groupId !== groupId; });
    policy.groups.push({ groupId: groupId, leagueId: value("admin-guide-league"), mode: value("admin-guide-mode") });
  } else if (action === "guide-add-link") {
    const channelId = value("admin-guide-channel"), eventId = value("admin-guide-event");
    if (!channelId || !eventId) return;
    policy.overrides = policy.overrides.filter(function(link) { return link.channelId !== channelId; });
    policy.overrides.push({ channelId: channelId, eventId: eventId === "__off" ? "" : eventId, disabled: eventId === "__off" });
  } else {
    const index = Number(target.getAttribute("data-guide-index"));
    const rows = action === "guide-remove-group" ? policy.groups : policy.overrides;
    if (!Number.isInteger(index) || index < 0 || index >= rows.length) return;
    rows.splice(index, 1);
  }
  settings.sportsGuide = policy;
  state.adminCategorySettings = settings;
  markAdminSettingsDraft();
  renderAdminSportsSettings();
}
