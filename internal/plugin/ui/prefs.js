// Part of the composed app.js bundle (order: ui_assets.go playerAppScriptParts).
// User preferences: defaults, normalization, local app cache, Silo settings sync.
function defaultPrefs() {
  return { favorites: {}, favoriteOrder: [], autoFavorites: {}, hiddenCategories: {}, sportsFavoriteTeams: {}, sportsFavoriteTeamLabels: {}, sportsFavoriteLeagues: {}, sportsPreferredChannels: {}, sportsPreferredNetworks: {}, sportsSpoilersHidden: false, sportsPlayerSpoilersHidden: false, featuredEvents: {}, keywordPasses: [], recentSearches: [], recentChannels: [], continueWatching: {}, playback: { backendProxySupported: false, streamMode: "redirect", outputFormat: "ts" }, categoryParsing: { enabled: false, mode: "off", delimiter: "pipe", regex: "", output: "" }, profileSelection: { mode: "all", profileIds: [] }, customGroups: [], customGroupMemberships: {}, savedLineups: [] };
}
function prefs() { return state.app && state.app.preferences ? state.app.preferences : defaultPrefs(); }
function availableChannelProfiles() {
  return items(state.app && state.app.source && state.app.source.profiles).filter(function(profile) {
    return profile && profile.id && profile.name;
  }).slice().sort(function(left, right) {
    return String(left.name || left.id).localeCompare(String(right.name || right.id));
  });
}
function normalizeProfileSelection(value) {
  value = value || {};
  let mode = value.mode === "selected" ? "selected" : "all";
  let profileIDs = uniqueIDs(items(value.profileIds).map(function(id) { return String(id || "").trim(); }).filter(Boolean));
  const profiles = availableChannelProfiles();
  if (profiles.length) {
    const valid = {};
    profiles.forEach(function(profile) { valid[profile.id] = true; });
    profileIDs = profileIDs.filter(function(id) { return !!valid[id]; });
  }
  if (mode === "selected" && !profileIDs.length) mode = "all";
  return { mode: mode, profileIds: mode === "all" ? [] : profileIDs };
}
function profileSelection() { return normalizeProfileSelection(prefs().profileSelection); }
function selectedProfileMap() {
  if (state.profileSelectionIDMap !== null) return state.profileSelectionIDMap || null;
  const selection = profileSelection();
  if (selection.mode !== "selected") {
    state.profileSelectionIDMap = false;
    return null;
  }
  const selected = {};
  selection.profileIds.forEach(function(id) { selected[id] = true; });
  state.profileSelectionIDMap = selected;
  return selected;
}
function profileSelectionIsAll() { return !selectedProfileMap(); }
function invalidateProfileSelectionCache() {
  state.profileSelectionIDMap = null;
  state.profileChannelFilterMap = null;
}
function selectedProfileChannelMap() {
  if (state.profileChannelFilterMap !== null) return state.profileChannelFilterMap || null;
  const selected = selectedProfileMap();
  if (!selected) {
    state.profileChannelFilterMap = false;
    return null;
  }
  const channels = {};
  items(state.app && state.app.channels).forEach(function(channel) {
    if (items(channel && channel.profileIds).some(function(id) { return !!selected[id]; })) channels[channel.id] = true;
  });
  state.profileChannelFilterMap = channels;
  return channels;
}
function channelMatchesProfileSelection(channel) {
  const allowed = selectedProfileChannelMap();
  if (!allowed) return true;
  if (!channel) return false;
  if (channel.id && allowed[channel.id]) return true;
  const selected = selectedProfileMap();
  return !channel.id && items(channel.profileIds).some(function(id) { return !!selected[id]; });
}
function defaultEventKeywordRules() {
  return [
    { categoryId: "awards", categoryName: "Awards", keywords: ["Academy Awards", "The Oscars", "Oscars", "Tony Awards", "The Tonys", "Golden Globes", "Grammy Awards", "Grammys", "Emmy Awards", "Emmys", "CMA Awards", "ACM Awards", "Billboard Music Awards", "American Music Awards", "BET Awards", "MTV Video Music Awards", "Critics Choice Awards", "SAG Awards"] },
    { categoryId: "civic", categoryName: "Civic", keywords: ["State of the Union", "Presidential Address", "Joint Session", "Inauguration", "Election Night", "Presidential Debate"] },
    { categoryId: "parades", categoryName: "Parades", keywords: ["Thanksgiving Day Parade", "Macy's Thanksgiving Day Parade", "Rose Parade", "Christmas Parade"] },
    { categoryId: "entertainment", categoryName: "Entertainment", keywords: [] }
  ];
}
function defaultAdminCategorySettings() {
  return { mode: "normal", delimiter: "pipe", virtualGroupLabel: "Groups", appDisplayName: "Live TV (Dispatcharr)", sideMenuMode: "guide", onLaterEnabled: true, virtualGroupSource: "group", collapseDuplicateVirtualGroups: true, flattenRedundantGroupWrappers: true, allowRecordingsByDefault: true, recordingPrePadMinutes: 0, recordingPostPadMinutes: 0, recordingComskipEnabled: false, sportsEnabled: true, separateSportsApp: false, sportsLibraryIds: [], sportsTeamChannels: [], sportsFirstPlayerEnabled: false, hlsBufferSeconds: 12, liveRewindEnabled: false, liveRewindCacheGB: 5, liveRewindWindowMinutes: 30, liveRewindMinFreeGB: 2, liveRewindMaxChannels: 20, inferChannelNameGroups: false, ecmEnabled: false, ecmURL: "", categoryRenames: [], categoryAliases: [], featuredEventIds: [], eventKeywords: defaultEventKeywordRules() };
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
function adminLineupDirty() {
  const current = normalizeAdminConnection(state.adminConnection);
  const saved = normalizeAdminConnection(state.savedAdminConnection);
  const signature = function(connection) {
    return JSON.stringify({
      profiles: connection.channelProfiles.slice().sort(),
      groups: connection.channelGroups.slice().sort()
    });
  };
  return signature(current) !== signature(saved);
}
function adminOrganizationDirty() {
  return adminSettingsDirty() || adminLineupDirty();
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
function isDispatcharrDirectSource() {
  const mode = sourceMode();
  return mode === "direct_login" || mode === "api_key";
}
function liveRewindEnabled() {
  return !!(isDispatcharrDirectSource() && adminSettings().liveRewindEnabled === true);
}
function sportsFirstPlayerEnabled() {
  return sportsEnabled() && (isSportsApp() || adminSettings().sportsFirstPlayerEnabled === true);
}
function sportsEnabled() { return adminSettings().sportsEnabled !== false; }
function separateSportsApp() { return sportsEnabled() && adminSettings().separateSportsApp === true; }
function isSportsApp() { return isSportsPath && sportsEnabled(); }
function sportsInLiveTVApp() { return sportsEnabled() && !separateSportsApp(); }
function defaultBrowseView() { return isSportsApp() ? "sports" : "home"; }
function liveTVOnlyView(view) {
  return ["home", "guide", "channels", "mytv", "events", "live", "favorites", "recordings"].indexOf(view) !== -1;
}
function channelGroupsInSideMenu() { return adminSettings().sideMenuMode === "channels"; }
function configuredSportsLibraryIDs() {
  const seen = {};
  return items(adminSettings().sportsLibraryIds).map(function(value) { return Number(value); }).filter(function(value) {
    if (!Number.isInteger(value) || value <= 0 || seen[value]) return false;
    seen[value] = true;
    return true;
  });
}
function sportsFirstPlayerActive() {
  return sportsFirstPlayerEnabled() && state.playerSportsMode === true;
}
function dvrEnabled() {
  return !!(state.app && state.app.capabilities && state.app.capabilities.recordings && isDispatcharrDirectSource() && adminSettings().allowRecordingsByDefault !== false);
}
function recordingSchedulingEnabled() {
  return !!(dvrEnabled() && state.recordingCapability && state.recordingCapability.canSchedule);
}
function recordingScheduleReason() {
  const capability = state.recordingCapability || {};
  return capability.reason || "Scheduling requires a Dispatcharr admin account or Admin API Key.";
}
async function loadRecordingCapability() {
  if (!dvrEnabled()) {
    state.recordingCapability = { available: false, canSchedule: false, reason: "Recordings require Dispatcharr Direct Connect." };
    return state.recordingCapability;
  }
  state.recordingCapability = await getJSON("/dispatcharr/api/recordings/capability").catch(function() {
    return { available: true, canSchedule: false, reason: "Unable to verify Dispatcharr recording permissions." };
  });
  return state.recordingCapability;
}
function favoriteMap() { return prefs().favorites || {}; }
function autoFavoriteMap() { return prefs().autoFavorites || {}; }
function hiddenMap() { return prefs().hiddenCategories || {}; }
function sportsFavoriteTeamMap() { return prefs().sportsFavoriteTeams || {}; }
function sportsFavoriteLeagueMap() { return prefs().sportsFavoriteLeagues || {}; }
function sportsPreferredChannelMap() { return prefs().sportsPreferredChannels || {}; }
function sportsPreferredNetworkMap() { return prefs().sportsPreferredNetworks || {}; }
function sportsScoresHidden(playerOnly) { return !!(prefs().sportsSpoilersHidden || (playerOnly && prefs().sportsPlayerSpoilersHidden)); }
function featuredEventMap() { return prefs().featuredEvents || {}; }
function adminFeaturedEventMap() {
  const featured = {};
  items(adminSettings().featuredEventIds).forEach(function(id) {
    id = String(id || "").trim();
    if (id) featured[id] = true;
  });
  return featured;
}
function normalizeKeywordPasses(value) {
  const seen = {};
  return items(value).map(function(pass) {
    pass = pass || {};
    const keyword = String(pass.keyword || pass.name || "").trim();
    if (!keyword) return null;
    const id = String(pass.id || ("keyword:" + lower(keyword).replace(/[^a-z0-9]+/g, "-"))).replace(/^-+|-+$/g, "");
    const key = lower(keyword);
    if (!id || seen[key]) return null;
    seen[key] = true;
    return { id: id, keyword: keyword, createdAt: Number(pass.createdAt || Date.now()) };
  }).filter(Boolean).slice(0, 24);
}
function keywordPasses() { return normalizeKeywordPasses(prefs().keywordPasses); }
function normalizeSavedLineups(value) {
  const seenIDs = {};
  const seenCategories = {};
  return items(value).map(function(lineup, index) {
    lineup = lineup || {};
    const categoryID = String(lineup.categoryId || "").trim();
    const name = String(lineup.name || "").trim();
    let id = String(lineup.id || "").trim();
    if (!categoryID || !name || seenCategories[categoryID]) return null;
    if (!id) id = "lineup-" + String(index + 1);
    if (seenIDs[id]) id += "-" + String(index + 1);
    seenIDs[id] = true;
    seenCategories[categoryID] = true;
    return { id: id, name: name, categoryId: categoryID, hideFinalGroups: lineup.hideFinalGroups === true };
  }).filter(Boolean).slice(0, 24);
}
function savedLineups() { return normalizeSavedLineups(prefs().savedLineups); }
function savedLineupByID(id) {
  return savedLineups().find(function(lineup) { return lineup.id === String(id || ""); }) || null;
}
function savedLineupForCategory(categoryID) {
  return savedLineups().find(function(lineup) { return lineup.categoryId === String(categoryID || ""); }) || null;
}
function mergePrefs(remote) {
  remote = Object.assign(defaultPrefs(), remote || {});
  const merged = {
    favorites: Object.assign({}, remote.favorites),
    favoriteOrder: uniqueIDs(items(remote.favoriteOrder)),
    autoFavorites: Object.assign({}, remote.autoFavorites),
    hiddenCategories: Object.assign({}, remote.hiddenCategories),
    sportsFavoriteTeams: Object.assign({}, remote.sportsFavoriteTeams),
    sportsFavoriteTeamLabels: Object.assign({}, remote.sportsFavoriteTeamLabels),
    sportsFavoriteLeagues: Object.assign({}, remote.sportsFavoriteLeagues),
    sportsPreferredChannels: Object.assign({}, remote.sportsPreferredChannels),
    sportsPreferredNetworks: Object.assign({}, remote.sportsPreferredNetworks),
    sportsSpoilersHidden: remote.sportsSpoilersHidden === true,
    sportsPlayerSpoilersHidden: remote.sportsPlayerSpoilersHidden === true,
    featuredEvents: Object.assign({}, remote.featuredEvents),
    keywordPasses: normalizeKeywordPasses(remote.keywordPasses),
    recentSearches: uniqueIDs(items(remote.recentSearches).map(function(value) { return String(value || "").trim(); }).filter(Boolean)).slice(0, 12),
    recentChannels: uniqueIDs(items(remote.recentChannels)).slice(0, 24),
    continueWatching: Object.assign({}, remote.continueWatching),
    playback: Object.assign({}, remote.playback),
    categoryParsing: Object.assign({}, remote.categoryParsing),
    profileSelection: normalizeProfileSelection(remote.profileSelection),
    customGroups: items(remote.customGroups),
    customGroupMemberships: Object.assign({}, remote.customGroupMemberships),
    savedLineups: normalizeSavedLineups(remote.savedLineups)
  };
  // The stored per-profile sections ride along on the raw remote blob only; the
  // viewer's own view never carries them (see profileView).
  if (prefsPlainObject(remote.profiles)) merged.profiles = Object.assign({}, remote.profiles);
  return merged;
}
function normalizePreferences() {
  if (!state.app || !state.app.preferences) return;
  state.app.preferences = Object.assign(defaultPrefs(), state.app.preferences || {});
  state.app.preferences.categoryParsing = Object.assign(defaultPrefs().categoryParsing, state.app.preferences.categoryParsing || {});
  state.app.preferences.profileSelection = normalizeProfileSelection(state.app.preferences.profileSelection);
  state.app.preferences.sportsFavoriteTeams = state.app.preferences.sportsFavoriteTeams || {};
  state.app.preferences.sportsFavoriteTeamLabels = state.app.preferences.sportsFavoriteTeamLabels || {};
  state.app.preferences.sportsFavoriteLeagues = state.app.preferences.sportsFavoriteLeagues || {};
  state.app.preferences.sportsPreferredChannels = state.app.preferences.sportsPreferredChannels || {};
  state.app.preferences.sportsPreferredNetworks = state.app.preferences.sportsPreferredNetworks || {};
  state.app.preferences.sportsSpoilersHidden = state.app.preferences.sportsSpoilersHidden === true;
  state.app.preferences.sportsPlayerSpoilersHidden = state.app.preferences.sportsPlayerSpoilersHidden === true;
  state.app.preferences.featuredEvents = state.app.preferences.featuredEvents || {};
  state.app.preferences.keywordPasses = normalizeKeywordPasses(state.app.preferences.keywordPasses);
  state.app.preferences.recentSearches = uniqueIDs(items(state.app.preferences.recentSearches).map(function(value) { return String(value || "").trim(); }).filter(Boolean)).slice(0, 12);
  state.app.preferences.customGroups = items(state.app.preferences.customGroups);
  state.app.preferences.customGroupMemberships = state.app.preferences.customGroupMemberships || {};
  state.app.preferences.savedLineups = normalizeSavedLineups(state.app.preferences.savedLineups);
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
  invalidateProfileSelectionCache();
}
function normalizeAdminCategorySettings() {
  state.adminCategorySettings = Object.assign(defaultAdminCategorySettings(), state.adminCategorySettings || {});
  const appDisplayName = String(state.adminCategorySettings.appDisplayName || "").trim();
  state.adminCategorySettings.appDisplayName = Array.from(appDisplayName || "Live TV (Dispatcharr)").slice(0, 80).join("");
  state.adminCategorySettings.sideMenuMode = state.adminCategorySettings.sideMenuMode === "channels" ? "channels" : "guide";
  if (state.adminCategorySettings.mode === "custom" || state.adminCategorySettings.mode === "admin_delimiter") state.adminCategorySettings.mode = "delimiter";
  if (["normal", "delimiter"].indexOf(state.adminCategorySettings.mode) === -1) state.adminCategorySettings.mode = "normal";
  if (!state.adminCategorySettings.delimiter) state.adminCategorySettings.delimiter = "pipe";
  if (state.adminCategorySettings.delimiter !== "pipe" && state.adminCategorySettings.delimiter !== "dash") state.adminCategorySettings.delimiter = "pipe";
  state.adminCategorySettings.virtualGroupLabel = virtualGroupLabelSuffix(state.adminCategorySettings.virtualGroupLabel);
  state.adminCategorySettings.allowRecordingsByDefault = state.adminCategorySettings.allowRecordingsByDefault !== false;
  state.adminCategorySettings.recordingPrePadMinutes = Math.max(0, Math.min(60, Math.round(Number(state.adminCategorySettings.recordingPrePadMinutes) || 0)));
  state.adminCategorySettings.recordingPostPadMinutes = Math.max(0, Math.min(60, Math.round(Number(state.adminCategorySettings.recordingPostPadMinutes) || 0)));
  state.adminCategorySettings.recordingComskipEnabled = state.adminCategorySettings.recordingComskipEnabled === true;
  state.adminCategorySettings.sportsEnabled = state.adminCategorySettings.sportsEnabled !== false;
  state.adminCategorySettings.separateSportsApp = state.adminCategorySettings.separateSportsApp === true;
  state.adminCategorySettings.sportsLibraryIds = uniqueIDs(items(state.adminCategorySettings.sportsLibraryIds).map(function(value) {
    const id = Number(value);
    return Number.isInteger(id) && id > 0 ? String(id) : "";
  }).filter(Boolean)).map(function(value) { return Number(value); });
  const seenTeamPins = {};
  state.adminCategorySettings.sportsTeamChannels = items(state.adminCategorySettings.sportsTeamChannels).map(function(pin) {
    return { leagueId: String((pin && pin.leagueId) || "").trim(), teamName: String((pin && pin.teamName) || "").trim(), channelId: String((pin && pin.channelId) || "").trim(), channelName: String((pin && pin.channelName) || "").trim() };
  }).filter(function(pin) {
    const key = pin.leagueId + "|" + lower(pin.teamName);
    if (!pin.leagueId || !pin.teamName || !pin.channelId || seenTeamPins[key]) return false;
    seenTeamPins[key] = true;
    return true;
  }).slice(0, 200);
  state.adminCategorySettings.sportsFirstPlayerEnabled = state.adminCategorySettings.sportsFirstPlayerEnabled === true;
  state.adminCategorySettings.featuredEventIds = uniqueIDs(items(state.adminCategorySettings.featuredEventIds).map(function(id) {
    return String(id || "").trim();
  }).filter(Boolean)).slice(0, 100);
  state.adminCategorySettings.hlsBufferSeconds = Math.max(5, Math.min(60, Math.round(Number(state.adminCategorySettings.hlsBufferSeconds) || 12)));
  state.adminCategorySettings.liveRewindEnabled = state.adminCategorySettings.liveRewindEnabled === true;
  state.adminCategorySettings.liveRewindCacheGB = Math.max(1, Math.min(500, Number(state.adminCategorySettings.liveRewindCacheGB) || 5));
  state.adminCategorySettings.liveRewindWindowMinutes = [15, 30, 60, 90, 120].indexOf(Number(state.adminCategorySettings.liveRewindWindowMinutes)) !== -1 ? Number(state.adminCategorySettings.liveRewindWindowMinutes) : 30;
  state.adminCategorySettings.liveRewindMinFreeGB = Math.max(1, Math.min(100, Number(state.adminCategorySettings.liveRewindMinFreeGB) || 2));
  state.adminCategorySettings.liveRewindMaxChannels = Math.max(1, Math.min(100, Math.round(Number(state.adminCategorySettings.liveRewindMaxChannels) || 20)));
  if (typeof state.adminCategorySettings.collapseDuplicateVirtualGroups === "undefined" && typeof state.adminCategorySettings.collapseDuplicateProfileGroups !== "undefined") {
    state.adminCategorySettings.collapseDuplicateVirtualGroups = state.adminCategorySettings.collapseDuplicateProfileGroups;
  }
  state.adminCategorySettings.collapseDuplicateVirtualGroups = state.adminCategorySettings.collapseDuplicateVirtualGroups !== false;
  state.adminCategorySettings.flattenRedundantGroupWrappers = state.adminCategorySettings.flattenRedundantGroupWrappers !== false;
  delete state.adminCategorySettings.collapseDuplicateProfileGroups;
  state.adminCategorySettings.virtualGroupSource = normalizeVirtualGroupSource(state.adminCategorySettings.virtualGroupSource, state.adminCategorySettings.inferChannelNameGroups === true);
  if (state.adminCategorySettings.virtualGroupSource === "profile" || state.adminCategorySettings.virtualGroupSource === "profile_group") state.adminCategorySettings.mode = "delimiter";
  state.adminCategorySettings.inferChannelNameGroups = state.adminCategorySettings.virtualGroupSource !== "group";
  state.adminCategorySettings.ecmURL = normalizeAdminECMURL(state.adminCategorySettings.ecmURL);
  state.adminCategorySettings.ecmEnabled = !!state.adminCategorySettings.ecmURL;
  state.adminCategorySettings.categoryRenames = [];
  state.adminCategorySettings.categoryAliases = normalizeCategoryAliases(state.adminCategorySettings.categoryAliases);
  state.adminCategorySettings.eventKeywords = normalizeEventKeywordRows(state.adminCategorySettings.eventKeywords);
  delete state.adminCategorySettings.groupAliases;
  delete state.adminCategorySettings.adminGroups;
  delete state.adminCategorySettings.adminGroupMemberships;
  delete state.adminCategorySettings.presentationOverrides;
}
function normalizeVirtualGroupSource(value, inferLegacy) {
  const mode = String(value || "").trim();
  if (mode === "group" || mode === "group_channel" || mode === "channel" || mode === "profile" || mode === "profile_group") return mode;
  return inferLegacy ? "group_channel" : "group";
}
function virtualGroupSourceMode() {
  return normalizeVirtualGroupSource(adminSettings().virtualGroupSource, adminSettings().inferChannelNameGroups === true);
}
function useSourceGroupVirtualPaths() {
  const source = virtualGroupSourceMode();
  return source === "group" || source === "group_channel" || source === "profile_group";
}
function useChannelNameVirtualPaths() {
  const source = virtualGroupSourceMode();
  return source === "channel" || source === "group_channel";
}
function useProfileGroupVirtualPaths() {
  return virtualGroupSourceMode() === "profile_group";
}
function useChannelProfileVirtualPaths() {
  const source = virtualGroupSourceMode();
  return source === "profile" || source === "profile_group";
}
function organizationLayerState() {
  const source = virtualGroupSourceMode();
  return {
    profiles: source === "profile" || source === "profile_group",
    groups: source === "group" || source === "group_channel" || source === "profile_group",
    channels: source === "channel" || source === "group_channel"
  };
}
function normalizeCategoryRenames(value) {
  const seen = {};
  return items(value).map(function(rename) {
    return {
      sourcePath: String((rename && rename.sourcePath) || "").trim(),
      displayName: String((rename && (rename.displayName || rename.aliasPath)) || "").trim()
    };
  }).filter(function(rename) {
    const key = lower(rename.sourcePath);
    if (!rename.sourcePath || !rename.displayName || seen[key]) return false;
    seen[key] = true;
    return true;
  });
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
    const excludeKeywords = normalizeKeywordList(row.excludeKeywords);
    const eventSeries = row.eventSeries === true;
    const groupWindowMinutes = eventSeries ? Math.max(15, Math.min(360, Number(row.groupWindowMinutes) || 60)) : 0;
    return { categoryId: categoryId, categoryName: categoryName, keywords: keywords, excludeKeywords: excludeKeywords, eventSeries: eventSeries, groupWindowMinutes: groupWindowMinutes };
  }).filter(function(row) { return row.categoryId && row.keywords.length && !isSportsEventCategoryId(row.categoryId); });
  const byID = {};
  defaults.concat(rows).forEach(function(row) {
    const id = normalizeEventCategoryId(row.categoryId || row.categoryName);
    if (!id) return;
    const existing = byID[id] || { categoryId: id, categoryName: row.categoryName || eventCategoryName(id), keywords: [], excludeKeywords: [], eventSeries: false, groupWindowMinutes: 0 };
    existing.keywords = normalizeKeywordList(existing.keywords.concat(row.keywords || []));
    existing.excludeKeywords = normalizeKeywordList(existing.excludeKeywords.concat(row.excludeKeywords || []));
    existing.eventSeries = existing.eventSeries || row.eventSeries === true;
    existing.groupWindowMinutes = existing.eventSeries ? Math.max(15, Math.min(360, Number(row.groupWindowMinutes || existing.groupWindowMinutes) || 60)) : 0;
    byID[id] = existing;
  });
  return Object.keys(byID).sort(function(left, right) {
    return eventCategoryName(left).localeCompare(eventCategoryName(right));
  }).map(function(id) {
    const row = byID[id];
    if (!row.eventSeries) delete row.groupWindowMinutes;
    return row;
  });
}
function isSportsEventCategoryId(categoryId) {
  return ["golf", "motor-racing", "combat-sports", "tennis"].indexOf(normalizeEventCategoryId(categoryId)) !== -1;
}
function normalizeKeywordList(value) {
  const rows = Array.isArray(value)
    ? value.reduce(function(list, item) { return list.concat(String(item || "").split(/\\n|[\n,]+/)); }, [])
    : String(value || "").split(/\\n|[\n,]+/);
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
function renamedCategoryDisplayName(rawName) {
  return categoryDisplayName(rawName);
}
function normalizeAdminECMURL(value) {
  const fallback = "";
  const trimmed = String(value || "").trim();
  const lower = trimmed.toLowerCase();
  if (lower.indexOf("https://") === 0 || lower.indexOf("http://") === 0) return trimmed;
  return fallback;
}
function adminECMEnabled() {
  return !!adminECMURL();
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
function readRecentSearches() {
  return uniqueIDs(items(prefs().recentSearches).map(function(value) { return String(value || "").trim(); }).filter(Boolean)).slice(0, 12);
}
function writeRecentSearches(searches) {
  state.recentSearches = uniqueIDs(items(searches).map(function(value) { return String(value || "").trim(); }).filter(Boolean)).slice(0, 12);
  if (state.app && state.app.preferences) {
    state.app.preferences.recentSearches = state.recentSearches;
    savePrefs({ quiet: true });
  }
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
  const payload = await coreGetJSON(siloCoreURL("/settings/plugins/" + encodeURIComponent(pluginInstallationID)));
  return payload && payload.values ? payload.values : {};
}
async function loadUserPrefs() {
  const values = await loadPluginSettingsValues();
  return viewerPrefsView(readSiloPrefsValue(values ? values.preferences : ""));
}
async function loadAdminCategorySettings() {
  return readAdminSettingsValue(await getJSON(adminSettingsURL()));
}
function adminSettingsURL() {
  return "/dispatcharr/api/admin-settings";
}
// Every plugin-settings write goes through one queue. A write always starts
// from a fresh, successful read so keys owned by other features (or other
// tabs) are never dropped; a failed read fails the write instead.
let pluginSettingsWriteChain = Promise.resolve();
function queuePluginSettingsWrite(task) {
  const run = pluginSettingsWriteChain.then(task, task);
  pluginSettingsWriteChain = run.catch(function() {});
  return run;
}
async function writePluginSettingsValues(update) {
  if (!pluginInstallationID) throw new Error("plugin installation settings unavailable");
  const loaded = await loadPluginSettingsValues();
  const values = Object.assign({}, loaded || {});
  const next = update(values) || values;
  await corePutNoContent(siloCoreURL("/settings/plugins/" + encodeURIComponent(pluginInstallationID)), { values: next });
  return next;
}
function savePluginSettingValue(key, value) {
  return queuePluginSettingsWrite(function() {
    return writePluginSettingsValues(function(values) {
      values[key] = value;
      return values;
    });
  });
}
async function persistAdminCategorySettingsInSilo(settings) {
  if (!pluginInstallationID) throw new Error("plugin installation settings unavailable");
  await corePutNoContent(siloCoreURL("/admin/plugins/installations/" + encodeURIComponent(pluginInstallationID) + "/config"), {
    key: "category_settings",
    value: settings
  });
}
// Preference sync. `prefsBaseline` is the last remote state this tab knows
// about (normalized). Local edits are the diff between it and
// state.app.preferences; every save re-reads the remote blob and applies only
// that diff, so concurrent tabs/devices do not overwrite each other.
const prefsSaveDebounceMs = 400;
const prefsMapKeys = ["favorites", "autoFavorites", "hiddenCategories", "sportsFavoriteTeams", "sportsFavoriteTeamLabels", "sportsFavoriteLeagues", "sportsPreferredChannels", "sportsPreferredNetworks", "featuredEvents", "continueWatching", "customGroupMemberships", "playback", "categoryParsing"];
const prefsUnionListKeys = { favoriteOrder: 0, recentSearches: 12, recentChannels: 24 };
// `profileID` is the Silo profile state.app.preferences and the baseline were
// loaded for; a save only ever writes into that profile's section.
const prefsSync = { loaded: false, profileID: "", baseline: null, dirty: false, quiet: true, timer: 0, retryTimer: 0, loading: null, lastRemoteCheck: 0, channel: null, tabID: Math.random().toString(36).slice(2) };
function prefsSnapshot(value) { return JSON.parse(JSON.stringify(value == null ? {} : value)); }
function prefsValueEqual(left, right) { return JSON.stringify(left === undefined ? null : left) === JSON.stringify(right === undefined ? null : right); }
function prefsPlainObject(value) { return !!value && typeof value === "object" && !Array.isArray(value); }
// Ordered ID lists: apply this tab's additions/removals to the remote list.
// favoriteOrder appends additions (unless this tab reordered, then its order
// wins); recent* lists are most-recent-first, so additions go in front.
function mergePrefsList(key, remote, baseline, local) {
  remote = uniqueIDs(remote);
  baseline = uniqueIDs(baseline);
  local = uniqueIDs(local);
  const inLocal = {};
  const inBaseline = {};
  local.forEach(function(value) { inLocal[value] = true; });
  baseline.forEach(function(value) { inBaseline[value] = true; });
  const added = local.filter(function(value) { return !inBaseline[value]; });
  const kept = remote.filter(function(value) { return !inBaseline[value] || inLocal[value]; });
  const sharedLocal = local.filter(function(value) { return inBaseline[value]; });
  const sharedBaseline = baseline.filter(function(value) { return inLocal[value]; });
  let merged;
  if (key !== "favoriteOrder") merged = uniqueIDs(added.concat(sharedLocal, kept));
  else if (!prefsValueEqual(sharedLocal, sharedBaseline)) merged = uniqueIDs(local.concat(kept));
  else merged = uniqueIDs(kept.concat(added));
  const limit = prefsUnionListKeys[key];
  return limit ? merged.slice(0, limit) : merged;
}
function applyPrefsDiff(remote, baseline, local) {
  const result = prefsSnapshot(remote);
  baseline = baseline || {};
  local = local || {};
  const keys = uniqueIDs(Object.keys(baseline).concat(Object.keys(local)));
  keys.forEach(function(key) {
    if (prefsValueEqual(local[key], baseline[key])) return;
    if (prefsMapKeys.indexOf(key) !== -1 && prefsPlainObject(local[key])) {
      const before = prefsPlainObject(baseline[key]) ? baseline[key] : {};
      const target = prefsPlainObject(result[key]) ? result[key] : {};
      uniqueIDs(Object.keys(before).concat(Object.keys(local[key]))).forEach(function(subKey) {
        if (prefsValueEqual(local[key][subKey], before[subKey])) return;
        if (Object.prototype.hasOwnProperty.call(local[key], subKey)) target[subKey] = prefsSnapshot(local[key][subKey]);
        else delete target[subKey];
      });
      result[key] = target;
      return;
    }
    if (Object.prototype.hasOwnProperty.call(prefsUnionListKeys, key) && Array.isArray(local[key])) {
      result[key] = mergePrefsList(key, items(result[key]), items(baseline[key]), local[key]);
      return;
    }
    if (local[key] === undefined) delete result[key];
    else result[key] = prefsSnapshot(local[key]);
  });
  if (result.favorites && Array.isArray(result.favoriteOrder)) {
    result.favoriteOrder = result.favoriteOrder.filter(function(id) { return !!result.favorites[id]; });
  }
  return result;
}
// Raw stored shape: legacy top-level keys plus the per-profile `profiles` map.
// Use viewerPrefsView/profileView for what a tab shows and diffs against.
function remotePrefsFromValues(values) {
  return mergePrefs(readSiloPrefsValue(values && values.preferences ? values.preferences : "") || defaultPrefs());
}
// Favorites, their order and hidden groups are kept per Silo profile under
// preferences.profiles[profileID]. Everything else stays top-level. A profile
// with no section yet shows the legacy top-level values, and so does a viewer
// with no profile id.
const profileScopedPrefKeys = ["favorites", "favoriteOrder", "hiddenCategories"];
function activeViewerProfileID() { return coreStoredValue("profile_id") || ""; }
function prefsProfileSection(remote, profileID) {
  const section = profileID && remote && prefsPlainObject(remote.profiles) ? remote.profiles[profileID] : null;
  return prefsPlainObject(section) ? section : null;
}
// A copy of the stored prefs as the given profile sees them. `profiles` is
// removed so the per-profile map is never diffed or edited by the tab.
function profileView(remote, profileID) {
  const view = prefsSnapshot(remote);
  delete view.profiles;
  const section = prefsProfileSection(remote, profileID);
  if (!section) return view;
  profileScopedPrefKeys.forEach(function(key) {
    if (Object.prototype.hasOwnProperty.call(section, key)) view[key] = prefsSnapshot(section[key]);
    else delete view[key];
  });
  return view;
}
function viewerPrefsView(remote) {
  return remote ? profileView(remote, activeViewerProfileID()) : null;
}
// Inverse of profileView: `merged` is the viewer's view after a save diff and
// `remote` the raw stored prefs it was merged against. Scoped keys go into
// profiles[profileID] (the legacy top-level ones keep the stored values), every
// other key comes from `merged`, and the other profiles are carried over.
function mergeProfileView(remote, merged, profileID) {
  const result = prefsSnapshot(merged);
  delete result.profiles;
  if (remote && prefsPlainObject(remote.profiles)) result.profiles = prefsSnapshot(remote.profiles);
  if (!profileID) return result;
  const section = Object.assign({}, prefsProfileSection(remote, profileID));
  profileScopedPrefKeys.forEach(function(key) {
    if (remote && Object.prototype.hasOwnProperty.call(remote, key)) result[key] = prefsSnapshot(remote[key]);
    else delete result[key];
    if (Object.prototype.hasOwnProperty.call(merged, key)) section[key] = prefsSnapshot(merged[key]);
    else delete section[key];
  });
  result.profiles = Object.assign({}, result.profiles);
  result.profiles[profileID] = section;
  return result;
}
// Silo switches profiles inside its SPA without reloading the page, so the
// viewer can change while this tab still holds the previous profile's view.
function prefsProfileSwitched() {
  return prefsSync.loaded && prefsSync.profileID !== activeViewerProfileID();
}
// Edits pending for a profile the viewer has left must not be saved anywhere.
function discardPendingPrefsEdits() {
  if (prefsSync.timer) { clearTimeout(prefsSync.timer); prefsSync.timer = 0; }
  prefsSync.dirty = false;
  prefsSync.quiet = true;
}
// Called after state.app.preferences has been replaced by freshly loaded,
// normalized remote prefs. Re-applies edits made while prefs were not loaded.
// `profileID` is the profile that view was built for (default: the current one).
function adoptLoadedPrefs(previousLocal, profileID) {
  const nextProfileID = profileID === undefined ? activeViewerProfileID() : profileID;
  if (prefsSync.loaded && prefsSync.profileID !== nextProfileID) discardPendingPrefsEdits();
  const previousBaseline = prefsSync.baseline;
  const hadPendingEdits = prefsSync.dirty && previousBaseline && previousLocal;
  prefsSync.loaded = true;
  prefsSync.profileID = nextProfileID;
  prefsSync.baseline = prefsSnapshot(state.app.preferences);
  prefsSync.lastRemoteCheck = Date.now();
  if (prefsSync.retryTimer) { clearTimeout(prefsSync.retryTimer); prefsSync.retryTimer = 0; }
  const droppedKnown = dropKnownPhantomSportsFollows();
  if (typeof dropPhantomSportsFollows === "function") dropPhantomSportsFollows();
  if (hadPendingEdits) {
    state.app.preferences = applyPrefsDiff(state.app.preferences, previousBaseline, previousLocal);
    normalizePreferences();
    dropKnownPhantomSportsFollows();
    schedulePrefsFlush();
  } else if (droppedKnown) {
    schedulePrefsFlush();
  }
}
// Follow IDs a past backend build emitted for a nameless phantom team. They
// are removed on every successful prefs load (baseline is already set, so the
// removal is a diff and persists through the normal save queue).
const knownPhantomSportsFollowIDs = ["sports-team:cbe5cfdf7c2118a9"];
function dropKnownPhantomSportsFollows() {
  const preferences = state.app && state.app.preferences;
  if (!preferences) return false;
  let dropped = false;
  ["sportsFavoriteTeams", "sportsFavoriteTeamLabels"].forEach(function(key) {
    const map = preferences[key];
    if (!map || typeof map !== "object") return;
    knownPhantomSportsFollowIDs.forEach(function(id) {
      if (!Object.prototype.hasOwnProperty.call(map, id)) return;
      delete map[id];
      dropped = true;
    });
  });
  return dropped;
}
function schedulePrefsLoadRetry() {
  if ((prefsSync.loaded && !prefsProfileSwitched()) || prefsSync.retryTimer || !pluginInstallationID) return;
  prefsSync.retryTimer = setTimeout(function() {
    prefsSync.retryTimer = 0;
    reloadPrefsFromRemote({ force: true }).catch(function() { schedulePrefsLoadRetry(); });
  }, 15000);
}
async function reloadPrefsFromRemote(options) {
  options = options || {};
  if (!state.app || !pluginInstallationID || isAdminRoute) return;
  if (!options.force && !prefsProfileSwitched() && Date.now() - prefsSync.lastRemoteCheck < 5000) return;
  if (prefsSync.loading) return prefsSync.loading;
  prefsSync.loading = (async function() {
    const values = await loadPluginSettingsValues();
    const profileID = activeViewerProfileID();
    const switched = prefsProfileSwitched();
    const before = prefsSnapshot(state.app.preferences);
    const local = state.app.preferences;
    if (switched) {
      // The pending edits belong to the previous profile; show the new one's view.
      discardPendingPrefsEdits();
    } else if (prefsSync.loaded && prefsSync.dirty) {
      // A save is pending; it re-reads and merges remote state itself.
      prefsSync.lastRemoteCheck = Date.now();
      return;
    }
    state.app.preferences = profileView(remotePrefsFromValues(values), profileID);
    normalizePreferences();
    adoptLoadedPrefs(switched ? null : local, profileID);
    if (!prefsValueEqual(before, state.app.preferences)) renderAfterPrefsSync();
  })().finally(function() { prefsSync.loading = null; });
  return prefsSync.loading;
}
function renderAfterPrefsSync() {
  if (!state.app || state.view === "player" || state.view === "multiview" || state.programDetails) return;
  try { render(); } catch (error) {
    try { console.warn("Dispatcharr render after preference sync failed", error); } catch (_) {}
  }
}
function prefsSyncChannelKey() { return "silo.ramindex.dispatcharr.prefsSync.v1." + localCacheSuffix; }
function announcePrefsSaved() {
  const message = { type: "prefs-saved", tab: prefsSync.tabID, at: Date.now() };
  try { if (prefsSync.channel) { prefsSync.channel.postMessage(message); return; } } catch (_) {}
  try { localStorage.setItem(prefsSyncChannelKey(), JSON.stringify(message)); } catch (_) {}
}
function handlePrefsSyncMessage(message) {
  if (!message || message.type !== "prefs-saved" || message.tab === prefsSync.tabID) return;
  reloadPrefsFromRemote({ force: true }).catch(function(error) {
    try { console.warn("Dispatcharr cross-tab preference reload failed", error); } catch (_) {}
  });
}
function startPrefsSync() {
  if (isAdminRoute || prefsSync.started) return;
  prefsSync.started = true;
  try {
    if (typeof BroadcastChannel === "function") {
      prefsSync.channel = new BroadcastChannel(prefsSyncChannelKey());
      prefsSync.channel.onmessage = function(event) { handlePrefsSyncMessage(event && event.data); };
    }
  } catch (_) { prefsSync.channel = null; }
  if (!prefsSync.channel) {
    window.addEventListener("storage", function(event) {
      if (!event || event.key !== prefsSyncChannelKey() || !event.newValue) return;
      try { handlePrefsSyncMessage(JSON.parse(event.newValue)); } catch (_) {}
    });
  }
  document.addEventListener("visibilitychange", function() {
    if (document.hidden) {
      if (prefsSync.dirty && prefsSync.loaded) flushPrefsSave();
      return;
    }
    reloadPrefsFromRemote().catch(function() { schedulePrefsLoadRetry(); });
  });
}
function schedulePrefsFlush() {
  prefsSync.dirty = true;
  if (prefsSync.timer) clearTimeout(prefsSync.timer);
  prefsSync.timer = setTimeout(flushPrefsSave, prefsSaveDebounceMs);
}
function flushPrefsSave() {
  if (prefsSync.timer) { clearTimeout(prefsSync.timer); prefsSync.timer = 0; }
  return queuePluginSettingsWrite(runPrefsSave);
}
const profileSwitchedDuringSave = new Error("viewer profile changed during save");
function dropEditsForSwitchedProfile() {
  discardPendingPrefsEdits();
  return reloadPrefsFromRemote({ force: true }).catch(function() { schedulePrefsLoadRetry(); });
}
async function runPrefsSave() {
  if (!state.app || !prefsSync.dirty || !prefsSync.loaded) return;
  if (prefsProfileSwitched()) return dropEditsForSwitchedProfile();
  const quiet = prefsSync.quiet;
  prefsSync.dirty = false;
  prefsSync.quiet = true;
  const local = prefsSnapshot(state.app.preferences);
  const baseline = prefsSync.baseline;
  // Always the profile the edits were made for, never whoever is viewing now.
  const profileID = prefsSync.profileID;
  let merged = null;
  try {
    await writePluginSettingsValues(function(values) {
      // The profile may have switched while the settings were being read.
      if (activeViewerProfileID() !== profileID) throw profileSwitchedDuringSave;
      const remoteRaw = remotePrefsFromValues(values);
      merged = applyPrefsDiff(profileView(remoteRaw, profileID), baseline, local);
      values.preferences = JSON.stringify(mergeProfileView(remoteRaw, merged, profileID));
      return values;
    });
  } catch (error) {
    if (error === profileSwitchedDuringSave) return dropEditsForSwitchedProfile();
    prefsSync.dirty = true;
    state.profileSaveStatus = "error";
    state.profileSaveMessage = "Could not save to your Silo profile.";
    if (!quiet) showAppToast(state.profileSaveMessage);
    if (state.view === "settings") renderSettings();
    try { console.warn("Dispatcharr profile preference save failed", error); } catch (_) {}
    if (!prefsSync.retryTimer) {
      prefsSync.retryTimer = setTimeout(function() {
        prefsSync.retryTimer = 0;
        if (prefsSync.dirty) flushPrefsSave();
      }, 15000);
    }
    return;
  }
  const inFlightEdits = state.app.preferences;
  state.app.preferences = merged;
  normalizePreferences();
  prefsSync.baseline = prefsSnapshot(state.app.preferences);
  prefsSync.lastRemoteCheck = Date.now();
  state.app.preferences = applyPrefsDiff(state.app.preferences, local, inFlightEdits);
  state.profileSaveStatus = prefsSync.dirty ? "saving" : "saved";
  state.profileSaveMessage = prefsSync.dirty ? "" : "Saved to your Silo profile.";
  announcePrefsSaved();
  if (!prefsValueEqual(local, merged)) renderAfterPrefsSync();
  else if (state.view === "settings") renderSettings();
}
function savePrefs(options) {
  if (!state.app || !state.app.preferences) return;
  options = options || {};
  if (!pluginInstallationID) {
    state.profileSaveStatus = "error";
    state.profileSaveMessage = "Could not save to your Silo profile.";
    if (!options.quiet) showAppToast(state.profileSaveMessage);
    return;
  }
  if (!options.quiet) prefsSync.quiet = false;
  state.profileSaveStatus = "saving";
  state.profileSaveMessage = "";
  if (!prefsSync.loaded) {
    // Real prefs have not loaded yet; never write defaults over them. Keep the
    // edit pending and flush it once a load succeeds.
    prefsSync.dirty = true;
    schedulePrefsLoadRetry();
    return;
  }
  schedulePrefsFlush();
}
