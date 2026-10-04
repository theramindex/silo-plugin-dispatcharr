const path = window.location.pathname;
const isAdminRoute = path.endsWith("/dispatcharr/admin");
const isPlayerRoute = path.endsWith("/dispatcharr/player");
const isSportsPath = path.endsWith("/dispatcharr/sports");
const base = isPlayerRoute ? path.slice(0, -"/dispatcharr/player".length) : (isAdminRoute ? path.slice(0, -"/dispatcharr/admin".length) : (isSportsPath ? path.slice(0, -"/dispatcharr/sports".length) : (path.endsWith("/dispatcharr") ? path.slice(0, -"/dispatcharr".length) : "")));
const adminSettingsKey = "adminCategorySettings";
const pluginInstallationID = (base.match(/\/api\/v\d+(?:\/plugin-content)?\/plugins\/(\d+)/) || [])[1] || "";
const siloAPIPrefix = (base.match(/\/api\/v\d+/) || ["/api/v1"])[0];
const localCacheSuffix = pluginInstallationID || "default";
const appCacheKey = "silo.ramindex.dispatcharr.appSnapshot.v1." + localCacheSuffix;
const assetVersionMeta = document.querySelector('meta[name="dispatcharr-asset-version"]');
const assetVersion = assetVersionMeta ? String(assetVersionMeta.content || "") : "";
const assetPrefix = path.endsWith("/dispatcharr") ? "dispatcharr/assets" : "assets";
const state = { app: null, appLoadedFromCache: false, programsByChannel: {}, sortedPrograms: [], view: isAdminRoute ? "admin" : (isSportsPath ? "sports" : "home"), category: "", query: "", folderQuery: "", folderGroupCategoryID: "", folderGroupPickerOpen: false, searchQuery: "", searchType: "all", searchAiringChannel: "", searchReturnView: "home", recentSearches: [], myTVQuery: "", onLaterTime: "all", onLaterType: "all", hls: null, tsPlayer: null, currentChannel: null, currentSession: null, heartbeat: null, muted: false, volume: 1, volumeMenuOpen: false, audioMenuOpen: false, moreMenuOpen: false, playerGuideOpen: false, playerGuideQuery: "", playerSportsMode: false, playerSportsOpen: false, playerSportsMoreOpen: false, playerSportsTimer: null, playerReturnContext: null, selectedAudioTrack: 0, selectedTextTrack: -1, aspectMode: "fill", playerChromeIdle: false, playerChromeTimer: null, playerWaiting: false, multiviewTiles: [], multiviewActiveTileID: "", multiviewQuery: "", multiviewHeartbeat: null, recordings: null, recordingsLoading: false, recordingCapability: null, sports: null, sportsLoading: false, sportsPollTimer: null, sportsPollAttempts: 0, sportsFailedMedia: {}, sportsTab: "today", sportsLeague: "", sportsTeam: "", sportsScoresFilter: "all", sportsNewsLeague: "", sportsTeamQuery: "", sportsNews: {}, sportsTeamSummaries: {}, sportsSelectedEventID: "", sportsExpandedEvents: {}, sportsLeagueTeams: {}, sportsLeagueTeamsLoading: {}, sportsLibraries: null, sportsLibrariesLoading: false, sportsLibrariesPromise: null, sportsLibrariesError: "", sportsReplayItems: [], sportsReplayMatches: {}, sportsReplaysLoading: false, sportsReplaysError: "", sportsReplayKey: "", events: null, eventsLoading: false, eventsTab: "upcoming", eventCategory: "", expandedEvents: {}, guideChannels: [], guideRendered: 0, guideLoading: false, guideWindowStart: -1, guideWindowEnd: -1, guideRenderFrame: 0, guideWarmPings: {}, guideAutoTimer: null, guideLastSlotStart: 0, guideLastAutoFetchAt: 0, guideAutoFetching: false, programDetails: null, savedLineupEditor: null, activeSavedLineupID: "", savedLineupGroupCategoryID: "", refreshing: false, virtualCategoryView: "guide", selectedCustomGroup: "", customGroupQuery: "", customGroupChannelID: "", profileSettingsQuery: "", profileSelectionIDMap: null, profileChannelFilterMap: null, adminTab: isAdminRoute ? "source" : "settings", adminConnection: null, savedAdminConnection: null, adminConnectionEditorOpen: false, adminConnectionEditorStep: "connection", adminConnectionStatus: "idle", adminConnectionMessage: "", adminConnectionLoading: false, adminConnectionLoadError: "", adminCategorySettings: null, savedAdminCategorySettings: null, profileSaveStatus: "idle", profileSaveMessage: "", adminSaveStatus: "idle", adminSaveMessage: "", adminStatusRefreshing: false, adminProfileRefreshing: false, adminSourceGroupsLoaded: false, adminSourceGroupsLoading: false, adminSourceGroupsError: "", timeShiftSession: null, timeShiftHeartbeat: null, timeShiftTimelineTimer: null, timeShiftAttempt: 0, timeShiftAdminStatus: null, timeShiftAdminLoading: false };
const appHistoryStateKey = "dispatcharrRoute";
state.onLaterShelfLimits = {};
state.myTVTeamCatalogLoading = null;
state.channelsBrowserGroup = "";
state.channelsBrowserChannel = "";
state.aspectMode = "fit";
state.guideCategoryPickerOpen = false;
state.guideCategoryQuery = "";
state.adminOrganizationTab = "profiles";
state.adminOrganizationQuery = "";
state.sportsReplayStandaloneEvents = [];
let redundantProfileWrapperCache = { channels: null, profiles: null, signature: "", wrappers: {} };
let playerSportsDrawerResizeObserver = null;

