const path = window.location.pathname;
const base = path.endsWith("/dispatcharr/player") ? path.slice(0, -"/dispatcharr/player".length) : (path.endsWith("/dispatcharr/admin") ? path.slice(0, -"/dispatcharr/admin".length) : (path.endsWith("/dispatcharr") ? path.slice(0, -"/dispatcharr".length) : ""));
const isAdminRoute = path.endsWith("/dispatcharr/admin");
const prefsKey = "silo.ramindex.dispatcharr.preferences.v1";
const searchHistoryKey = "silo.ramindex.dispatcharr.searchHistory.v1";
const adminSettingsKey = "adminCategorySettings";
const pluginInstallationID = (base.match(/\/api\/v1\/plugins\/(\d+)/) || [])[1] || "";
const localCacheSuffix = pluginInstallationID || "default";
const appCacheKey = "silo.ramindex.dispatcharr.appSnapshot.v1." + localCacheSuffix;
const adminSettingsLocalKey = "silo.ramindex.dispatcharr.adminSettings.v1." + localCacheSuffix;
const adminSettingsToken = "__ADMIN_SETTINGS_TOKEN__";
const state = { app: null, appLoadedFromCache: false, programsByChannel: {}, sortedPrograms: [], view: isAdminRoute ? "admin" : "home", category: "", query: "", searchQuery: "", searchType: "all", searchReturnView: "home", recentSearches: [], hls: null, tsPlayer: null, currentChannel: null, currentSession: null, heartbeat: null, muted: false, volume: 1, volumeMenuOpen: false, audioMenuOpen: false, moreMenuOpen: false, playerGuideOpen: false, selectedAudioTrack: 0, selectedTextTrack: -1, aspectMode: "fill", playerChromeIdle: false, playerChromeTimer: null, playerWaiting: false, multiviewTiles: [], multiviewActiveTileID: "", multiviewHeartbeat: null, recordings: null, recordingsLoading: false, sports: null, sportsLoading: false, sportsTab: "live", sportsLeague: "", sportsExpandedEvents: {}, events: null, eventsLoading: false, eventsTab: "upcoming", eventCategory: "", expandedEvents: {}, guideChannels: [], guideRendered: 0, guideLoading: false, refreshing: false, virtualCategoryView: "guide", selectedCustomGroup: "", customGroupQuery: "", customGroupChannelID: "", adminTab: "settings", adminCategorySettings: null, savedAdminCategorySettings: null, profileSaveStatus: "idle", profileSaveMessage: "", adminSaveStatus: "idle", adminSaveMessage: "" };

function applySiloTheme() {
  const params = new URLSearchParams(window.location.search);
  const theme = String(params.get("theme") || document.documentElement.dataset.siloTheme || "").trim().toLowerCase().replace(/[^a-z0-9_-]/g, "");
  if (theme) document.documentElement.dataset.siloTheme = theme;
}

applySiloTheme();