state.sportsReplayPromise = null;
state.sportsReplayGeneration = 0;
state.sportsReplayRefreshQueued = false;

function route(url) { return base + url; }
function siloCoreURL(path) {
  if (!path) return siloAPIPrefix;
  return siloAPIPrefix + (path.charAt(0) === "/" ? path : "/" + path);
}
function assetURL(filename) {
  return assetPrefix + "/" + filename + (assetVersion ? "?v=" + encodeURIComponent(assetVersion) : "");
}
const playerLibraryPromises = {};
let programModalReturnFocus = null;
function loadPlayerLibrary(filename, globalName) {
  if (window[globalName]) return Promise.resolve();
  if (playerLibraryPromises[globalName]) return playerLibraryPromises[globalName];
  playerLibraryPromises[globalName] = new Promise(function(resolve, reject) {
    const script = document.createElement("script");
    script.src = assetURL(filename);
    script.async = true;
    script.onload = function() {
      if (window[globalName]) resolve();
      else reject(new Error(globalName + " did not initialize"));
    };
    script.onerror = function() { reject(new Error("could not load " + filename)); };
    document.head.appendChild(script);
  }).catch(function(error) {
    delete playerLibraryPromises[globalName];
    throw error;
  });
  return playerLibraryPromises[globalName];
}
function ensurePlayerLibraries(format) {
  if (format === "hls") return loadPlayerLibrary("hls.min.js", "Hls");
  if (format === "mpegts") return loadPlayerLibrary("mpegts.min.js", "mpegts");
  return Promise.all([
    loadPlayerLibrary("hls.min.js", "Hls"),
    loadPlayerLibrary("mpegts.min.js", "mpegts")
  ]);
}
// CSP forbids inline handlers, so rendered images declare their fallback with
// data-img-error / data-img-load and one capture-phase listener applies it
// (load/error do not bubble, but they do pass through the capture phase).
const imageErrorActions = {
  "logo-fallback": function(image) {
    image.hidden = true;
    if (image.nextElementSibling) image.nextElementSibling.hidden = false;
  },
  "card-art": function(image) {
    image.hidden = true;
    if (image.nextElementSibling) image.nextElementSibling.hidden = false;
    if (image.parentElement) image.parentElement.classList.remove("has-art");
  },
  "remove": function(image) { image.remove(); },
  "sports-media": function(image) { markSportsMediaFailed(image); },
  "sports-detail-bg": function(image) { markSportsDetailBackgroundFailed(image); },
  "sports-bg": function(image) { markSportsBackgroundFailed(image); },
  "event-art": function(image) { eventArtworkFailed(image); }
};
const imageLoadActions = {
  "generated-art": function(image) {
    if (image.classList.contains("sports-generated-bg") && image.parentElement) image.parentElement.classList.add("has-generated-art");
  },
  "event-art": function(image) { eventArtworkLoaded(image); }
};
function handleDeclarativeImageEvent(event) {
  const image = event && event.target;
  if (!image || image.tagName !== "IMG" || !image.dataset) return;
  const actions = event.type === "error" ? imageErrorActions : imageLoadActions;
  const name = event.type === "error" ? image.dataset.imgError : image.dataset.imgLoad;
  const action = name && Object.prototype.hasOwnProperty.call(actions, name) ? actions[name] : null;
  if (!action) return;
  try { action(image); } catch (error) {
    try { console.warn("Dispatcharr image " + event.type + " handler failed", error); } catch (_) {}
  }
}
document.addEventListener("error", handleDeclarativeImageEvent, true);
document.addEventListener("load", handleDeclarativeImageEvent, true);
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
  return (value == null ? "" : String(value)).replace(/[&<>"']/g, function(ch) {
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
    "chevron-right": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='m9 6 6 6-6 6'/></svg>",
    "chevron-down": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='m6 9 6 6 6-6'/></svg>",
    "check": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='m5 12.5 4.25 4.25L19 7'/></svg>",
    "plus": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' d='M12 5v14M5 12h14'/></svg>",
    "ellipsis": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M6.75 12a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0Zm6 0a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0Zm6 0a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0Z'/></svg>",
    "play": "<svg viewBox='0 0 24 24' fill='currentColor' aria-hidden='true'><path d='M8 5.6v12.8c0 .55.6.9 1.08.62l10.1-6.4a.73.73 0 0 0 0-1.24L9.08 4.98A.72.72 0 0 0 8 5.6Z'/></svg>",
    "record": "<svg viewBox='0 0 24 24' fill='currentColor' aria-hidden='true'><path d='M12 20.25a8.25 8.25 0 1 0 0-16.5 8.25 8.25 0 0 0 0 16.5Zm0-4a4.25 4.25 0 1 1 0-8.5 4.25 4.25 0 0 1 0 8.5Z'/></svg>",
    "stop": "<svg viewBox='0 0 24 24' fill='currentColor' aria-hidden='true'><path d='M6.75 6.75h10.5v10.5H6.75z'/></svg>",
    "trash": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M6 7.5h12M9.5 7.5V6.25A1.25 1.25 0 0 1 10.75 5h2.5A1.25 1.25 0 0 1 14.5 6.25V7.5M8.5 7.5v10.25A1.25 1.25 0 0 0 9.75 19h4.5a1.25 1.25 0 0 0 1.25-1.25V7.5'/></svg>",
    "pause": "<svg viewBox='0 0 24 24' fill='currentColor' aria-hidden='true'><path d='M7.25 5.25h3.25v13.5H7.25zM13.5 5.25h3.25v13.5H13.5z'/></svg>",
    "loader": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' d='M12 3a9 9 0 1 1-8.3 5.5'/></svg>",
    "refresh": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M19.5 12a7.5 7.5 0 0 1-13.1 5M4.5 12a7.5 7.5 0 0 1 13.1-5M17.6 3.75V7h-3.25M6.4 20.25V17h3.25'/></svg>",
    "speaker": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M19.1 8.9a7 7 0 0 1 0 6.2M16.2 10.9a3 3 0 0 1 0 2.2M4.5 14.25h3l4.25 3.25V6.5L7.5 9.75h-3v4.5Z'/></svg>",
    "speaker-off": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='m4.5 4.5 15 15M5 14.25h2.5l4.25 3.25v-5.75M11.75 8.7V6.5L8.8 8.75M16 10.8a3 3 0 0 1 .2 2.2'/></svg>",
    "airplay": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M6.75 17.25h-1.5A2.25 2.25 0 0 1 3 15V6.75A2.25 2.25 0 0 1 5.25 4.5h13.5A2.25 2.25 0 0 1 21 6.75V15a2.25 2.25 0 0 1-2.25 2.25h-1.5M8.25 21h7.5L12 16.5 8.25 21Z'/></svg>",
    "guide": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M4.5 6.75h15M4.5 12h15M4.5 17.25h15M8.25 4.5v15M15.75 4.5v15'/></svg>",
    "kids": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><circle cx='9' cy='8.25' r='3'/><circle cx='16.5' cy='9.5' r='2.25'/><path stroke-linecap='round' stroke-linejoin='round' d='M3.75 19c.45-3.5 2.2-5.25 5.25-5.25S13.8 15.5 14.25 19M14.25 14.25c3.5-.65 5.5.95 6 4.75'/></svg>",
    "news": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M5.25 5.25h11.5v13.5H6.5a2.25 2.25 0 0 1-2.25-2.25V6.25a1 1 0 0 1 1-1ZM16.75 8h3v8.5a2.25 2.25 0 0 1-2.25 2.25h-.75'/><path stroke-linecap='round' d='M7.75 9h6.5M7.75 12h6.5M7.75 15h4'/></svg>",
    "tv": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='m8.5 3.75 3.5 3 3.5-3M5.5 6.75h13A2.25 2.25 0 0 1 20.75 9v8.25a2.25 2.25 0 0 1-2.25 2.25h-13a2.25 2.25 0 0 1-2.25-2.25V9A2.25 2.25 0 0 1 5.5 6.75Z'/><path stroke-linecap='round' stroke-linejoin='round' d='M8.5 10.25v5.75l5-2.875-5-2.875ZM17.5 11.25h.01M17.5 15h.01'/></svg>",
    "collection": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M4.5 5.25h6.25v5.5H4.5zM13.25 5.25h6.25v5.5h-6.25zM4.5 13.25h6.25v5.5H4.5zM13.25 13.25h6.25v5.5h-6.25z'/></svg>",
    "clock": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18Z'/><path stroke-linecap='round' stroke-linejoin='round' d='M12 7.5V12l3 2.25'/></svg>",
    "multiview": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M4.5 5.75h6.25v5.75H4.5zM13.25 5.75h6.25v5.75h-6.25zM4.5 14h6.25v4.25H4.5zM13.25 14h6.25v4.25h-6.25z'/></svg>",
    "trophy": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M8 4.5h8v4.25a4 4 0 0 1-8 0V4.5ZM9.5 14.5h5M12 12.75V18M8.5 20h7'/><path stroke-linecap='round' stroke-linejoin='round' d='M8 6H5.25v1.5A3.5 3.5 0 0 0 8.5 11M16 6h2.75v1.5A3.5 3.5 0 0 1 15.5 11'/></svg>",
    "settings": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M12 8.25a3.75 3.75 0 1 1 0 7.5 3.75 3.75 0 0 1 0-7.5Z'/><path stroke-linecap='round' stroke-linejoin='round' d='M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.6 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 8.92 4.6a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9c.23.64.84 1 1.51 1H21a2 2 0 0 1 0 4h-.09A1.65 1.65 0 0 0 19.4 15Z'/></svg>",
    "fullscreen": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M8.25 4.5H4.5v3.75M15.75 4.5h3.75v3.75M19.5 15.75v3.75h-3.75M4.5 15.75v3.75h3.75M9 9 4.5 4.5M15 9l4.5-4.5M15 15l4.5 4.5M9 15l-4.5 4.5'/></svg>",
    "fullscreen-exit": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M4.5 9h4.25V4.75M15.25 4.75V9h4.25M19.5 15h-4.25v4.25M8.75 19.25V15H4.5M8.75 9 4.5 4.75M15.25 9l4.25-4.25M15.25 15l4.25 4.25M8.75 15 4.5 19.25'/></svg>",
    "heart": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M21 8.25c0 6.25-9 11.25-9 11.25s-9-5-9-11.25A4.75 4.75 0 0 1 11.25 5 4.75 4.75 0 0 1 21 8.25Z'/></svg>",
    "heart-solid": "<svg viewBox='0 0 24 24' fill='currentColor' aria-hidden='true'><path d='M12 21s-9-5.1-9-12.25A5.45 5.45 0 0 1 12 4.7a5.45 5.45 0 0 1 9 4.05C21 15.9 12 21 12 21Z'/></svg>",
    "eye": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M2.75 12s3.25-6 9.25-6 9.25 6 9.25 6-3.25 6-9.25 6-9.25-6-9.25-6Z'/><circle cx='12' cy='12' r='2.75'/></svg>",
    "eye-off": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='m3 3 18 18M10.55 6.13A9.7 9.7 0 0 1 12 6c6 0 9.25 6 9.25 6a14.5 14.5 0 0 1-2.14 2.82M6.1 6.1C3.9 7.73 2.75 12 2.75 12s3.25 6 9.25 6a9.8 9.8 0 0 0 3.2-.53M9.8 9.8a3.1 3.1 0 0 0 4.4 4.4'/></svg>",
    "pip": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M4.5 6.75A2.25 2.25 0 0 1 6.75 4.5h10.5a2.25 2.25 0 0 1 2.25 2.25v10.5a2.25 2.25 0 0 1-2.25 2.25H6.75a2.25 2.25 0 0 1-2.25-2.25V6.75Z'/><path stroke-linecap='round' stroke-linejoin='round' d='M13.25 13.25h4.25v3.25h-4.25z'/></svg>",
    "captions": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M4.5 7.5A2.5 2.5 0 0 1 7 5h10a2.5 2.5 0 0 1 2.5 2.5v9A2.5 2.5 0 0 1 17 19H7a2.5 2.5 0 0 1-2.5-2.5v-9Z'/><path stroke-linecap='round' stroke-linejoin='round' d='M8.25 10.5h3M8.25 14h2.25M13.5 14h2.25'/></svg>",
    "language": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18Z'/><path stroke-linecap='round' stroke-linejoin='round' d='M3.75 9h16.5M3.75 15h16.5M12 3c2.25 2.35 3.25 5.25 3.25 9S14.25 18.65 12 21c-2.25-2.35-3.25-5.25-3.25-9S9.75 5.35 12 3Z'/></svg>",
    "aspect": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M4.5 7.25A2.75 2.75 0 0 1 7.25 4.5h9.5a2.75 2.75 0 0 1 2.75 2.75v9.5a2.75 2.75 0 0 1-2.75 2.75h-9.5a2.75 2.75 0 0 1-2.75-2.75v-9.5Z'/><path stroke-linecap='round' stroke-linejoin='round' d='M8 8h3M8 8v3M16 16h-3M16 16v-3'/></svg>",
    "rewind": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M8 8H4V4M4.75 8.5A8 8 0 1 1 4 14'/><path stroke-linecap='round' stroke-linejoin='round' d='M9 10.5v5M9 10.5H7.75M13 11.25a1.75 1.75 0 0 1 3.5 0v3a1.75 1.75 0 0 1-3.5 0v-3Z'/></svg>",
    "forward": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M16 8h4V4M19.25 8.5A8 8 0 1 0 20 14'/><path stroke-linecap='round' stroke-linejoin='round' d='M8 10.5v5M8 10.5H6.75M12 11.25a1.75 1.75 0 0 1 3.5 0v3a1.75 1.75 0 0 1-3.5 0v-3Z'/></svg>",
    "search": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='m20 20-4.5-4.5M10.5 18a7.5 7.5 0 1 1 0-15 7.5 7.5 0 0 1 0 15Z'/></svg>",
    "copy": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M8 8h9.25A1.75 1.75 0 0 1 19 9.75v9.5A1.75 1.75 0 0 1 17.25 21h-9.5A1.75 1.75 0 0 1 6 19.25V10'/><path stroke-linecap='round' stroke-linejoin='round' d='M5.75 16H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v.75'/></svg>",
    "external": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M13.5 4.5H19.5V10.5M19.25 4.75 11 13M10.5 6H6.75A2.25 2.25 0 0 0 4.5 8.25v9A2.25 2.25 0 0 0 6.75 19.5h9A2.25 2.25 0 0 0 18 17.25V13.5'/></svg>",
    "integrations": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M8.25 8.25h7.5v7.5h-7.5zM4.75 12h3.5M15.75 12h3.5M12 4.75v3.5M12 15.75v3.5'/><path stroke-linecap='round' stroke-linejoin='round' d='M6.25 6.25 8.5 8.5M17.75 6.25 15.5 8.5M6.25 17.75 8.5 15.5M17.75 17.75 15.5 15.5'/></svg>",
    "x": "<svg viewBox='0 0 24 24' fill='none' stroke='currentColor' aria-hidden='true'><path stroke-linecap='round' stroke-linejoin='round' d='M6 6l12 12M18 6 6 18'/></svg>"
  };
  return icons[name] || "";
}
function menuIcon(name) { return "<span class=\"menu-icon\">" + icon(name) + "</span>"; }
async function getJSON(url) {
  const response = await coreFetch(route(url));
  if (!response.ok) throw await requestError(response);
  return response.json();
}
async function getJSONWithin(url, timeoutMs, timeoutMessage) {
  if (typeof AbortController === "undefined") return getJSON(url);
  const controller = new AbortController();
  const timer = setTimeout(function() { controller.abort(); }, Math.max(1000, Number(timeoutMs || 0)));
  try {
    const response = await coreFetch(route(url), { signal: controller.signal });
    if (!response.ok) throw await requestError(response);
    return response.json();
  } catch (error) {
    if (error && error.name === "AbortError") throw new Error(timeoutMessage || "The request took too long. Try again.");
    throw error;
  } finally {
    clearTimeout(timer);
  }
}
async function postJSON(url, body) {
  const response = await coreFetch(route(url), { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body) });
  if (!response.ok) throw await requestError(response);
  return response.json();
}
let coreAccessToken = "";
let coreRefreshPromise = null;
function coreStoredValue(key) {
  try { return localStorage.getItem(key) || ""; }
  catch (_) { return ""; }
}
function coreStoreValue(key, value) {
  try {
    if (value) localStorage.setItem(key, value);
  } catch (_) {}
}
function coreRequestOptions(options) {
  const next = Object.assign({}, options || {});
  const headers = Object.assign({}, next.headers || {});
  if (coreAccessToken) headers.Authorization = "Bearer " + coreAccessToken;
  const profileID = coreStoredValue("profile_id");
  const profileToken = coreStoredValue("profile_token");
  if (profileID) headers["X-Profile-Id"] = profileID;
  if (profileToken) headers["X-Profile-Token"] = profileToken;
  next.credentials = "include";
  next.headers = headers;
  return next;
}
function coreMediaRequestHeaders(url) {
  try {
    if (new URL(String(url || ""), window.location.href).origin !== window.location.origin) return {};
  } catch (_) {
    return {};
  }
  return Object.assign({}, coreRequestOptions({}).headers || {});
}
function applyCoreMediaRequest(xhr, url) {
  const headers = coreMediaRequestHeaders(url);
  Object.keys(headers).forEach(function(name) { xhr.setRequestHeader(name, headers[name]); });
  if (Object.keys(headers).length) xhr.withCredentials = true;
}
async function refreshCoreSession() {
  const refreshToken = coreStoredValue("refresh_token");
  if (!refreshToken) return false;
  const response = await fetch(siloCoreURL("/auth/refresh"), {
    method: "POST",
    credentials: "include",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ refresh_token: refreshToken })
  });
  if (!response.ok) return false;
  const payload = await response.json().catch(function() { return {}; });
  if (!payload.access_token) return false;
  coreAccessToken = String(payload.access_token);
  if (payload.refresh_token) coreStoreValue("refresh_token", String(payload.refresh_token));
  return true;
}
async function coreFetch(url, options) {
  if (!coreAccessToken && coreStoredValue("refresh_token")) {
    if (!coreRefreshPromise) coreRefreshPromise = refreshCoreSession().finally(function() { coreRefreshPromise = null; });
    await coreRefreshPromise;
  }
  let response = await fetch(url, coreRequestOptions(options));
  if (response.status !== 401) return response;
  if (!coreRefreshPromise) {
    coreRefreshPromise = refreshCoreSession().finally(function() { coreRefreshPromise = null; });
  }
  if (await coreRefreshPromise) response = await fetch(url, coreRequestOptions(options));
  return response;
}
async function coreGetJSON(url) {
  const response = await coreFetch(url);
  if (!response.ok) throw await requestError(response);
  return response.json();
}
async function corePostJSON(url, body) {
  const response = await coreFetch(url, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body) });
  if (!response.ok) throw await requestError(response);
  return response.json();
}
async function corePutNoContent(url, body) {
  const response = await coreFetch(url, { method: "PUT", headers: { "content-type": "application/json" }, body: JSON.stringify(body) });
  if (!response.ok) throw await requestError(response);
}
async function requestError(response) {
  const text = await response.text().catch(function() { return ""; });
  const detail = text ? ": " + text.slice(0, 240) : "";
  const error = new Error("request failed (" + response.status + ")" + detail);
  error.status = response.status;
  return error;
}
function readableError(error) {
  const status = Number(error && error.status || 0);
  const message = String(error && error.message ? error.message : error || "unknown error");
  if (status === 401) {
    return "Your Silo session expired. Refresh the page or sign in again.";
  }
  if (/unexpected status 401|unauthorized/i.test(message)) {
    return "Dispatcharr rejected these credentials. Check the username and password or API key, and make sure nothing in front of Dispatcharr requires its own login.";
  }
  if (/html instead of json/i.test(message)) {
    return "Dispatcharr returned a web page instead of API data. The base URL may point at a login page or proxy instead of Dispatcharr.";
  }
  if (status === 403 || /request failed \(403\)|unexpected status 403|forbidden|permission/i.test(message)) {
    return message;
  }
  return message;
}