function route(url) { return base + url; }
function routeHeaders(extra) {
  const headers = Object.assign({}, extra || {});
  if (isAdminRoute && adminSettingsToken) headers["x-dispatcharr-admin-token"] = adminSettingsToken;
  return headers;
}
function byId(id) { return document.getElementById(id); }
function items(value) { return Array.isArray(value) ? value : []; }
function lower(value) { return String(value || "").toLowerCase(); }
function uniqueIDs(values) {
  const seen = {};
  const result = [];
  items(values).forEach(function(value) {
    value = String(value || "");
    if (!value || seen[value]) return;
    seen[value] = true;
    result.push(value);
  });
  return result;
}
function escapeHTML(value) {
  return String(value || "").replace(/[&<>"']/g, function(ch) {
    return ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "\"": "&quot;", "'": "&#39;" })[ch];
  });
}
function cssEscape(value) {
  if (window.CSS && CSS.escape) return CSS.escape(String(value || ""));
  return String(value || "").replace(/\\/g, "\\\\").replace(/"/g, "\\\"");
}
function icon(name) {
  const icons = {
    "arrow-left": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M15.75 19.5 8.25 12l7.5-7.5'/></svg>",
    "chevron-down": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='m6 9 6 6 6-6'/></svg>",
    "ellipsis": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M6.75 12a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0Zm6 0a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0Zm6 0a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0Z'/></svg>",
    "play": "<svg viewBox='0 0 24 24' fill='currentColor' aria-hidden='true'><path d='M8 5.6v12.8c0 .55.6.9 1.08.62l10.1-6.4a.73.73 0 0 0 0-1.24L9.08 4.98A.72.72 0 0 0 8 5.6Z'/></svg>",
    "record": "<svg viewBox='0 0 24 24' fill='currentColor' aria-hidden='true'><path d='M12 20.25a8.25 8.25 0 1 0 0-16.5 8.25 8.25 0 0 0 0 16.5Zm0-4a4.25 4.25 0 1 1 0-8.5 4.25 4.25 0 0 1 0 8.5Z'/></svg>",
    "pause": "<svg viewBox='0 0 24 24' fill='currentColor' aria-hidden='true'><path d='M7.25 5.25h3.25v13.5H7.25zM13.5 5.25h3.25v13.5H13.5z'/></svg>",
    "loader": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' d='M12 3a9 9 0 1 1-8.3 5.5'/></svg>",
    "speaker": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M19.1 8.9a7 7 0 0 1 0 6.2M16.2 10.9a3 3 0 0 1 0 2.2M4.5 14.25h3l4.25 3.25V6.5L7.5 9.75h-3v4.5Z'/></svg>",
    "speaker-off": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='m4.5 4.5 15 15M5 14.25h2.5l4.25 3.25v-5.75M11.75 8.7V6.5L8.8 8.75M16 10.8a3 3 0 0 1 .2 2.2'/></svg>",
    "airplay": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M6.75 17.25h-1.5A2.25 2.25 0 0 1 3 15V6.75A2.25 2.25 0 0 1 5.25 4.5h13.5A2.25 2.25 0 0 1 21 6.75V15a2.25 2.25 0 0 1-2.25 2.25h-1.5M8.25 21h7.5L12 16.5 8.25 21Z'/></svg>",
    "guide": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M4.5 6.75h15M4.5 12h15M4.5 17.25h15M8.25 4.5v15M15.75 4.5v15'/></svg>",
    "multiview": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M4.5 5.75h6.25v5.75H4.5zM13.25 5.75h6.25v5.75h-6.25zM4.5 14h6.25v4.25H4.5zM13.25 14h6.25v4.25h-6.25z'/></svg>",
    "settings": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M12 8.25a3.75 3.75 0 1 1 0 7.5 3.75 3.75 0 0 1 0-7.5Z'/><path stroke-linecap='round' stroke-linejoin='round' d='M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.6 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 8.92 4.6a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9c.23.64.84 1 1.51 1H21a2 2 0 0 1 0 4h-.09A1.65 1.65 0 0 0 19.4 15Z'/></svg>",
    "fullscreen": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M8.25 4.5H4.5v3.75M15.75 4.5h3.75v3.75M19.5 15.75v3.75h-3.75M4.5 15.75v3.75h3.75M9 9 4.5 4.5M15 9l4.5-4.5M15 15l4.5 4.5M9 15l-4.5 4.5'/></svg>",
    "fullscreen-exit": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M4.5 9h4.25V4.75M15.25 4.75V9h4.25M19.5 15h-4.25v4.25M8.75 19.25V15H4.5M8.75 9 4.5 4.75M15.25 9l4.25-4.25M15.25 15l4.25 4.25M8.75 15 4.5 19.25'/></svg>",
    "heart": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M21 8.25c0 6.25-9 11.25-9 11.25s-9-5-9-11.25A4.75 4.75 0 0 1 11.25 5 4.75 4.75 0 0 1 21 8.25Z'/></svg>",
    "heart-solid": "<svg viewBox='0 0 24 24' fill='currentColor' aria-hidden='true'><path d='M12 21s-9-5.1-9-12.25A5.45 5.45 0 0 1 12 4.7a5.45 5.45 0 0 1 9 4.05C21 15.9 12 21 12 21Z'/></svg>",
    "pip": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M4.5 6.75A2.25 2.25 0 0 1 6.75 4.5h10.5a2.25 2.25 0 0 1 2.25 2.25v10.5a2.25 2.25 0 0 1-2.25 2.25H6.75a2.25 2.25 0 0 1-2.25-2.25V6.75Z'/><path stroke-linecap='round' stroke-linejoin='round' d='M13.25 13.25h4.25v3.25h-4.25z'/></svg>",
    "captions": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M4.5 7.5A2.5 2.5 0 0 1 7 5h10a2.5 2.5 0 0 1 2.5 2.5v9A2.5 2.5 0 0 1 17 19H7a2.5 2.5 0 0 1-2.5-2.5v-9Z'/><path stroke-linecap='round' stroke-linejoin='round' d='M8.25 10.5h3M8.25 14h2.25M13.5 14h2.25'/></svg>",
    "language": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18Z'/><path stroke-linecap='round' stroke-linejoin='round' d='M3.75 9h16.5M3.75 15h16.5M12 3c2.25 2.35 3.25 5.25 3.25 9S14.25 18.65 12 21c-2.25-2.35-3.25-5.25-3.25-9S9.75 5.35 12 3Z'/></svg>",
    "aspect": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M4.5 7.25A2.75 2.75 0 0 1 7.25 4.5h9.5a2.75 2.75 0 0 1 2.75 2.75v9.5a2.75 2.75 0 0 1-2.75 2.75h-9.5a2.75 2.75 0 0 1-2.75-2.75v-9.5Z'/><path stroke-linecap='round' stroke-linejoin='round' d='M8 8h3M8 8v3M16 16h-3M16 16v-3'/></svg>",
    "search": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='m20 20-4.5-4.5M10.5 18a7.5 7.5 0 1 1 0-15 7.5 7.5 0 0 1 0 15Z'/></svg>",
    "copy": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M8 8h9.25A1.75 1.75 0 0 1 19 9.75v9.5A1.75 1.75 0 0 1 17.25 21h-9.5A1.75 1.75 0 0 1 6 19.25V10'/><path stroke-linecap='round' stroke-linejoin='round' d='M5.75 16H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v.75'/></svg>",
    "external": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M13.5 4.5H19.5V10.5M19.25 4.75 11 13M10.5 6H6.75A2.25 2.25 0 0 0 4.5 8.25v9A2.25 2.25 0 0 0 6.75 19.5h9A2.25 2.25 0 0 0 18 17.25V13.5'/></svg>",
    "x": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M6 6l12 12M18 6 6 18'/></svg>"
  };
  return icons[name] || "";
}
function menuIcon(name) { return "<span class=\"menu-icon\">" + icon(name) + "</span>"; }
function defaultPrefs() {
  return { favorites: {}, favoriteOrder: [], autoFavorites: {}, hiddenCategories: {}, sportsFavoriteTeams: {}, recentChannels: [], continueWatching: {}, playback: { backendProxySupported: false, streamMode: "redirect", outputFormat: "ts" }, categoryParsing: { enabled: false, mode: "off", delimiter: "pipe", regex: "", output: "" }, customGroups: [], customGroupMemberships: {} };
}
function prefs() { return state.app && state.app.preferences ? state.app.preferences : defaultPrefs(); }
function defaultEventKeywordRules() {
  return [
    { categoryId: "awards", categoryName: "Awards", keywords: ["Academy Awards", "The Oscars", "Oscars", "Tony Awards", "The Tonys", "Golden Globes", "Grammy Awards", "Grammys", "Emmy Awards", "Emmys", "CMA Awards", "ACM Awards", "Billboard Music Awards", "American Music Awards", "BET Awards", "MTV Video Music Awards", "Critics Choice Awards", "SAG Awards"] },
    { categoryId: "civic", categoryName: "Civic", keywords: ["State of the Union", "Presidential Address", "Joint Session", "Inauguration", "Election Night", "Presidential Debate"] },
    { categoryId: "parades", categoryName: "Parades", keywords: ["Thanksgiving Day Parade", "Macy's Thanksgiving Day Parade", "Rose Parade", "Christmas Parade"] },
    { categoryId: "entertainment", categoryName: "Entertainment", keywords: ["Live Special", "Special Presentation", "Red Carpet", "Ceremony", "Tribute Concert", "Benefit Concert", "Festival"] }
  ];
}
function defaultAdminCategorySettings() {
  return { mode: "normal", delimiter: "pipe", ecmEnabled: false, ecmURL: "", categoryAliases: [], eventKeywords: defaultEventKeywordRules() };
}
function cloneAdminCategorySettings(settings) {
  try { return JSON.parse(JSON.stringify(Object.assign(defaultAdminCategorySettings(), settings || {}))); }
  catch (_) { return defaultAdminCategorySettings(); }
}
function adminSettingsSignature(settings) {
  return JSON.stringify(cloneAdminCategorySettings(settings));
}
function adminSettingsDirty() {
  return adminSettingsSignature(state.adminCategorySettings) !== adminSettingsSignature(state.savedAdminCategorySettings);
}
function markAdminSettingsDraft() {
  if (state.adminSaveStatus !== "saving") {
    if (adminSettingsDirty()) {
      state.adminSaveStatus = "dirty";
      state.adminSaveMessage = "Unsaved changes.";
    } else {
      state.adminSaveStatus = "idle";
      state.adminSaveMessage = "";
    }
  }
}
function adminSettings() {
  return Object.assign(defaultAdminCategorySettings(), state.adminCategorySettings || {});
}
function sourceMode() { return state.app && state.app.source ? String(state.app.source.mode || "") : ""; }
function dvrEnabled() {
  return !!(state.app && state.app.capabilities && state.app.capabilities.recordings && sourceMode() === "direct_login");
}
function favoriteMap() { return prefs().favorites || {}; }
function autoFavoriteMap() { return prefs().autoFavorites || {}; }
function hiddenMap() { return prefs().hiddenCategories || {}; }
function sportsFavoriteTeamMap() { return prefs().sportsFavoriteTeams || {}; }
function mergePrefs(remote, local) {
  remote = Object.assign(defaultPrefs(), remote || {});
  local = Object.assign(defaultPrefs(), local || {});
  return {
    favorites: Object.assign({}, remote.favorites, local.favorites),
    favoriteOrder: uniqueIDs(items(local.favoriteOrder).length ? items(local.favoriteOrder) : items(remote.favoriteOrder)),
    autoFavorites: Object.assign({}, remote.autoFavorites, local.autoFavorites),
    hiddenCategories: Object.assign({}, remote.hiddenCategories, local.hiddenCategories),
    sportsFavoriteTeams: Object.assign({}, remote.sportsFavoriteTeams, local.sportsFavoriteTeams),
    recentChannels: uniqueIDs(items(remote.recentChannels).concat(items(local.recentChannels))).slice(0, 24),
    continueWatching: Object.assign({}, remote.continueWatching, local.continueWatching),
    playback: Object.assign({}, remote.playback, local.playback),
    categoryParsing: Object.assign({}, remote.categoryParsing, local.categoryParsing),
    customGroups: items(local.customGroups).length ? items(local.customGroups) : items(remote.customGroups),
    customGroupMemberships: Object.assign({}, remote.customGroupMemberships, local.customGroupMemberships)
  };
}
function normalizePreferences() {
  if (!state.app || !state.app.preferences) return;
  state.app.preferences = Object.assign(defaultPrefs(), state.app.preferences || {});
  state.app.preferences.categoryParsing = Object.assign(defaultPrefs().categoryParsing, state.app.preferences.categoryParsing || {});
  state.app.preferences.sportsFavoriteTeams = state.app.preferences.sportsFavoriteTeams || {};
  state.app.preferences.customGroups = items(state.app.preferences.customGroups);
  state.app.preferences.customGroupMemberships = state.app.preferences.customGroupMemberships || {};
  const valid = {};
  items(state.app.channels).forEach(function(channel) { valid[channel.id] = true; });
  const explicitFavorites = Object.keys(state.app.preferences.favorites || {}).filter(function(id) { return !!state.app.preferences.favorites[id] && !!valid[id]; });
  state.app.preferences.favoriteOrder = uniqueIDs(items(state.app.preferences.favoriteOrder).filter(function(id) { return !!state.app.preferences.favorites[id] && !!valid[id]; }).concat(explicitFavorites));
  const recent = uniqueIDs(items(state.app.preferences.recentChannels).filter(function(id) { return !!valid[id]; }));
  const watched = Object.keys(state.app.preferences.continueWatching || {}).sort(function(left, right) {
    const leftPlayed = Number((state.app.preferences.continueWatching[left] || {}).playedAt || 0);
    const rightPlayed = Number((state.app.preferences.continueWatching[right] || {}).playedAt || 0);
    return rightPlayed - leftPlayed;
  }).filter(function(id) { return !!valid[id]; });
  state.app.preferences.recentChannels = uniqueIDs(recent.concat(watched)).slice(0, 24);
  Object.keys(state.app.preferences.customGroupMemberships).forEach(function(groupID) {
    state.app.preferences.customGroupMemberships[groupID] = uniqueIDs(items(state.app.preferences.customGroupMemberships[groupID]).filter(function(id) { return !!valid[id]; }));
  });
}
function normalizeAdminCategorySettings() {
  state.adminCategorySettings = Object.assign(defaultAdminCategorySettings(), state.adminCategorySettings || {});
  if (state.adminCategorySettings.mode === "custom" || state.adminCategorySettings.mode === "admin_delimiter") state.adminCategorySettings.mode = "delimiter";
  if (["normal", "delimiter"].indexOf(state.adminCategorySettings.mode) === -1) state.adminCategorySettings.mode = "normal";
  if (!state.adminCategorySettings.delimiter) state.adminCategorySettings.delimiter = "pipe";
  if (state.adminCategorySettings.delimiter !== "pipe" && state.adminCategorySettings.delimiter !== "dash") state.adminCategorySettings.delimiter = "pipe";
  state.adminCategorySettings.ecmEnabled = state.adminCategorySettings.ecmEnabled === true;
  state.adminCategorySettings.ecmURL = normalizeAdminECMURL(state.adminCategorySettings.ecmURL);
  state.adminCategorySettings.categoryAliases = normalizeCategoryAliases(state.adminCategorySettings.categoryAliases);
  state.adminCategorySettings.eventKeywords = normalizeEventKeywordRows(state.adminCategorySettings.eventKeywords);
  delete state.adminCategorySettings.groupAliases;
  delete state.adminCategorySettings.adminGroups;
  delete state.adminCategorySettings.adminGroupMemberships;
  delete state.adminCategorySettings.presentationOverrides;
}
function normalizeCategoryAliases(value) {
  const seen = {};
  return items(value).map(function(alias) {
    return {
      sourcePath: String((alias && alias.sourcePath) || "").trim(),
      aliasPath: String((alias && alias.aliasPath) || "").trim()
    };
  }).filter(function(alias) {
    if (!alias.sourcePath || !alias.aliasPath) return false;
    const key = alias.sourcePath + "\u0000" + alias.aliasPath;
    if (seen[key]) return false;
    seen[key] = true;
    return true;
  });
}
function normalizeEventKeywordRows(value) {
  const defaults = defaultEventKeywordRules();
  const rows = items(value).map(function(row) {
    row = row || {};
    const categoryId = normalizeEventCategoryId(row.categoryId || row.categoryName || "");
    const categoryName = String(row.categoryName || eventCategoryName(categoryId)).trim();
    const keywords = normalizeKeywordList(row.keywords);
    return { categoryId: categoryId, categoryName: categoryName, keywords: keywords };
  }).filter(function(row) { return row.categoryId && row.keywords.length; });
  const byID = {};
  defaults.concat(rows).forEach(function(row) {
    const id = normalizeEventCategoryId(row.categoryId || row.categoryName);
    if (!id) return;
    const existing = byID[id] || { categoryId: id, categoryName: row.categoryName || eventCategoryName(id), keywords: [] };
    existing.keywords = normalizeKeywordList(existing.keywords.concat(row.keywords || []));
    byID[id] = existing;
  });
  return Object.keys(byID).sort(function(left, right) {
    return eventCategoryName(left).localeCompare(eventCategoryName(right));
  }).map(function(id) { return byID[id]; });
}
function normalizeKeywordList(value) {
  const rows = Array.isArray(value) ? value : String(value || "").split(/[\n,]+/);
  const seen = {};
  return rows.map(function(item) { return String(item || "").trim(); }).filter(function(item) {
    const key = lower(item);
    if (!key || seen[key]) return false;
    seen[key] = true;
    return true;
  });
}
function normalizeEventCategoryId(value) {
  value = lower(String(value || "").replace(/[^a-z0-9]+/gi, " ")).trim();
  if (value === "award") return "awards";
  if (value === "politics" || value === "political") return "civic";
  if (value === "parade") return "parades";
  if (value === "special" || value === "specials") return "entertainment";
  return value.replace(/\s+/g, "-");
}
function eventCategoryName(categoryId) {
  return ({ awards: "Awards", civic: "Civic", parades: "Parades", entertainment: "Entertainment" })[categoryId] || String(categoryId || "Events");
}
function categoryAliases() {
  return normalizeCategoryAliases(adminSettings().categoryAliases);
}
function normalizeAdminECMURL(value) {
  const fallback = "";
  const trimmed = String(value || "").trim();
  const lower = trimmed.toLowerCase();
  if (lower.indexOf("https://") === 0 || lower.indexOf("http://") === 0) return trimmed;
  return fallback;
}
function adminECMEnabled() {
  return adminSettings().ecmEnabled === true && !!adminECMURL();
}
function adminECMURL() {
  return normalizeAdminECMURL(adminSettings().ecmURL);
}
function recordWatchPreference(channel) {
  if (!state.app || !state.app.preferences || !channel) return;
  const id = String(channel.id || "");
  if (!id) return;
  const now = Math.floor(Date.now() / 1000);
  const existing = state.app.preferences.continueWatching[id] || {};
  const plays = Number(existing.plays || 0) + 1;
  state.app.preferences.recentChannels = uniqueIDs([id].concat(items(state.app.preferences.recentChannels))).slice(0, 24);
  state.app.preferences.continueWatching[id] = {
    itemKind: "channel",
    itemId: id,
    itemName: channel.name || id,
    playedAt: now,
    plays: plays
  };
  if (plays >= 3 && !favoriteMap()[id]) state.app.preferences.autoFavorites[id] = true;
  normalizePreferences();
  savePrefs();
}
function readLocalPrefs() {
  try { return Object.assign(defaultPrefs(), JSON.parse(localStorage.getItem(prefsKey) || "{}")); }
  catch (_) { return defaultPrefs(); }
}
function readRecentSearches() {
  try { return uniqueIDs(items(JSON.parse(localStorage.getItem(searchHistoryKey) || "[]")).map(function(value) { return String(value || "").trim(); })).slice(0, 12); }
  catch (_) { return []; }
}
function writeRecentSearches(searches) {
  state.recentSearches = uniqueIDs(items(searches).map(function(value) { return String(value || "").trim(); }).filter(Boolean)).slice(0, 12);
  try { localStorage.setItem(searchHistoryKey, JSON.stringify(state.recentSearches)); } catch (_) {}
}
function rememberSearch(value) {
  value = String(value || "").trim();
  if (!value) return;
  writeRecentSearches([value].concat(state.recentSearches));
}
function clearRecentSearches() {
  writeRecentSearches([]);
}
function cacheProgramWindow() {
  const now = Math.floor(Date.now() / 1000);
  return { start: now - (2 * 3600), end: now + (30 * 3600) };
}
function compactProgramsForCache(programs, stripSummary) {
  const windowInfo = cacheProgramWindow();
  return items(programs).filter(function(program) {
    const start = Number(program.startUnix || 0);
    const end = Number(program.endUnix || 0);
    return (!end || end >= windowInfo.start) && (!start || start <= windowInfo.end);
  }).map(function(program) {
    const compact = {
      id: program.id,
      channelId: program.channelId,
      title: program.title,
      startUnix: program.startUnix,
      endUnix: program.endUnix
    };
    if (!stripSummary && program.summary) compact.summary = program.summary;
    return compact;
  });
}
function compactAppPayloadForCache(payload, stripSummary, channelsOnly) {
  if (!payload || !items(payload.channels).length) return null;
  return {
    cachedAtUnix: Math.floor(Date.now() / 1000),
    status: payload.status || {},
    source: payload.source || {},
    channels: items(payload.channels),
    categories: items(payload.categories),
    programs: channelsOnly ? [] : compactProgramsForCache(payload.programs, stripSummary),
    vod: { available: !!(payload.vod && payload.vod.available), categories: [], items: [] },
    series: { available: !!(payload.series && payload.series.available), categories: [], items: [] },
    preferences: defaultPrefs(),
    sessions: [],
    capabilities: payload.capabilities || {}
  };
}
function writeLocalAppCache(payload) {
  const variants = [
    compactAppPayloadForCache(payload, false, false),
    compactAppPayloadForCache(payload, true, false),
    compactAppPayloadForCache(payload, true, true)
  ].filter(Boolean);
  for (let index = 0; index < variants.length; index++) {
    try {
      localStorage.removeItem(appCacheKey);
      localStorage.setItem(appCacheKey, JSON.stringify(variants[index]));
      return;
    } catch (_) {}
  }
}
function readLocalAppCache() {
  try {
    const cached = JSON.parse(localStorage.getItem(appCacheKey) || "null");
    if (!cached || !items(cached.channels).length) return null;
    const age = Math.floor(Date.now() / 1000) - Number(cached.cachedAtUnix || 0);
    if (age < 0 || age > 72 * 3600) return null;
    cached.preferences = defaultPrefs();
    cached.sessions = [];
    cached.programs = items(cached.programs);
    cached.categories = items(cached.categories);
    cached.channels = items(cached.channels);
    cached.capabilities = cached.capabilities || {};
    return cached;
  } catch (_) {
    return null;
  }
}
function readLocalAdminSettings() {
  try { return readAdminSettingsValue(JSON.parse(localStorage.getItem(adminSettingsLocalKey) || "null")); }
  catch (_) { return defaultAdminCategorySettings(); }
}
function writeLocalAdminSettings(settings) {
  try { localStorage.setItem(adminSettingsLocalKey, JSON.stringify(cloneAdminCategorySettings(settings))); } catch (_) {}
}
function readSiloPrefsValue(value) {
  if (!value) return null;
  try { return Object.assign(defaultPrefs(), JSON.parse(value)); }
  catch (_) { return null; }
}
function readAdminSettingsValue(value) {
  if (!value) return defaultAdminCategorySettings();
  try {
    if (typeof value === "string") return Object.assign(defaultAdminCategorySettings(), JSON.parse(value));
    if (typeof value === "object") return Object.assign(defaultAdminCategorySettings(), value);
  }
  catch (_) { return defaultAdminCategorySettings(); }
  return defaultAdminCategorySettings();
}
async function loadPluginSettingsValues() {
  if (!pluginInstallationID) return null;
  const payload = await coreGetJSON("/api/v1/settings/plugins/" + encodeURIComponent(pluginInstallationID));
  return payload && payload.values ? payload.values : {};
}
async function loadUserPrefs() {
  const values = await loadPluginSettingsValues();
  return readSiloPrefsValue(values ? values.preferences : "");
}
async function loadAdminCategorySettings() {
  return readAdminSettingsValue(await getJSON(adminSettingsURL()));
}
function adminSettingsURL() {
  if (!adminSettingsToken) return "/dispatcharr/api/admin-settings";
  return "/dispatcharr/api/admin-settings?admin_token=" + encodeURIComponent(adminSettingsToken);
}
async function savePluginSettingValue(key, value) {
  if (!pluginInstallationID) throw new Error("plugin installation settings unavailable");
  const values = await loadPluginSettingsValues().catch(function() { return {}; }) || {};
  values[key] = value;
  await corePutNoContent("/api/v1/settings/plugins/" + encodeURIComponent(pluginInstallationID), { values: values });
}
function writeLocalPrefs() {
  try { localStorage.setItem(prefsKey, JSON.stringify(state.app.preferences)); } catch (_) {}
}
function savePrefs(options) {
  if (!state.app || !state.app.preferences) return;
  options = options || {};
  writeLocalPrefs();
  if (pluginInstallationID) {
    state.profileSaveStatus = "saving";
    state.profileSaveMessage = "";
    savePluginSettingValue("preferences", JSON.stringify(state.app.preferences)).then(function() {
      state.profileSaveStatus = "saved";
      state.profileSaveMessage = "Saved to your Silo profile.";
      if (state.view === "settings") renderSettings();
    }).catch(function(error) {
      state.profileSaveStatus = "error";
      state.profileSaveMessage = "Saved on this device, but not to your Silo profile.";
      if (!options.quiet) showAppToast(state.profileSaveMessage);
      if (state.view === "settings") renderSettings();
      try { console.warn("Dispatcharr profile preference save failed", error); } catch (_) {}
    });
  } else {
    state.profileSaveStatus = "local";
    state.profileSaveMessage = "Saved on this device, but not to your Silo profile.";
  }
  postJSON("/dispatcharr/api/preferences", state.app.preferences).catch(function() {});
}
function saveAdminCategorySettings() {
  state.adminCategorySettings = Object.assign(defaultAdminCategorySettings(), state.adminCategorySettings || {});
  normalizeAdminCategorySettings();
  state.adminSaveStatus = "saving";
  state.adminSaveMessage = "Saving...";
  if (state.view === "admin") renderAdminPage();
  postJSON(adminSettingsURL(), state.adminCategorySettings).then(function(saved) {
    state.adminCategorySettings = readAdminSettingsValue(saved);
    normalizeAdminCategorySettings();
  }).then(function() {
    state.savedAdminCategorySettings = cloneAdminCategorySettings(state.adminCategorySettings);
    writeLocalAdminSettings(state.adminCategorySettings);
    state.adminSaveStatus = "saved";
    state.adminSaveMessage = "Saved group settings.";
    if (state.view === "admin") renderAdminPage();
  }).catch(function(error) {
    state.adminSaveStatus = "error";
    state.adminSaveMessage = "Could not save group settings: " + readableError(error);
    if (state.view === "admin") renderAdminPage();
    try { console.warn("Dispatcharr admin group settings save failed", error); } catch (_) {}
  });
}
function discardAdminCategorySettings() {
  state.adminCategorySettings = cloneAdminCategorySettings(state.savedAdminCategorySettings);
  state.adminSaveStatus = "idle";
  state.adminSaveMessage = "";
  if (state.category.indexOf("virtual:") === 0 && !categoryName(state.category)) state.category = "";
  renderAdminPage();
}
async function getJSON(url) {
  const response = await fetch(route(url), { credentials: "include", headers: routeHeaders() });
  if (!response.ok) throw await requestError(response);
  return response.json();
}
async function postJSON(url, body) {
  const response = await fetch(route(url), { method: "POST", credentials: "include", headers: routeHeaders({ "content-type": "application/json" }), body: JSON.stringify(body) });
  if (!response.ok) throw await requestError(response);
  return response.json();
}
async function coreGetJSON(url) {
  const response = await fetch(url, { credentials: "include" });
  if (!response.ok) throw await requestError(response);
  return response.json();
}
async function corePutNoContent(url, body) {
  const response = await fetch(url, { method: "PUT", credentials: "include", headers: { "content-type": "application/json" }, body: JSON.stringify(body) });
  if (!response.ok) throw await requestError(response);
}
async function requestError(response) {
  const text = await response.text().catch(function() { return ""; });
  const detail = text ? ": " + text.slice(0, 240) : "";
  return new Error("request failed (" + response.status + ")" + detail);
}
function readableError(error) {
  return String(error && error.message ? error.message : error || "unknown error");
}
function channelByID(id) {
  const channel = rawChannelByID(id);
  return channel ? effectiveChannel(channel) : null;
}
function rawChannelByID(id) {
  return items(state.app.channels).find(function(channel) { return channel.id === id; }) || null;
}
function categoryStartsFeatured(name) {
  return String(name || "").trim().indexOf("*") === 0;
}
function categoryDisplayName(name) {
  name = String(name || "").trim();
  return categoryStartsFeatured(name) ? name.slice(1).trim() : name;
}
function effectiveChannel(channel) {
  if (!channel) return null;
  const copy = Object.assign({}, channel);
  const label = sourceCategoryLabel(channel);
  if (label) copy.categoryName = label;
  return copy;
}
function effectiveChannels(includeHidden) {
  return items(state.app.channels).map(function(channel, index) {
    const copy = effectiveChannel(channel);
    copy.sourceIndex = index;
    return copy;
  }).sort(function(left, right) {
    return (left.sourceIndex || 0) - (right.sourceIndex || 0);
  });
}
function sourceCategoryID(id) { return "source:" + String(id || ""); }
function customCategoryID(id) { return "custom:" + String(id || ""); }
function virtualCategoryID(path) { return "virtual:" + String(path || ""); }
function featuredCategoryID(path) { return "featured:" + String(path || ""); }
function virtualCategoryPath(id) { return String(id || "").indexOf("virtual:") === 0 ? String(id || "").slice("virtual:".length) : ""; }
function featuredCategoryPath(id) { return String(id || "").indexOf("featured:") === 0 ? String(id || "").slice("featured:".length) : ""; }
function categoryParsing() {
  const settings = adminSettings();
  const delimiterEnabled = settings.mode === "delimiter";
  return { enabled: delimiterEnabled, mode: delimiterEnabled ? "delimiter" : "off", delimiter: settings.delimiter || "pipe", regex: "", output: "" };
}
function customGroups() {
  return items(prefs().customGroups).slice().filter(function(group) {
    return group && group.id && group.name;
  }).sort(function(left, right) {
    return (Number(left.order || 0) - Number(right.order || 0)) || String(left.name || "").localeCompare(String(right.name || ""));
  });
}
function customMemberships(groupID) {
  return uniqueIDs(items((prefs().customGroupMemberships || {})[groupID]));
}
function sourceCategoryRawName(id) {
  const category = items(state.app.categories).find(function(item) { return item.id === id; });
  return category ? category.name : "";
}
function sourceCategoryName(id) {
  return categoryDisplayName(sourceCategoryRawName(id));
}
function sourceCategoryRawLabel(channel) {
  return sourceCategoryRawName(channel.categoryId) || channel.categoryName || "";
}
function sourceCategoryLabel(channel) {
  return categoryDisplayName(sourceCategoryRawLabel(channel));
}
function normalizedGroupName(value) {
  return categoryDisplayName(value).toLowerCase();
}
function isWorldCupReplayGroup(value) {
  return normalizedGroupName(value) === "world cup replays";
}
function delimiterPattern() {
  return (adminSettings().delimiter || "pipe") === "pipe" ? /\s*\|\s*/ : /\s+-\s*/;
}
function parsedDelimitedPath(name) {
  name = String(name || "").trim();
  if (!name) return [];
  return name.split(delimiterPattern()).map(function(part) { return String(part || "").trim(); }).filter(Boolean);
}
function parsedCategoryPath(name) {
  const settings = categoryParsing();
  name = String(name || "").trim();
  if (!settings.enabled || !name) return [];
  let parts = [];
  if (settings.mode === "delimiter") {
    parts = parsedDelimitedPath(name);
  } else if (settings.mode === "regex" && settings.regex) {
    try {
      const pattern = new RegExp(settings.regex);
      const match = name.match(pattern);
      if (!match) return [];
      if (settings.output) parts = name.replace(pattern, settings.output).split("/");
      else parts = match.slice(1);
    } catch (_) {
      return [];
    }
  }
  parts = parts.map(function(part) { return String(part || "").trim(); }).filter(Boolean);
  return parts.length > 1 ? parts : [];
}
function categoryPathFromDisplayName(name) {
  const display = categoryDisplayName(name);
  const parts = parsedCategoryPath(display);
  return parts.length > 1 ? parts.join(" / ") : display;
}
function virtualPathForChannel(channel) {
  const paths = virtualPathsForChannel(channel);
  return paths.length ? paths[0] : "";
}
function sourceVirtualPathForChannel(channel) {
  return parsedCategoryPath(sourceCategoryLabel(channel)).join(" / ");
}
function featuredPathForSourceName(name) {
  if (!categoryStartsFeatured(name)) return "";
  return categoryPathFromDisplayName(name);
}
function featuredPathsForChannel(channel) {
  const path = featuredPathForSourceName(sourceCategoryRawLabel(channel));
  return path ? [path] : [];
}
function configuredCategoryPath(value) {
  const display = categoryDisplayName(value);
  const parts = parsedCategoryPath(display);
  if (parts.length > 1) return parts.join(" / ");
  const slashParts = String(display || "").split(/\s*\/\s*/).map(function(part) { return String(part || "").trim(); }).filter(Boolean);
  return slashParts.length > 1 ? slashParts.join(" / ") : "";
}
function aliasVirtualPathsForSourcePath(sourcePath) {
  return categoryAliases().filter(function(alias) {
    return configuredCategoryPath(alias.sourcePath) === sourcePath;
  }).map(function(alias) {
    return configuredCategoryPath(alias.aliasPath);
  }).filter(Boolean);
}
function virtualPathsForChannel(channel) {
  const paths = [];
  const sourcePath = sourceVirtualPathForChannel(channel);
  if (sourcePath) paths.push(sourcePath);
  aliasVirtualPathsForSourcePath(sourcePath).forEach(function(path) { paths.push(path); });
  return uniqueIDs(paths);
}
function isRewindableChannel(channel) {
  if (!channel) return false;
  if (isWorldCupReplayGroup(sourceCategoryRawLabel(channel)) || isWorldCupReplayGroup(sourceCategoryLabel(channel))) return true;
  return virtualPathsForChannel(channel).some(function(path) {
    if (isWorldCupReplayGroup(path)) return true;
    return path.split(" / ").some(isWorldCupReplayGroup);
  });
}
function channelInSelectedCategory(channel, id) {
  if (!id) return true;
  if (id.indexOf("source:") === 0) return channel.categoryId === id.slice("source:".length);
  if (id.indexOf("custom:") === 0) return customMemberships(id.slice("custom:".length)).indexOf(channel.id) !== -1;
  if (id.indexOf("featured:") === 0) {
    const selected = featuredCategoryPath(id);
    return featuredPathsForChannel(channel).some(function(path) {
      return path === selected || path.indexOf(selected + " / ") === 0;
    });
  }
  if (id.indexOf("virtual:") === 0) {
    const selected = virtualCategoryPath(id);
    return virtualPathsForChannel(channel).some(function(path) {
      return path === selected || path.indexOf(selected + " / ") === 0;
    });
  }
  return channel.categoryId === id;
}
function visibleChannels(ignoreQuery) {
  const hidden = hiddenMap();
  const channels = effectiveChannels(false).filter(function(channel) {
    if (channel.categoryId && hidden[channel.categoryId]) return false;
    if (state.view !== "favorites" && state.category && !channelInSelectedCategory(channel, state.category)) return false;
    if (!ignoreQuery && state.query && !guideChannelMatchesQuery(channel)) return false;
    if (state.view === "favorites" && !favoriteMap()[channel.id] && !autoFavoriteMap()[channel.id]) return false;
    return true;
  });
  return state.view === "favorites" ? orderedFavoriteChannels(channels) : channels;
}
function orderedFavoriteChannels(channels) {
  const byID = {};
  items(channels || effectiveChannels(false)).forEach(function(channel) { byID[channel.id] = channel; });
  const ordered = uniqueIDs(items(prefs().favoriteOrder)).map(function(id) { return byID[id]; }).filter(Boolean);
  const missing = items(channels || effectiveChannels(false)).filter(function(channel) {
    return (favoriteMap()[channel.id] || autoFavoriteMap()[channel.id]) && ordered.indexOf(channel) === -1;
  });
  return ordered.concat(missing);
}
function moveFavorite(channelID, direction) {
  const favorites = orderedFavoriteChannels(visibleChannels(true)).filter(function(channel) { return !!favoriteMap()[channel.id]; });
  const order = favorites.map(function(channel) { return channel.id; });
  const index = order.indexOf(channelID);
  if (index === -1) return;
  const target = direction === "up" ? index - 1 : index + 1;
  if (target < 0 || target >= order.length) return;
  const value = order[index];
  order[index] = order[target];
  order[target] = value;
  state.app.preferences.favoriteOrder = order;
  savePrefs();
  render();
}
function channelMatchesQuery(channel) {
  if (!state.query) return true;
  return lower([channel.name, channel.categoryName, channel.number].join(" ")).indexOf(lower(state.query)) !== -1;
}
function programMatchesQuery(program) {
  if (!state.query) return true;
  return lower([program.title, program.description].join(" ")).indexOf(lower(state.query)) !== -1;
}
function guideChannelMatchesQuery(channel) {
  if (!state.query || channelMatchesQuery(channel)) return true;
  return programsFor(channel.id).some(programMatchesQuery);
}
function rebuildProgramIndex() {
  const sorted = items(state.app && state.app.programs).slice().sort(function(a, b) {
    return (a.startUnix || 0) - (b.startUnix || 0);
  });
  const byChannel = {};
  sorted.forEach(function(program) {
    const id = String(program.channelId || "");
    if (!id) return;
    if (!byChannel[id]) byChannel[id] = [];
    byChannel[id].push(program);
  });
  state.sortedPrograms = sorted;
  state.programsByChannel = byChannel;
}
function programsFor(channelID) {
  const now = Math.floor(Date.now() / 1000);
  const source = channelID ? items(state.programsByChannel[channelID]) : items(state.sortedPrograms);
  return source.filter(function(program) {
    return (!channelID || program.channelId === channelID) && (!program.endUnix || program.endUnix >= now - 3600);
  });
}
function timeLabel(unix) {
  if (!unix) return "";
  return new Date(unix * 1000).toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
}
function dateTimeLabel(unix) {
  if (!unix) return "Never";
  return new Date(unix * 1000).toLocaleString([], { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" });
}
function sourceModeLabel(mode) {
  mode = String(mode || sourceMode() || "");
  if (mode === "direct_login") return "Dispatcharr Direct";
  if (mode === "api_key") return "Dispatcharr API Key";
  if (mode === "xtream") return "Xtream Codes";
  if (mode === "m3u_xmltv") return "M3U + XMLTV";
  return mode || "Not configured";
}
function guideSlotStart() {
  const now = Math.floor(Date.now() / 1000);
  return Math.floor(now / 1800) * 1800;
}
function guideSlots() {
  const start = guideSlotStart();
  const slots = [];
  for (let index = 0; index < 50; index++) slots.push(start + index * 1800);
  return slots;
}
function guideTimelineStyle(slots) {
  return "--epg-slots: " + slots.length + "; --epg-width: " + (slots.length * 11.25) + "rem;";
}
function guideWindow() {
  const start = guideSlotStart();
  return { start: start, end: start + (25 * 3600), slotCount: 50 };
}
function epgCellStyle(startUnix, endUnix, windowInfo) {
  const start = Math.max(startUnix || windowInfo.start, windowInfo.start);
  const end = Math.min(endUnix || start + 1800, windowInfo.end);
  const leftSlots = (start - windowInfo.start) / 1800;
  const widthSlots = Math.max((end - start) / 1800, 0);
  return "left: calc(" + leftSlots.toFixed(4) + " * var(--epg-slot)); width: calc(" + widthSlots.toFixed(4) + " * var(--epg-slot) - 0.0625rem);";
}
function stopPlayback() {
  const video = byId("player");
  if (state.hls) { state.hls.destroy(); state.hls = null; }
  if (state.tsPlayer) { state.tsPlayer.destroy(); state.tsPlayer = null; }
  if (state.playerChromeTimer) {
    clearTimeout(state.playerChromeTimer);
    state.playerChromeTimer = null;
  }
  state.playerChromeIdle = false;
  if (video) {
    video.pause();
    video.removeAttribute("src");
    video.load();
  }
}
function stopCurrentWatch(reason) {
  if (!state.currentSession) return;
  postJSON("/dispatcharr/api/watch/stop", { sessionId: state.currentSession.id, reason: reason || "stop" }).catch(function() {});
  state.currentSession = null;
  if (state.heartbeat) {
    clearInterval(state.heartbeat);
    state.heartbeat = null;
  }
}
function multiviewTileKey(channelID) {
  return "mv-" + String(channelID || "").replace(/[^A-Za-z0-9_-]+/g, "-").replace(/^-+|-+$/g, "").slice(0, 48) + "-" + Math.random().toString(36).slice(2, 8);
}
function multiviewTileByID(tileID) {
  return items(state.multiviewTiles).find(function(tile) { return tile.id === tileID; }) || null;
}
function destroyMultiviewMedia(tile) {
  if (!tile) return;
  if (tile.hls) { tile.hls.destroy(); tile.hls = null; }
  if (tile.tsPlayer) { tile.tsPlayer.destroy(); tile.tsPlayer = null; }
  tile.attached = false;
}
function resetMultiviewMedia() {
  items(state.multiviewTiles).forEach(destroyMultiviewMedia);
}
function syncMultiviewAudio() {
  if (!items(state.multiviewTiles).length) state.multiviewActiveTileID = "";
  if (!state.multiviewActiveTileID && state.multiviewTiles[0]) state.multiviewActiveTileID = state.multiviewTiles[0].id;
  items(state.multiviewTiles).forEach(function(tile) {
    const video = byId(tile.videoID);
    const active = tile.id === state.multiviewActiveTileID;
    if (video) {
      video.muted = !active;
      video.volume = active ? state.volume : 0;
    }
    const root = document.querySelector("[data-multiview-tile=\"" + cssEscape(tile.id) + "\"]");
    if (root) root.classList.toggle("active", active);
  });
}
function startMultiviewHeartbeat() {
  if (state.multiviewHeartbeat) clearInterval(state.multiviewHeartbeat);
  state.multiviewHeartbeat = setInterval(function() {
    items(state.multiviewTiles).forEach(function(tile) {
      if (tile.session) postJSON("/dispatcharr/api/watch/heartbeat", { sessionId: tile.session.id }).catch(function() {});
    });
  }, 30000);
}
function startMultiviewWatch(tile) {
  if (!tile || tile.session || !tile.channel) return;
  recordWatchPreference(tile.channel);
  postJSON("/dispatcharr/api/watch/start", { itemKind: "channel", itemId: tile.channel.id, itemName: tile.channel.name }).then(function(payload) {
    tile.session = payload.session;
    startMultiviewHeartbeat();
    renderRail();
  }).catch(function() {});
}
function stopMultiviewWatch(tile, reason) {
  if (!tile || !tile.session) return;
  postJSON("/dispatcharr/api/watch/stop", { sessionId: tile.session.id, reason: reason || "stop" }).catch(function() {});
  tile.session = null;
}
function stopAllMultiview(reason) {
  items(state.multiviewTiles).forEach(function(tile) {
    destroyMultiviewMedia(tile);
    stopMultiviewWatch(tile, reason || "stop_multiview");
  });
  state.multiviewTiles = [];
  state.multiviewActiveTileID = "";
  if (state.multiviewHeartbeat) {
    clearInterval(state.multiviewHeartbeat);
    state.multiviewHeartbeat = null;
  }
}
function setView(view) {
  if (view === "search" && state.view !== "search" && state.view !== "player") {
    state.searchReturnView = state.view || "home";
  }
  if (view !== "player") {
    stopPlayback();
    if (state.view === "player") stopCurrentWatch("leave_player");
  }
  if (view !== "multiview" && state.view === "multiview") stopAllMultiview("leave_multiview");
  state.view = view;
  if (view === "favorites") state.category = "";
  if (view === "sports") {
    state.category = "";
    loadSports(false);
  }
  if (view === "events") {
    state.category = "";
    loadEvents(false);
  }
  render();
}
function setCategory(id) {
  state.category = id || "";
  state.view = id ? "live" : "home";
  render();
}
async function hydrateApp(payload, options) {
  options = options || {};
  state.app = payload;
  const localPrefs = readLocalPrefs();
  if (options.localCache) {
    state.app.preferences = mergePrefs(state.app.preferences, localPrefs);
    state.adminCategorySettings = readLocalAdminSettings();
  } else {
    const values = await loadPluginSettingsValues().catch(function() { return null; });
    const siloPrefs = readSiloPrefsValue(values && values.preferences ? values.preferences : "");
    state.app.preferences = siloPrefs ? mergePrefs(siloPrefs, {}) : mergePrefs(state.app.preferences, localPrefs);
    const savedAdminSettings = values && values[adminSettingsKey] ? readAdminSettingsValue(values[adminSettingsKey]) : null;
    const localAdminSettings = readLocalAdminSettings();
    state.adminCategorySettings = await loadAdminCategorySettings().catch(function() {
      return savedAdminSettings || localAdminSettings;
    });
  }
  state.savedAdminCategorySettings = cloneAdminCategorySettings(state.adminCategorySettings);
  state.app.programs = items(state.app.programs);
  state.recentSearches = readRecentSearches();
  rebuildProgramIndex();
  normalizePreferences();
  normalizeAdminCategorySettings();
  state.savedAdminCategorySettings = cloneAdminCategorySettings(state.adminCategorySettings);
  writeLocalAdminSettings(state.adminCategorySettings);
  if (!options.localCache) writeLocalAppCache(state.app);
  if (!isAdminRoute && !options.localCache) savePrefs({ quiet: true });
}
async function loadApp() {
  const cached = readLocalAppCache();
  let renderedCachedApp = false;
  if (cached) {
    await hydrateApp(cached, { localCache: true });
    state.appLoadedFromCache = true;
    renderedCachedApp = true;
    render();
  }
  try {
    await hydrateApp(await getJSON("/dispatcharr/api/app"));
    state.appLoadedFromCache = false;
    render();
  } catch (error) {
    if (!renderedCachedApp) throw error;
    showAppToast("Showing saved guide. Refresh failed.");
    try { console.warn("Dispatcharr app refresh failed", error); } catch (_) {}
  }
}
async function refreshAppData() {
  if (state.refreshing) return;
  const buttons = Array.prototype.slice.call(document.querySelectorAll("[data-guide-refresh]"));
  state.refreshing = true;
  buttons.forEach(function(button) {
    button.classList.add("is-loading");
    button.disabled = true;
  });
  try {
    await hydrateApp(await postJSON("/dispatcharr/api/refresh", {}));
    if (guideNeedsFollowupRefresh()) {
      showAppToast("Guide refresh started. Waiting for EPG data...");
      await pollGuideRefresh();
    }
    state.recordings = null;
    render();
    showAppToast(guideHasPrograms() ? "Guide refreshed from Dispatcharr." : "Guide refreshed, but no EPG entries are available yet.");
  } catch (error) {
    showAppToast("Dispatcharr refresh failed.");
  } finally {
    state.refreshing = false;
    buttons.forEach(function(button) {
      button.classList.remove("is-loading");
      button.disabled = false;
    });
  }
}
function guideHasPrograms() {
  return items(state.app && state.app.programs).length > 0;
}
function guideNeedsFollowupRefresh() {
  const status = state.app && state.app.status ? state.app.status : {};
  const epgStatus = String(status.epgStatus || "").toLowerCase();
  return !guideHasPrograms() || epgStatus === "loading";
}
async function pollGuideRefresh() {
  for (let attempt = 0; attempt < 4 && guideNeedsFollowupRefresh(); attempt++) {
    await new Promise(function(resolve) { setTimeout(resolve, 1800); });
    await hydrateApp(await getJSON("/dispatcharr/api/app"));
  }
}
function renderRail() {
  document.querySelectorAll("[data-view]").forEach(function(button) {
    const unavailable = button.dataset.view === "recordings" && !dvrEnabled();
    const activeViews = String(button.dataset.activeViews || button.dataset.view || "").split(/\s+/).filter(Boolean);
    button.hidden = unavailable;
    button.classList.toggle("active", !unavailable && activeViews.indexOf(state.view) !== -1);
  });
  const favoriteCount = byId("favorite-count");
  if (favoriteCount) favoriteCount.textContent = Object.keys(favoriteMap()).length + Object.keys(autoFavoriteMap()).length;
}
function channelLogoFallback(channel) {
  const name = String((channel && channel.name) || "TV").trim();
  const region = name.match(/^\(([A-Za-z0-9]{2,4})\)/);
  if (region) return region[1].slice(0, 4);
  const parts = name.replace(/[^A-Za-z0-9]+/g, " ").trim().split(/\s+/).filter(Boolean);
  if (parts.length > 1) return parts.slice(0, 2).map(function(part) { return part.charAt(0); }).join("");
  return (parts[0] || name || "TV").slice(0, 5);
}
function logoHTML(channel) {
  const fallback = "<span class=\"logo logo-fallback\"" + (channel && channel.logoUrl ? " hidden" : "") + " aria-hidden=\"true\">" + escapeHTML(channelLogoFallback(channel)) + "</span>";
  if (channel && channel.logoUrl) return "<img class=\"logo\" src=\"" + escapeHTML(channel.logoUrl) + "\" alt=\"\" onerror=\"this.hidden = true; this.nextElementSibling.hidden = false;\">" + fallback;
  return fallback;
}
function renderGuideChannelButton(channel) {
  const channelName = channel.name || "Untitled";
  return "<button class=\"epg-channel\" data-channel=\"" + escapeHTML(channel.id) + "\" data-channel-name=\"" + escapeHTML(channelName) + "\" aria-label=\"" + escapeHTML(channelName) + "\" title=\"" + escapeHTML(channelName) + "\">" + logoHTML(channel) + "<span class=\"epg-channel-title\">" + escapeHTML(channelName) + "</span></button>";
}
function render() {
  if (!state.app) return;
  if (state.view === "recordings" && !dvrEnabled()) state.view = "home";
  if (state.view === "admin" && !isAdminRoute) state.view = "home";
  document.querySelector(".shell").classList.toggle("is-player", state.view === "player");
  document.querySelector(".shell").classList.toggle("is-guide", state.view === "guide");
  document.querySelector(".shell").classList.toggle("is-sports", state.view === "sports");
  document.querySelector(".shell").classList.toggle("is-events", state.view === "events");
  document.querySelector(".shell").classList.toggle("is-multiview", state.view === "multiview");
  document.querySelector(".shell").classList.toggle("is-search", state.view === "search");
  renderRail();
  renderSportsTopbarTabs();
  if (state.view === "guide") renderGuidePage();
  else if (state.view === "player") renderPlayerPage();
  else if (state.view === "multiview") renderMultiviewPage();
  else if (state.view === "live" || state.view === "favorites") renderLivePage();
  else if (state.view === "sports") renderSportsPage();
  else if (state.view === "events") renderEventsPage();
  else if (state.view === "search") renderSearchPage();
  else if (state.view === "recordings") renderRecordingsPage();
  else if (state.view === "admin") renderAdminPage();
  else if (state.view === "settings") renderSettings();
  else renderHome();
}
function renderHome() {
  const root = byId("view");
  const recent = recentChannels(10);
  root.innerHTML = sectionHeader("Recently watched") + rowCards(recent.length ? recent : visibleChannels(false).slice(0, 6)) + renderHomeGuide(recent) + categoryGrid();
}
function emptyStateHTML(title, detail) {
  detail = String(detail || "").trim();
  return "<div class=\"empty\"><strong>" + escapeHTML(title) + "</strong>" + (detail ? "<div class=\"muted\">" + escapeHTML(detail) + "</div>" : "") + "</div>";
}
function catalogEmptyDetail() {
  if (!state.app || !state.app.status) return "Check your connection in Live TV Admin or press Refresh.";
  const status = state.app.status;
  if (status.status === "error" && status.lastError) return status.lastError;
  if (!status.channelCount) return "No channels synced yet. Run a sync from Live TV Admin or press Refresh.";
  return "Try Refresh or open Live TV Admin to verify the connection.";
}
function sectionHeader(title) {
  return "<div class=\"section-title\"><span>" + escapeHTML(title) + "</span></div>";
}
function sectionHeaderWithActions(title, actions) {
  return "<div class=\"section-title\"><span>" + escapeHTML(title) + "</span>" + (actions || "") + "</div>";
}
function sectionActions(actions) {
  return "<div class=\"section-title actions-only\">" + (actions || "") + "</div>";
}
function rowCards(channels) {
  if (!channels.length) return emptyStateHTML("No channels yet.", catalogEmptyDetail());
  return "<div class=\"row-scroll\">" + channels.map(function(channel) {
    return "<button class=\"continue-card\" data-channel=\"" + escapeHTML(channel.id) + "\"><div class=\"poster-box\">" + (channel.logoUrl ? "<img src=\"" + escapeHTML(channel.logoUrl) + "\" alt=\"\">" : "<span>" + escapeHTML((channel.name || "TV").slice(0, 5)) + "</span>") + "</div><strong>" + escapeHTML(channel.name || "Untitled") + "</strong><div class=\"muted\">" + escapeHTML(channel.categoryName || "Live TV") + "</div></button>";
  }).join("") + "</div>";
}
function searchNeedle() {
  return lower(state.searchQuery).trim();
}
function searchableChannels() {
  const hidden = hiddenMap();
  return effectiveChannels(false).filter(function(channel) {
    return !(channel.categoryId && hidden[channel.categoryId]);
  });
}
function channelMatchesSearch(channel, query) {
  const haystack = [channel.name, channel.number, channel.categoryName, sourceCategoryLabel(channel), sourceCategoryRawLabel(channel)].join(" ");
  return lower(haystack).indexOf(query) !== -1;
}
function programMatchesSearch(program, query) {
  const channel = channelByID(program.channelId) || {};
  const haystack = [program.title, program.summary, program.description, channel.name, channel.categoryName].join(" ");
  return lower(haystack).indexOf(query) !== -1;
}
function contentCategoryName(kind, item) {
  const payload = state.app && state.app[kind] ? state.app[kind] : {};
  const match = items(payload.categories).find(function(category) { return category.id === item.categoryId; });
  return (match && match.name) || "";
}
function contentMatchesSearch(kind, item, query) {
  const haystack = [item.name, item.title, item.description, item.rating, contentCategoryName(kind, item)].join(" ");
  return lower(haystack).indexOf(query) !== -1;
}
function searchFilters() {
  return [
    { id: "all", label: "All" },
    { id: "channels", label: "Channels" },
    { id: "programs", label: "Programs" },
    { id: "movies", label: "Movies" },
    { id: "shows", label: "Shows" }
  ];
}
function searchResultSections(query) {
  const filter = state.searchType || "all";
  const include = function(id) { return filter === "all" || filter === id; };
  const sections = [];
  if (include("channels")) {
    const channels = searchableChannels().filter(function(channel) { return channelMatchesSearch(channel, query); }).slice(0, 18);
    sections.push({ id: "channels", title: "Channels", rows: channels.map(function(channel) {
      return {
        attrs: "data-search-channel=\"" + escapeHTML(channel.id) + "\"",
        art: logoHTML(channel),
        title: channel.name || "Untitled",
        meta: ["Channel", channel.categoryName || "Live TV"].filter(Boolean).join(" - "),
        action: "Watch"
      };
    }) });
  }
  if (include("programs")) {
    const programs = programsFor("").filter(function(program) { return programMatchesSearch(program, query); }).slice(0, 18);
    sections.push({ id: "programs", title: "Guide Programs", rows: programs.map(function(program) {
      const channel = channelByID(program.channelId) || {};
      return {
        attrs: "data-search-channel=\"" + escapeHTML(program.channelId || "") + "\"",
        art: logoHTML(channel),
        title: program.title || "Untitled program",
        meta: [timeLabel(program.startUnix), channel.name || "Live TV"].filter(Boolean).join(" - "),
        action: "Watch"
      };
    }) });
  }
  if (include("movies")) {
    const movies = items(state.app && state.app.vod && state.app.vod.items).filter(function(item) { return contentMatchesSearch("vod", item, query); }).slice(0, 12);
    sections.push({ id: "movies", title: "Movies", rows: movies.map(function(item) {
      return {
        attrs: "",
        disabled: true,
        art: item.posterUrl ? "<img src=\"" + escapeHTML(item.posterUrl) + "\" alt=\"\">" : "<span class=\"logo logo-fallback\">VOD</span>",
        title: item.name || "Untitled movie",
        meta: ["Movie", contentCategoryName("vod", item), item.rating].filter(Boolean).join(" - "),
        action: "On Demand"
      };
    }) });
  }
  if (include("shows")) {
    const shows = items(state.app && state.app.series && state.app.series.items).filter(function(item) { return contentMatchesSearch("series", item, query); }).slice(0, 12);
    sections.push({ id: "shows", title: "Shows", rows: shows.map(function(item) {
      return {
        attrs: "",
        disabled: true,
        art: item.posterUrl ? "<img src=\"" + escapeHTML(item.posterUrl) + "\" alt=\"\">" : "<span class=\"logo logo-fallback\">TV</span>",
        title: item.name || "Untitled show",
        meta: ["Show", contentCategoryName("series", item), item.releaseDate].filter(Boolean).join(" - "),
        action: "On Demand"
      };
    }) });
  }
  return sections.filter(function(section) { return section.rows.length; });
}
function renderSearchResultRow(row) {
  return "<button class=\"search-result\" type=\"button\" " + (row.attrs || "") + (row.disabled ? " disabled" : "") + "><span class=\"search-result-art\">" + row.art + "</span><span class=\"search-result-main\"><strong>" + escapeHTML(row.title) + "</strong><small>" + escapeHTML(row.meta || "") + "</small></span><span class=\"search-result-action\">" + escapeHTML(row.action || "") + "</span></button>";
}
function renderSearchResults(query) {
  const sections = searchResultSections(query);
  if (!sections.length) return "<div class=\"search-empty\">No matches found.</div>";
  return "<div class=\"search-results\">" + sections.map(function(section) {
    return sectionHeader(section.title) + "<div class=\"search-result-list\">" + section.rows.map(renderSearchResultRow).join("") + "</div>";
  }).join("") + "</div>";
}
function renderSearchStart() {
  const recent = items(state.recentSearches);
  const recentHTML = recent.length ? sectionHeaderWithActions("Recent searches", "<button class=\"search-clear\" type=\"button\" data-search-clear=\"true\">Clear All</button>") + "<div class=\"search-chip-row\">" + recent.map(function(value) {
    return "<button class=\"search-chip\" type=\"button\" data-search-recent=\"" + escapeHTML(value) + "\">" + escapeHTML(value) + "</button>";
  }).join("") + "</div>" : "";
  const categoryHTML = sectionHeader("Categories") + "<div class=\"search-category-grid\">" + [
    { id: "channels", label: "Channels", icon: "guide" },
    { id: "programs", label: "Programs", icon: "search" },
    { id: "movies", label: "Movies", icon: "play" },
    { id: "shows", label: "Shows", icon: "multiview" }
  ].map(function(item) {
    return "<button class=\"search-category-tile\" type=\"button\" data-search-type=\"" + escapeHTML(item.id) + "\">" + icon(item.icon) + "<strong>" + escapeHTML(item.label) + "</strong></button>";
  }).join("") + "</div>";
  const browsed = recentChannels(10);
  const browsedHTML = browsed.length ? sectionHeader("Recently browsed") + rowCards(browsed) : "";
  return recentHTML + categoryHTML + browsedHTML;
}
function renderSearchPage() {
  const root = byId("view");
  const query = state.searchQuery || "";
  const filter = state.searchType || "all";
  const filterHTML = "<div class=\"search-chip-row\">" + searchFilters().map(function(item) {
    return "<button class=\"search-chip" + (filter === item.id ? " active" : "") + "\" type=\"button\" data-search-type=\"" + escapeHTML(item.id) + "\">" + escapeHTML(item.label) + "</button>";
  }).join("") + "</div>";
  root.innerHTML = "<div class=\"search-page\"><div class=\"search-hero\"><h2>Search</h2><div class=\"search-form\"><input id=\"search-page-input\" class=\"search-field\" value=\"" + escapeHTML(query) + "\" placeholder=\"Search movies, tv shows, channels and more\" autocomplete=\"off\"><button class=\"search-cancel\" type=\"button\" data-search-cancel=\"true\">Cancel</button></div></div>" + filterHTML + (searchNeedle() ? renderSearchResults(searchNeedle()) : renderSearchStart()) + "</div>";
  const input = byId("search-page-input");
  if (input && document.activeElement !== input) {
    setTimeout(function() {
      const focused = byId("search-page-input");
      if (focused) {
        focused.focus();
        focused.setSelectionRange(focused.value.length, focused.value.length);
      }
    }, 0);
  }
}
function loadSports(force) {
  if (state.sportsLoading) return Promise.resolve(state.sports || { events: [], leagues: [] });
  if (state.sports && !force) return Promise.resolve(state.sports);
  state.sportsLoading = true;
  return getJSON("/dispatcharr/api/sports" + (force ? "?refresh=1" : "")).then(function(payload) {
    state.sports = payload || { events: [], leagues: [] };
    applySportsFavoritesToPayload();
    return state.sports;
  }).catch(function(error) {
    if (!state.sports) state.sports = { events: [], leagues: [], error: readableError(error) };
    else state.sports.error = readableError(error);
    showAppToast("Could not refresh sports.");
    return state.sports;
  }).finally(function() {
    state.sportsLoading = false;
    if (state.view === "sports") renderSportsPage();
  });
}
function applySportsFavoritesToPayload() {
  const favorites = sportsFavoriteTeamMap();
  items(state.sports && state.sports.events).forEach(function(event) {
    if (event.home) event.home.favorite = !!favorites[event.home.id];
    if (event.away) event.away.favorite = !!favorites[event.away.id];
  });
}
function sportsTabLabel(tab) {
  return ({ live: "Live", upcoming: "Upcoming", favorites: "Favorites", all: "All" })[tab] || "Live";
}
function sportsTabButtonsHTML() {
  return ["live", "upcoming", "favorites", "all"].map(function(tab) {
    return "<button type=\"button\" data-sports-tab=\"" + tab + "\" class=\"" + (state.sportsTab === tab ? "active" : "") + "\" aria-pressed=\"" + (state.sportsTab === tab ? "true" : "false") + "\">" + escapeHTML(sportsTabLabel(tab)) + "</button>";
  }).join("");
}
function renderSportsTopbarTabs() {
  const root = byId("sports-topbar-tabs");
  if (!root) return;
  root.innerHTML = "";
}
function renderSportsTabFilters() {
  return "<div class=\"sports-filter-row\"><div class=\"view-toggle\" aria-label=\"Sports filter\">" + sportsTabButtonsHTML() + "</div></div>";
}
function renderSportsPage() {
  const root = byId("view");
  if (!state.sports && !state.sportsLoading) loadSports(false);
  renderSportsTopbarTabs();
  const payload = state.sports || { events: [], leagues: [] };
  const events = filteredSportsEvents(payload);
  root.innerHTML = "<div class=\"sports-page\"><div class=\"sports-pinned\">" + renderSportsTabFilters() + renderSportsLeagueFilters(payload)
    + (payload.error ? "<div class=\"sports-error\">" + escapeHTML(payload.error) + "</div>" : "")
    + (state.sportsLoading && !events.length ? "<div class=\"empty\">Loading sports...</div>" : "")
    + "</div><div class=\"sports-score-scroll\">"
    + (events.length ? "<div class=\"sports-board\">" + events.map(renderSportsEventCard).join("") + "</div>" : (!state.sportsLoading ? "<div class=\"empty\">No sports matches.</div>" : ""))
    + "</div>"
    + "</div>";
}
function renderSportsLeagueFilters(payload) {
  const leagues = items(payload && payload.leagues);
  if (!leagues.length) return "";
  const chips = ["<button class=\"chip" + (!state.sportsLeague ? " active" : "") + "\" data-sports-league=\"\">All leagues</button>"].concat(leagues.map(function(league) {
    const label = league.name || league.id || "League";
    return "<button class=\"chip" + (state.sportsLeague === league.id ? " active" : "") + "\" data-sports-league=\"" + escapeHTML(league.id) + "\">" + escapeHTML(label) + "</button>";
  }));
  return "<div class=\"sports-leagues\">" + chips.join("") + "</div>";
}
function filteredSportsEvents(payload) {
  const now = Math.floor(Date.now() / 1000);
  return items(payload && payload.events).filter(function(event) {
    if (state.sportsLeague && event.leagueId !== state.sportsLeague) return false;
    if (state.sportsTab === "live" && !event.live) return false;
    const startUnix = Number(event.startUnix || 0);
    if (state.sportsTab === "upcoming" && (event.completed || event.live || (startUnix > 0 && startUnix < now - 3600))) return false;
    if (state.sportsTab === "favorites" && !sportsEventHasFavoriteTeam(event)) return false;
    return true;
  });
}
function sportsEventHasFavoriteTeam(event) {
  const favorites = sportsFavoriteTeamMap();
  return !!(favorites[(event.home || {}).id] || favorites[(event.away || {}).id]);
}
function sportsEventMatchesQuery(event) {
  const channels = items(event.channels).map(function(channel) { return [channel.name, channel.categoryName, channel.reason].join(" "); }).join(" ");
  const text = [event.name, event.shortName, event.leagueName, event.statusText, (event.home || {}).name, (event.home || {}).abbreviation, (event.away || {}).name, (event.away || {}).abbreviation, channels].join(" ");
  return lower(text).indexOf(lower(state.query)) !== -1;
}
function renderSportsEventCard(event) {
  const status = sportsStatusLabel(event);
  const title = event.shortName || event.name || "Game";
  return "<article class=\"sports-card" + (event.live ? " live" : "") + "\"><div class=\"sports-card-head\"><div class=\"sports-card-title\"><span class=\"sports-league-pill\">" + escapeHTML(event.leagueName || "Sports") + "</span><strong data-overflow-tooltip=\"" + escapeHTML(event.name || title) + "\">" + escapeHTML(title) + "</strong></div><div class=\"sports-status\">" + escapeHTML(status) + "</div></div>"
    + renderSportsMatchup(event, status)
    + renderSportsChannels(event)
    + "</article>";
}
function sportsStatusLabel(event) {
  if (event.live) return event.statusText || "Live";
  if (event.completed) return event.statusText || "Final";
  if (event.startUnix) return sportsDateLabel(event.startUnix);
  return event.statusText || "Time TBD";
}
function sportsDateLabel(unix) {
  const date = new Date(Number(unix || 0) * 1000);
  return date.toLocaleDateString([], { weekday: "short", month: "short", day: "numeric" }) + " " + date.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
}
function sportsTeamName(team) {
  return (team && (team.name || team.abbreviation)) || "Team";
}
function sportsTeamAbbreviation(team) {
  const name = sportsTeamName(team);
  return (team && team.abbreviation) || name.split(/\s+/).map(function(part) { return part.slice(0, 1); }).join("").slice(0, 3);
}
function renderSportsTeamLogo(team, className) {
  const name = sportsTeamName(team);
  const label = sportsTeamAbbreviation(team).slice(0, 3);
  if (team && team.logoUrl) return "<img class=\"" + className + "\" src=\"" + escapeHTML(team.logoUrl) + "\" alt=\"\" onerror=\"this.hidden = true; this.nextElementSibling.hidden = false;\"><span class=\"" + className + " logo-fallback\" hidden>" + escapeHTML(label) + "</span>";
  return "<span class=\"" + className + " logo-fallback\">" + escapeHTML(label) + "</span>";
}
function renderSportsMatchup(event, status) {
  const center = event.live ? "Live" : (event.completed ? "Final" : "VS");
  const detail = event.live || event.completed ? status : "";
  return "<div class=\"sports-matchup\">" + renderSportsMatchTeam(event.away || {}, event.awayScore) + "<div class=\"sports-versus\"><strong>" + escapeHTML(center) + "</strong>" + (detail ? "<span>" + escapeHTML(detail) + "</span>" : "") + "</div>" + renderSportsMatchTeam(event.home || {}, event.homeScore) + "</div>";
}
function renderSportsMatchTeam(team, score) {
  const name = team.name || team.abbreviation || "Team";
  const favorite = !!sportsFavoriteTeamMap()[team.id];
  const logo = renderSportsTeamLogo(team, "sports-match-team-logo");
  const favoriteControl = team.id ? "<button class=\"sports-team-favorite" + (favorite ? " active" : "") + "\" type=\"button\" data-sports-favorite-team=\"" + escapeHTML(team.id || "") + "\" data-sports-favorite-enabled=\"" + (favorite ? "false" : "true") + "\" aria-label=\"" + escapeHTML(favorite ? "Remove favorite team" : "Favorite team") + "\">&#9733;<span>" + (favorite ? "Following" : "Follow") + "</span></button>" : "<span class=\"sports-team-favorite placeholder\" aria-hidden=\"true\"></span>";
  return "<div class=\"sports-match-team\">" + logo + "<strong data-overflow-tooltip=\"" + escapeHTML(name) + "\">" + escapeHTML(name) + "</strong><span class=\"sports-match-team-score\">" + escapeHTML(score || "") + "</span>" + favoriteControl + "</div>";
}
function renderSportsChannels(event) {
  const channels = items(event.channels);
  if (!channels.length) return "<div class=\"muted\">No matching channels.</div>";
  const expanded = !!state.sportsExpandedEvents[event.id];
  const visible = expanded ? channels : channels.slice(0, 3);
  const hiddenCount = channels.length - visible.length;
  const more = hiddenCount > 0 ? "<button class=\"sports-channel-more\" type=\"button\" data-sports-expand-event=\"" + escapeHTML(event.id || "") + "\">+" + hiddenCount + " more</button>" : (expanded && channels.length > 3 ? "<button class=\"sports-channel-more\" type=\"button\" data-sports-expand-event=\"" + escapeHTML(event.id || "") + "\">Show less</button>" : "");
  return "<div class=\"sports-channels\">" + visible.map(function(channel) {
    const meta = channel.categoryName || channel.reason || "Live TV";
    return "<div class=\"sports-channel-wrap\"><button class=\"sports-channel\" type=\"button\" data-channel=\"" + escapeHTML(channel.id) + "\" title=\"" + escapeHTML(channel.reason || meta) + "\"><strong data-overflow-tooltip=\"" + escapeHTML(channel.name || "Channel") + "\">" + escapeHTML(channel.name || "Channel") + "</strong><small>" + escapeHTML(meta) + "</small></button><button class=\"sports-channel-multiview\" type=\"button\" data-multiview-channel=\"" + escapeHTML(channel.id) + "\" aria-label=\"Add " + escapeHTML(channel.name || "channel") + " to multiview\" title=\"Add to multiview\">" + icon("multiview") + "<span>Multiview</span></button></div>";
  }).join("") + more + "</div>";
}
function setSportsTab(tab) {
  state.sportsTab = tab || "live";
  state.sportsExpandedEvents = {};
  renderSportsPage();
}
function setSportsLeague(leagueID) {
  state.sportsLeague = leagueID || "";
  state.sportsExpandedEvents = {};
  renderSportsPage();
}
function toggleSportsEventChannels(eventID) {
  eventID = String(eventID || "");
  if (!eventID) return;
  if (state.sportsExpandedEvents[eventID]) delete state.sportsExpandedEvents[eventID];
  else state.sportsExpandedEvents[eventID] = true;
  renderSportsPage();
}
function toggleSportsTeamFavorite(teamID, enabled) {
  teamID = String(teamID || "");
  if (!teamID) return;
  if (enabled) state.app.preferences.sportsFavoriteTeams[teamID] = true;
  else delete state.app.preferences.sportsFavoriteTeams[teamID];
  applySportsFavoritesToPayload();
  savePrefs();
  renderSportsPage();
  postJSON("/dispatcharr/api/sports/favorites", { teamId: teamID, enabled: !!enabled }).catch(function() {});
}
function loadEvents(force) {
  if (state.eventsLoading) return Promise.resolve(state.events || { events: [], categories: [] });
  if (state.events && !force) return Promise.resolve(state.events);
  state.eventsLoading = true;
  return getJSON("/dispatcharr/api/events" + (force ? "?refresh=1" : "")).then(function(payload) {
    state.events = payload || { events: [], categories: [] };
    return state.events;
  }).catch(function(error) {
    if (!state.events) state.events = { events: [], categories: [], error: readableError(error) };
    else state.events.error = readableError(error);
    showAppToast("Could not refresh events.");
    return state.events;
  }).finally(function() {
    state.eventsLoading = false;
    if (state.view === "events") renderEventsPage();
  });
}
function eventTabLabel(tab) {
  return ({ live: "Live", upcoming: "Upcoming", all: "All" })[tab] || "Live";
}
function eventTabButtonsHTML() {
  return ["upcoming", "live", "all"].map(function(tab) {
    return "<button type=\"button\" data-event-tab=\"" + tab + "\" class=\"" + (state.eventsTab === tab ? "active" : "") + "\" aria-pressed=\"" + (state.eventsTab === tab ? "true" : "false") + "\">" + escapeHTML(eventTabLabel(tab)) + "</button>";
  }).join("");
}
function renderEventTabFilters() {
  return "<div class=\"sports-filter-row\"><div class=\"view-toggle\" aria-label=\"Events filter\">" + eventTabButtonsHTML() + "</div></div>";
}
function renderEventsPage() {
  const root = byId("view");
  if (!state.events && !state.eventsLoading) loadEvents(false);
  renderSportsTopbarTabs();
  const payload = state.events || { events: [], categories: [] };
  const events = filteredBroadcastEvents(payload);
  root.innerHTML = "<div class=\"sports-page\"><div class=\"sports-pinned\">" + renderEventTabFilters() + renderEventCategoryFilters(payload)
    + (payload.error ? "<div class=\"sports-error\">" + escapeHTML(payload.error) + "</div>" : "")
    + (state.eventsLoading && !events.length ? "<div class=\"empty\">Loading events...</div>" : "")
    + "</div><div class=\"sports-score-scroll\">"
    + (events.length ? "<div class=\"sports-board\">" + events.map(renderBroadcastEventCard).join("") + "</div>" : (!state.eventsLoading ? "<div class=\"empty\">No matching events.</div>" : ""))
    + "</div>"
    + "</div>";
}
function renderEventCategoryFilters(payload) {
  const categories = items(payload && payload.categories);
  if (!categories.length) return "";
  const chips = ["<button class=\"chip" + (!state.eventCategory ? " active" : "") + "\" data-event-category=\"\">All events</button>"].concat(categories.map(function(category) {
    const label = category.name || category.id || "Events";
    return "<button class=\"chip" + (state.eventCategory === category.id ? " active" : "") + "\" data-event-category=\"" + escapeHTML(category.id) + "\">" + escapeHTML(label) + "</button>";
  }));
  return "<div class=\"sports-leagues\">" + chips.join("") + "</div>";
}
function filteredBroadcastEvents(payload) {
  const now = Math.floor(Date.now() / 1000);
  return items(payload && payload.events).filter(function(event) {
    if (state.eventCategory && event.categoryId !== state.eventCategory) return false;
    if (state.eventsTab === "live" && !event.live) return false;
    const startUnix = Number(event.startUnix || 0);
    if (state.eventsTab === "upcoming" && (event.completed || event.live || (startUnix > 0 && startUnix < now - 3600))) return false;
    return true;
  });
}
function broadcastEventMatchesQuery(event) {
  const channels = items(event.channels).map(function(channel) { return [channel.name, channel.categoryName, channel.reason].join(" "); }).join(" ");
  const text = [event.name, event.shortName, event.categoryName, event.keyword, event.description, channels].join(" ");
  return lower(text).indexOf(lower(state.query)) !== -1;
}
function renderBroadcastEventCard(event) {
  const status = eventStatusLabel(event);
  const title = event.shortName || event.name || "Event";
  const poster = "<span>" + escapeHTML((event.categoryName || "Event").slice(0, 14)) + "</span>";
  const meta = [event.keyword ? "Matched: " + event.keyword : "", event.channels && event.channels.length ? event.channels.length + " channel" + (event.channels.length === 1 ? "" : "s") : ""].filter(Boolean).map(function(value) { return "<span>" + escapeHTML(value) + "</span>"; }).join("");
  return "<article class=\"sports-card" + (event.live ? " live" : "") + "\"><div class=\"sports-card-head\"><div class=\"sports-card-title\"><span class=\"sports-league-pill\">" + escapeHTML(event.categoryName || "Events") + "</span><strong data-overflow-tooltip=\"" + escapeHTML(event.name || title) + "\">" + escapeHTML(title) + "</strong></div><div class=\"sports-status\">" + escapeHTML(status) + "</div></div>"
    + "<div class=\"event-card-body\"><div class=\"event-poster\">" + poster + "</div><div class=\"event-details\"><p data-overflow-description=\"true\">" + escapeHTML(event.description || "No event details available.") + "</p><div class=\"event-meta\">" + meta + "</div></div></div>"
    + renderBroadcastEventChannels(event)
    + "</article>";
}
function eventStatusLabel(event) {
  if (event.live) return "Live";
  if (event.completed) return "Ended";
  if (event.startUnix) return sportsDateLabel(event.startUnix);
  return "Time TBD";
}
function renderBroadcastEventChannels(event) {
  const channels = items(event.channels);
  if (!channels.length) return "<div class=\"muted\">No matching channels.</div>";
  const expanded = !!state.expandedEvents[event.id];
  const visible = expanded ? channels : channels.slice(0, 3);
  const hiddenCount = channels.length - visible.length;
  const more = hiddenCount > 0 ? "<button class=\"sports-channel-more\" type=\"button\" data-event-expand=\"" + escapeHTML(event.id || "") + "\">+" + hiddenCount + " more</button>" : (expanded && channels.length > 3 ? "<button class=\"sports-channel-more\" type=\"button\" data-event-expand=\"" + escapeHTML(event.id || "") + "\">Show less</button>" : "");
  return "<div class=\"sports-channels\">" + visible.map(function(channel) {
    const meta = channel.categoryName || channel.reason || "Live TV";
    return "<div class=\"sports-channel-wrap\"><button class=\"sports-channel\" type=\"button\" data-channel=\"" + escapeHTML(channel.id) + "\" title=\"" + escapeHTML(channel.reason || meta) + "\"><strong data-overflow-tooltip=\"" + escapeHTML(channel.name || "Channel") + "\">" + escapeHTML(channel.name || "Channel") + "</strong><small>" + escapeHTML(meta) + "</small></button><button class=\"sports-channel-multiview\" type=\"button\" data-multiview-channel=\"" + escapeHTML(channel.id) + "\" aria-label=\"Add " + escapeHTML(channel.name || "channel") + " to multiview\" title=\"Add to multiview\">" + icon("multiview") + "<span>Multiview</span></button></div>";
  }).join("") + more + "</div>";
}
function setEventTab(tab) {
  state.eventsTab = tab || "live";
  state.expandedEvents = {};
  renderEventsPage();
}
function setEventCategory(categoryID) {
  state.eventCategory = categoryID || "";
  state.expandedEvents = {};
  renderEventsPage();
}
function toggleBroadcastEventChannels(eventID) {
  eventID = String(eventID || "");
  if (!eventID) return;
  if (state.expandedEvents[eventID]) delete state.expandedEvents[eventID];
  else state.expandedEvents[eventID] = true;
  renderEventsPage();
}
function favoriteCards(channels) {
  if (!channels.length) return "<div class=\"empty\">No favorite channels yet.</div>";
  return "<div class=\"row-scroll\">" + channels.map(function(channel, index) {
    const controls = favoriteMap()[channel.id] ? "<div class=\"settings-actions\"><button data-favorite-move=\"up\" data-channel-id=\"" + escapeHTML(channel.id) + "\"" + (index === 0 ? " disabled" : "") + ">Move up</button><button data-favorite-move=\"down\" data-channel-id=\"" + escapeHTML(channel.id) + "\"" + (index === channels.length - 1 ? " disabled" : "") + ">Move down</button></div>" : "";
    return "<div class=\"favorite-card\"><button class=\"continue-card\" data-channel=\"" + escapeHTML(channel.id) + "\"><div class=\"poster-box\">" + (channel.logoUrl ? "<img src=\"" + escapeHTML(channel.logoUrl) + "\" alt=\"\">" : "<span>" + escapeHTML((channel.name || "TV").slice(0, 5)) + "</span>") + "</div><strong>" + escapeHTML(channel.name || "Untitled") + "</strong><div class=\"muted\">" + escapeHTML(channel.categoryName || "Live TV") + "</div></button>" + controls + "</div>";
  }).join("") + "</div>";
}
function compareCategoryDisplayName(left, right) {
  const leftName = String((left && (left.name || left.id)) || "");
  const rightName = String((right && (right.name || right.id)) || "");
  return leftName.localeCompare(rightName, undefined, { numeric: true, sensitivity: "base" }) || leftName.localeCompare(rightName) || String((left && left.id) || "").localeCompare(String((right && right.id) || ""));
}
function categoryGrid() {
  const hidden = hiddenMap();
  const sourceCategories = sourceCategoriesWithChannels(function(channel) {
    return !(channel.categoryId && hidden[channel.categoryId]);
  });
  const custom = customGroupCategories();
  const listing = adminListingCategories("");
  const featured = sourceCategories.filter(function(category) { return !!category.featured; }).sort(compareCategoryDisplayName);
  const featuredSourceIDs = {};
  featured.forEach(function(category) { featuredSourceIDs[category.sourceID] = true; });
  const regularListing = listing.filter(function(category) {
    return !(category.kind === "source" && featuredSourceIDs[category.sourceID]);
  });
  const sections = [];
  if (featured.length) sections.push(categoryGridSection("Featured Groups", featured));
  if (custom.length) sections.push(categoryGridSection("My Groups", custom));
  if (regularListing.length) sections.push(categoryGridSection(adminListingTitle(), regularListing));
  if (!listing.length && sourceCategories.length) sections.push(categoryGridSection("Channel Groups", sourceCategories));
  return sections.length ? sections.join("") : "<div class=\"empty\">No groups yet.</div>";
}
function categoryGridSection(title, categories) {
  return sectionHeader(title) + "<div class=\"category-grid\">" + categories.map(categoryTileHTML).join("") + "</div>";
}
function categoryTileHTML(category) {
  const name = String((category && (category.name || category.id)) || "");
  const meta = String((category && category.count ? category.count + " channels" : (category && category.kind) || "") || "");
  return "<button class=\"tile" + (state.category === category.id ? " active" : "") + "\" data-category=\"" + escapeHTML(category.id) + "\" aria-label=\"" + escapeHTML(meta ? name + ", " + meta : name) + "\"><strong data-overflow-tooltip=\"" + escapeHTML(name) + "\">" + escapeHTML(name) + "</strong><span>" + escapeHTML(meta) + "</span></button>";
}
function activeVirtualCategoryID(path, featured) {
  return featured ? featuredCategoryID(path) : virtualCategoryID(path);
}
function virtualFolderHeader(path, featured) {
  return "<div class=\"section-title\">" + virtualFolderBreadcrumbs(path, featured) + "</div>";
}
function virtualFolderBreadcrumbs(path, featured) {
  const parts = path.split(" / ").filter(Boolean);
  const rootLabel = featured ? "Featured Groups" : "Virtual Groups";
  const crumbs = ["<button data-category=\"\">" + escapeHTML(rootLabel) + "</button>"];
  parts.forEach(function(part, index) {
    const crumbPath = parts.slice(0, index + 1).join(" / ");
    crumbs.push("<span class=\"sep\">/</span><button data-category=\"" + escapeHTML(activeVirtualCategoryID(crumbPath, featured)) + "\">" + escapeHTML(part) + "</button>");
  });
  return "<div class=\"breadcrumbs\" aria-label=\"Virtual folder breadcrumbs\">" + crumbs.join("") + "</div>";
}
function sourceCategoriesWithChannels(includeChannel) {
  const categoryCounts = {};
  effectiveChannels(false).forEach(function(channel) {
    if (includeChannel && !includeChannel(channel)) return;
    if (channel.categoryId) categoryCounts[channel.categoryId] = (categoryCounts[channel.categoryId] || 0) + 1;
  });
  return items(state.app.categories).filter(function(category) {
    return !!categoryCounts[category.id];
  }).map(function(category) {
    const rawName = category.name || category.id;
    const name = categoryDisplayName(rawName);
    const featured = categoryStartsFeatured(rawName);
    const featuredPath = featured ? featuredPathForSourceName(rawName) : "";
    return { id: featuredPath ? featuredCategoryID(featuredPath) : sourceCategoryID(category.id), sourceID: category.id, name: name, featured: featured, kind: featuredPath ? "featured" : "source", count: categoryCounts[category.id] || 0 };
  });
}
function customGroupCategories() {
  return customGroups().map(function(group) {
    return { id: customCategoryID(group.id), name: group.name, kind: "custom", count: customMemberships(group.id).filter(channelByID).length };
  }).filter(function(group) {
    return group.count > 0;
  });
}
function virtualGroupCategories(includeChannel) {
  return virtualCategoriesFromPaths("", includeChannel, true);
}
function virtualCategoriesFromPaths(parentPath, includeChannel, includeAllDescendants) {
  parentPath = String(parentPath || "");
  const groups = {};
  effectiveChannels(false).forEach(function(channel) {
    if (includeChannel && !includeChannel(channel)) return;
    virtualPathsForChannel(channel).forEach(function(path) {
      const parts = path.split(" / ").filter(Boolean);
      const parentParts = parentPath ? parentPath.split(" / ").filter(Boolean) : [];
      if (parts.length <= parentParts.length) return;
      for (let index = 0; index < parentParts.length; index++) {
        if (parts[index] !== parentParts[index]) return;
      }
      const limit = includeAllDescendants ? parts.length : parentParts.length + 1;
      for (let index = parentParts.length; index < limit; index++) {
        const childPath = parts.slice(0, index + 1).join(" / ");
        groups[childPath] = groups[childPath] || { id: virtualCategoryID(childPath), name: includeAllDescendants ? childPath : parts[index], kind: "virtual", count: 0, channelIDs: {} };
        groups[childPath].channelIDs[channel.id] = true;
      }
    });
  });
  return Object.keys(groups).sort().map(function(path) {
    const group = groups[path];
    group.count = Object.keys(group.channelIDs).length;
    delete group.channelIDs;
    return group;
  });
}
function sourceVirtualChildCategories(parentPath, includeChannel) {
  return childCategoriesFromChannelPaths(parentPath, includeChannel, virtualPathsForChannel, virtualCategoryID, "virtual");
}
function featuredChildCategories(parentPath, includeChannel) {
  return childCategoriesFromChannelPaths(parentPath, includeChannel, featuredPathsForChannel, featuredCategoryID, "featured");
}
function childCategoriesFromChannelPaths(parentPath, includeChannel, pathsForChannel, categoryIDForPath, kind) {
  parentPath = String(parentPath || "");
  const children = {};
  effectiveChannels(false).forEach(function(channel) {
    if (includeChannel && !includeChannel(channel)) return;
    pathsForChannel(channel).forEach(function(groupPath) {
      const parts = groupPath.split(" / ").filter(Boolean);
      for (let index = 0; index < parts.length; index++) {
        const path = parts.slice(0, index + 1).join(" / ");
        const parentParts = parentPath ? parentPath.split(" / ").filter(Boolean) : [];
        if (parts.length <= parentParts.length) return;
        for (let parentIndex = 0; parentIndex < parentParts.length; parentIndex++) {
          if (parts[parentIndex] !== parentParts[parentIndex]) return;
        }
        if (index !== parentParts.length) continue;
        children[path] = children[path] || { id: categoryIDForPath(path), name: parts[parentParts.length], kind: kind, count: 0, channelIDs: {} };
        children[path].channelIDs[channel.id] = true;
      }
    });
  });
  return Object.keys(children).sort().map(function(path) {
    const child = children[path];
    child.count = Object.keys(child.channelIDs || {}).length;
    delete child.channelIDs;
    return child;
  });
}
function virtualChildCategories(parentPath, includeChannel) {
  return sourceVirtualChildCategories(parentPath, includeChannel);
}
function allFilterCategories() {
  const hidden = hiddenMap();
  return customGroupCategories().concat(adminListingCategories("", function(channel) { return !(channel.categoryId && hidden[channel.categoryId]); }));
}
function adminListingTitle() {
  const mode = adminSettings().mode || "normal";
  if (mode === "delimiter") return "Virtual Groups";
  return "Channel Groups";
}
function adminListingCategories(parentPath, includeChannel) {
  const hidden = hiddenMap();
  includeChannel = includeChannel || function(channel) { return !(channel.categoryId && hidden[channel.categoryId]); };
  const mode = adminSettings().mode || "normal";
  if (mode === "delimiter") return parentPath ? sourceVirtualChildCategories(parentPath, includeChannel) : sourceVirtualChildCategories("", includeChannel);
  return sourceCategoriesWithChannels(includeChannel);
}
function virtualCategoriesActive() {
  const hidden = hiddenMap();
  return adminSettings().mode === "delimiter" && virtualGroupCategories(function(channel) { return !(channel.categoryId && hidden[channel.categoryId]); }).length > 0;
}
function recentChannels(limit) {
  const seen = {};
  const channels = [];
  items(prefs().recentChannels).forEach(function(id) {
    if (seen[id]) return;
    const channel = channelByID(id);
    if (!channel) return;
    seen[id] = true;
    channels.push(channel);
  });
  return channels.slice(0, limit || channels.length);
}
function renderHomeGuide(channels, emptyMessage) {
  if (!channels.length) return "<div class=\"empty\">" + escapeHTML(emptyMessage || "No recently watched channels yet.") + "</div>";
  const slots = guideSlots();
  return "<div class=\"home-guide guide-scroll\"><div class=\"guide-page guide-timeline\" style=\"" + guideTimelineStyle(slots) + "\"><div class=\"time-head\"><span>Today</span>" + slots.map(function(slot) { return "<span>" + escapeHTML(timeLabel(slot)) + "</span>"; }).join("") + "</div>" + channels.map(function(channel, channelIndex) {
    return "<div class=\"epg-row\">" + renderGuideChannelButton(channel) + "<div class=\"epg-programs\">" + renderEPGCells(channel, channelIndex) + "</div></div>";
  }).join("") + "</div></div>";
}
function renderVirtualCategoryGuide(channels) {
  return sectionActions(renderVirtualCategoryViewToggle()) + renderHomeGuide(channels, "No channels in this virtual group yet.");
}
function virtualCategoryView() {
  return state.virtualCategoryView === "list" ? "list" : "guide";
}
function renderVirtualCategoryViewToggle() {
  const active = virtualCategoryView();
  return "<div class=\"view-toggle\" aria-label=\"Virtual category view\"><button type=\"button\" data-virtual-category-view=\"guide\" class=\"" + (active === "guide" ? "active" : "") + "\" aria-pressed=\"" + (active === "guide" ? "true" : "false") + "\">Guide</button><button type=\"button\" data-virtual-category-view=\"list\" class=\"" + (active === "list" ? "active" : "") + "\" aria-pressed=\"" + (active === "list" ? "true" : "false") + "\">List</button></div>";
}
function renderVirtualCategoryChannelList(channels) {
  if (!channels.length) return sectionHeaderWithActions("Channels", renderVirtualCategoryViewToggle()) + "<div class=\"empty\">No channels in this virtual group yet.</div>";
  return sectionHeaderWithActions("Channels", renderVirtualCategoryViewToggle()) + "<div class=\"channel-button-list\">" + channels.map(function(channel) {
    const program = currentProgram(channel) || {};
    const subtitle = program.title || channel.categoryName || "Live TV";
    return "<button class=\"virtual-channel-button\" data-channel=\"" + escapeHTML(channel.id) + "\">" + logoHTML(channel) + "<span><strong>" + escapeHTML(channel.name || "Untitled") + "</strong><span>" + escapeHTML(subtitle) + "</span></span></button>";
  }).join("") + "</div>";
}
function renderVirtualCategoryContent(channels) {
  return virtualCategoryView() === "list" ? renderVirtualCategoryChannelList(channels) : renderVirtualCategoryGuide(channels);
}
function setVirtualCategoryView(view) {
  state.virtualCategoryView = view === "list" ? "list" : "guide";
  renderLivePage();
}
function renderLivePage() {
  const channels = visibleChannels(false);
  if (state.view === "favorites") {
    byId("view").innerHTML = sectionHeader("Favorite channels") + favoriteCards(channels.slice(0, 60));
    return;
  }
  if (state.category.indexOf("virtual:") === 0 || state.category.indexOf("featured:") === 0) {
    const featured = state.category.indexOf("featured:") === 0;
    const path = featured ? featuredCategoryPath(state.category) : virtualCategoryPath(state.category);
    const hidden = hiddenMap();
    const children = (featured ? featuredChildCategories : virtualChildCategories)(path, function(channel) {
      return !(channel.categoryId && hidden[channel.categoryId]);
    });
    byId("view").innerHTML = virtualFolderHeader(path, featured)
      + (children.length ? "<div class=\"category-grid\">" + children.map(categoryTileHTML).join("") + "</div>" : "")
      + renderVirtualCategoryContent(channels);
    return;
  }
  byId("view").innerHTML = categoryGrid() + sectionHeader(categoryName(state.category) || "Channels") + rowCards(channels.slice(0, 24));
}
function recordingCustom(recording) {
  return recording && recording.custom_properties && typeof recording.custom_properties === "object" ? recording.custom_properties : {};
}
function recordingProgram(recording) {
  const custom = recordingCustom(recording);
  return custom.program && typeof custom.program === "object" ? custom.program : {};
}
function recordingStatus(recording) {
  const custom = recordingCustom(recording);
  const now = Date.now();
  const start = Date.parse(recording.start_time || custom.start_time || "");
  const end = Date.parse(recording.end_time || custom.end_time || "");
  if (custom.status) return String(custom.status);
  if (!Number.isNaN(start) && start > now) return "upcoming";
  if (!Number.isNaN(start) && !Number.isNaN(end) && start <= now && end >= now) return "recording";
  return "completed";
}
function recordingTitle(recording) {
  const custom = recordingCustom(recording);
  const program = recordingProgram(recording);
  return custom.title || program.title || custom.file_name || "Untitled recording";
}
function recordingChannelName(recording) {
  const custom = recordingCustom(recording);
  const program = recordingProgram(recording);
  return custom.channel_name || program.channel || program.channel_name || "Dispatcharr";
}
function recordingTimeLabel(value) {
  const date = new Date(value || "");
  if (Number.isNaN(date.getTime())) return "";
  return date.toLocaleString([], { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" });
}
function recordingWindow(recording) {
  const start = recordingTimeLabel(recording.start_time);
  const end = recordingTimeLabel(recording.end_time);
  if (start && end) return start + " - " + end;
  return start || end || "Time unavailable";
}
function normalizeRecordings(payload) {
  if (!payload || !payload.available) return [];
  return items(payload.items).slice().sort(function(a, b) {
    const aTime = Date.parse(a.start_time || "");
    const bTime = Date.parse(b.start_time || "");
    return (Number.isNaN(bTime) ? 0 : bTime) - (Number.isNaN(aTime) ? 0 : aTime);
  });
}
function recordingPlaybackURL(recording) {
  const silo = recording && recording._silo ? recording._silo : {};
  return silo.playback_url || "";
}
function recordingMatchesQuery(recording) {
  if (!state.query) return true;
  const haystack = [recordingTitle(recording), recordingChannelName(recording), recordingStatus(recording)].join(" ").toLowerCase();
  return haystack.indexOf(lower(state.query)) !== -1;
}
function renderRecordingCard(recording) {
  const status = recordingStatus(recording).toLowerCase();
  const playbackURL = recordingPlaybackURL(recording);
  const action = playbackURL ? "<button class=\"recording-action\" data-recording-playback=\"" + escapeHTML(playbackURL) + "\">" + icon("play") + "<span>Playback</span></button>" : "";
  return "<div class=\"recording-card\"><span><strong>" + escapeHTML(recordingTitle(recording)) + "</strong><span class=\"recording-meta\">" + escapeHTML(recordingChannelName(recording) + " - " + recordingWindow(recording)) + "</span></span><div class=\"recording-actions\">" + action + "<span class=\"recording-badge " + escapeHTML(status) + "\">" + escapeHTML(status.split("_").join(" ")) + "</span></div></div>";
}
function renderRecordingSection(title, recordings) {
  if (!recordings.length) return "";
  return sectionHeader(title) + "<div class=\"recording-list\">" + recordings.map(renderRecordingCard).join("") + "</div>";
}
function loadRecordings(force) {
  if (!dvrEnabled()) {
    state.recordings = { available: false, reason: "Recordings require Dispatcharr Direct Connect.", items: [] };
    return;
  }
  if (state.recordingsLoading || (state.recordings && !force)) return;
  state.recordingsLoading = true;
  getJSON("/dispatcharr/api/recordings").then(function(payload) {
    state.recordings = payload;
  }).catch(function(error) {
    state.recordings = { available: false, reason: "Unable to load Dispatcharr recordings.", items: [] };
  }).finally(function() {
    state.recordingsLoading = false;
    if (state.view === "recordings") render();
  });
}
function programByID(channelID, programID) {
  return programsFor(channelID).find(function(program) { return String(program.id || "") === String(programID || ""); }) || null;
}
function scheduleProgram(channelID, programID, button) {
  if (!dvrEnabled()) {
    showAppToast("Recordings require Dispatcharr Direct Connect.");
    return;
  }
  const channel = channelByID(channelID);
  const program = programByID(channelID, programID);
  if (!channel || !program) {
    showAppToast("Could not find that guide entry.");
    return;
  }
  if (button) button.disabled = true;
  postJSON("/dispatcharr/api/recordings", {
    channelId: channel.id,
    programId: program.id || "",
    title: program.title || channel.name || "Recording",
    description: program.description || "",
    startUnix: program.startUnix || 0,
    endUnix: program.endUnix || 0
  }).then(function() {
    state.recordings = null;
    loadRecordings(true);
    showAppToast("Recording scheduled in Dispatcharr.");
  }).catch(function() {
    showAppToast("Dispatcharr could not schedule that recording.");
  }).finally(function() {
    if (button) button.disabled = false;
  });
}
function renderRecordingsPage() {
  const root = byId("view");
  if (!state.recordings) {
    root.innerHTML = sectionHeader("Recordings") + "<div class=\"empty\">Loading Dispatcharr recordings...</div>";
    loadRecordings(false);
    return;
  }
  const toolbar = "<div class=\"recording-toolbar\"><button class=\"recording-refresh\" data-recordings-refresh=\"true\">Refresh recordings</button></div>";
  if (!state.recordings.available) {
    root.innerHTML = toolbar + sectionHeader("Recordings") + "<div class=\"empty\">" + escapeHTML(state.recordings.reason || "Recordings are not available for this connection mode.") + "</div>";
    return;
  }
  const recordings = normalizeRecordings(state.recordings).filter(recordingMatchesQuery);
  const active = recordings.filter(function(recording) { return recordingStatus(recording).toLowerCase() === "recording"; });
  const upcoming = recordings.filter(function(recording) { return recordingStatus(recording).toLowerCase() === "upcoming"; });
  const completed = recordings.filter(function(recording) {
    const status = recordingStatus(recording).toLowerCase();
    return status !== "recording" && status !== "upcoming";
  });
  root.innerHTML = toolbar
    + renderRecordingSection("Recording now", active)
    + renderRecordingSection("Upcoming", upcoming)
    + renderRecordingSection("Completed", completed.slice(0, 80))
    + (!recordings.length ? "<div class=\"empty\">No Dispatcharr recordings found.</div>" : "");
}
function currentProgram(channel) {
  if (!channel) return null;
  const now = Math.floor(Date.now() / 1000);
  return programsFor(channel.id).find(function(program) {
    return (!program.startUnix || program.startUnix <= now + 600) && (!program.endUnix || program.endUnix >= now);
  }) || programsFor(channel.id)[0] || null;
}
function playerLogoHTML(channel) {
  if (channel && channel.logoUrl) return "<img class=\"player-logo\" src=\"" + escapeHTML(channel.logoUrl) + "\" alt=\"\">";
  return "<div class=\"player-logo player-logo-fallback\">" + escapeHTML(((channel && channel.name) || "TV").slice(0, 5)) + "</div>";
}
function playerFavoriteButtonHTML(channel) {
  const isFavorite = !!(channel && favoriteMap()[channel.id]);
  return "<button id=\"player-favorite-button\" class=\"player-icon favorite" + (isFavorite ? " active" : "") + "\" data-player-action=\"favorite\" aria-label=\"" + (isFavorite ? "Remove channel from favorites" : "Favorite channel") + "\" aria-pressed=\"" + (isFavorite ? "true" : "false") + "\">" + icon(isFavorite ? "heart-solid" : "heart") + "</button>";
}
function renderPlayerPage() {
  const channel = state.currentChannel || visibleChannels(false)[0] || null;
  const program = currentProgram(channel) || {};
  const channelName = channel ? channel.name || "Untitled channel" : "Choose a channel";
  const categoryNameText = channel ? channel.categoryName || "Live TV" : "Live TV";
  const replayMode = isRewindableChannel(channel);
  const title = program.title || channelName;
  const description = program.description || categoryNameText;
  const start = timeLabel(program.startUnix) || "LIVE";
  const end = timeLabel(program.endUnix) || "Now";
  const playbackShellClass = replayMode ? "playback-shell is-replay" : "playback-shell";
  const videoAttributes = replayMode ? " autoplay playsinline controls" : " autoplay playsinline";
  const modeTag = replayMode ? "Replay" : "AV";
  const timelineEnd = replayMode ? escapeHTML(end) : "<span class=\"live-dot\"></span>LIVE&nbsp;&nbsp;" + escapeHTML(end);
  byId("view").innerHTML = "<section class=\"" + playbackShellClass + "\"><div class=\"playback-stage\"><video id=\"player\" class=\"playback-video\"" + videoAttributes + "></video><div class=\"playback-scrim\"></div><button id=\"player-center-button\" class=\"player-center-button hidden\" data-player-action=\"play-toggle\" aria-label=\"Play\">" + icon("play") + "</button><div class=\"player-top\"><button class=\"player-exit\" data-player-action=\"back\" aria-label=\"Back to Live TV browse\"><span class=\"player-icon\">" + icon("x") + "</span><span>Exit</span></button><div class=\"player-top-actions\"><div class=\"player-audio\"><button id=\"player-audio-button\" class=\"player-chip\" data-player-action=\"audio-menu\" aria-haspopup=\"true\" aria-expanded=\"false\">" + icon("language") + "<span>Audio</span>" + icon("chevron-down") + "</button><div id=\"player-audio-menu\" class=\"player-menu\" role=\"menu\"></div></div><div class=\"player-volume\"><button id=\"player-volume-button\" class=\"player-icon\" data-player-action=\"volume-menu\" aria-label=\"Volume\" aria-haspopup=\"true\" aria-expanded=\"false\">" + icon("speaker") + "</button><div id=\"player-volume-popover\" class=\"volume-popover\"><span>VOL</span><input id=\"player-volume-slider\" type=\"range\" min=\"0\" max=\"100\" step=\"1\" value=\"" + Math.round(state.volume * 100) + "\" aria-label=\"Volume\"><span id=\"player-volume-value\" class=\"volume-value\"></span></div></div><button class=\"player-icon\" data-player-action=\"cast\" aria-label=\"AirPlay or Cast\">" + icon("airplay") + "</button><button id=\"player-guide-button\" class=\"player-icon player-guide-button\" data-player-action=\"guide\" aria-label=\"Guide\" aria-haspopup=\"true\" aria-expanded=\"false\">" + icon("guide") + "</button><button id=\"player-fullscreen-button\" class=\"player-icon\" data-player-action=\"fullscreen\" aria-label=\"Fullscreen\" aria-pressed=\"false\">" + icon("fullscreen") + "</button><div class=\"player-more\"><button id=\"player-more-button\" class=\"player-icon\" data-player-action=\"more\" aria-label=\"More\" aria-haspopup=\"true\" aria-expanded=\"false\">" + icon("ellipsis") + "</button><div id=\"player-more-menu\" class=\"player-more-menu\"></div></div></div></div><div id=\"player-toast\" class=\"player-toast\" role=\"status\"></div><div id=\"player-guide-panel\" class=\"player-guide-panel\"></div><div class=\"player-bottom\"><div class=\"player-bottom-row\"><div class=\"player-meta\">" + playerLogoHTML(channel) + "<div class=\"player-kicker\">" + escapeHTML(channelName) + "</div><h2 class=\"player-title\">" + escapeHTML(title) + "</h2><p class=\"player-description\" data-overflow-description=\"true\">" + escapeHTML(description) + "</p><div class=\"player-tags\"><span class=\"player-tag\">" + escapeHTML(categoryNameText) + "</span><span class=\"player-tag\">" + escapeHTML(modeTag) + "</span></div></div><div class=\"player-bottom-actions\">" + playerFavoriteButtonHTML(channel) + "<button class=\"player-icon\" data-player-action=\"pip\" aria-label=\"Picture in Picture\">" + icon("pip") + "</button><button id=\"player-subtitles-button\" class=\"player-icon\" data-player-action=\"subtitles\" aria-label=\"Subtitles\" aria-pressed=\"false\">" + icon("captions") + "</button><button id=\"player-language-button\" class=\"player-icon\" data-player-action=\"language-menu\" aria-label=\"Audio language\" aria-haspopup=\"true\" aria-expanded=\"false\">" + icon("language") + "</button></div></div><div class=\"timeline\"><span>" + escapeHTML(start) + "</span><div class=\"timeline-bar\"><div class=\"timeline-fill\"></div><div class=\"timeline-knob\"></div></div><span>" + timelineEnd + "</span></div></div></div></section>";
  updateAudioMenu();
  updateVolumeMenu();
  renderPlayerGuidePanel();
  renderPlayerMoreMenu();
  updateFullscreenButton();
  wakePlayerChrome(1800);
}
function renderMultiviewPage() {
  resetMultiviewMedia();
  const tiles = items(state.multiviewTiles).filter(function(tile) {
    tile.channel = channelByID((tile.channel || {}).id || tile.channelID) || tile.channel;
    return tile.channel;
  }).slice(0, 4);
  state.multiviewTiles = tiles;
  if (!state.multiviewActiveTileID && tiles[0]) state.multiviewActiveTileID = tiles[0].id;
  if (state.multiviewActiveTileID && !tiles.some(function(tile) { return tile.id === state.multiviewActiveTileID; })) state.multiviewActiveTileID = tiles[0] ? tiles[0].id : "";
  const countClass = "count-" + Math.max(tiles.length, 1);
  const title = tiles.length ? tiles.length + " channel" + (tiles.length === 1 ? "" : "s") : "Choose channels";
  byId("view").innerHTML = "<section class=\"multiview-page\"><div class=\"multiview-toolbar\"><div><h2>Multiview</h2><p>" + escapeHTML(title) + " · focused tile owns audio</p></div><div class=\"multiview-actions\">" + (tiles.length ? "<button class=\"chip\" type=\"button\" data-multiview-action=\"clear\">Clear</button>" : "") + "</div></div>"
    + (tiles.length ? "<div class=\"multiview-grid " + countClass + "\">" + tiles.map(renderMultiviewTile).join("") + "</div>" : renderMultiviewEmpty())
    + "</section>";
  attachMultiviewPlayers();
}
function renderMultiviewTile(tile) {
  const channel = tile.channel || {};
  const active = tile.id === state.multiviewActiveTileID;
  const program = currentProgram(channel) || {};
  const title = program.title || channel.name || "Live TV";
  const subtitle = channel.categoryName || "Live TV";
  const muted = active ? "Audio" : "Muted";
  return "<article class=\"multiview-tile" + (active ? " active" : "") + "\" data-multiview-tile=\"" + escapeHTML(tile.id) + "\" data-multiview-focus=\"" + escapeHTML(tile.id) + "\"><video id=\"" + escapeHTML(tile.videoID) + "\" class=\"multiview-video\" autoplay playsinline" + (active ? "" : " muted") + "></video><div class=\"multiview-tile-controls\"><button type=\"button\" data-multiview-action=\"focus\" data-multiview-tile-id=\"" + escapeHTML(tile.id) + "\" aria-label=\"Use audio from this tile\">" + icon("speaker") + "</button><button type=\"button\" data-multiview-action=\"single\" data-multiview-tile-id=\"" + escapeHTML(tile.id) + "\" aria-label=\"Open channel player\">" + icon("external") + "</button><button type=\"button\" data-multiview-action=\"remove\" data-multiview-tile-id=\"" + escapeHTML(tile.id) + "\" aria-label=\"Remove from multiview\">" + icon("x") + "</button></div><div class=\"multiview-tile-meta\"><div><strong data-overflow-tooltip=\"" + escapeHTML(title) + "\">" + escapeHTML(title) + "</strong><small data-overflow-tooltip=\"" + escapeHTML(channel.name || subtitle) + "\">" + escapeHTML(channel.name || subtitle) + "</small></div><span class=\"multiview-audio-badge\">" + escapeHTML(muted) + "</span></div></article>";
}
function renderMultiviewEmpty() {
  const picks = recentChannels(8).concat(visibleChannels(false).slice(0, 8)).filter(Boolean);
  const unique = [];
  const seen = {};
  picks.forEach(function(channel) {
    if (!channel || seen[channel.id]) return;
    seen[channel.id] = true;
    unique.push(channel);
  });
  return "<div class=\"multiview-empty\"><div class=\"empty\">Add up to four live channels. The active tile is the only one with audio.</div><div class=\"multiview-channel-grid\">" + unique.slice(0, 12).map(function(channel) {
    return "<button class=\"multiview-channel-add\" type=\"button\" data-multiview-channel=\"" + escapeHTML(channel.id) + "\">" + logoHTML(channel) + "<span><strong>" + escapeHTML(channel.name || "Untitled") + "</strong><small>" + escapeHTML(channel.categoryName || "Live TV") + "</small></span></button>";
  }).join("") + "</div></div>";
}
function attachMultiviewPlayers() {
  items(state.multiviewTiles).forEach(function(tile) {
    const video = byId(tile.videoID);
    if (!video || tile.attached || !tile.channel) return;
    const attachment = attachVideoSource(video, browserStreamURL(tile.channel), { rewindable: isRewindableChannel(tile.channel) });
    tile.hls = attachment.hls;
    tile.tsPlayer = attachment.tsPlayer;
    tile.attached = true;
    video.addEventListener("click", function() { focusMultiviewTile(tile.id); });
    video.addEventListener("dblclick", function() { openMultiviewTileSingle(tile.id); });
    video.play().catch(function() {});
    startMultiviewWatch(tile);
  });
  syncMultiviewAudio();
}
function addChannelToMultiview(channel) {
  if (!channel) return;
  if (state.view === "player") {
    stopPlayback();
    stopCurrentWatch("open_multiview");
  }
  const existing = state.multiviewTiles.find(function(tile) { return tile.channel && tile.channel.id === channel.id; });
  if (existing) {
    state.multiviewActiveTileID = existing.id;
    state.view = "multiview";
    render();
    return;
  }
  if (state.multiviewTiles.length >= 4) {
    showAppToast("Multiview supports up to four channels.");
    state.view = "multiview";
    render();
    return;
  }
  const tile = { id: multiviewTileKey(channel.id), channelID: channel.id, channel: channel, videoID: multiviewTileKey("video-" + channel.id), hls: null, tsPlayer: null, session: null, attached: false };
  state.multiviewTiles.push(tile);
  state.multiviewActiveTileID = tile.id;
  state.view = "multiview";
  render();
}
function focusMultiviewTile(tileID) {
  if (!multiviewTileByID(tileID)) return;
  state.multiviewActiveTileID = tileID;
  syncMultiviewAudio();
}
function removeMultiviewTile(tileID) {
  const tile = multiviewTileByID(tileID);
  if (!tile) return;
  destroyMultiviewMedia(tile);
  stopMultiviewWatch(tile, "remove_multiview_tile");
  state.multiviewTiles = state.multiviewTiles.filter(function(item) { return item.id !== tileID; });
  if (state.multiviewActiveTileID === tileID) state.multiviewActiveTileID = state.multiviewTiles[0] ? state.multiviewTiles[0].id : "";
  renderMultiviewPage();
}
function openMultiviewTileSingle(tileID) {
  const tile = multiviewTileByID(tileID);
  if (!tile || !tile.channel) return;
  stopAllMultiview("open_single_player");
  playChannel(tile.channel);
}
function handleMultiviewAction(action, tileID) {
  if (action === "clear") {
    stopAllMultiview("clear_multiview");
    renderMultiviewPage();
    return;
  }
  if (action === "focus") focusMultiviewTile(tileID);
  if (action === "remove") removeMultiviewTile(tileID);
  if (action === "single") openMultiviewTileSingle(tileID);
}
function hasOpenPlayerOverlay() {
  return state.audioMenuOpen || state.volumeMenuOpen || state.moreMenuOpen || state.playerGuideOpen;
}
function updatePlayerChrome() {
  const shell = document.querySelector(".playback-shell");
  if (!shell) return;
  shell.classList.toggle("is-idle", state.playerChromeIdle && !hasOpenPlayerOverlay());
}
function wakePlayerChrome(delay) {
  if (state.view !== "player") return;
  state.playerChromeIdle = false;
  updatePlayerChrome();
  if (state.playerChromeTimer) clearTimeout(state.playerChromeTimer);
  state.playerChromeTimer = setTimeout(function() {
    state.playerChromeIdle = true;
    updatePlayerChrome();
  }, delay || 2400);
}
function renderPlayerGuidePanel() {
  const panel = byId("player-guide-panel");
  const button = byId("player-guide-button");
  if (!panel) return;
  const channels = visibleChannels(true).slice(0, 42);
  panel.classList.toggle("open", state.playerGuideOpen);
  if (button) {
    button.classList.toggle("active", state.playerGuideOpen);
    button.setAttribute("aria-expanded", state.playerGuideOpen ? "true" : "false");
  }
  updatePlayerChrome();
  if (!state.playerGuideOpen) return;
  panel.innerHTML = "<div class=\"player-guide-head\"><div><strong>Channel Guide</strong><span>" + escapeHTML(categoryName(state.category) || "Live TV") + "</span></div><button class=\"player-icon\" data-player-action=\"guide-close\" aria-label=\"Close guide\">" + icon("x") + "</button></div><div class=\"player-guide-list\">" + channels.map(function(channel) {
    const program = currentProgram(channel) || {};
    const title = program.title || "Data not available";
    const time = timeLabel(program.startUnix) || "Live";
    return "<button class=\"player-guide-row" + (state.currentChannel && state.currentChannel.id === channel.id ? " active" : "") + "\" data-channel=\"" + escapeHTML(channel.id) + "\">" + logoHTML(channel) + "<span><strong>" + escapeHTML(channel.name || "Untitled") + "</strong><small>" + escapeHTML(time + " - " + title) + "</small></span></button>";
  }).join("") + "</div>";
}
function currentStreamURL() {
  return state.currentChannel ? route("/dispatcharr/stream?channel_id=" + encodeURIComponent(state.currentChannel.id)) : "";
}
function browserStreamURL(channel) {
  return route("/dispatcharr/stream?channel_id=" + encodeURIComponent(channel.id) + "&output_profile=2");
}
function applyAspectMode() {
  const video = byId("player");
  if (video) video.style.objectFit = state.aspectMode === "fit" ? "contain" : "cover";
}
function attachVideoSource(video, url, options) {
  const rewindable = !!(options && options.rewindable);
  const attachment = {
    hls: null,
    tsPlayer: null,
    destroy: function() {
      if (attachment.hls) { attachment.hls.destroy(); attachment.hls = null; }
      if (attachment.tsPlayer) { attachment.tsPlayer.destroy(); attachment.tsPlayer = null; }
      if (video) {
        video.pause();
        video.removeAttribute("src");
        video.load();
      }
    }
  };
  const isHLS = url.indexOf(".m3u8") !== -1;
  if (window.Hls && Hls.isSupported() && isHLS) {
    attachment.hls = new Hls();
    attachment.hls.loadSource(url);
    attachment.hls.attachMedia(video);
  } else if (window.mpegts && mpegts.isSupported() && !isHLS) {
    attachment.tsPlayer = mpegts.createPlayer({ type: "mpegts", isLive: !rewindable, url: url });
    attachment.tsPlayer.attachMediaElement(video);
    attachment.tsPlayer.load();
  } else {
    video.src = url;
  }
  return attachment;
}
function renderPlayerMoreMenu() {
  const button = byId("player-more-button");
  const menu = byId("player-more-menu");
  if (!menu) return;
  if (button) button.setAttribute("aria-expanded", state.moreMenuOpen ? "true" : "false");
  menu.classList.toggle("open", state.moreMenuOpen);
  updatePlayerChrome();
  if (!state.moreMenuOpen) return;
  const recent = items(prefs().recentChannels).map(channelByID).filter(Boolean).filter(function(channel) {
    return !state.currentChannel || channel.id !== state.currentChannel.id;
  }).slice(0, 3);
  menu.innerHTML = "<div class=\"player-more-kicker\">Video settings & controls</div>"
    + "<button data-player-action=\"aspect\">" + menuIcon("aspect") + "<span>Aspect ratio<small>" + (state.aspectMode === "fit" ? "Fit to screen" : "Fill screen") + "</small></span></button>"
    + "<button data-player-action=\"fullscreen\">" + menuIcon(document.fullscreenElement ? "fullscreen-exit" : "fullscreen") + "<span>Fullscreen<small>" + (document.fullscreenElement ? "Exit player fullscreen" : "Fill the display") + "</small></span></button>"
    + "<button data-player-action=\"guide\">" + menuIcon("guide") + "<span>Channel guide<small>Browse channels without leaving playback</small></span></button>"
    + "<button data-player-action=\"add-multiview\">" + menuIcon("multiview") + "<span>Add to multiview<small>Tile this channel with up to three more</small></span></button>"
    + "<button data-player-action=\"search-channel\">" + menuIcon("search") + "<span>Search channel<small>Jump to the channel list search</small></span></button>"
    + (recent.length ? "<div class=\"player-more-separator\"></div><div class=\"player-more-kicker\">Channels history</div>" + recent.map(function(channel) { return "<button data-channel=\"" + escapeHTML(channel.id) + "\">" + logoHTML(channel) + "<span>" + escapeHTML(channel.name || "Untitled") + "<small>" + escapeHTML(channel.categoryName || "Live TV") + "</small></span></button>"; }).join("") : "")
    + "<div class=\"player-more-separator\"></div><div class=\"player-more-kicker\">Video & audio casting</div>"
    + "<button data-player-action=\"cast\">" + menuIcon("airplay") + "<span>AirPlay or Cast<small>Use browser playback target picker</small></span></button>"
    + "<button data-player-action=\"copy-stream\">" + menuIcon("copy") + "<span>Copy stream URL<small>For an external player</small></span></button>"
    + "<button data-player-action=\"open-stream\">" + menuIcon("external") + "<span>Use external video player<small>Open the stream route in a new tab</small></span></button>";
}
function overflowTooltip() {
  let tooltip = byId("overflow-tooltip");
  if (tooltip) return tooltip;
  tooltip = document.createElement("div");
  tooltip.id = "overflow-tooltip";
  tooltip.className = "overflow-tooltip";
  tooltip.setAttribute("role", "tooltip");
  document.body.appendChild(tooltip);
  return tooltip;
}
function overflowTooltipTarget(event) {
  if (!event.target || !event.target.closest) return null;
  return event.target.closest("[data-overflow-tooltip], [data-overflow-description]");
}
function descriptionOverflows(target) {
  if (!target) return false;
  return target.scrollWidth > target.clientWidth + 1 || target.scrollHeight > target.clientHeight + 1;
}
function positionOverflowTooltip(tooltip, target, event) {
  const padding = 12;
  const gap = 8;
  const rect = target.getBoundingClientRect();
  const anchorX = event && typeof event.clientX === "number" ? event.clientX : rect.left + rect.width / 2;
  const width = tooltip.offsetWidth;
  const height = tooltip.offsetHeight;
  const maxLeft = Math.max(padding, window.innerWidth - width - padding);
  const left = Math.min(Math.max(anchorX - width / 2, padding), maxLeft);
  let top = rect.top - height - gap;
  if (top < padding) top = rect.bottom + gap;
  tooltip.style.left = left + "px";
  tooltip.style.top = Math.min(top, Math.max(padding, window.innerHeight - height - padding)) + "px";
}
function showOverflowTooltip(target, event) {
  if (!descriptionOverflows(target)) return;
  const description = target ? String(target.getAttribute("data-overflow-tooltip") || target.textContent || "").trim() : "";
  if (!description) return;
  const tooltip = overflowTooltip();
  tooltip.textContent = description;
  tooltip.classList.add("visible");
  positionOverflowTooltip(tooltip, target, event);
}
function hideOverflowTooltip() {
  const tooltip = byId("overflow-tooltip");
  if (tooltip) tooltip.classList.remove("visible");
}
function renderGuidePage() {
  const categories = allFilterCategories();
  const slots = guideSlots();
  byId("view").innerHTML = "<div class=\"guide-page\"><div class=\"guide-tools\"><select id=\"category-select\" class=\"select\"><option value=\"\">All groups</option>" + categories.map(function(category) { return "<option value=\"" + escapeHTML(category.id) + "\"" + (state.category === category.id ? " selected" : "") + ">" + escapeHTML(category.name || category.id) + "</option>"; }).join("") + "</select><button id=\"guide-inline-refresh\" class=\"refresh-button\" type=\"button\" data-guide-refresh=\"true\" aria-label=\"Refresh guide\" title=\"Refresh guide\"><svg viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" aria-hidden=\"true\"><path stroke-linecap=\"round\" stroke-linejoin=\"round\" d=\"M20 12a8 8 0 0 1-14.1 5.15M4 12A8 8 0 0 1 18.1 6.85\"/><path stroke-linecap=\"round\" stroke-linejoin=\"round\" d=\"M6 17.25H3.75V19.5M18 6.75h2.25V4.5\"/></svg></button><input id=\"guide-search\" class=\"search\" placeholder=\"Search by program or channel\" value=\"" + escapeHTML(state.query) + "\"></div><div class=\"guide-scroll\"><div class=\"guide-timeline\" style=\"" + guideTimelineStyle(slots) + "\"><div class=\"time-head\"><span>Today</span>" + slots.map(function(slot) { return "<span>" + escapeHTML(timeLabel(slot)) + "</span>"; }).join("") + "</div><div id=\"epg\"></div></div></div></div>";
  byId("category-select").onchange = function(event) { state.category = event.target.value; renderGuidePage(); };
  byId("guide-search").oninput = function(event) { state.query = event.target.value; resetGuideRows(); renderEPG(); };
  resetGuideRows();
  renderEPG();
}
function guideBatchSize() { return 40; }
function resetGuideRows() {
  state.guideChannels = visibleChannels(true).filter(guideChannelMatchesQuery);
  state.guideRendered = 0;
  state.guideLoading = false;
}
function renderEPGCells(channel, channelIndex) {
  const windowInfo = guideWindow();
  const windowStart = windowInfo.start;
  const windowEnd = windowInfo.end;
  const now = Math.floor(Date.now() / 1000);
  const channelMatched = channelMatchesQuery(channel);
  const programs = programsFor(channel.id).map(function(program) {
    const rawStart = program.startUnix || windowStart;
    const rawEnd = program.endUnix || rawStart + 1800;
    return {
      program: program,
      start: Math.max(rawStart, windowStart),
      end: Math.min(rawEnd, windowEnd),
      matchesQuery: channelMatched || programMatchesQuery(program)
    };
  }).filter(function(entry) {
    return entry.matchesQuery && entry.end > windowStart && entry.start < windowEnd;
  }).sort(function(a, b) {
    return a.start - b.start || a.end - b.end;
  });
  if (!programs.length) {
    return renderEPGGapCell(channel, windowStart, windowEnd, windowInfo);
  }
  const cells = [];
  let cursor = windowStart;
  programs.forEach(function(entry, index) {
    const program = entry.program;
    const start = Math.max(entry.start, cursor);
    const end = entry.end;
    if (end <= start) return;
    if (start > cursor) cells.push(renderEPGGapCell(channel, cursor, start, windowInfo));
    const canSchedule = dvrEnabled() && (program.endUnix || 0) > now;
    const programTitle = program.title || "Data not available";
    const programTime = epgVisibleTime(start, windowStart);
    cells.push("<div class=\"epg-cell program " + colorClass(index + channelIndex) + "\" style=\"" + epgCellStyle(start, end, windowInfo) + "\"><button class=\"epg-play\" data-channel=\"" + escapeHTML(channel.id) + "\" aria-label=\"" + escapeHTML(programTime + " " + programTitle) + "\"><time>" + escapeHTML(programTime) + "</time><strong data-overflow-tooltip=\"" + escapeHTML(programTime + " " + programTitle) + "\">" + escapeHTML(programTitle) + "</strong></button>" + (canSchedule ? "<button class=\"epg-schedule\" data-schedule-channel=\"" + escapeHTML(channel.id) + "\" data-schedule-program=\"" + escapeHTML(program.id || "") + "\" aria-label=\"Schedule recording\">" + icon("record") + "</button>" : "") + "</div>");
    cursor = end;
  });
  if (cursor < windowEnd) cells.push(renderEPGGapCell(channel, cursor, windowEnd, windowInfo));
  return cells.join("");
}
function epgVisibleTime(startUnix, windowStart) {
  return timeLabel(Math.max(startUnix || windowStart, windowStart));
}
function renderEPGGapCell(channel, startUnix, endUnix, windowInfo) {
  if (endUnix <= startUnix) return "";
  const emptyTitle = "Data not available";
  const emptyTime = timeLabel(startUnix);
  return "<button class=\"epg-cell program gray epg-gap\" data-channel=\"" + escapeHTML(channel.id) + "\" aria-label=\"" + escapeHTML(emptyTime + " " + emptyTitle) + "\" style=\"" + epgCellStyle(startUnix, endUnix, windowInfo) + "\"><time>" + escapeHTML(emptyTime) + "</time><strong>" + escapeHTML(emptyTitle) + "</strong></button>";
}
function renderEPGRow(channel, channelIndex) {
  return "<div class=\"epg-row\">" + renderGuideChannelButton(channel) + "<div class=\"epg-programs\">" + renderEPGCells(channel, channelIndex) + "</div></div>";
}
function renderEPG() {
  const root = byId("epg");
  root.innerHTML = "";
  appendGuideRows();
}
function appendGuideRows() {
  if (state.view !== "guide" || state.guideLoading) return;
  const root = byId("epg");
  if (!root) return;
  if (!state.guideChannels.length) {
    root.innerHTML = "<div class=\"empty\">No guide matches.</div>";
    return;
  }
  if (state.guideRendered >= state.guideChannels.length) return;
  state.guideLoading = true;
  const start = state.guideRendered;
  const end = Math.min(start + guideBatchSize(), state.guideChannels.length);
  const rows = state.guideChannels.slice(start, end).map(function(channel, offset) {
    return renderEPGRow(channel, start + offset);
  }).join("");
  root.insertAdjacentHTML("beforeend", rows);
  state.guideRendered = end;
  state.guideLoading = false;
  if (isNearGuideEnd()) appendGuideRows();
}
function isNearGuideEnd() {
  return window.innerHeight + window.scrollY > document.documentElement.scrollHeight - 900;
}
function renderSettings() {
  ensureSelectedCustomGroup();
  const showSourceCategorySettings = !virtualCategoriesActive();
  byId("view").innerHTML = "<div class=\"settings-stack\">"
    + "<div class=\"settings-card\"><h2>Custom groups</h2><div id=\"custom-group-settings\" class=\"settings-list\"></div></div>"
    + (showSourceCategorySettings ? "<div class=\"settings-card\"><h2>Hidden channel groups</h2><div id=\"settings-list\" class=\"settings-list\"></div></div>" : "")
    + "</div>";
  renderCustomGroupSettings();
  if (!showSourceCategorySettings) return;
  const root = byId("settings-list");
  const categories = sourceCategoriesWithChannels();
  root.innerHTML = categories.map(function(category) {
    return "<label><span>" + escapeHTML(category.name || category.sourceID) + "</span><input type=\"checkbox\" data-hide=\"" + escapeHTML(category.sourceID) + "\"" + (hiddenMap()[category.sourceID] ? " checked" : "") + "></label>";
  }).join("") || "<div class=\"empty\">No channel groups available for this connection.</div>";
}
function categoryName(id) {
  if (String(id || "").indexOf("source:") === 0) return sourceCategoryName(String(id || "").slice("source:".length));
  const category = allFilterCategories().find(function(item) { return item.id === id; });
  return category ? category.name : "";
}
function profileSaveStatusHTML() {
  if (!state.profileSaveMessage) return "";
  const warning = state.profileSaveStatus === "error" || state.profileSaveStatus === "local";
  return "<div class=\"settings-note" + (warning ? " settings-warning" : "") + "\">" + escapeHTML(state.profileSaveMessage) + "</div>";
}
function adminSaveStatusHTML() {
  if (!state.adminSaveMessage) return "";
  const warning = state.adminSaveStatus === "error" || state.adminSaveStatus === "dirty";
  return "<div class=\"settings-note" + (warning ? " settings-warning" : "") + "\">" + escapeHTML(state.adminSaveMessage) + "</div>";
}
function updateCategoryParsingField(field, target) {
  const settings = state.adminCategorySettings || defaultAdminCategorySettings();
  settings[field] = target.value;
  if (!settings.delimiter) settings.delimiter = "pipe";
  state.adminCategorySettings = settings;
  if (state.category.indexOf("virtual:") === 0 && !categoryName(state.category)) state.category = "";
  normalizeAdminCategorySettings();
  markAdminSettingsDraft();
  renderAdminPage();
}
function updateAdminECMField(field, target) {
  const settings = state.adminCategorySettings || defaultAdminCategorySettings();
  if (field === "enabled") settings.ecmEnabled = !!target.checked;
  if (field === "url") settings.ecmURL = target.value;
  state.adminCategorySettings = settings;
  normalizeAdminCategorySettings();
  if (!adminECMEnabled() && state.adminTab === "manager") state.adminTab = "settings";
  markAdminSettingsDraft();
  renderAdminPage();
}
function renderAdminPage() {
  normalizeAdminCategorySettings();
  if (!adminECMEnabled() && state.adminTab === "manager") state.adminTab = "settings";
  renderAdminTopbarTabs();
  renderAdminTopbarActions();
  const shell = document.querySelector(".shell");
  if (shell) shell.classList.toggle("is-admin-manager", state.adminTab === "manager");
  byId("view").innerHTML = state.adminTab === "manager" ? renderExternalChannelManager() : "<div class=\"settings-stack\">" + renderAdminSettingsTab() + "</div>";
  if (state.adminTab !== "manager") {
    renderAdminCategorySettings();
    renderAdminCategoryAliasSettings();
    renderAdminEventKeywordSettings();
    renderAdminECMSettings();
  }
}
function renderAdminTopbarTabs() {
  const root = byId("admin-tabs");
  if (!root) return;
  root.innerHTML = "<button type=\"button\" data-admin-tab=\"settings\" class=\"" + (state.adminTab === "settings" ? "active" : "") + "\">" + icon("settings") + "<span>Settings</span></button>"
    + (adminECMEnabled() ? "<button type=\"button\" data-admin-tab=\"manager\" class=\"" + (state.adminTab === "manager" ? "active" : "") + "\">" + icon("external") + "<span>Channel Manager</span></button>" : "");
}
function renderAdminTopbarActions() {
  const root = byId("admin-actions");
  if (!root) return;
  if (state.adminTab !== "settings") {
    root.innerHTML = "";
    return;
  }
  const dirty = adminSettingsDirty();
  const saving = state.adminSaveStatus === "saving";
  root.innerHTML = "<button data-admin-settings-action=\"save\"" + ((!dirty || saving) ? " disabled" : "") + ">Save</button><button data-admin-settings-action=\"discard\"" + ((!dirty || saving) ? " disabled" : "") + ">Discard</button>";
}
function setAdminTab(tab) {
  state.adminTab = tab === "manager" ? "manager" : "settings";
  renderAdminPage();
}
function renderAdminSettingsTab() {
  return ""
    + "<div class=\"settings-card\"><h2>Connection Status</h2>" + adminStatusPanel() + "</div>"
    + "<div class=\"settings-card\"><h2>Group method</h2><div id=\"admin-category-settings\" class=\"settings-list\"></div></div>"
    + "<div class=\"settings-card\"><h2>Presentation Overrides</h2><div class=\"settings-note admin-status-note\">Alternative Group Names add alternate virtual group paths without changing the original Dispatcharr groups. The original group remains visible.</div><div id=\"admin-category-alias-settings\" class=\"settings-list\"></div></div>"
    + "<div class=\"settings-card\"><h2>Event Keywords</h2><div class=\"settings-note admin-status-note\">Events are detected from the Dispatcharr guide. One keyword per line or comma-separated.</div><div id=\"admin-event-keyword-settings\" class=\"settings-list\"></div></div>"
    + "<div class=\"settings-card\"><h2>ECM</h2><div id=\"admin-ecm-settings\" class=\"settings-list\"></div></div>"
    + "";
}
function adminStatusPill(status) {
  status = String(status || "ok").toLowerCase();
  const label = status === "failed" ? "Error" : (status === "loading" ? "Updating" : (status === "error" ? "Error" : "Healthy"));
  const cls = status === "loading" ? " loading" : ((status === "error" || status === "failed") ? " error" : "");
  return "<span class=\"admin-status-pill" + cls + "\">" + escapeHTML(label) + "</span>";
}
function adminStatusItem(label, value, detail) {
  return "<div class=\"admin-status-item\"><span>" + escapeHTML(label) + "</span><strong title=\"" + escapeHTML(value) + "\">" + value + "</strong>" + (detail ? "<small title=\"" + escapeHTML(detail) + "\">" + escapeHTML(detail) + "</small>" : "") + "</div>";
}
function adminStatusPanel() {
  const status = state.app && state.app.status ? state.app.status : {};
  const source = state.app && state.app.source ? state.app.source : {};
  const guideStatus = String(status.epgStatus || (status.epgProgramCount ? "ok" : "unknown"));
  const sourceLabel = source.name || source.sourceName || "Dispatcharr";
  const error = status.lastError || status.epgLastError || "";
  return "<div class=\"admin-status-grid\">"
    + adminStatusItem("Connection", adminStatusPill(status.status || "ok"), sourceModeLabel(source.mode))
    + adminStatusItem("Source", escapeHTML(sourceLabel), "Credentials are stored in Silo settings and hidden here.")
    + adminStatusItem("Channels", escapeHTML(String(status.channelCount || items(state.app.channels).length || 0)), "Last catalog sync: " + dateTimeLabel(status.lastSuccessUnix))
    + adminStatusItem("Guide", adminStatusPill(guideStatus), String(status.epgProgramCount || items(state.app.programs).length || 0) + " programs · " + dateTimeLabel(status.epgLastSuccessUnix))
    + "</div>"
    + (error ? "<div class=\"settings-note settings-warning admin-status-note\">" + escapeHTML(error) + "</div>" : "");
}
function renderExternalChannelManager() {
  const managerURL = adminECMURL();
  return "<div class=\"external-manager-surface\"><iframe class=\"external-manager-frame\" src=\"" + escapeHTML(managerURL) + "\" title=\"Channel Manager\"></iframe></div>";
}
function renderAdminECMSettings() {
  const settings = adminSettings();
  const root = byId("admin-ecm-settings");
  if (!root) return;
  root.innerHTML = "<label><span>Enable ECM</span><input type=\"checkbox\" data-admin-ecm-field=\"enabled\"" + (settings.ecmEnabled === true ? " checked" : "") + "></label>"
    + "<div class=\"settings-row ecm-url-row\"><span>ECM URL</span><input type=\"url\" data-admin-ecm-field=\"url\" value=\"" + escapeHTML(settings.ecmURL || "") + "\"></div>"
    + "<div class=\"settings-note\">When enabled, the Channel Manager tab embeds this ECM instance for admin channel management.</div>";
}
function renderAdminCategorySettings() {
  const settings = adminSettings();
  const root = byId("admin-category-settings");
  root.innerHTML = adminSaveStatusHTML()
    + "<div class=\"settings-row\"><span>Mode</span><select data-admin-category-field=\"mode\"><option value=\"normal\"" + (settings.mode === "normal" ? " selected" : "") + ">Normal</option><option value=\"delimiter\"" + (settings.mode === "delimiter" ? " selected" : "") + ">By delimiter</option></select></div>"
    + (settings.mode !== "normal" ? "<div class=\"settings-row\"><span>Delimiter</span><select data-admin-category-field=\"delimiter\"><option value=\"pipe\"" + (settings.delimiter === "pipe" ? " selected" : "") + ">Pipe: Sports | NHL Teams</option><option value=\"dash\"" + (settings.delimiter === "dash" ? " selected" : "") + ">Dash: Sports - NHL Teams</option></select></div>" : "")
    + (settings.mode === "normal" ? "<div class=\"settings-note\">Channel groups are shown as provided, without remapping or resorting.</div>" : "")
    + (settings.mode === "delimiter" ? "<div class=\"settings-note\">Channel group names are split into virtual groups using the selected delimiter.</div>" : "");
}
function adminSourceGroups() {
  const groups = {};
  effectiveChannels(false).forEach(function(channel) {
    const sourcePath = sourceCategoryLabel(channel);
    if (!sourcePath) return;
    groups[sourcePath] = groups[sourcePath] || { sourcePath: sourcePath, count: 0 };
    groups[sourcePath].count++;
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
  const addRow = "<div class=\"settings-row alias-add-row\"><span>Source group</span><select id=\"admin-alias-source\"" + (!sourceGroups.length ? " disabled" : "") + ">" + sourceOptions + "</select><input id=\"admin-alias-path\" placeholder=\"Sports | Arabic\"" + (!sourceGroups.length ? " disabled" : "") + "><button data-admin-alias-action=\"add\"" + (!sourceGroups.length ? " disabled" : "") + ">Add</button></div>";
  const rows = aliases.map(function(alias, index) {
    const count = adminSourceGroupCount(alias.sourcePath);
    return "<div class=\"alias-table-row" + (!count ? " stale" : "") + "\"><div class=\"alias-table-source\" title=\"" + escapeHTML(alias.sourcePath) + "\"><strong>" + escapeHTML(alias.sourcePath) + "</strong>" + (!count ? "<small>Source not found</small>" : "") + "</div><span class=\"alias-table-count\">" + escapeHTML(String(count)) + "</span><input data-admin-alias-index=\"" + index + "\" data-admin-alias-field=\"aliasPath\" value=\"" + escapeHTML(alias.aliasPath) + "\" title=\"" + escapeHTML(alias.aliasPath) + "\" aria-label=\"Alternative group name\"><div class=\"alias-table-actions\"><button data-admin-alias-action=\"remove\" data-admin-alias-index=\"" + index + "\">Remove</button></div></div>";
  }).join("");
  root.innerHTML = (settings.mode !== "delimiter" ? "<div class=\"settings-note settings-warning\">Alternative group names apply when category mode is By delimiter.</div>" : "")
    + addRow
    + "<div class=\"alias-table\"><div class=\"alias-table-head\"><span>Source group</span><span>Channels</span><span>Alternative group name</span><span>Actions</span></div>" + (rows || "<div class=\"empty\">No alternative group names yet.</div>") + "</div>";
}
function renderAdminEventKeywordSettings() {
  const root = byId("admin-event-keyword-settings");
  if (!root) return;
  const rows = normalizeEventKeywordRows(adminSettings().eventKeywords);
  root.innerHTML = rows.map(function(row, index) {
    return "<div class=\"settings-row event-keyword-row\"><span>" + escapeHTML(row.categoryName || eventCategoryName(row.categoryId)) + "</span><textarea data-admin-event-keyword-index=\"" + index + "\" aria-label=\"" + escapeHTML((row.categoryName || row.categoryId) + " event keywords") + "\">" + escapeHTML(row.keywords.join("\\n")) + "</textarea></div>";
  }).join("");
}
function updateAdminEventKeywords(index, value) {
  const settings = state.adminCategorySettings || defaultAdminCategorySettings();
  const rows = normalizeEventKeywordRows(settings.eventKeywords);
  if (!rows[index]) return;
  rows[index] = Object.assign({}, rows[index], { keywords: normalizeKeywordList(value) });
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
  const groups = customGroups();
  const selected = selectedCustomGroup();
  const memberships = selected ? customMemberships(selected.id) : [];
  const query = lower(state.customGroupQuery);
  const availableChannels = items(state.app.channels).filter(function(channel) {
    if (selected && memberships.indexOf(channel.id) !== -1) return false;
    if (!query) return true;
    return lower(channel.name || channel.id).indexOf(query) !== -1 || lower(sourceCategoryLabel(channel)).indexOf(query) !== -1;
  });
  if (!availableChannels.some(function(channel) { return channel.id === state.customGroupChannelID; })) state.customGroupChannelID = availableChannels.length ? availableChannels[0].id : "";
  const pickerChannels = availableChannels.slice(0, 12);
  root.innerHTML = "<div class=\"settings-row\"><span>New group</span><input id=\"custom-group-name\" placeholder=\"Spanish\"><button data-custom-group-action=\"create\">Create</button></div>"
    + (groups.length ? "<div class=\"settings-row\"><span>Edit group</span><select id=\"custom-group-select\">" + groups.map(function(group) { return "<option value=\"" + escapeHTML(group.id) + "\"" + (selected && selected.id === group.id ? " selected" : "") + ">" + escapeHTML(group.name) + "</option>"; }).join("") + "</select><button data-custom-group-action=\"delete\">Delete</button></div>" : "<div class=\"empty\">Create a group to build your own channel lineup.</div>")
    + (selected ? "<div class=\"settings-row custom-channel-picker\"><span>Add channel</span><div class=\"custom-channel-combobox\"><input id=\"custom-group-channel-search\" role=\"combobox\" aria-controls=\"custom-group-channel-options\" aria-expanded=\"true\" aria-autocomplete=\"list\" placeholder=\"Search by name or group\" value=\"" + escapeHTML(state.customGroupQuery) + "\"><div id=\"custom-group-channel-options\" class=\"custom-channel-options\" role=\"listbox\">" + (pickerChannels.length ? pickerChannels.map(function(channel) { const active = channel.id === state.customGroupChannelID; return "<button class=\"custom-channel-option" + (active ? " selected" : "") + "\" type=\"button\" role=\"option\" aria-selected=\"" + (active ? "true" : "false") + "\" data-custom-group-channel-option=\"" + escapeHTML(channel.id) + "\"><strong>" + escapeHTML(channel.name || channel.id) + "</strong><small>" + escapeHTML(sourceCategoryLabel(channel) || "Live TV") + "</small></button>"; }).join("") : "<div class=\"empty\">No matching channels.</div>") + "</div><button data-custom-group-action=\"add-channel\"" + (!state.customGroupChannelID ? " disabled" : "") + ">Add</button></div></div>"
    + "<div class=\"settings-note\">" + escapeHTML(availableChannels.length ? "Showing " + Math.min(availableChannels.length, 12) + " of " + availableChannels.length + " matching channels. Keep typing to narrow it down." : "No matching channels.") + "</div>"
    + "<div class=\"settings-preview\">" + (memberships.length ? memberships.map(function(id) { const channel = channelByID(id); return "<div>" + escapeHTML((channel && channel.name) || id) + " <button data-custom-group-action=\"remove-channel\" data-channel-id=\"" + escapeHTML(id) + "\">Remove</button></div>"; }).join("") : "<div>No channels in this group yet.</div>") + "</div>" : "");
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
function colorClass(index) {
  return ["purple", "green", "red", "gray", "blue"][index % 5];
}
function audioTrackList() {
  const video = byId("player");
  if (!video || !video.audioTracks || typeof video.audioTracks.length !== "number") return [];
  const tracks = [];
  for (let index = 0; index < video.audioTracks.length; index++) tracks.push(video.audioTracks[index]);
  return tracks;
}
function audioTrackName(track, index) {
  return track && (track.label || track.language || track.kind || track.id) ? (track.label || track.language || track.kind || track.id) : "Audio " + (index + 1);
}
function textTrackList() {
  const video = byId("player");
  if (!video || !video.textTracks || typeof video.textTracks.length !== "number") return [];
  const tracks = [];
  for (let index = 0; index < video.textTracks.length; index++) {
    const track = video.textTracks[index];
    if (!track || (track.kind && ["subtitles", "captions"].indexOf(track.kind) === -1)) continue;
    tracks.push(track);
  }
  return tracks;
}
function textTrackName(track, index) {
  return track && (track.label || track.language || track.kind || track.id) ? (track.label || track.language || track.kind || track.id) : "Subtitles " + (index + 1);
}
function updateSubtitlesButton() {
  const button = byId("player-subtitles-button");
  if (!button) return;
  const tracks = textTrackList();
  const activeIndex = tracks.findIndex(function(track) { return track.mode === "showing"; });
  if (activeIndex >= 0) state.selectedTextTrack = activeIndex;
  button.classList.toggle("active", activeIndex >= 0);
  button.setAttribute("aria-pressed", activeIndex >= 0 ? "true" : "false");
  button.setAttribute("aria-label", activeIndex >= 0 ? "Subtitles: " + textTrackName(tracks[activeIndex], activeIndex) : "Subtitles");
}
function toggleSubtitles() {
  const tracks = textTrackList();
  closePlayerPopovers();
  if (!tracks.length) {
    showPlayerToast("No subtitles are available for this stream.");
    updateSubtitlesButton();
    return;
  }
  const activeIndex = tracks.findIndex(function(track) { return track.mode === "showing"; });
  const nextIndex = activeIndex >= 0 && activeIndex < tracks.length - 1 ? activeIndex + 1 : (activeIndex >= 0 ? -1 : Math.max(0, state.selectedTextTrack));
  tracks.forEach(function(track, index) {
    track.mode = index === nextIndex ? "showing" : "disabled";
  });
  state.selectedTextTrack = nextIndex;
  updateSubtitlesButton();
  showPlayerToast(nextIndex >= 0 ? "Subtitles: " + textTrackName(tracks[nextIndex], nextIndex) : "Subtitles off.");
}
function updateAudioMenu() {
  const button = byId("player-audio-button");
  const languageButton = byId("player-language-button");
  const menu = byId("player-audio-menu");
  if (!button || !menu) return;
  const tracks = audioTrackList();
  const activeIndex = tracks.findIndex(function(track) { return !!track.enabled; });
  state.selectedAudioTrack = activeIndex >= 0 ? activeIndex : state.selectedAudioTrack;
  const activeLabel = tracks.length ? audioTrackName(tracks[state.selectedAudioTrack] || tracks[0], state.selectedAudioTrack || 0) : "Default audio";
  button.innerHTML = icon("language") + "<span>" + escapeHTML(activeLabel) + "</span>" + icon("chevron-down");
  button.setAttribute("aria-expanded", state.audioMenuOpen ? "true" : "false");
  if (languageButton) {
    languageButton.classList.toggle("active", state.audioMenuOpen && tracks.length > 1);
    languageButton.setAttribute("aria-expanded", state.audioMenuOpen && tracks.length > 1 ? "true" : "false");
    languageButton.setAttribute("aria-label", tracks.length > 1 ? "Audio language: " + activeLabel : "Audio language");
  }
  menu.classList.toggle("open", state.audioMenuOpen);
  updatePlayerChrome();
  menu.innerHTML = tracks.length ? tracks.map(function(track, index) {
    return "<button type=\"button\" role=\"menuitem\" data-player-action=\"audio-track\" data-audio-index=\"" + index + "\" class=\"" + (index === state.selectedAudioTrack ? "active" : "") + "\">" + escapeHTML(audioTrackName(track, index)) + "</button>";
  }).join("") : "<button type=\"button\" role=\"menuitem\" class=\"active\" data-player-action=\"audio-track\" data-audio-index=\"0\">Default audio</button>";
}
function toggleLanguageMenu() {
  const tracks = audioTrackList();
  if (tracks.length <= 1) {
    closePlayerPopovers();
    showPlayerToast("No alternate audio languages are available for this stream.");
    return;
  }
  state.audioMenuOpen = !state.audioMenuOpen;
  closePlayerPopovers("audio");
  updateAudioMenu();
}
function selectAudioTrack(index) {
  const tracks = audioTrackList();
  if (!tracks.length) {
    state.selectedAudioTrack = 0;
    state.audioMenuOpen = false;
    updateAudioMenu();
    return;
  }
  tracks.forEach(function(track, trackIndex) { track.enabled = trackIndex === index; });
  state.selectedAudioTrack = index;
  state.audioMenuOpen = false;
  updateAudioMenu();
}
function volumeLabel() {
  if (state.muted || state.volume <= 0) return "0%";
  return Math.round(state.volume * 100) + "%";
}
function applyVolumeToVideo() {
  const video = byId("player");
  state.volume = Math.max(0, Math.min(1, Number(state.volume) || 0));
  state.muted = state.volume <= 0;
  if (video) {
    video.volume = state.volume;
    video.muted = state.muted;
  }
  updateVolumeMenu();
}
function updateVolumeMenu() {
  const button = byId("player-volume-button");
  const popover = byId("player-volume-popover");
  const slider = byId("player-volume-slider");
  const value = byId("player-volume-value");
  if (!button || !popover) return;
  button.innerHTML = icon(state.muted || state.volume <= 0 ? "speaker-off" : "speaker");
  button.setAttribute("aria-expanded", state.volumeMenuOpen ? "true" : "false");
  popover.classList.toggle("open", state.volumeMenuOpen);
  if (slider) slider.value = String(Math.round(state.volume * 100));
  if (value) value.textContent = volumeLabel();
  updatePlayerChrome();
}
function closePlayerPopovers(except) {
  if (except !== "audio") state.audioMenuOpen = false;
  if (except !== "volume") state.volumeMenuOpen = false;
  if (except !== "more") state.moreMenuOpen = false;
  updateAudioMenu();
  updateVolumeMenu();
  renderPlayerMoreMenu();
}
function showPlayerToast(message) {
  const toast = byId("player-toast");
  if (!toast) return;
  toast.textContent = message;
  toast.classList.add("show");
  clearTimeout(state.toastTimer);
  state.toastTimer = setTimeout(function() { toast.classList.remove("show"); }, 2400);
}
function showAppToast(message) {
  let toast = byId("app-toast");
  if (!toast) {
    toast = document.createElement("div");
    toast.id = "app-toast";
    toast.className = "app-toast";
    toast.setAttribute("role", "status");
    document.body.appendChild(toast);
  }
  toast.textContent = message;
  toast.classList.add("show");
  clearTimeout(state.appToastTimer);
  state.appToastTimer = setTimeout(function() { toast.classList.remove("show"); }, 2600);
}
async function openCastPicker() {
  const video = byId("player");
  if (!video) return;
  closePlayerPopovers();
  try {
    if (typeof video.webkitShowPlaybackTargetPicker === "function") {
      video.webkitShowPlaybackTargetPicker();
      return;
    }
    if (video.remote && typeof video.remote.prompt === "function") {
      await video.remote.prompt();
      return;
    }
    showPlayerToast("AirPlay or Cast is not available in this browser.");
  } catch (error) {
    showPlayerToast("No playback target selected.");
  }
}
async function togglePictureInPicture() {
  const video = byId("player");
  if (!video) return;
  closePlayerPopovers();
  if (!document.pictureInPictureEnabled || typeof video.requestPictureInPicture !== "function") {
    showPlayerToast("Picture in Picture is not available in this browser.");
    return;
  }
  try {
    if (document.pictureInPictureElement) await document.exitPictureInPicture();
    else await video.requestPictureInPicture();
  } catch (error) {
    showPlayerToast("Picture in Picture could not be opened.");
  }
}
function updateCenterPlayButton() {
  const video = byId("player");
  const button = byId("player-center-button");
  if (!video || !button) return;
  const loading = !!state.playerWaiting && !video.paused;
  const show = loading || video.paused;
  button.classList.toggle("hidden", !show);
  button.classList.toggle("loading", loading);
  button.innerHTML = loading ? icon("loader") : icon(video.paused ? "play" : "pause");
  button.setAttribute("aria-label", loading ? "Loading stream" : (video.paused ? "Play" : "Pause"));
  button.disabled = loading;
}
function togglePlayPause() {
  const video = byId("player");
  if (!video) return;
  closePlayerPopovers();
  if (video.paused) video.play().catch(function() { showPlayerToast("Playback could not be started."); });
  else video.pause();
  updateCenterPlayButton();
}
function fullscreenElement() {
  return document.fullscreenElement || document.webkitFullscreenElement || null;
}
function updateFullscreenButton() {
  const button = byId("player-fullscreen-button");
  if (!button) return;
  const active = !!fullscreenElement();
  button.innerHTML = icon(active ? "fullscreen-exit" : "fullscreen");
  button.classList.toggle("active", active);
  button.setAttribute("aria-pressed", active ? "true" : "false");
  button.setAttribute("aria-label", active ? "Exit fullscreen" : "Fullscreen");
  renderPlayerMoreMenu();
}
async function toggleFullscreen() {
  const shell = document.querySelector(".playback-shell");
  closePlayerPopovers();
  try {
    if (fullscreenElement()) {
      if (document.exitFullscreen) await document.exitFullscreen();
      else if (document.webkitExitFullscreen) document.webkitExitFullscreen();
    } else if (shell) {
      if (shell.requestFullscreen) await shell.requestFullscreen();
      else if (shell.webkitRequestFullscreen) shell.webkitRequestFullscreen();
      else showPlayerToast("Fullscreen is not available in this browser.");
    }
  } catch (error) {
    showPlayerToast("Fullscreen could not be changed.");
  }
  updateFullscreenButton();
}
function setVideoSource(url, options) {
  const video = byId("player");
  if (!video) return;
  const rewindable = !!(options && options.rewindable);
  video.controls = rewindable;
  applyVolumeToVideo();
  state.selectedAudioTrack = 0;
  state.selectedTextTrack = -1;
  state.audioMenuOpen = false;
  state.volumeMenuOpen = false;
  state.moreMenuOpen = false;
  updateAudioMenu();
  updateSubtitlesButton();
  updateVolumeMenu();
  renderPlayerMoreMenu();
  if (video.audioTracks && video.audioTracks.addEventListener) {
    video.audioTracks.addEventListener("addtrack", updateAudioMenu);
    video.audioTracks.addEventListener("removetrack", updateAudioMenu);
    video.audioTracks.addEventListener("change", updateAudioMenu);
  }
  video.addEventListener("loadedmetadata", updateAudioMenu, { once: true });
  video.addEventListener("loadedmetadata", updateSubtitlesButton, { once: true });
  video.addEventListener("waiting", function() { state.playerWaiting = true; updateCenterPlayButton(); });
  video.addEventListener("stalled", function() { state.playerWaiting = true; updateCenterPlayButton(); });
  video.addEventListener("canplay", function() { state.playerWaiting = false; updateCenterPlayButton(); });
  video.addEventListener("playing", function() { state.playerWaiting = false; updateCenterPlayButton(); });
  video.addEventListener("pause", updateCenterPlayButton);
  video.addEventListener("play", updateCenterPlayButton);
  video.addEventListener("error", function() { state.playerWaiting = false; updateCenterPlayButton(); });
  if (video.textTracks && video.textTracks.addEventListener) {
    video.textTracks.addEventListener("addtrack", updateSubtitlesButton);
    video.textTracks.addEventListener("removetrack", updateSubtitlesButton);
    video.textTracks.addEventListener("change", updateSubtitlesButton);
  }
  if (state.hls) { state.hls.destroy(); state.hls = null; }
  if (state.tsPlayer) { state.tsPlayer.destroy(); state.tsPlayer = null; }
  const attachment = attachVideoSource(video, url, { rewindable: rewindable });
  state.hls = attachment.hls;
  state.tsPlayer = attachment.tsPlayer;
  setTimeout(updateAudioMenu, 500);
  setTimeout(updateAudioMenu, 1800);
  setTimeout(updateSubtitlesButton, 500);
  setTimeout(updateSubtitlesButton, 1800);
  updateCenterPlayButton();
  applyAspectMode();
  video.play().then(updateCenterPlayButton).catch(function() { updateCenterPlayButton(); });
}
async function playChannel(channel) {
  state.currentChannel = channel;
  state.view = "player";
  render();
  setVideoSource(browserStreamURL(channel), { rewindable: isRewindableChannel(channel) });
  startWatch(channel);
  const guide = await getJSON("/dispatcharr/api/guide?channel_id=" + encodeURIComponent(channel.id)).catch(function() { return { programs: [] }; });
  const nowGuide = byId("now-guide");
  if (nowGuide) nowGuide.innerHTML = items(guide.programs).slice(0, 6).map(function(program) { return "<div class=\"program\"><time>" + escapeHTML(timeLabel(program.startUnix)) + "</time><strong>" + escapeHTML(program.title || "Untitled") + "</strong></div>"; }).join("") || "<div class=\"empty\">No guide entries.</div>";
}
function startWatch(channel) {
  if (state.currentSession) postJSON("/dispatcharr/api/watch/stop", { sessionId: state.currentSession.id, reason: "switch_channel" }).catch(function() {});
  recordWatchPreference(channel);
  postJSON("/dispatcharr/api/watch/start", { itemKind: "channel", itemId: channel.id, itemName: channel.name }).then(function(payload) {
    state.currentSession = payload.session;
    if (state.heartbeat) clearInterval(state.heartbeat);
    state.heartbeat = setInterval(function() {
      if (state.currentSession) postJSON("/dispatcharr/api/watch/heartbeat", { sessionId: state.currentSession.id }).catch(function() {});
    }, 30000);
    renderRail();
  }).catch(function() {});
}
function handlePlayerAction(action, button) {
  const video = byId("player");
  wakePlayerChrome();
  if (action === "back") {
    setView("live");
    return;
  }
  if (action === "guide") {
    state.playerGuideOpen = !state.playerGuideOpen;
    closePlayerPopovers();
    renderPlayerGuidePanel();
    return;
  }
  if (action === "guide-close") {
    state.playerGuideOpen = false;
    renderPlayerGuidePanel();
    return;
  }
  if (action === "cast") {
    closePlayerPopovers();
    openCastPicker();
    return;
  }
  if (action === "pip") {
    togglePictureInPicture();
    return;
  }
  if (action === "play-toggle") {
    togglePlayPause();
    return;
  }
  if (action === "fullscreen") {
    toggleFullscreen();
    return;
  }
  if (action === "subtitles") {
    toggleSubtitles();
    return;
  }
  if (action === "volume-menu") {
    state.volumeMenuOpen = !state.volumeMenuOpen;
    closePlayerPopovers("volume");
    updateVolumeMenu();
    return;
  }
  if (action === "audio-menu") {
    state.audioMenuOpen = !state.audioMenuOpen;
    closePlayerPopovers("audio");
    updateAudioMenu();
    return;
  }
  if (action === "language-menu") {
    toggleLanguageMenu();
    return;
  }
  if (action === "audio-track") {
    selectAudioTrack(Number(button && button.getAttribute("data-audio-index")) || 0);
    return;
  }
  if (action === "more") {
    state.moreMenuOpen = !state.moreMenuOpen;
    closePlayerPopovers("more");
    renderPlayerMoreMenu();
    return;
  }
  if (action === "aspect") {
    state.aspectMode = state.aspectMode === "fit" ? "fill" : "fit";
    applyAspectMode();
    renderPlayerMoreMenu();
    return;
  }
  if (action === "search-channel") {
    state.moreMenuOpen = false;
    renderPlayerMoreMenu();
    const search = byId("global-search");
    if (search) search.focus();
    return;
  }
  if (action === "add-multiview" && state.currentChannel) {
    state.moreMenuOpen = false;
    addChannelToMultiview(state.currentChannel);
    return;
  }
  if (action === "copy-stream") {
    const url = currentStreamURL();
    if (url && navigator.clipboard) navigator.clipboard.writeText(new URL(url, window.location.href).href).then(function() { showPlayerToast("Stream URL copied."); }).catch(function() { showPlayerToast("Could not copy stream URL."); });
    else showPlayerToast("No stream URL available.");
    state.moreMenuOpen = false;
    renderPlayerMoreMenu();
    return;
  }
  if (action === "open-stream") {
    const url = currentStreamURL();
    if (url) window.open(url, "_blank", "noopener");
    state.moreMenuOpen = false;
    renderPlayerMoreMenu();
    return;
  }
  if (action === "favorite" && state.currentChannel) {
    const id = state.currentChannel.id;
    if (favoriteMap()[id]) {
      delete state.app.preferences.favorites[id];
      state.app.preferences.favoriteOrder = items(state.app.preferences.favoriteOrder).filter(function(item) { return item !== id; });
    } else {
      state.app.preferences.favorites[id] = true;
      state.app.preferences.favoriteOrder = uniqueIDs(items(state.app.preferences.favoriteOrder).concat([id]));
    }
    if (button) {
      const isFavorite = !!favoriteMap()[id];
      button.innerHTML = icon(isFavorite ? "heart-solid" : "heart");
      button.classList.toggle("active", isFavorite);
      button.setAttribute("aria-pressed", isFavorite ? "true" : "false");
      button.setAttribute("aria-label", isFavorite ? "Remove channel from favorites" : "Favorite channel");
    }
    savePrefs();
    postJSON("/dispatcharr/api/favorites", { id: id, enabled: !!favoriteMap()[id] }).catch(function() {});
    renderRail();
  }
}
document.addEventListener("click", function(event) {
  const playerTarget = event.target.closest("[data-player-action]");
  if (playerTarget) {
    event.preventDefault();
    handlePlayerAction(playerTarget.getAttribute("data-player-action"), playerTarget);
    return;
  }
  const recordingsRefresh = event.target.closest("[data-recordings-refresh]");
  if (recordingsRefresh) {
    event.preventDefault();
    state.recordings = null;
    loadRecordings(true);
    renderRecordingsPage();
    return;
  }
  const guideRefresh = event.target.closest("[data-guide-refresh]");
  if (guideRefresh) {
    event.preventDefault();
    if (state.view === "sports") {
      const buttons = Array.prototype.slice.call(document.querySelectorAll("[data-guide-refresh]"));
      buttons.forEach(function(button) {
        button.classList.add("is-loading");
        button.disabled = true;
      });
      loadSports(true).finally(function() {
        buttons.forEach(function(button) {
          button.classList.remove("is-loading");
          button.disabled = false;
        });
      });
      renderSportsPage();
      return;
    }
    if (state.view === "events") {
      const buttons = Array.prototype.slice.call(document.querySelectorAll("[data-guide-refresh]"));
      buttons.forEach(function(button) {
        button.classList.add("is-loading");
        button.disabled = true;
      });
      loadEvents(true).finally(function() {
        buttons.forEach(function(button) {
          button.classList.remove("is-loading");
          button.disabled = false;
        });
      });
      renderEventsPage();
      return;
    }
    refreshAppData();
    return;
  }
  const searchCancel = event.target.closest("[data-search-cancel]");
  if (searchCancel) {
    event.preventDefault();
    setView(state.searchReturnView || "home");
    return;
  }
  const searchClear = event.target.closest("[data-search-clear]");
  if (searchClear) {
    event.preventDefault();
    clearRecentSearches();
    renderSearchPage();
    return;
  }
  const searchRecent = event.target.closest("[data-search-recent]");
  if (searchRecent) {
    event.preventDefault();
    state.searchQuery = searchRecent.getAttribute("data-search-recent") || "";
    rememberSearch(state.searchQuery);
    renderSearchPage();
    return;
  }
  const searchType = event.target.closest("[data-search-type]");
  if (searchType) {
    event.preventDefault();
    state.searchType = searchType.getAttribute("data-search-type") || "all";
    renderSearchPage();
    return;
  }
  const searchChannel = event.target.closest("[data-search-channel]");
  if (searchChannel) {
    event.preventDefault();
    rememberSearch(state.searchQuery);
    const channel = channelByID(searchChannel.getAttribute("data-search-channel"));
    if (channel) playChannel(channel);
    return;
  }
  const sportsTab = event.target.closest("[data-sports-tab]");
  if (sportsTab) {
    event.preventDefault();
    setSportsTab(sportsTab.getAttribute("data-sports-tab"));
    return;
  }
  const sportsLeague = event.target.closest("[data-sports-league]");
  if (sportsLeague) {
    event.preventDefault();
    setSportsLeague(sportsLeague.getAttribute("data-sports-league"));
    return;
  }
  const sportsExpand = event.target.closest("[data-sports-expand-event]");
  if (sportsExpand) {
    event.preventDefault();
    toggleSportsEventChannels(sportsExpand.getAttribute("data-sports-expand-event"));
    return;
  }
  const sportsFavorite = event.target.closest("[data-sports-favorite-team]");
  if (sportsFavorite) {
    event.preventDefault();
    toggleSportsTeamFavorite(sportsFavorite.getAttribute("data-sports-favorite-team"), sportsFavorite.getAttribute("data-sports-favorite-enabled") === "true");
    return;
  }
  const eventTab = event.target.closest("[data-event-tab]");
  if (eventTab) {
    event.preventDefault();
    setEventTab(eventTab.getAttribute("data-event-tab"));
    return;
  }
  const eventCategory = event.target.closest("[data-event-category]");
  if (eventCategory) {
    event.preventDefault();
    setEventCategory(eventCategory.getAttribute("data-event-category"));
    return;
  }
  const eventExpand = event.target.closest("[data-event-expand]");
  if (eventExpand) {
    event.preventDefault();
    toggleBroadcastEventChannels(eventExpand.getAttribute("data-event-expand"));
    return;
  }
  const recordingPlayback = event.target.closest("[data-recording-playback]");
  if (recordingPlayback) {
    event.preventDefault();
    const url = recordingPlayback.getAttribute("data-recording-playback");
    if (url) window.open(url, "_blank", "noopener");
    return;
  }
  const scheduleTarget = event.target.closest("[data-schedule-channel]");
  if (scheduleTarget) {
    event.preventDefault();
    event.stopPropagation();
    scheduleProgram(scheduleTarget.getAttribute("data-schedule-channel"), scheduleTarget.getAttribute("data-schedule-program"), scheduleTarget);
    return;
  }
  const customGroupAction = event.target.closest("[data-custom-group-action]");
  if (customGroupAction) {
    event.preventDefault();
    const action = customGroupAction.getAttribute("data-custom-group-action");
    if (action === "create") createCustomGroup((byId("custom-group-name") || {}).value || "");
    if (action === "delete") deleteSelectedCustomGroup();
    if (action === "add-channel") addChannelToSelectedGroup(state.customGroupChannelID || "");
    if (action === "remove-channel") removeChannelFromSelectedGroup(customGroupAction.getAttribute("data-channel-id"));
    return;
  }
  const customGroupChannelOption = event.target.closest("[data-custom-group-channel-option]");
  if (customGroupChannelOption) {
    event.preventDefault();
    selectCustomGroupChannel(customGroupChannelOption.getAttribute("data-custom-group-channel-option"));
    return;
  }
  const adminAliasAction = event.target.closest("[data-admin-alias-action]");
  if (adminAliasAction) {
    event.preventDefault();
    const action = adminAliasAction.getAttribute("data-admin-alias-action");
    if (action === "add") addAdminCategoryAlias();
    if (action === "remove") removeAdminCategoryAlias(Number(adminAliasAction.getAttribute("data-admin-alias-index")));
    return;
  }
  const adminSettingsAction = event.target.closest("[data-admin-settings-action]");
  if (adminSettingsAction) {
    event.preventDefault();
    const action = adminSettingsAction.getAttribute("data-admin-settings-action");
    if (action === "save") saveAdminCategorySettings();
    if (action === "discard") discardAdminCategorySettings();
    return;
  }
  const adminTab = event.target.closest("[data-admin-tab]");
  if (adminTab) {
    event.preventDefault();
    setAdminTab(adminTab.getAttribute("data-admin-tab"));
    return;
  }
  const virtualCategoryViewTarget = event.target.closest("[data-virtual-category-view]");
  if (virtualCategoryViewTarget) {
    event.preventDefault();
    setVirtualCategoryView(virtualCategoryViewTarget.getAttribute("data-virtual-category-view"));
    return;
  }
  const favoriteMove = event.target.closest("[data-favorite-move]");
  if (favoriteMove) {
    event.preventDefault();
    moveFavorite(favoriteMove.getAttribute("data-channel-id"), favoriteMove.getAttribute("data-favorite-move"));
    return;
  }
  const multiviewAction = event.target.closest("[data-multiview-action]");
  if (multiviewAction) {
    event.preventDefault();
    event.stopPropagation();
    handleMultiviewAction(multiviewAction.getAttribute("data-multiview-action"), multiviewAction.getAttribute("data-multiview-tile-id"));
    return;
  }
  const multiviewChannel = event.target.closest("[data-multiview-channel]");
  if (multiviewChannel) {
    event.preventDefault();
    event.stopPropagation();
    const channel = channelByID(multiviewChannel.getAttribute("data-multiview-channel"));
    if (channel) addChannelToMultiview(channel);
    return;
  }
  const multiviewFocus = event.target.closest("[data-multiview-focus]");
  if (multiviewFocus) {
    event.preventDefault();
    focusMultiviewTile(multiviewFocus.getAttribute("data-multiview-focus"));
    return;
  }
  const channelTarget = event.target.closest("[data-channel]");
  if (channelTarget) {
    const channel = channelByID(channelTarget.getAttribute("data-channel"));
    if (channel) playChannel(channel);
  }
  const categoryTarget = event.target.closest("[data-category]");
  if (categoryTarget) setCategory(categoryTarget.getAttribute("data-category"));
});
document.addEventListener("mouseover", function(event) {
  const target = overflowTooltipTarget(event);
  if (!target || (event.relatedTarget && target.contains(event.relatedTarget))) return;
  showOverflowTooltip(target, event);
});
document.addEventListener("mousemove", function(event) {
  const target = overflowTooltipTarget(event);
  if (target) showOverflowTooltip(target, event);
}, { passive: true });
document.addEventListener("mouseout", function(event) {
  const target = overflowTooltipTarget(event);
  if (!target || (event.relatedTarget && target.contains(event.relatedTarget))) return;
  hideOverflowTooltip();
});
document.addEventListener("focusin", function(event) {
  const target = overflowTooltipTarget(event);
  if (target) showOverflowTooltip(target, event);
});
document.addEventListener("focusout", function(event) {
  const target = overflowTooltipTarget(event);
  if (target) hideOverflowTooltip();
});
document.addEventListener("fullscreenchange", updateFullscreenButton);
document.addEventListener("webkitfullscreenchange", updateFullscreenButton);
document.addEventListener("keydown", function(event) {
  if (event.target && event.target.id === "search-page-input" && event.key === "Enter") {
    event.preventDefault();
    rememberSearch(state.searchQuery);
    renderSearchPage();
    return;
  }
  if (state.view === "search" && event.key === "Escape") {
    event.preventDefault();
    setView(state.searchReturnView || "home");
    return;
  }
  if (state.view !== "player") return;
  const tag = event.target && event.target.tagName ? event.target.tagName.toLowerCase() : "";
  if (tag === "input" || tag === "textarea" || tag === "select") return;
  if (event.key === " " || event.key === "k" || event.key === "K") {
    event.preventDefault();
    togglePlayPause();
  }
  if (event.key === "f" || event.key === "F") {
    event.preventDefault();
    toggleFullscreen();
  }
});
["mousemove", "mousedown", "touchstart", "keydown"].forEach(function(eventName) {
  document.addEventListener(eventName, function(event) {
    if (state.view !== "player") return;
    if (eventName === "mousemove" && event.movementX === 0 && event.movementY === 0) return;
    wakePlayerChrome();
  }, { passive: true });
});
document.addEventListener("change", function(event) {
  const adminField = event.target.getAttribute("data-admin-category-field");
  if (adminField) {
    updateCategoryParsingField(adminField, event.target);
    return;
  }
  const adminECMField = event.target.getAttribute("data-admin-ecm-field");
  if (adminECMField) {
    updateAdminECMField(adminECMField, event.target);
    return;
  }
  const adminAliasField = event.target.getAttribute("data-admin-alias-field");
  if (adminAliasField) {
    updateAdminCategoryAlias(Number(event.target.getAttribute("data-admin-alias-index")), adminAliasField, event.target.value || "");
    return;
  }
  if (event.target && event.target.id === "custom-group-select") {
    state.selectedCustomGroup = event.target.value;
    state.customGroupQuery = "";
    state.customGroupChannelID = "";
    renderSettings();
    return;
  }
  const id = event.target.getAttribute("data-hide");
  if (!id) return;
  if (event.target.checked) state.app.preferences.hiddenCategories[id] = true;
  else delete state.app.preferences.hiddenCategories[id];
  savePrefs();
  postJSON("/dispatcharr/api/hidden-categories", { id: id, hidden: event.target.checked }).catch(function() {});
  render();
});
document.addEventListener("input", function(event) {
  if (event.target && event.target.id === "player-volume-slider") {
    state.volume = Number(event.target.value || 0) / 100;
    applyVolumeToVideo();
    syncMultiviewAudio();
  }
  if (event.target && event.target.id === "search-page-input") {
    state.searchQuery = event.target.value || "";
    renderSearchPage();
    return;
  }
  const adminEventKeywordIndex = event.target.getAttribute("data-admin-event-keyword-index");
  if (adminEventKeywordIndex !== null) {
    updateAdminEventKeywords(Number(adminEventKeywordIndex), event.target.value || "");
    return;
  }
  if (event.target && event.target.id === "custom-group-channel-search") {
    state.customGroupQuery = event.target.value || "";
    state.customGroupChannelID = "";
    renderSettings();
    const input = byId("custom-group-channel-search");
    if (input) {
      input.focus();
      input.setSelectionRange(input.value.length, input.value.length);
    }
  }
});
document.querySelectorAll("[data-view]").forEach(function(button) {
  button.onclick = function() { setView(button.dataset.view); };
});
const appSearchButton = byId("app-search-button");
if (appSearchButton) appSearchButton.onclick = function() { setView("search"); };
const globalSearch = byId("global-search");
if (globalSearch) {
  globalSearch.oninput = function(event) { state.query = event.target.value; render(); };
  globalSearch.onkeydown = function(event) {
    if (event.key !== "Enter") return;
    state.searchQuery = event.target.value || "";
    rememberSearch(state.searchQuery);
    setView("search");
  };
}
window.addEventListener("scroll", function() {
  if (state.view === "guide" && isNearGuideEnd()) appendGuideRows();
}, { passive: true });
window.addEventListener("beforeunload", function() {
  if (state.currentSession) navigator.sendBeacon(route("/dispatcharr/api/watch/stop"), JSON.stringify({ sessionId: state.currentSession.id, reason: "page_unload" }));
  items(state.multiviewTiles).forEach(function(tile) {
    if (tile.session) navigator.sendBeacon(route("/dispatcharr/api/watch/stop"), JSON.stringify({ sessionId: tile.session.id, reason: "page_unload" }));
  });
});
loadApp().catch(function() {
  byId("view").innerHTML = emptyStateHTML("Unable to load Live TV.", "Check your Dispatcharr connection in Live TV Admin, then refresh this page.");
});
