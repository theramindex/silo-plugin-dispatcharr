// Last part of the composed app.js bundle (order: ui_assets.go
// playerAppScriptParts); it ends with the load-time event wiring, so it must
// stay last. core.js (globals, utilities, fetch/auth) is first.
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
  return items(state.app.channels).filter(channelMatchesProfileSelection).map(function(channel, index) {
    const copy = effectiveChannel(channel);
    copy.sourceIndex = index;
    return copy;
  }).sort(function(left, right) {
    return (left.sourceIndex || 0) - (right.sourceIndex || 0);
  });
}
function channelLogicalKey(channel) {
  if (!channel) return "";
  const guideID = lower(channel.guideId).trim();
  if (guideID) return "guide:" + guideID;
  const name = lower(channel.name).replace(/\s+/g, " ").trim();
  const category = lower(sourceCategoryLabel(channel)).replace(/\s+/g, " ").trim();
  const logo = lower(channel.logoUrl).trim();
  return "channel:" + [name, category, logo].join("|");
}
function dedupeChannels(channels) {
  const seen = {};
  return items(channels).filter(function(channel) {
    const key = channelLogicalKey(channel) || ("id:" + String(channel && channel.id || ""));
    if (seen[key]) return false;
    seen[key] = true;
    return true;
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
  return renamedCategoryDisplayName(sourceCategoryRawName(id));
}
function sourceCategoryRawLabel(channel) {
  return sourceCategoryRawName(channel.categoryId) || channel.categoryName || "";
}
function sourceCategoryLabel(channel) {
  return renamedCategoryDisplayName(sourceCategoryRawLabel(channel));
}
function sourceCategoryOriginalLabel(channel) {
  return categoryDisplayName(sourceCategoryRawLabel(channel));
}
function normalizedGroupName(value) {
  return categoryDisplayName(value).toLowerCase();
}
function isWorldCupReplayGroup(value) {
  return normalizedGroupName(value) === "world cup replays";
}
function parsedDelimitedPath(name) {
  return window.DispatcharrLineup.parsedDelimitedPath(name, (adminSettings().delimiter || "pipe"));
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
  return categoryPathFromDisplayName(renamedCategoryDisplayName(name));
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
function configuredCategoryPathOrLabel(value) {
  return configuredCategoryPath(value) || categoryDisplayName(value).trim();
}
function aliasVirtualPathsForSourcePath(sourcePath) {
  const normalizedSourcePath = configuredCategoryPathOrLabel(sourcePath);
  if (!normalizedSourcePath) return [];
  const paths = [];
  const seen = {};
  categoryAliases().forEach(function(alias) {
    const fromPath = configuredCategoryPathOrLabel(alias.sourcePath);
    const toPath = configuredCategoryPathOrLabel(alias.aliasPath);
    if (!fromPath || !toPath) return;
    if (normalizedSourcePath !== fromPath && normalizedSourcePath.indexOf(fromPath + " / ") !== 0) return;
    const suffix = normalizedSourcePath === fromPath ? "" : normalizedSourcePath.slice(fromPath.length + 3);
    const remappedPath = suffix ? toPath + " / " + suffix : toPath;
    if (!seen[remappedPath]) {
      seen[remappedPath] = true;
      paths.push(remappedPath);
    }
  });
  return paths;
}
function channelNamePathParts(channel) {
  return window.DispatcharrLineup.channelNamePathParts(channel);
}
function channelNameFolderPathParts(channel) {
  const parts = channelNamePathParts(channel).map(normalizedNameToken).filter(Boolean);
  if (parts.length < 2) return [];
  return parts.slice(0, -1);
}
function appendUnduplicatedPathParts(basePath, extraParts) {
  return window.DispatcharrLineup.appendUnduplicatedPathParts(basePath, extraParts);
}
function appendVirtualPathParts(basePath, extraParts) {
  return window.DispatcharrLineup.appendVirtualPathParts(basePath, extraParts, adminSettings().collapseDuplicateVirtualGroups);
}
function normalizedNameToken(value) {
  return window.DispatcharrLineup.normalizedNameToken(value);
}
function looksLikeUSStateCode(value) {
  const code = normalizedNameToken(value).toUpperCase();
  const states = {
    AL: true, AK: true, AZ: true, AR: true, CA: true, CO: true, CT: true, DE: true, FL: true, GA: true,
    HI: true, ID: true, IL: true, IN: true, IA: true, KS: true, KY: true, LA: true, ME: true, MD: true,
    MA: true, MI: true, MN: true, MS: true, MO: true, MT: true, NE: true, NV: true, NH: true, NJ: true,
    NM: true, NY: true, NC: true, ND: true, OH: true, OK: true, OR: true, PA: true, RI: true, SC: true,
    SD: true, TN: true, TX: true, UT: true, VT: true, VA: true, WA: true, WV: true, WI: true, WY: true,
    DC: true
  };
  return states[code] === true;
}
function inferredInternationalRoot(channel, parts) {
  const text = (parts.join(" ") + " " + sourceCategoryLabel(channel || {}) + " " + ((channel && channel.categoryName) || "")).toLowerCase();
  if (/\b(sport|sports|team|teams|league|football|soccer|futbol|fútbol|mlb|nba|nfl|nhl|mls|f1)\b/.test(text)) return "International Sports";
  if (/\b(news|noticias|nouvelles)\b/.test(text)) return "International News";
  if (/\b(kids|children|cartoon|junior)\b/.test(text)) return "International Kids";
  return "International TV";
}
function inferredChannelNameGroupPaths(channel) {
  if (!useChannelNameVirtualPaths()) return [];
  const parts = channelNamePathParts(channel).map(normalizedNameToken).filter(Boolean);
  if (parts.length < 2) return [];
  const first = parts[0];
  const second = parts[1];
  const paths = [];
  if (looksLikeUSStateCode(first)) {
    const state = first.toUpperCase();
    paths.push("US TV / Locals / " + state);
    if (second) paths.push("US TV / Locals / " + state + " / " + second);
    return uniqueIDs(paths);
  }
  const root = inferredInternationalRoot(channel, parts);
  paths.push(root + " / " + first);
  if (second) paths.push(root + " / " + first + " / " + second);
  return uniqueIDs(paths);
}
function localMarketPathPartsForChannel(channel, sourceParts) {
  const sourceTail = normalizedNameToken(items(sourceParts).slice(-1)[0] || "").toLowerCase();
  if (sourceTail !== "local" && sourceTail !== "locals") return [];
  const parts = channelNamePathParts(channel).map(normalizedNameToken).filter(Boolean);
  if (parts.length < 2) return [];
  if (looksLikeUSStateCode(parts[0])) return parts[1] ? [parts[1]] : [];
  return [parts[0]];
}
function virtualPathsForChannel(channel) {
  const paths = [];
  if (virtualGroupSourceMode() === "profile") {
    return uniqueIDs(profilePathsForChannel(channel));
  }
  if (useProfileGroupVirtualPaths()) {
    profileVirtualPathsForChannel(channel).forEach(function(path) { paths.push(path); });
    return uniqueIDs(paths);
  }
  const sourcePath = sourceVirtualPathForChannel(channel);
  if (sourcePath && useSourceGroupVirtualPaths()) {
    paths.push(sourcePath);
    aliasVirtualPathsForSourcePath(sourcePath).forEach(function(path) { paths.push(path); });
    if (virtualGroupSourceMode() === "group_channel") {
      const combinedPath = appendVirtualPathParts(sourcePath, channelNameFolderPathParts(channel));
      if (combinedPath) {
        paths.push(combinedPath);
        aliasVirtualPathsForSourcePath(combinedPath).forEach(function(path) { paths.push(path); });
      }
    }
  }
  if (useChannelNameVirtualPaths()) {
    inferredChannelNameGroupPaths(channel).forEach(function(path) { paths.push(path); });
  }
  return uniqueIDs(paths);
}
function profileVirtualPathsForChannel(channel) {
  const sourcePath = sourceGroupPathForChannel(channel);
  if (!sourcePath) return [];
  const paths = [];
  const sourceParts = String(sourcePath || "").split(" / ").map(normalizedNameToken).filter(Boolean);
  const aliasPaths = aliasVirtualPathsForSourcePath(sourcePath);
  const groupPaths = [sourcePath].concat(aliasPaths);
  const marketParts = localMarketPathPartsForChannel(channel, sourceParts);
  aliasPaths.forEach(function(aliasPath) { paths.push(aliasPath); });
  const redundantWrappers = redundantProfileGroupWrappers();
  profilePathsForChannel(channel).forEach(function(profilePath) {
    paths.push(profilePath);
    groupPaths.forEach(function(groupPath) {
      let groupParts = String(groupPath || "").split(" / ").map(normalizedNameToken).filter(Boolean);
      const redundantWrapper = redundantWrappers[lower(profilePath)];
      if (redundantWrapper && groupParts.length > 1 && lower(groupParts[0]) === redundantWrapper) groupParts = groupParts.slice(1);
      const combinedPath = appendVirtualPathParts(profilePath, groupParts);
      if (combinedPath) paths.push(combinedPath);
      const marketPath = appendVirtualPathParts(combinedPath || (profilePath + " / " + groupPath), marketParts);
      if (marketPath) paths.push(marketPath);
    });
  });
  return uniqueIDs(paths);
}
function redundantProfileGroupWrappers() {
  const settings = adminSettings();
  if (settings.flattenRedundantGroupWrappers === false || !useProfileGroupVirtualPaths()) return {};
  const channels = state.app && state.app.channels;
  const profiles = state.app && state.app.source && state.app.source.profiles;
  const selectedProfile = state.app && state.app.source && state.app.source.channelProfile;
  const signature = [
    settings.mode,
    settings.delimiter,
    settings.virtualGroupSource,
    JSON.stringify(profileSelection()),
    selectedProfile && selectedProfile.id ? selectedProfile.id : ""
  ].join("|");
  if (redundantProfileWrapperCache.channels === channels && redundantProfileWrapperCache.profiles === profiles && redundantProfileWrapperCache.signature === signature) {
    return redundantProfileWrapperCache.wrappers;
  }
  const candidates = {};
  effectiveChannels(false).forEach(function(candidateChannel) {
    const candidateGroupPath = sourceGroupPathForChannel(candidateChannel);
    const groupParts = String(candidateGroupPath || "").split(" / ").map(normalizedNameToken).filter(Boolean);
    if (!groupParts.length) return;
    profilePathsForChannel(candidateChannel).forEach(function(profilePath) {
      const key = lower(profilePath);
      const wrapper = lower(groupParts[0]);
      if (!candidates[key]) {
        candidates[key] = { wrapper: wrapper, eligible: groupParts.length > 1, tails: {} };
        if (groupParts.length > 1) candidates[key].tails[lower(groupParts.slice(1).join(" / "))] = true;
        return;
      }
      if (candidates[key].wrapper !== wrapper || groupParts.length < 2) {
        candidates[key].eligible = false;
        return;
      }
      candidates[key].tails[lower(groupParts.slice(1).join(" / "))] = true;
    });
  });
  const wrappers = {};
  Object.keys(candidates).forEach(function(key) {
    if (candidates[key].eligible && Object.keys(candidates[key].tails).length > 1) wrappers[key] = candidates[key].wrapper;
  });
  redundantProfileWrapperCache = { channels: channels, profiles: profiles, signature: signature, wrappers: wrappers };
  return wrappers;
}
function sourceGroupPathForChannel(channel) {
  return sourceVirtualPathForChannel(channel) || categoryDisplayName(sourceCategoryLabel(channel));
}
function profilePathsForChannel(channel) {
  const profiles = profileMapByID();
  const selectedProfile = state.app && state.app.source && state.app.source.channelProfile;
  let profileIDs = selectedProfile && selectedProfile.id ? [selectedProfile.id] : items(channel && channel.profileIds);
  if (!profileSelectionIsAll()) {
    const selected = selectedProfileMap();
    profileIDs = profileIDs.filter(function(profileID) { return !!selected[profileID]; });
  }
  return profileIDs.map(function(profileID) {
    return profileVirtualPathForName((profiles[profileID] || {}).name || profileID);
  }).filter(Boolean);
}
function profileVirtualPathForName(name) {
  return String(name || "").split("|").map(function(part) {
    return part.trim();
  }).filter(Boolean).join(" / ");
}
function profileMapByID() {
  const map = {};
  items(state.app && state.app.source && state.app.source.profiles).forEach(function(profile) {
    if (profile && profile.id) map[profile.id] = profile;
  });
  return map;
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
  const channels = dedupeChannels(effectiveChannels(false).filter(function(channel) {
    if (channel.categoryId && hidden[channel.categoryId]) return false;
    if (state.view !== "favorites" && state.category && !channelInSelectedCategory(channel, state.category)) return false;
    if (!ignoreQuery && state.query && !guideChannelMatchesQuery(channel)) return false;
    if (state.view === "favorites" && !favoriteMap()[channel.id] && !autoFavoriteMap()[channel.id]) return false;
    return true;
  }));
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
function setChannelFavorite(channelID, enabled) {
  const id = String(channelID || "");
  if (!id || !state.app || !state.app.preferences) return false;
  if (enabled) {
    state.app.preferences.favorites[id] = true;
    state.app.preferences.favoriteOrder = uniqueIDs(items(state.app.preferences.favoriteOrder).concat([id]));
  } else {
    delete state.app.preferences.favorites[id];
    state.app.preferences.favoriteOrder = items(state.app.preferences.favoriteOrder).filter(function(item) { return item !== id; });
  }
  normalizePreferences();
  savePrefs();
  return !!favoriteMap()[id];
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
const providerLiveMarker = "\u1d38\u1da6\u1d5b\u1d49";
function cleanProviderText(value) {
  return String(value || "")
    .replace(/[\u02b0-\u02b8\u02e0-\u02e4\u1d2c-\u1d6a\u1d9b-\u1dbf\u2070-\u209f]+/g, " ")
    .replace(/^[\s\u2022\u25a0\u25a1\u25aa\u25ab\u25c6\u25c7\u25c8\u25cb\u25cf\u2605\u2606\u2666|]+/, "")
    .replace(/\s{2,}/g, " ")
    .trim();
}
function cleanProgramText(program) {
  if (!program || program.providerTextCleaned) return;
  const title = String(program.title || "").trimEnd();
  if (title.endsWith(providerLiveMarker)) program.liveMarker = true;
  program.title = cleanProviderText(title);
  program.providerTextCleaned = true;
}
function cleanChannelText(channel) {
  if (!channel || !channel.name) return;
  channel.name = cleanProviderText(channel.name) || channel.name;
}
function rebuildProgramIndex() {
  items(state.app && state.app.programs).forEach(cleanProgramText);
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
  const allowed = selectedProfileChannelMap();
  if (channelID && allowed && !allowed[channelID]) return [];
  const source = channelID ? items(state.programsByChannel[channelID]) : items(state.sortedPrograms);
  return source.filter(function(program) {
    return (!allowed || !!allowed[program.channelId]) && (!channelID || program.channelId === channelID) && (!program.endUnix || program.endUnix >= now - 3600);
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
function relativeUpdatedLabel(unix) {
  unix = Number(unix || 0);
  if (!unix) return "Guide not synced yet";
  const seconds = Math.max(0, Math.floor(Date.now() / 1000) - unix);
  if (seconds < 60) return "Updated just now";
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return "Updated " + minutes + " minute" + (minutes === 1 ? "" : "s") + " ago";
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return "Updated " + hours + " hour" + (hours === 1 ? "" : "s") + " ago";
  const days = Math.floor(hours / 24);
  return "Updated " + days + " day" + (days === 1 ? "" : "s") + " ago";
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
function guideTimeHeadInnerHTML(slots) {
  return "<span>Today</span>" + slots.map(function(slot) { return "<span><b>" + escapeHTML(timeLabel(slot)) + "</b></span>"; }).join("");
}
function guideTimeHeadHTML(slots) {
  return "<div class=\"time-head\">" + guideTimeHeadInnerHTML(slots) + "</div>" + guideNowLineHTML();
}
function guideNowLineStyle() {
  const windowInfo = guideWindow();
  const offsetSlots = (Math.floor(Date.now() / 1000) - windowInfo.start) / 1800;
  return "left: calc(var(--epg-logo-col) + " + offsetSlots.toFixed(4) + " * var(--epg-slot));";
}
function guideNowLineHTML() {
  return "<div class=\"epg-now-line\" aria-hidden=\"true\" style=\"" + guideNowLineStyle() + "\"></div>";
}
function updateGuideNowLines() {
  const style = guideNowLineStyle();
  document.querySelectorAll(".epg-now-line").forEach(function(line) { line.setAttribute("style", style); });
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
  state.playerSportsMode = false;
  state.playerSportsOpen = false;
  state.playerSportsMoreOpen = false;
  stopPlayerSportsRefresh();
  resetPlayerSportsDrawerLayout();
  if (video) {
    video.pause();
    video.removeAttribute("src");
    video.load();
  }
  stopTimeShiftSession();
}
function stopCurrentWatch(reason) {
  // Invalidate any /watch/start still in flight so its session is stopped on arrival.
  state.watchAttempt = (state.watchAttempt || 0) + 1;
  if (!state.currentSession) return;
  postJSON("/dispatcharr/api/watch/stop", { sessionId: state.currentSession.id, reason: reason || "stop" }).catch(function() {});
  state.currentSession = null;
  if (state.heartbeat) {
    clearInterval(state.heartbeat);
    state.heartbeat = null;
  }
}
function appRoutePart(value) {
  return encodeURIComponent(String(value || ""));
}
function readAppRoutePart(value) {
  try { return decodeURIComponent(String(value || "")); } catch (_) { return ""; }
}
function appRouteSnapshot() {
  return {
    view: isAdminRoute ? "admin" : state.view,
    category: state.category || "",
    sportsTab: state.sportsTab || "today",
    sportsLeague: state.sportsLeague || "",
    sportsTeam: state.sportsTeam || "",
    sportsEvent: state.sportsSelectedEventID || "",
    eventsTab: state.eventsTab || "upcoming",
    eventCategory: state.eventCategory || "",
    adminTab: state.adminTab || "source",
    channelID: state.view === "player" && state.currentChannel ? state.currentChannel.id : ""
  };
}
function appRouteHash(snapshot) {
  snapshot = snapshot || appRouteSnapshot();
  if (snapshot.view === "admin") return "#/admin/" + appRoutePart(snapshot.adminTab || "source");
  if (snapshot.view === "player" && snapshot.channelID) return "#/watch/" + appRoutePart(snapshot.channelID);
  if (snapshot.view === "sports") {
    let route = (isSportsPath ? "#/" : "#/sports/") + appRoutePart(snapshot.sportsTab || "today");
    if (snapshot.sportsEvent) route += "/event/" + appRoutePart(snapshot.sportsEvent);
    else if (snapshot.sportsTeam) route += "/team/" + appRoutePart(snapshot.sportsTeam);
    else if (snapshot.sportsLeague) route += "/league/" + appRoutePart(snapshot.sportsLeague);
    return route;
  }
  if (snapshot.view === "events") {
    let route = "#/events/" + appRoutePart(snapshot.eventsTab || "upcoming");
    if (snapshot.eventCategory) route += "/category/" + appRoutePart(snapshot.eventCategory);
    return route;
  }
  if (snapshot.view === "live" && snapshot.category) return "#/channels/" + appRoutePart(snapshot.category);
  const view = ["home", "channels", "live", "favorites", "guide", "mytv", "search", "recordings", "settings", "multiview"].indexOf(snapshot.view) !== -1 ? snapshot.view : "home";
  return "#/" + view;
}
function readAppRouteHash() {
  const raw = String(window.location.hash || "").replace(/^#\/?/, "");
  const parts = raw.split("/").filter(Boolean).map(readAppRoutePart);
  if (isAdminRoute) return { view: "admin", adminTab: parts[0] === "admin" ? (parts[1] || "source") : "source" };
  if (!parts.length) return { view: defaultBrowseView() };
  if (parts[0] === "watch") return { view: "player", channelID: parts[1] || "" };
  if (parts[0] === "channels" && parts[1]) return { view: "live", category: parts[1] };
  const otherViews = ["home", "channels", "live", "favorites", "guide", "mytv", "onlater", "search", "recordings", "settings", "multiview", "events"];
  const sportsParts = isSportsPath ? (otherViews.indexOf(parts[0]) === -1 ? parts : null) : (parts[0] === "sports" ? parts.slice(1) : null);
  if (sportsParts) {
    const route = { view: "sports", sportsTab: sportsParts[0] || "today" };
    if (sportsParts[1] === "event") route.sportsEvent = sportsParts[2] || "";
    if (sportsParts[1] === "team") route.sportsTeam = sportsParts[2] || "";
    if (sportsParts[1] === "league") route.sportsLeague = sportsParts[2] || "";
    return route;
  }
  if (parts[0] === "events") return { view: "events", eventsTab: parts[1] || "upcoming", eventCategory: parts[2] === "category" ? (parts[3] || "") : "" };
  if (parts[0] === "onlater") return { view: "mytv" };
  const view = ["home", "channels", "live", "favorites", "guide", "mytv", "search", "recordings", "settings", "multiview"].indexOf(parts[0]) !== -1 ? parts[0] : "home";
  return { view: view };
}
function commitAppRoute(mode) {
  if (!window.history || typeof window.history.pushState !== "function" || typeof window.history.replaceState !== "function") return;
  const snapshot = appRouteSnapshot();
  const hash = appRouteHash(snapshot);
  const historyState = {};
  historyState[appHistoryStateKey] = snapshot;
  const target = window.location.pathname + window.location.search + hash;
  if (mode === "replace" || window.location.hash === hash) window.history.replaceState(historyState, "", target);
  else window.history.pushState(historyState, "", target);
}
function applyAppRouteSnapshot(snapshot) {
  snapshot = snapshot || {};
  state.category = snapshot.category || "";
  state.sportsTab = snapshot.sportsTab || "today";
  state.sportsLeague = snapshot.sportsLeague || "";
  state.sportsTeam = snapshot.sportsTeam || "";
  state.sportsSelectedEventID = snapshot.sportsEvent || "";
  state.eventsTab = snapshot.eventsTab || "upcoming";
  state.eventCategory = snapshot.eventCategory || "";
  if (snapshot.adminTab) state.adminTab = snapshot.adminTab;
}
function restoreAppRoute(snapshot) {
  snapshot = snapshot || readAppRouteHash();
  if (snapshot.view === "player" && snapshot.channelID) {
    const channel = channelByID(snapshot.channelID);
    if (channel) {
      playChannel(channel, { historyMode: "none" });
      return;
    }
    snapshot = { view: defaultBrowseView() };
  }
  applyAppRouteSnapshot(snapshot);
  setView(snapshot.view || defaultBrowseView(), { historyMode: "none", preserveBrowseState: true });
  if (snapshot.view === "admin" && state.app) setAdminTab(snapshot.adminTab || "source", { historyMode: "none" });
}
function sportsNavAvailable() {
  if (!sportsEnabled()) return false;
  if (isSportsPath) return true;
  return !separateSportsApp();
}
function eventsNavAvailable() {
  return true;
}
function setView(view, options) {
  if (view !== "guide") clearGuideSearchTimer();
  options = options || {};
  if (view === "sports" && !sportsNavAvailable()) view = defaultBrowseView();
  if (view === "events" && !eventsNavAvailable()) view = defaultBrowseView();
  if (isSportsApp() && liveTVOnlyView(view)) view = "sports";
  if (view === "search" && state.view !== "search" && state.view !== "player") {
    state.searchReturnView = state.view || "home";
  }
  if (view === "search") {
    loadSports(false);
    loadEvents(false);
  }
  if (view !== "player") {
    stopPlayback();
    if (state.view === "player") stopCurrentWatch("leave_player");
  }
  if (view !== "multiview" && state.view === "multiview") stopAllMultiview("leave_multiview");
  if (view !== state.view && !options.preserveBrowseState) state.folderQuery = "";
  state.view = view;
  if ((view === "favorites" || view === "mytv") && !options.preserveBrowseState) state.category = "";
  if ((view === "search" || view === "mytv") && dvrEnabled()) loadRecordings(false);
  if (view === "mytv") {
    state.category = "";
    loadSports(false);
    loadEvents(false);
  }
  if (view === "sports") {
    state.category = "";
    loadSports(false);
  }
  if (view === "events") {
    state.category = "";
    loadEvents(false);
  }
  if (view === "channels") state.category = "";
  render();
  ensureGuideCoverageForView(view);
  if (options.historyMode !== "none") commitAppRoute("push");
}
function setCategory(id, options) {
  options = options || {};
  state.folderGroupPickerOpen = false;
  if ((id || "") !== state.category) {
    state.folderQuery = "";
    state.folderGroupCategoryID = "";
  }
  if (!options.preserveSavedLineup) {
    state.activeSavedLineupID = "";
    state.savedLineupGroupCategoryID = "";
  }
  state.category = id || "";
  state.view = id ? "live" : (channelGroupsInSideMenu() ? "channels" : "home");
  render();
  if (options.historyMode !== "none") commitAppRoute("push");
}
function openSavedLineup(id) {
  const lineup = savedLineupByID(id);
  if (!lineup) return;
  state.activeSavedLineupID = lineup.id;
  state.savedLineupGroupCategoryID = "";
  setCategory(lineup.categoryId, { preserveSavedLineup: true });
}
function activeSavedLineup() {
  const lineup = savedLineupByID(state.activeSavedLineupID);
  return lineup && lineup.categoryId === state.category ? lineup : null;
}
async function hydrateApp(payload, options) {
  options = options || {};
  const previousApp = state.app || {};
  payload = payload || {};
  payload.programs = Array.isArray(payload.programs) ? payload.programs : items(previousApp.programs);
  if (typeof payload.isAdmin !== "boolean" && options.reuseSettings && typeof previousApp.isAdmin === "boolean") payload.isAdmin = previousApp.isAdmin;
  state.app = payload;
  const previousPrefs = previousApp.preferences || null;
  let prefsJustLoaded = false;
  if (options.localCache) {
    state.app.preferences = previousPrefs || defaultPrefs();
    state.adminCategorySettings = defaultAdminCategorySettings();
  } else if (options.reuseSettings) {
    state.app.preferences = previousPrefs || defaultPrefs();
  } else {
    const loadFailed = {};
    const values = await loadPluginSettingsValues().catch(function(error) {
      try { console.warn("Dispatcharr profile preference load failed", error); } catch (_) {}
      return loadFailed;
    });
    if (values === loadFailed) {
      // Keep whatever this tab already has; saves stay pending until a load succeeds.
      state.app.preferences = previousPrefs || mergePrefs(state.app.preferences);
    } else {
      const siloPrefs = readSiloPrefsValue(values && values.preferences ? values.preferences : "");
      state.app.preferences = mergePrefs(siloPrefs || state.app.preferences);
      prefsJustLoaded = true;
    }
    const savedAdminSettings = values && values[adminSettingsKey] ? readAdminSettingsValue(values[adminSettingsKey]) : null;
    state.adminCategorySettings = await loadAdminCategorySettings().catch(function() {
      return savedAdminSettings || defaultAdminCategorySettings();
    });
  }
  state.savedAdminCategorySettings = cloneAdminCategorySettings(state.adminCategorySettings);
  state.app.programs = items(state.app.programs);
  items(state.app.channels).forEach(cleanChannelText);
  state.recentSearches = readRecentSearches();
  rebuildProgramIndex();
  normalizePreferences();
  if (prefsJustLoaded) adoptLoadedPrefs(previousPrefs);
  else if (!prefsSync.baseline) prefsSync.baseline = prefsSnapshot(state.app.preferences);
  if (!prefsSync.loaded && !options.localCache && !options.reuseSettings) schedulePrefsLoadRetry();
  normalizeAdminCategorySettings();
  state.savedAdminCategorySettings = cloneAdminCategorySettings(state.adminCategorySettings);
  if (!options.localCache) writeLocalAppCache(state.app);
}
async function refreshStatusData() {
  const status = await getJSON("/dispatcharr/api/status");
  if (state.app) {
    state.app.status = status || {};
    if (state.app.source && status && status.profileAccess) state.app.source.profileAccess = status.profileAccess;
  }
  return status || {};
}
// The guide is loaded in time windows (/api/guide?start=&end=) instead of as
// one full blob: the initial load covers now-3h..now+24h, and later windows are
// fetched on demand (guide window moving forward, search / My TV look-ahead).
// If the backend ignores the window params the full guide comes back; that is
// detected and treated as full coverage.
const guideLookbehindSeconds = 3 * 3600;
const guideInitialLookaheadSeconds = 24 * 3600;
const guideSearchLookaheadSeconds = 7 * 24 * 3600;
const guideCoverage = { start: 0, end: 0, full: false, pending: null };
function guideProgramKey(program) {
  if (program && program.id !== undefined && program.id !== null && String(program.id)) return "id:" + String(program.id);
  return String(program && program.channelId || "") + "@" + Number(program && program.startUnix || 0) + "@" + String(program && program.title || "");
}
function programOverlapsWindow(program, start, end) {
  const programStart = Number(program && program.startUnix || 0);
  const programEnd = Number(program && program.endUnix || 0) || programStart + 1800;
  return programEnd > start && programStart < end;
}
function mergeGuideWindowPrograms(existing, fresh, start, end) {
  const floor = Math.floor(Date.now() / 1000) - guideLookbehindSeconds - 3600;
  const merged = {};
  const order = [];
  const add = function(program) {
    const key = guideProgramKey(program);
    if (!merged[key]) order.push(key);
    merged[key] = program;
  };
  items(existing).forEach(function(program) {
    const programEnd = Number(program && program.endUnix || 0);
    if (programEnd && programEnd < floor) return;
    // Programs inside the refreshed window are replaced by the fresh copy.
    if (programOverlapsWindow(program, start, end)) return;
    add(program);
  });
  items(fresh).forEach(add);
  return order.map(function(key) { return merged[key]; });
}
function guideWindowIgnoredByBackend(programs, start, end) {
  return items(programs).some(function(program) {
    const programStart = Number(program && program.startUnix || 0);
    const programEnd = Number(program && program.endUnix || 0);
    return programStart >= end || (programEnd && programEnd <= start);
  });
}
async function fetchGuideWindow(start, end) {
  if (!state.app) return false;
  start = Math.floor(start);
  end = Math.floor(end);
  if (end <= start) return false;
  const payload = guideCoverage.full ? await getJSON("/dispatcharr/api/guide") : await getJSON("/dispatcharr/api/guide?start=" + start + "&end=" + end);
  if (!state.app || !payload) return false;
  const programs = items(payload.programs);
  if (guideCoverage.full || guideWindowIgnoredByBackend(programs, start, end)) {
    guideCoverage.full = true;
    state.app.programs = programs;
  } else {
    state.app.programs = mergeGuideWindowPrograms(state.app.programs, programs, start, end);
    guideCoverage.start = guideCoverage.start ? Math.min(guideCoverage.start, start) : start;
    guideCoverage.end = Math.max(guideCoverage.end, end);
  }
  rebuildProgramIndex();
  return true;
}
// Make sure programs up to `untilUnix` are loaded; resolves true when new data arrived.
function ensureGuideCoverage(untilUnix) {
  if (!state.app || guideCoverage.full || !guideCoverage.end) return Promise.resolve(false);
  untilUnix = Math.floor(Number(untilUnix || 0));
  if (untilUnix <= guideCoverage.end) return Promise.resolve(false);
  if (guideCoverage.pending) return guideCoverage.pending;
  guideCoverage.pending = fetchGuideWindow(guideCoverage.end, untilUnix).catch(function(error) {
    try { console.warn("Dispatcharr guide window load failed", error); } catch (_) {}
    return false;
  }).finally(function() {
    guideCoverage.pending = null;
  });
  return guideCoverage.pending;
}
function ensureGuideCoverageForView(view) {
  let until = 0;
  if (view === "guide") until = guideWindow().end;
  else if (view === "search" || view === "mytv") until = Math.floor(Date.now() / 1000) + guideSearchLookaheadSeconds;
  if (!until) return;
  ensureGuideCoverage(until).then(function(loaded) {
    if (!loaded || state.view !== view) return;
    if (view === "guide") refreshVisibleGuideBlock();
    else if (view === "search") updateSearchPageResults();
    else if (view === "mytv") updateMyTVSearchSurface();
  });
}
async function refreshSupplementalData() {
  if (!state.app) return;
  const now = Math.floor(Date.now() / 1000);
  const start = now - guideLookbehindSeconds;
  // Periodic refreshes only re-read the near window; farther windows fetched on
  // demand stay merged in.
  const end = now + guideInitialLookaheadSeconds;
  await fetchGuideWindow(start, end).catch(function(error) {
    try { console.warn("Dispatcharr guide load failed", error); } catch (_) {}
    return false;
  });
  rebuildProgramIndex();
  writeLocalAppCache(state.app);
  if (state.view === "guide" || state.view === "search" || state.view === "mytv") ensureGuideCoverageForView(state.view);
}
async function loadApp() {
  const cached = readLocalAppCache();
  let renderedCachedApp = false;
  if (cached) {
    await hydrateApp(cached, { localCache: true });
    state.appLoadedFromCache = true;
    renderedCachedApp = true;
    try { render(); } catch (error) {
      try { console.warn("Dispatcharr cached render failed", error); } catch (_) {}
    }
  }
  try {
    await hydrateApp(await getJSON("/dispatcharr/api/app"));
    state.appLoadedFromCache = false;
    await loadRecordingCapability();
    render();
    await refreshSupplementalData();
    render();
  } catch (error) {
    if (!renderedCachedApp) throw error;
    showAppToast("Showing saved guide. Refresh failed.");
    try { console.warn("Dispatcharr app refresh failed", error); } catch (_) {}
  }
}
async function loadAdminApp() {
  state.app = { preferences: defaultPrefs(), channels: [], categories: [], programs: [], source: { mode: "direct_login", profiles: [] }, status: {} };
  state.adminConnectionLoading = true;
  state.adminConnectionLoadError = "";
  state.adminSourceGroupsLoaded = false;
  state.adminSourceGroupsLoading = false;
  state.adminSourceGroupsError = "";
  const results = await Promise.all([
    getJSON("/dispatcharr/api/status").catch(function() { return {}; }),
    getJSON("/dispatcharr/api/admin-settings").catch(function() { return defaultAdminCategorySettings(); }),
    getJSON("/dispatcharr/api/admin-connection").catch(function(error) {
      state.adminConnectionLoadError = readableError(error) || "Could not load the saved connection.";
      return null;
    })
  ]);
  state.app.status = results[0] || {};
  state.adminCategorySettings = readAdminSettingsValue(results[1]);
  normalizeAdminCategorySettings();
  state.savedAdminCategorySettings = cloneAdminCategorySettings(state.adminCategorySettings);
  state.adminConnection = normalizeAdminConnection(results[2] || state.adminConnection || state.savedAdminConnection);
  state.savedAdminConnection = cloneAdminConnection(state.adminConnection);
  state.adminConnectionLoading = false;
  state.app.source.mode = state.adminConnection.sourceMode;
  if (state.app.status && state.app.status.profileAccess) state.app.source.profileAccess = state.app.status.profileAccess;
  render();
  if (state.adminTab === "organization") loadAdminSourceGroups(false);
}
async function reloadAdminConnection() {
  if (state.adminConnectionLoading) return;
  state.adminConnectionLoading = true;
  state.adminConnectionLoadError = "";
  renderAdminPage();
  try {
    const connection = normalizeAdminConnection(await getJSON("/dispatcharr/api/admin-connection"));
    state.adminConnection = connection;
    state.savedAdminConnection = cloneAdminConnection(connection);
    if (state.app && state.app.source) state.app.source.mode = connection.sourceMode;
  } catch (error) {
    state.adminConnectionLoadError = readableError(error) || "Could not load the saved connection.";
  } finally {
    state.adminConnectionLoading = false;
    renderAdminPage();
  }
}
async function loadAdminSourceGroups(force) {
  if (state.adminSourceGroupsLoading || (state.adminSourceGroupsLoaded && !force)) return;
  state.adminSourceGroupsLoading = true;
  state.adminSourceGroupsError = "";
  renderAdminPage();
  try {
    const payload = await getJSON("/dispatcharr/api/channels");
    state.app.channels = items(payload && payload.channels);
    state.app.categories = items(payload && payload.categories);
    state.app.source.profiles = items(payload && payload.profiles);
    state.profileSelectionIDMap = null;
    state.profileChannelFilterMap = null;
    state.adminSourceGroupsLoaded = true;
  } catch (error) {
    state.adminSourceGroupsError = readableError(error) || "Could not load source groups.";
  } finally {
    state.adminSourceGroupsLoading = false;
    renderAdminPage();
  }
}
function guideHasPrograms() {
  return items(state.app && state.app.programs).length > 0;
}
function epgLastSuccessUnix() {
  const status = state.app && state.app.status ? state.app.status : {};
  return Number(status.epgLastSuccessUnix || 0);
}
function guideUpdatedUnix() {
  const status = state.app && state.app.status ? state.app.status : {};
  return epgLastSuccessUnix() || Number(status.lastSuccessUnix || 0);
}
function guideFreshnessHTML() {
  if (state.refreshing) {
    return "<span class=\"guide-freshness is-refreshing\" role=\"status\" aria-live=\"polite\" aria-atomic=\"true\">Refreshing guide...</span>";
  }
  const unix = guideUpdatedUnix();
  const title = unix ? dateTimeLabel(unix) : "Guide has not synced yet";
  const stale = !unix || Math.floor(Date.now() / 1000) - unix > 6 * 3600;
  return "<span class=\"guide-freshness" + (stale ? " is-stale" : "") + "\" title=\"" + escapeHTML(title) + "\">" + escapeHTML(relativeUpdatedLabel(unix)) + "</span>";
}
function setSettingsMenuOpen(open) {
  const menu = byId("settings-menu");
  const button = byId("settings-menu-button");
  if (!menu || !button) return;
  const shell = menu.closest(".settings-menu");
  if (shell) shell.classList.toggle("open", !!open);
  button.setAttribute("aria-expanded", open ? "true" : "false");
}
function settingsMenuOpen() {
  const menu = byId("settings-menu");
  return !!(menu && menu.closest(".settings-menu") && menu.closest(".settings-menu").classList.contains("open"));
}
function guideRefreshAdvanced(previousEPGSuccess) {
  const previous = Number(previousEPGSuccess || 0);
  return epgLastSuccessUnix() > previous || (!previous && guideHasPrograms());
}
function guideNeedsFollowupRefresh(previousEPGSuccess) {
  const status = state.app && state.app.status ? state.app.status : {};
  const epgStatus = String(status.epgStatus || "").toLowerCase();
  const refreshState = String((status.refresh || {}).state || "").toLowerCase();
  return refreshState === "queued" || refreshState === "running" || epgStatus === "loading" || !guideRefreshAdvanced(previousEPGSuccess);
}
async function pollGuideRefresh(previousEPGSuccess) {
  for (let attempt = 0; attempt < 300 && guideNeedsFollowupRefresh(previousEPGSuccess); attempt++) {
    await new Promise(function(resolve) { setTimeout(resolve, 2000); });
    await refreshStatusData();
    const status = state.app && state.app.status ? state.app.status : {};
    const refresh = status.refresh || {};
    const refreshState = String(refresh.state || "").toLowerCase();
    if (refreshState === "failed" || refreshState === "canceled") throw new Error(refresh.error || "guide refresh did not complete");
    if (refreshState === "succeeded") return;
    if (String(status.epgStatus || "").toLowerCase() === "failed") break;
  }
  if (guideNeedsFollowupRefresh(previousEPGSuccess)) throw new Error("guide refresh timed out");
}
function updateGuideFreshnessBlock() {
  const freshness = document.querySelector(".guide-freshness");
  if (freshness) freshness.outerHTML = guideFreshnessHTML();
}
function refreshVisibleGuideBlock() {
  updateGuideFreshnessBlock();
  if (state.view === "guide") {
    const guideScroll = byId("guide-scroll");
    const scrollLeft = guideScroll ? guideScroll.scrollLeft : 0;
    const scrollTop = guideScroll ? guideScroll.scrollTop : 0;
    refreshGuideTimeline();
    resetGuideRows();
    renderEPG();
    if (guideScroll) {
      guideScroll.scrollLeft = scrollLeft;
      guideScroll.scrollTop = scrollTop;
    }
    renderGuideProgramSearch();
    return;
  }
  render();
}
function setGuideRefreshButtonsLoading(loading) {
  Array.prototype.slice.call(document.querySelectorAll("[data-guide-refresh]")).forEach(function(button) {
    button.classList.toggle("is-loading", !!loading);
    button.disabled = !!loading;
    button.setAttribute("aria-busy", loading ? "true" : "false");
  });
}
async function refreshGuideBlockData() {
  if (state.refreshing) return;
  const previousEPGSuccess = epgLastSuccessUnix();
  state.refreshing = true;
  setGuideRefreshButtonsLoading(true);
  refreshVisibleGuideBlock();
  showAppToast("Refreshing guide...");
  try {
    await hydrateApp(await postJSON("/dispatcharr/api/refresh", {}), { reuseSettings: true });
    refreshVisibleGuideBlock();
    if (guideNeedsFollowupRefresh(previousEPGSuccess)) {
      await pollGuideRefresh(previousEPGSuccess);
    }
    await refreshSupplementalData();
    state.recordings = null;
    refreshVisibleGuideBlock();
    showAppToast(guideRefreshAdvanced(previousEPGSuccess) ? "Guide refreshed from Dispatcharr." : "Guide refresh finished without newer EPG data.");
  } catch (error) {
    if (!adminOnlyDenied(error, "force a guide refresh")) showAppToast("Dispatcharr refresh failed.");
  } finally {
    state.refreshing = false;
    setGuideRefreshButtonsLoading(false);
    refreshVisibleGuideBlock();
  }
}
function renderRail() {
  const browseButton = byId("primary-browse-nav");
  if (browseButton && browseButton.dataset) {
    const showChannels = channelGroupsInSideMenu();
    browseButton.dataset.view = showChannels ? "channels" : "guide";
    browseButton.dataset.activeViews = showChannels ? "channels live" : "guide";
    const label = browseButton.querySelector("span");
    if (label) label.textContent = showChannels ? "Channels" : "Guide";
  }
  document.querySelectorAll("[data-view]").forEach(function(button) {
    const unavailable = (button.dataset.view === "recordings" && !dvrEnabled()) || (button.dataset.view === "sports" && !sportsNavAvailable()) || (button.dataset.view === "events" && !eventsNavAvailable()) || (isSportsApp() && liveTVOnlyView(button.dataset.view));
    const activeViews = String(button.dataset.activeViews || button.dataset.view || "").split(/\s+/).filter(Boolean);
    button.hidden = unavailable;
    const active = !unavailable && activeViews.indexOf(state.view) !== -1;
    button.classList.toggle("active", active);
    if (active) button.setAttribute("aria-current", "page");
    else button.removeAttribute("aria-current");
  });
  document.querySelectorAll("[data-guide-refresh]").forEach(function(button) {
    button.hidden = isSportsApp() || !siloUserIsAdmin();
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
  if (channel && channel.logoUrl) return "<img class=\"logo\" src=\"" + escapeHTML(channel.logoUrl) + "\" alt=\"\" data-img-error=\"logo-fallback\">" + fallback;
  return fallback;
}
function resetMainHorizontalScroll() {
  const main = document.querySelector(".main");
  if (main && main.scrollLeft) main.scrollLeft = 0;
}
function render() {
  if (!state.app) return;
  if (state.view === "recordings" && !dvrEnabled()) state.view = defaultBrowseView();
  if (state.view === "sports" && !sportsNavAvailable()) state.view = defaultBrowseView();
  if (state.view === "events" && !eventsNavAvailable()) state.view = defaultBrowseView();
  if (state.view === "admin" && !isAdminRoute) state.view = defaultBrowseView();
  if (isSportsApp() && liveTVOnlyView(state.view)) state.view = "sports";
  const shell = document.querySelector(".shell");
  if (shell) {
    shell.classList.toggle("is-player", state.view === "player");
    shell.classList.toggle("is-guide", state.view === "guide");
    shell.classList.toggle("is-channels", state.view === "channels");
    shell.classList.toggle("is-sports", state.view === "sports");
    shell.classList.toggle("is-sports-app", isSportsApp());
    shell.classList.toggle("is-events", state.view === "events");
    shell.classList.toggle("is-multiview", state.view === "multiview");
    shell.classList.toggle("is-search", state.view === "search");
    shell.classList.toggle("is-onlater", state.view === "mytv");
    shell.classList.toggle("is-mytv", state.view === "mytv");
  }
  if (state.view !== "channels") stopChannelsPreview();
  resetMainHorizontalScroll();
  renderRail();
  renderSportsTopbarTabs();
  if (state.view === "guide") renderGuidePage();
  else if (state.view === "channels") renderChannelsPage();
  else if (state.view === "player") renderPlayerPage();
  else if (state.view === "multiview") renderMultiviewPage();
  else if (state.view === "live" || state.view === "favorites") renderLivePage();
  else if (state.view === "sports") renderSportsPage();
  else if (state.view === "events") renderEventsPage();
  else if (state.view === "mytv") renderMyTVPage();
  else if (state.view === "search") renderSearchPage();
  else if (state.view === "recordings") renderRecordingsPage();
  else if (state.view === "admin") renderAdminPage();
  else if (state.view === "settings") renderSettings();
  else renderHome();
}
function guideSearchFocused() {
  return document.activeElement && document.activeElement.id === "guide-search";
}
function startGuideAutoRefresh() {
  if (isAdminRoute || state.guideAutoTimer) return;
  state.guideAutoTimer = setInterval(tickGuideAutoRefresh, 60000);
  document.addEventListener("visibilitychange", function() {
    if (!document.hidden) tickGuideAutoRefresh();
  });
}
async function tickGuideAutoRefresh() {
  updateGuideNowLines();
  if (!state.app || state.view !== "guide" || document.hidden || state.programDetails) return;
  const slotStart = guideSlotStart();
  if (!state.guideLastSlotStart) state.guideLastSlotStart = slotStart;
  if (slotStart !== state.guideLastSlotStart && !guideSearchFocused()) {
    state.guideLastSlotStart = slotStart;
    refreshVisibleGuideBlock();
  }

  const now = Date.now();
  if (state.guideAutoFetching || state.refreshing || now - state.guideLastAutoFetchAt < 5 * 60 * 1000) return;
  state.guideAutoFetching = true;
  state.guideLastAutoFetchAt = now;
  try {
    await refreshStatusData();
    await refreshSupplementalData();
    if (state.view !== "guide") return;
    refreshVisibleGuideBlock();
  } catch (error) {
    try { console.warn("Dispatcharr guide auto-refresh failed", error); } catch (_) {}
  } finally {
    state.guideAutoFetching = false;
  }
}
function renderHome() {
  const root = byId("view");
  const recent = recentChannels(5);
  const watched = recent.length ? recent : visibleChannels(false).slice(0, 5);
  const watchedIDs = {};
  watched.forEach(function(channel) { watchedIDs[channel.id] = true; });
  const favorites = homeFavoriteChannels().filter(function(channel) { return !watchedIDs[channel.id]; });
  const watchedOften = homeWatchedOftenChannels().filter(function(channel) { return !watchedIDs[channel.id]; });
  root.innerHTML = sectionHeader("Recently watched")
    + rowCards(watched)
    + (favorites.length ? sectionHeader("Favorites") + favoriteHomeCards(favorites) : "")
    + (!favorites.length && watchedOften.length ? sectionHeader("Watched often") + favoriteHomeCards(watchedOften) : "")
    + renderSavedLineupsHome()
    + sectionHeaderWithActions("TV Guide", "<button type=\"button\" class=\"section-action\" data-view=\"guide\">Open Full Guide</button>" + guideFreshnessHTML())
    + renderHomeGuide(homeGuideChannels(watched), "No current guide data for recently watched channels.", { hideFreshness: true, shelf: true })
    + (channelGroupsInSideMenu() ? "" : categoryGrid());
  const openGuide = root.querySelector("[data-view=\"guide\"]");
  if (openGuide) openGuide.onclick = function() { setView("guide"); };
  attachHomeGuidePageScroll();
}
function renderChannelsPage() {
  const previousGroup = state.channelsBrowserRenderedGroup || "";
  const groups = document.querySelector(".channels-groups");
  const list = document.querySelector(".channels-list");
  const groupsTop = groups ? groups.scrollTop : 0;
  const listTop = list ? list.scrollTop : 0;
  const html = channelsBrowserHTML();
  const nextID = state.channelsBrowserChannel || "";
  const previewRoot = document.querySelector(".channels-preview");
  const keepPreview = !!(previewRoot && state.channelsPreview && previewRoot.getAttribute("data-channels-preview") === nextID);
  if (keepPreview) previewRoot.remove();
  else stopChannelsPreview();
  byId("view").innerHTML = html;
  const slot = document.querySelector(".channels-preview");
  if (keepPreview && slot) slot.replaceWith(previewRoot);
  else if (nextID) attachChannelsPreview();
  const nextGroups = document.querySelector(".channels-groups");
  const nextList = document.querySelector(".channels-list");
  if (nextGroups) nextGroups.scrollTop = groupsTop;
  if (nextList && previousGroup === state.channelsBrowserGroup) nextList.scrollTop = listTop;
  state.channelsBrowserRenderedGroup = state.channelsBrowserGroup || "";
}
function stopChannelsPreview() {
  if (state.channelsPreview && typeof state.channelsPreview.destroy === "function") state.channelsPreview.destroy();
  state.channelsPreview = null;
  state.channelsPreviewChannelID = "";
}
function markChannelsPreviewFailed(channelID) {
  if (state.channelsPreviewChannelID !== channelID) return;
  const root = document.querySelector(".channels-preview");
  if (root && root.getAttribute("data-channels-preview") === channelID) root.classList.add("is-failed");
}
function attachChannelsPreview() {
  const channel = channelByID(state.channelsBrowserChannel);
  const video = byId("channels-preview");
  if (!channel || !video || state.view !== "channels") return;
  if (state.channelsPreview && state.channelsPreviewChannelID === channel.id) return;
  stopChannelsPreview();
  state.channelsPreviewChannelID = channel.id;
  const format = channel.streamFormat || "";
  ensurePlayerLibraries(format).then(function() {
    const current = byId("channels-preview");
    if (!current || current !== video || state.view !== "channels" || state.channelsBrowserChannel !== channel.id) return;
    video.muted = true;
    video.defaultMuted = true;
    video.playsInline = true;
    const attachment = attachVideoSource(video, browserStreamURL(channel), {
      format: format,
      hlsBufferSeconds: channel.hlsBufferSeconds,
      onFatal: function() { markChannelsPreviewFailed(channel.id); }
    });
    if (state.view !== "channels" || state.channelsBrowserChannel !== channel.id || byId("channels-preview") !== video) {
      attachment.destroy();
      return;
    }
    state.channelsPreview = attachment;
    video.addEventListener("playing", function() {
      const root = video.closest(".channels-preview");
      if (root) root.classList.add("is-playing");
    });
    video.play().catch(function() {});
  }).catch(function() {
    markChannelsPreviewFailed(channel.id);
  });
}
function channelsPreviewHTML(channel) {
  const name = channel.name || "channel";
  return "<div class=\"channels-preview\" data-channels-preview=\"" + escapeHTML(channel.id) + "\"><video id=\"channels-preview\" muted playsinline autoplay aria-label=\"" + escapeHTML("Live preview, " + name) + "\"></video><div class=\"channels-preview-fallback\">" + logoHTML(channel) + "</div><div class=\"channels-preview-status\">Preview unavailable</div></div>";
}
function channelsBrowserIncludeChannel(channel) {
  const hidden = hiddenMap();
  return !(channel && channel.categoryId && hidden[channel.categoryId]);
}
function channelsBrowserCategories() {
  const sourceCategories = sourceCategoriesWithChannels(channelsBrowserIncludeChannel);
  const listing = adminListingCategories("");
  const featured = sourceCategories.filter(function(category) { return !!category.featured; }).sort(compareCategoryDisplayName);
  const featuredSourceIDs = {};
  featured.forEach(function(category) { featuredSourceIDs[category.sourceID] = true; });
  const regularListing = listing.filter(function(category) {
    return !(category.kind === "source" && featuredSourceIDs[category.sourceID]);
  });
  const categories = featured.concat(regularListing);
  return categories.length ? categories : sourceCategories;
}
function channelsBrowserChildGroups(category) {
  const id = String(category && category.id || "");
  let children = [];
  if (id.indexOf("virtual:") === 0) children = virtualChildCategories(virtualCategoryPath(id), channelsBrowserIncludeChannel);
  else if (id.indexOf("featured:") === 0) children = featuredChildCategories(featuredCategoryPath(id), channelsBrowserIncludeChannel);
  return children.map(function(child) {
    return { category: child, label: String(child.name || child.id || "") };
  });
}
function channelsBrowserSetups(categories) {
  const setups = [];
  const byKey = {};
  const loose = [];
  items(categories).forEach(function(category) {
    const split = categoryPrefixSplit(category && (category.name || category.id));
    if (split) {
      const key = split.prefix.toLowerCase();
      if (!byKey[key]) {
        byKey[key] = { prefix: split.prefix, groups: [] };
        setups.push(byKey[key]);
      }
      byKey[key].groups.push({ category: category, label: split.rest || split.prefix });
      return;
    }
    const children = channelsBrowserChildGroups(category);
    if (children.length) {
      setups.push({ prefix: String(category.name || category.id || ""), groups: children });
      return;
    }
    loose.push({ category: category, label: String(category.name || category.id || "") });
  });
  if (loose.length) setups.push({ prefix: "", groups: loose });
  return setups;
}
function channelsBrowserSelection(setups) {
  const groups = [];
  items(setups).forEach(function(setup) {
    items(setup.groups).forEach(function(group) {
      groups.push({ setup: setup, group: group });
    });
  });
  let selected = groups.find(function(entry) { return entry.group.category && entry.group.category.id === state.channelsBrowserGroup; });
  if (!selected) selected = groups[0] || null;
  state.channelsBrowserGroup = selected && selected.group.category ? selected.group.category.id : "";
  return selected;
}
function channelsForBrowserGroup(categoryID) {
  return dedupeChannels(effectiveChannels(false).filter(function(channel) {
    if (!channelsBrowserIncludeChannel(channel)) return false;
    return !categoryID || channelInSelectedCategory(channel, categoryID);
  }));
}
function channelsBrowserContext(setup, group) {
  const parts = [];
  if (setup && setup.prefix) parts.push(setup.prefix);
  if (group && group.label) parts.push(group.label);
  return parts.join(" · ");
}
function channelsBrowserProgramSubtitle(program) {
  const title = String(program && program.title || "").trim();
  const text = String(program && program.description || "").replace(/\s+/g, " ").trim();
  if (!text || lower(text) === lower(title)) return "";
  return text;
}
function channelsBrowserProgress(program) {
  const start = Number(program && program.startUnix || 0);
  const end = Number(program && program.endUnix || 0);
  if (!start || !end || end <= start) return 0;
  return Math.max(0, Math.min(1, (Date.now() / 1000 - start) / (end - start)));
}
function channelsBrowserWindow(program) {
  const start = timeLabel(program && program.startUnix);
  const end = timeLabel(program && program.endUnix);
  if (start && end) return start + "–" + end;
  return start || end || "";
}
function channelsBrowserUpcoming(channel) {
  const now = Math.floor(Date.now() / 1000);
  return programsFor(channel && channel.id).filter(function(program) {
    return (program.startUnix || 0) > now;
  }).slice(0, 3);
}
function channelsBrowserGroupButton(setup, group, active) {
  const category = group.category || {};
  const count = Number(category.count) || 0;
  const context = channelsBrowserContext(setup, group);
  return "<button type=\"button\" class=\"channels-group" + (active ? " active" : "") + "\" data-channels-group=\"" + escapeHTML(category.id || "") + "\" aria-pressed=\"" + (active ? "true" : "false") + "\" aria-label=\"" + escapeHTML(context || group.label || "Group") + "\"><span>" + escapeHTML(group.label || "Group") + "</span><small>" + escapeHTML(String(count)) + "</small></button>";
}
function channelsBrowserRow(channel, context, active) {
  const program = currentProgram(channel) || {};
  const title = program.title || "No program data";
  const subtitle = channelsBrowserProgramSubtitle(program);
  const progress = program.startUnix ? Math.round(channelsBrowserProgress(program) * 100) : 0;
  const when = channelsBrowserWindow(program);
  return "<button type=\"button\" class=\"channels-row" + (active ? " active" : "") + "\" data-channels-channel=\"" + escapeHTML(channel.id) + "\" aria-pressed=\"" + (active ? "true" : "false") + "\"><span class=\"channels-mark\">" + logoHTML(channel) + "</span><span class=\"channels-name\"><strong>" + escapeHTML(channel.name || "Untitled") + "</strong><span>" + escapeHTML(context || channel.categoryName || "Live TV") + "</span></span><span class=\"channels-program\"><strong>" + escapeHTML(title) + "</strong>" + (subtitle ? "<span>" + escapeHTML(subtitle) + "</span>" : "") + (program.startUnix ? "<span class=\"channels-bar\"><i style=\"width:" + progress + "%\"></i></span>" : "") + "</span><span class=\"channels-when\">" + escapeHTML(when) + "</span></button>";
}
function channelsBrowserDetail(channel, context) {
  if (!channel) return "<aside class=\"channels-detail\"><div class=\"empty\">Select a channel.</div></aside>";
  const program = currentProgram(channel) || {};
  const title = program.title || "No program data";
  const subtitle = channelsBrowserProgramSubtitle(program);
  const progress = program.startUnix ? Math.round(channelsBrowserProgress(program) * 100) : 0;
  const when = channelsBrowserWindow(program);
  const number = channel.number ? "Channel " + channel.number : "";
  const meta = [context, number].filter(Boolean).join(" · ");
  const upcoming = channelsBrowserUpcoming(channel);
  const next = upcoming.length ? "<div class=\"channels-next\"><span class=\"channels-eyebrow\">Next</span>" + upcoming.map(function(item) {
    return "<div class=\"channels-next-row\"><b>" + escapeHTML(timeLabel(item.startUnix) || "Later") + "</b><span>" + escapeHTML(item.title || "Untitled") + "</span></div>";
  }).join("") + "</div>" : "";
  return "<aside class=\"channels-detail\">" + channelsPreviewHTML(channel) + "<div class=\"channels-detail-copy\"><h3>" + escapeHTML(channel.name || "Untitled") + "</h3><p class=\"channels-sub\">" + escapeHTML(meta || "Live TV") + "</p><div class=\"channels-now\"><span class=\"channels-eyebrow\">On now</span><strong>" + escapeHTML(title) + "</strong>" + (subtitle ? "<p>" + escapeHTML(subtitle) + (when ? " " + escapeHTML(when) + "." : "") + "</p>" : (when ? "<p>" + escapeHTML(when) + "</p>" : "")) + (program.startUnix ? "<span class=\"channels-bar\"><i style=\"width:" + progress + "%\"></i></span>" : "") + "<button type=\"button\" class=\"channels-watch\" data-channel=\"" + escapeHTML(channel.id) + "\">" + icon("play") + "<span>Watch</span></button></div>" + next + "</div></aside>";
}
function channelsBrowserHTML() {
  const setups = channelsBrowserSetups(channelsBrowserCategories());
  const selected = channelsBrowserSelection(setups);
  if (!selected) return "<div class=\"empty\">No groups yet.</div>";
  const context = channelsBrowserContext(selected.setup, selected.group);
  const channels = channelsForBrowserGroup(selected.group.category && selected.group.category.id);
  let channel = channels.find(function(item) { return item.id === state.channelsBrowserChannel; });
  if (!channel) channel = channels[0] || null;
  state.channelsBrowserChannel = channel ? channel.id : "";
  maybeWarmGuideForChannels(channels.slice(0, 40), "channels:" + state.channelsBrowserGroup);
  const groups = setups.map(function(setup) {
    const total = setup.groups.reduce(function(sum, group) { return sum + (Number(group.category && group.category.count) || 0); }, 0);
    const heading = setup.prefix ? "<h2>" + escapeHTML(setup.prefix) + "<span>" + escapeHTML(String(total)) + "</span></h2>" : "";
    return "<section class=\"channels-setup\">" + heading + setup.groups.map(function(group) {
      const active = !!(selected.group.category && group.category && group.category.id === selected.group.category.id);
      return channelsBrowserGroupButton(setup, group, active);
    }).join("") + "</section>";
  }).join("");
  const rows = channels.length
    ? channels.map(function(item) { return channelsBrowserRow(item, context, !!(channel && item.id === channel.id)); }).join("")
    : "<div class=\"empty\">No channels in this group.</div>";
  return "<div class=\"channels-stage\"><aside class=\"channels-groups\" aria-label=\"Channel groups\">" + groups + "</aside><div class=\"channels-list\" aria-label=\"" + escapeHTML(context || "Channels") + "\">" + rows + "</div>" + channelsBrowserDetail(channel, context) + "</div>";
}
function selectChannelsBrowserGroup(id) {
  id = String(id || "");
  if (!id || id === state.channelsBrowserGroup) return;
  state.channelsBrowserGroup = id;
  state.channelsBrowserChannel = "";
  renderChannelsPage();
}
function selectChannelsBrowserChannel(id) {
  id = String(id || "");
  if (!id || id === state.channelsBrowserChannel) return;
  state.channelsBrowserChannel = id;
  renderChannelsPage();
}
function channelsForSavedLineup(lineup) {
  if (!lineup || !lineup.categoryId) return [];
  const hidden = hiddenMap();
  return dedupeChannels(effectiveChannels(false).filter(function(channel) {
    if (channel.categoryId && hidden[channel.categoryId]) return false;
    return channelInSelectedCategory(channel, lineup.categoryId);
  }));
}
function savedLineupCategoryLabel(categoryID) {
  const id = String(categoryID || "");
  const path = id.indexOf("virtual:") === 0 ? virtualCategoryPath(id) : (id.indexOf("featured:") === 0 ? featuredCategoryPath(id) : "");
  if (path) return path.split(" / ").join(" | ");
  return categoryName(id) || "Saved lineup";
}
function savedLineupCountLabel(count) {
  const total = Number(count) || 0;
  if (!total) return "No channels";
  return total === 1 ? "1 channel" : total + " channels";
}
function savedLineupFaceHTML(channels) {
  const channel = items(channels).find(function(item) { return item && item.logoUrl; });
  if (!channel) return "<span class=\"saved-lineup-glyph\">" + icon("collection") + "</span>";
  return "<img src=\"" + escapeHTML(channel.logoUrl) + "\" alt=\"\">";
}
function savedLineupHomeCard(name, meta, openAttribute, channels, editHTML) {
  return "<div class=\"saved-lineup-card\"><button type=\"button\" class=\"continue-card recent-channel-card saved-lineup-open\" " + openAttribute + " aria-label=\"" + escapeHTML(name + " - " + meta) + "\"><div class=\"poster-box\">" + savedLineupFaceHTML(channels) + "</div><span class=\"recent-channel-copy\"><strong>" + escapeHTML(name) + "</strong><span class=\"muted\">" + escapeHTML(meta) + "</span></span></button>" + (editHTML || "") + "</div>";
}
function renderSavedLineupsHome() {
  const lineups = savedLineups();
  const groups = customGroupCategories();
  if (!lineups.length && !groups.length) return "";
  const lineupCards = lineups.map(function(lineup) {
    const channels = channelsForSavedLineup(lineup);
    const meta = savedLineupCountLabel(channels.length);
    const edit = "<button type=\"button\" class=\"saved-lineup-edit\" data-saved-lineup-edit=\"" + escapeHTML(lineup.id) + "\" aria-label=\"Edit " + escapeHTML(lineup.name) + "\" title=\"Edit saved lineup\">" + icon("settings") + "</button>";
    return savedLineupHomeCard(lineup.name, meta, "data-saved-lineup-open=\"" + escapeHTML(lineup.id) + "\"", channels, edit);
  });
  const groupCards = groups.map(function(group) {
    const channels = channelsForSavedLineup({ categoryId: group.id });
    return savedLineupHomeCard(group.name, savedLineupCountLabel(group.count || channels.length), "data-category=\"" + escapeHTML(group.id) + "\"", channels, "");
  });
  return sectionHeader("Saved Lineups") + "<div class=\"row-scroll recent-channel-row saved-lineup-row\">" + lineupCards.concat(groupCards).join("") + "</div>";
}
function emptyStateHTML(title, detail) {
  detail = String(detail || "").trim();
  return "<div class=\"empty\"><strong>" + escapeHTML(title) + "</strong>" + (detail ? "<div class=\"muted\">" + escapeHTML(detail) + "</div>" : "") + "</div>";
}
function handleAppBootFailure(error) {
  try { console.error("Dispatcharr boot failed", error); } catch (_) {}
  if (state.app) {
    try { render(); } catch (_) {}
    return;
  }
  const root = byId("view");
  if (!root) return;
  root.setAttribute("role", "status");
  root.innerHTML = emptyStateHTML(
    isAdminRoute ? "Unable to load Dispatcharr Admin." : "Unable to load Live TV.",
    isAdminRoute ? "Refresh this page or return to Silo Admin." : "Check your Dispatcharr connection in Dispatcharr Admin, then refresh this page."
  );
}
function friendlyCatalogError(message) {
  const text = lower(message);
  if (isAdminRoute) return String(message || "");
  if (/html instead of json|status 30[12]|status 40[13]|unauthori[sz]ed|forbidden/.test(text)) return "Live TV can't sign in to Dispatcharr right now. An administrator needs to check the connection in Dispatcharr Admin.";
  if (/timeout|deadline|timed out|connection refused|no such host|unreachable/.test(text)) return "Live TV can't reach Dispatcharr right now. Try again in a few minutes.";
  return "Live TV couldn't load channels from Dispatcharr. Try again in a few minutes.";
}
function catalogEmptyDetail() {
  if (!state.app || !state.app.status) return "Check your connection in Dispatcharr Admin or press Refresh.";
  const status = state.app.status;
  if (status.status === "error" && status.lastError) return friendlyCatalogError(status.lastError);
  if (!status.channelCount) return "No channels synced yet. Run a sync from Dispatcharr Admin or press Refresh.";
  return "Try Refresh or open Dispatcharr Admin to verify the connection.";
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
function realProgram(program, channel) {
  return !!(program && String(program.title || "").trim() && !programIsGuidePlaceholder(program) && !programEchoesChannel(program, channel));
}
function channelProgramLine(channel) {
  const program = currentProgram(channel);
  if (realProgram(program, channel)) return String(program.title).trim();
  const next = nextProgram(channel);
  if (realProgram(next, channel)) return "Next " + (timeLabel(next.startUnix) || "soon") + " · " + String(next.title).trim();
  return "No guide listing";
}
function channelCardHTML(channel, cardClass) {
  const channelName = channel.name || "Untitled";
  const subtitle = channelProgramLine(channel);
  return "<button type=\"button\" class=\"continue-card " + cardClass + "\" data-channel=\"" + escapeHTML(channel.id) + "\" aria-label=\"Watch " + escapeHTML(channelName + " - " + subtitle) + "\"><div class=\"poster-box\">" + (channel.logoUrl ? "<img src=\"" + escapeHTML(channel.logoUrl) + "\" alt=\"\">" : "<span>" + escapeHTML(channelName.slice(0, 5)) + "</span>") + "</div><span class=\"recent-channel-copy\"><strong>" + escapeHTML(channelName) + "</strong><span class=\"muted\" data-overflow-tooltip=\"" + escapeHTML(subtitle) + "\">" + escapeHTML(subtitle) + "</span></span></button>";
}
function rowCards(channels) {
  if (!channels.length) return emptyStateHTML("No channels yet.", catalogEmptyDetail());
  return "<div class=\"row-scroll recent-channel-row\">" + channels.map(function(channel) {
    return channelCardHTML(channel, "recent-channel-card");
  }).join("") + "</div>";
}
function homeFavoriteChannels() {
  return orderedFavoriteChannels(visibleChannels(true)).filter(function(channel) {
    return !!favoriteMap()[channel.id];
  }).slice(0, 10);
}
function homeWatchedOftenChannels() {
  return visibleChannels(true).filter(function(channel) {
    return !!autoFavoriteMap()[channel.id] && !favoriteMap()[channel.id];
  }).slice(0, 10);
}
function favoriteHomeCards(channels) {
  return "<div class=\"row-scroll recent-channel-row favorites-row\">" + channels.map(function(channel) {
    return channelCardHTML(channel, "recent-channel-card home-favorite-card");
  }).join("") + "</div>";
}
function searchNeedle() {
  return lower(state.searchQuery).trim();
}
function searchableChannels() {
  const hidden = hiddenMap();
  return dedupeChannels(effectiveChannels(false).filter(function(channel) {
    return !(channel.categoryId && hidden[channel.categoryId]);
  }));
}
function channelMatchesSearch(channel, query) {
  const haystack = [channel.name, channel.number, channel.categoryName, sourceCategoryLabel(channel), sourceCategoryRawLabel(channel)].join(" ");
  return lower(haystack).indexOf(query) !== -1;
}
function programMatchesSearch(program, query) {
  if (programIsGuidePlaceholder(program)) return false;
  const channel = channelByID(program.channelId) || {};
  const haystack = [program.title, program.summary, program.description, channel.name, channel.categoryName].join(" ");
  return lower(haystack).indexOf(query) !== -1;
}
function allDiscoveryGroups() {
  const hidden = hiddenMap();
  const includeChannel = function(channel) { return !(channel.categoryId && hidden[channel.categoryId]); };
  return customGroupCategories()
    .concat(featuredCategoriesFromPaths("", includeChannel, true))
    .concat(virtualCategoriesFromPaths("", includeChannel, true))
    .concat(sourceCategoriesWithChannels(includeChannel))
    .sort(compareCategoryDisplayName);
}
function groupMatchesSearch(group, query) {
  return lower([group.name, group.id, group.kind].join(" ")).indexOf(query) !== -1;
}
function normalizeProgramTitle(title) {
  return lower(title).replace(/\s+/g, " ").replace(/\s+\(\d{4}\)$/, "").trim();
}
function guideUnavailableLabel() {
  return "No Guide Data Available";
}
function programIsGuidePlaceholder(program) {
  const title = normalizeProgramTitle(program && program.title).replace(/[.!…]+$/, "");
  return /^(tba|tbd|to be announced|to be determined|sendepause|sign(ing|ed)? ?off|off[- ]?air|station (is )?off[- ]?air|no games? today|data not available|no (guide )?(data|information|programming|listings?)( available)?|programming unavailable|information not available|close ?down|end of (transmission|broadcast))$/.test(title);
}
function comparableChannelText(value) {
  return lower(cleanProviderText(value)).replace(/[^a-z0-9]+/g, "");
}
function programEchoesChannel(program, channel) {
  const title = comparableChannelText(program && program.title);
  return !!title && title === comparableChannelText(channel && channel.name);
}
function programSearchText(program) {
  const channel = channelByID(program.channelId) || {};
  return [program.title, program.summary, program.description, channel.name, channel.categoryName].join(" ");
}
function recordingMatchesSearch(recording, query) {
  const haystack = [recordingTitle(recording), recordingChannelName(recording), recordingStatus(recording)].join(" ");
  return lower(haystack).indexOf(query) !== -1;
}
function programIsLive(program) {
  const now = Math.floor(Date.now() / 1000);
  return (program.startUnix || 0) <= now && (program.endUnix || 0) > now;
}
function programIsUpcoming(program) {
  return (program.startUnix || 0) > Math.floor(Date.now() / 1000);
}
function programLooksSports(program) {
  if (programIsGuidePlaceholder(program)) return false;
  const channel = channelByID(program.channelId) || {};
  const text = [program.title, channel.name, channel.categoryName].join(" ");
  return /\b(vs\.?| at |cup|league|racing|grand prix|football|baseball|basketball|soccer|hockey|golf|tennis|boxing|mma|ufc|f1|formula|mlb|nba|nfl|nhl|mls|wnba|premier|sports?)\b/i.test(" " + text + " ");
}
function programLooksMovie(program) {
  if (programIsGuidePlaceholder(program)) return false;
  return /\b(movie|film|premiere|cinema|starring)\b/i.test(programSearchText(program));
}
function programLooksEvent(program) {
  if (programIsGuidePlaceholder(program)) return false;
  return /\b(awards|parade|special|ceremony|debate|concert|festival|inauguration|election night)\b/i.test(String(program.title || ""));
}
function groupedUpcomingAirings(programs, query) {
  const groups = {};
  items(programs).filter(programIsUpcoming).forEach(function(program) {
    if (programIsGuidePlaceholder(program)) return;
    const searchKey = normalizeProgramTitle(program.title);
    if (!searchKey || (query && searchKey.indexOf(query) === -1 && lower(programSearchText(program)).indexOf(query) === -1)) return;
    // The guide payload has airing IDs, not a trustworthy series ID. Keep
    // same-title programs on different channels separate instead
    // of pretending they are one authoritative series.
    const groupID = [searchKey, String(program.channelId || "")].join("|");
    groups[groupID] = groups[groupID] || { id: groupID, key: searchKey, title: program.title || "Untitled", programs: [] };
    groups[groupID].programs.push(program);
  });
  return Object.keys(groups).map(function(key) {
    const group = groups[key];
    group.programs.sort(function(left, right) { return (left.startUnix || 0) - (right.startUnix || 0); });
    return group;
  }).sort(function(left, right) {
    return (left.programs[0].startUnix || 0) - (right.programs[0].startUnix || 0);
  });
}
function searchFilters() {
  return [
    { id: "all", label: "All" },
    { id: "channels", label: "Channels" },
    { id: "groups", label: "Groups" },
    { id: "programs", label: "Programs" },
    { id: "airings", label: "Upcoming Airings" },
    { id: "sports", label: "Sports" },
    { id: "events", label: "Events" },
    { id: "movies", label: "Movies" },
    { id: "recordings", label: "Recordings" }
  ];
}
function firstSearchMatches(collection, predicate, limit) {
  const matches = [];
  const source = items(collection);
  for (let index = 0; index < source.length && matches.length < limit; index += 1) {
    if (predicate(source[index])) matches.push(source[index]);
  }
  return matches;
}
function searchMatchScore(primary, secondary, query) {
  const needle = lower(query).trim();
  const title = lower(primary).replace(/\s+/g, " ").trim();
  const details = lower(secondary);
  if (!needle) return 0;
  if (title === needle) return 1000;
  if (title.indexOf(needle) === 0) return 800;
  if (title.indexOf(needle) !== -1) return 600;
  if (details.indexOf(needle) !== -1) return 300;
  return 0;
}
function rankedSearchMatches(collection, scorer, limit) {
  return items(collection).map(function(item, index) {
    return { item: item, index: index, score: scorer(item) };
  }).filter(function(result) {
    return result.score > 0;
  }).sort(function(left, right) {
    return right.score - left.score || left.index - right.index;
  }).slice(0, limit).map(function(result) {
    return result.item;
  });
}
function searchResultSections(query) {
  const filter = state.searchType || "all";
  const include = function(id) { return filter === "all" || filter === id; };
  const sections = [];
  let programCatalog = null;
  const programs = function() {
    if (!programCatalog) programCatalog = programsFor("");
    return programCatalog;
  };
  if (include("channels")) {
    const channels = rankedSearchMatches(searchableChannels(), function(channel) {
      return searchMatchScore(channel.name, [channel.number, channel.categoryName, sourceCategoryLabel(channel), sourceCategoryRawLabel(channel)].join(" "), query);
    }, 18);
    const nameCounts = {};
    channels.forEach(function(channel) { const key = lower(channel.name); nameCounts[key] = (nameCounts[key] || 0) + 1; });
    sections.push({ id: "channels", title: "Channels", rows: channels.map(function(channel) {
      const region = nameCounts[lower(channel.name)] > 1 ? channelRegionLabel(channel) : "";
      return {
        attrs: "data-search-channel=\"" + escapeHTML(channel.id) + "\"",
        favoriteChannelId: channel.id,
        art: logoHTML(channel),
        title: channel.name || "Untitled",
        badge: region,
        meta: channelProgramLine(channel),
        action: "Watch"
      };
    }) });
  }
  if (include("groups")) {
    const groups = rankedSearchMatches(allDiscoveryGroups(), function(group) {
      return searchMatchScore(group.name, [group.id, group.kind].join(" "), query);
    }, 18);
    sections.push({ id: "groups", title: "Groups", rows: groups.map(function(group) {
      return {
        attrs: "data-search-category=\"" + escapeHTML(group.id) + "\"",
        art: "<span class=\"logo logo-fallback\">GRP</span>",
        title: group.name || "Untitled group",
        meta: [(group.kind === "custom" ? "My Group" : "Group"), group.count ? group.count + " channels" : ""].filter(Boolean).join(" - "),
        action: "Open"
      };
    }) });
  }
  if (include("programs")) {
    const matchingPrograms = rankedSearchMatches(programs().filter(function(program) {
      return !programIsGuidePlaceholder(program) && (!state.searchAiringChannel || String(program.channelId || "") === state.searchAiringChannel);
    }), function(program) {
      const channel = channelByID(program.channelId) || {};
      return searchMatchScore(program.title, [program.summary, program.description, channel.name, channel.categoryName].join(" "), query);
    }, 18);
    sections.push({ id: "programs", title: "Guide Programs", rows: matchingPrograms.map(function(program) {
      const channel = channelByID(program.channelId) || {};
      return {
        attrs: "data-search-program-channel=\"" + escapeHTML(program.channelId || "") + "\" data-search-program=\"" + escapeHTML(program.id || "") + "\"",
        art: logoHTML(channel),
        title: program.title || "Untitled program",
        meta: [(programIsLive(program) ? "Live now" : dateTimeLabel(program.startUnix)), channel.name || "Live TV"].filter(Boolean).join(" - "),
        action: "Details",
        recordable: recordingSchedulingEnabled() && programIsUpcoming(program),
        channelId: program.channelId,
        programId: program.id || ""
      };
    }) });
  }
  if (include("airings")) {
    const airings = groupedUpcomingAirings(programs(), query).slice(0, 12);
    sections.push({ id: "airings", title: "Upcoming Airings", rows: airings.map(function(group) {
      const first = group.programs[0] || {};
      const channel = channelByID(first.channelId) || {};
      return {
        attrs: "data-search-airing=\"" + escapeHTML(group.key) + "\" data-search-airing-channel=\"" + escapeHTML(first.channelId || "") + "\"",
        art: logoHTML(channel),
        title: group.title,
        meta: [group.programs.length + " airings", dateTimeLabel(first.startUnix), channel.name || ""].filter(Boolean).join(" - "),
        action: "Show"
      };
    }) });
  }
  if (include("sports")) {
    const people = rankedSearchMatches(myTVSportsPeople(), function(team) {
      return searchMatchScore(team.name, [team.abbreviation, team.kind, team.leagueName].join(" "), query);
    }, 12);
    sections.push({ id: "sports-people", title: "Teams & Fighters", rows: people.map(function(team) {
      const followed = !!sportsFavoriteTeamMap()[team.id];
      return {
        attrs: "data-sports-favorite-team=\"" + escapeHTML(team.id || "") + "\"" + sportsTeamLabelAttrs(team) + " data-sports-favorite-enabled=\"" + (followed ? "false" : "true") + "\"",
        art: renderSportsTeamLogo(team, "logo"),
        title: team.name || "Team",
        meta: [team.kind || "Team", team.leagueName || ""].filter(Boolean).join(" - "),
        action: followed ? "Following" : "Follow"
      };
    }) });
    const leagues = rankedSearchMatches(items(state.sports && state.sports.leagues), function(league) {
      return searchMatchScore(league.name, [league.sportName, league.id].join(" "), query);
    }, 8);
    sections.push({ id: "sports-leagues", title: "Leagues", rows: leagues.map(function(league) {
      const followed = !!sportsFavoriteLeagueMap()[league.id];
      return {
        attrs: "data-sports-favorite-league=\"" + escapeHTML(league.id || "") + "\" data-sports-favorite-enabled=\"" + (followed ? "false" : "true") + "\"",
        art: renderSportsLeagueMark(league),
        title: league.name || "League",
        meta: [league.sportName || "Sports", "League"].join(" - "),
        action: followed ? "Following" : "Follow"
      };
    }) });
    const matchingPrograms = rankedSearchMatches(programs().filter(programLooksSports), function(program) {
      const channel = channelByID(program.channelId) || {};
      return searchMatchScore(program.title, [channel.name, channel.categoryName].join(" "), query);
    }, 12);
    sections.push({ id: "sports", title: "Sports From Guide", rows: matchingPrograms.map(function(program) {
      const channel = channelByID(program.channelId) || {};
      return {
        attrs: "data-search-program-channel=\"" + escapeHTML(program.channelId || "") + "\" data-search-program=\"" + escapeHTML(program.id || "") + "\"",
        art: logoHTML(channel),
        title: program.title || "Sports",
        meta: [(programIsLive(program) ? "Live now" : dateTimeLabel(program.startUnix)), channel.name || ""].filter(Boolean).join(" - "),
        action: "Details"
      };
    }) });
  }
  if (include("events")) {
    const trackedEvents = rankedSearchMatches(items(state.events && state.events.events), function(event) {
      return searchMatchScore(event.name || event.shortName, [event.categoryName, event.description, event.keyword].join(" "), query);
    }, 10);
    sections.push({ id: "tracked-events", title: "Events", rows: trackedEvents.map(function(event) {
      const followed = !!(featuredEventMap()[event.id] || adminFeaturedEventMap()[event.id]);
      return {
        attrs: "data-event-feature=\"" + escapeHTML(event.id || "") + "\"",
        disabled: !!adminFeaturedEventMap()[event.id],
        art: "<span class=\"logo logo-fallback\">EVT</span>",
        title: event.shortName || event.name || "Event",
        meta: [event.categoryName || "Event", eventStatusLabel(event)].filter(Boolean).join(" - "),
        action: followed ? "Following" : "Follow"
      };
    }) });
    const matchingPrograms = rankedSearchMatches(programs().filter(programLooksEvent), function(program) {
      const channel = channelByID(program.channelId) || {};
      return searchMatchScore(program.title, [channel.name, channel.categoryName].join(" "), query);
    }, 12);
    sections.push({ id: "events", title: "Events From Guide", rows: matchingPrograms.map(function(program) {
      const channel = channelByID(program.channelId) || {};
      return {
        attrs: "data-search-program-channel=\"" + escapeHTML(program.channelId || "") + "\" data-search-program=\"" + escapeHTML(program.id || "") + "\"",
        art: logoHTML(channel),
        title: program.title || "Event",
        meta: [(programIsLive(program) ? "Live now" : dateTimeLabel(program.startUnix)), channel.name || ""].filter(Boolean).join(" - "),
        action: "Details"
      };
    }) });
  }
  if (include("movies")) {
    const guideMovies = firstSearchMatches(programs(), function(program) { return programLooksMovie(program) && programMatchesSearch(program, query); }, 12);
    sections.push({ id: "movies", title: "Movies From Guide", rows: guideMovies.map(function(program) {
      const channel = channelByID(program.channelId) || {};
      return {
        attrs: "data-search-program-channel=\"" + escapeHTML(program.channelId || "") + "\" data-search-program=\"" + escapeHTML(program.id || "") + "\"",
        art: logoHTML(channel),
        title: program.title || "Movie",
        meta: ["Guide", dateTimeLabel(program.startUnix), channel.name || ""].filter(Boolean).join(" - "),
        action: "Details"
      };
    }) });
  }
  if (include("recordings") && state.recordings && state.recordings.available) {
    const recordings = firstSearchMatches(normalizeRecordings(state.recordings), function(recording) { return recordingMatchesSearch(recording, query); }, 12);
    sections.push({ id: "recordings", title: "Recordings", rows: recordings.map(function(recording) {
      const playbackURL = recordingPlaybackURL(recording);
      return {
        attrs: playbackURL ? "data-recording-playback=\"" + escapeHTML(playbackURL) + "\"" : "",
        disabled: !playbackURL,
        art: "<span class=\"logo logo-fallback\">REC</span>",
        title: recordingTitle(recording),
        meta: [recordingChannelName(recording), recordingWindow(recording), recordingStatus(recording)].filter(Boolean).join(" - "),
        action: playbackURL ? "Play" : "Saved"
      };
    }) });
  }
  return sections.filter(function(section) { return section.rows.length; });
}
function channelFavoriteButton(channelID, name) {
  const saved = !!favoriteMap()[channelID];
  const label = (saved ? "Remove " : "Save ") + (name || "channel") + (saved ? " from My TV favorites" : " to My TV favorites");
  return '<button class="search-channel-favorite' + (saved ? ' active' : '') + '" type="button" data-save-channel="' + escapeHTML(channelID) + '" aria-label="' + escapeHTML(label) + '" title="' + escapeHTML(label) + '" aria-pressed="' + saved + '">' + icon(saved ? "heart-solid" : "heart") + '</button>';
}
function channelRegionLabel(channel) {
  const parts = String((channel && channel.categoryName) || "").split("|").map(function(part) { return part.trim(); }).filter(Boolean);
  return (parts.length > 1 ? parts.slice(0, -1) : parts).join(" · ");
}
function renderSearchResultRow(row) {
  const record = row.recordable ? "<button class=\"search-result-record\" type=\"button\" data-schedule-channel=\"" + escapeHTML(row.channelId || "") + "\" data-schedule-program=\"" + escapeHTML(row.programId || "") + "\">Record</button>" : "";
  const badge = row.badge ? " <span class=\"search-result-badge\">" + escapeHTML(row.badge) + "</span>" : "";
  return "<div class=\"search-result-row\"><button class=\"search-result\" type=\"button\" " + (row.attrs || "") + (row.disabled ? " disabled" : "") + "><span class=\"search-result-art\">" + row.art + "</span><span class=\"search-result-main\"><strong>" + escapeHTML(row.title) + badge + "</strong><small>" + escapeHTML(row.meta || "") + "</small></span><span class=\"search-result-action\">" + escapeHTML(row.action || "") + "</span></button>" + record + (row.favoriteChannelId ? channelFavoriteButton(row.favoriteChannelId, row.title) : "") + "</div>";
}
function renderSearchResultCard(row) {
  const record = row.recordable ? "<button class=\"search-result-record\" type=\"button\" data-schedule-channel=\"" + escapeHTML(row.channelId || "") + "\" data-schedule-program=\"" + escapeHTML(row.programId || "") + "\">Record</button>" : "";
  return "<div class=\"search-result-card\"><button class=\"search-result-card-main\" type=\"button\" " + (row.attrs || "") + (row.disabled ? " disabled" : "") + "><span class=\"search-result-card-art\">" + row.art + "</span><span class=\"search-result-card-copy\"><strong>" + escapeHTML(row.title) + "</strong><small>" + escapeHTML(row.meta || "") + "</small></span><span class=\"search-result-card-action\">" + escapeHTML(row.action || "") + "</span></button>" + record + "</div>";
}
function renderSearchResults(query) {
  const sections = searchResultSections(query);
  const savePass = query && !keywordPasses().some(function(pass) { return lower(pass.keyword) === lower(query); }) ? "<button class=\"search-save-pass\" type=\"button\" data-keyword-pass-add=\"" + escapeHTML(query) + "\">Follow search</button>" : "";
  if (!sections.length) return "<div class=\"search-empty\"><span>" + icon("search") + "</span><strong>No current matches for &ldquo;" + escapeHTML(query) + "&rdquo;</strong><p>Add it to My TV and we’ll watch future guide listings.</p>" + savePass + "</div>";
  return "<div class=\"search-results\">" + sections.map(function(section) {
    return "<section class=\"search-result-section\"><header class=\"search-result-section-head\"><h3>" + escapeHTML(section.title) + "</h3><span>" + section.rows.length + (section.rows.length === 1 ? " result" : " results") + "</span></header><div class=\"search-result-list\">" + section.rows.map(renderSearchResultRow).join("") + "</div></section>";
  }).join("") + "</div>" + (savePass ? "<div class=\"search-pass-action\"><span class=\"search-pass-copy\"><strong>Follow &ldquo;" + escapeHTML(query) + "&rdquo; in My TV</strong><small>See matching guide listings as they become available.</small></span>" + savePass + "</div>" : "");
}
const SEARCH_RESULTS_DELAY_MS = 180;
const SEARCH_MIN_QUERY_LENGTH = 2;
let searchResultsTimer = null;
function clearSearchResultsTimer() {
  if (!searchResultsTimer) return;
  clearTimeout(searchResultsTimer);
  searchResultsTimer = null;
}
function renderSearchPageResults() {
  const query = searchNeedle();
  if (!query) return renderSearchStart();
  if (query.length < SEARCH_MIN_QUERY_LENGTH) return "<div class=\"search-start-empty search-query-hint\"><span>" + icon("search") + "</span><strong>Keep typing</strong><p>Enter at least two characters to search the full lineup.</p></div>";
  return renderSearchResults(query);
}
function updateSearchPageResults() {
  const root = byId("search-page-results");
  if (!root) {
    renderSearchPage();
    return;
  }
  root.innerHTML = renderSearchPageResults();
}
function scheduleSearchResultsUpdate() {
  clearSearchResultsTimer();
  searchResultsTimer = setTimeout(function() {
    searchResultsTimer = null;
    if (state.view === "search") updateSearchPageResults();
  }, SEARCH_RESULTS_DELAY_MS);
}
function updateLiveSearchFilter() {
  if (refreshGuideRowsForQuery()) return;
  render();
}
function renderSearchStart() {
  const recent = items(state.recentSearches);
  const recentHTML = recent.length ? sectionHeaderWithActions("Recent searches", "<button class=\"search-clear\" type=\"button\" data-search-clear=\"true\">Clear All</button>") + "<div class=\"search-chip-row\">" + recent.map(function(value) {
    return "<button class=\"search-chip\" type=\"button\" data-search-recent=\"" + escapeHTML(value) + "\">" + escapeHTML(value) + "</button>";
  }).join("") + "</div>" : "";
  const passes = keywordPasses();
  const passHTML = passes.length ? sectionHeader("Keyword Passes") + "<div class=\"search-chip-row\">" + passes.map(function(pass) {
    return "<button class=\"search-chip\" type=\"button\" data-search-recent=\"" + escapeHTML(pass.keyword) + "\">" + escapeHTML(pass.keyword) + "</button>";
  }).join("") + "</div>" : "";
  const categoryHTML = sectionHeader("Categories") + "<div class=\"search-category-grid\">" + [
    { id: "channels", label: "Channels", icon: "guide" },
    { id: "groups", label: "Groups", icon: "multiview" },
    { id: "programs", label: "Programs", icon: "search" },
    { id: "airings", label: "Upcoming Airings", icon: "guide" },
    { id: "sports", label: "Sports", icon: "search" },
    { id: "events", label: "Events", icon: "guide" },
    { id: "movies", label: "Movies", icon: "play" }
  ].map(function(item) {
    return "<button class=\"search-category-tile\" type=\"button\" data-search-type=\"" + escapeHTML(item.id) + "\">" + icon(item.icon) + "<strong>" + escapeHTML(item.label) + "</strong></button>";
  }).join("") + "</div>";
  const browsed = recentChannels(10);
  const browsedHTML = browsed.length ? sectionHeader("Recently browsed") + rowCards(browsed) : "";
  return recentHTML + passHTML + categoryHTML + browsedHTML;
}
function renderSearchPage() {
  const root = byId("view");
  const main = document.querySelector(".main");
  if (main) main.scrollLeft = 0;
  if (root) root.scrollLeft = 0;
  const query = state.searchQuery || "";
  const filter = state.searchType || "all";
  const filterHTML = "<div class=\"search-scope-row\" aria-label=\"Search scope\">" + searchFilters().map(function(item) {
    return "<button class=\"search-chip" + (filter === item.id ? " active" : "") + "\" type=\"button\" data-search-type=\"" + escapeHTML(item.id) + "\">" + escapeHTML(item.label) + "</button>";
  }).join("") + "</div>";
  const clear = query ? "<button class=\"search-query-clear\" type=\"button\" data-search-query-clear=\"true\" aria-label=\"Clear search\">" + icon("x") + "</button>" : "";
  root.innerHTML = "<div class=\"search-page\"><header class=\"search-commandbar\"><div class=\"search-commandbar-title\"><h2>Search</h2><span>Channels, shows, teams, fighters, leagues, and events</span></div><div class=\"search-form\"><label class=\"search-input-shell\"><span>" + icon("search") + "</span><input id=\"search-page-input\" class=\"search-field\" value=\"" + escapeHTML(query) + "\" placeholder=\"Search everything on TV\" autocomplete=\"off\" spellcheck=\"false\">" + clear + "</label><button class=\"search-cancel\" type=\"button\" data-search-cancel=\"true\">Done</button></div>" + filterHTML + "</header><div id=\"search-page-results\" class=\"search-page-results\" aria-live=\"polite\">" + renderSearchPageResults() + "</div></div>";
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
function addKeywordPass(keyword) {
  keyword = String(keyword || "").trim();
  if (!keyword || !state.app || !state.app.preferences) return;
  state.app.preferences.keywordPasses = normalizeKeywordPasses(keywordPasses().concat([{ keyword: keyword, createdAt: Date.now() }]));
  savePrefs();
  showAppToast("Added to My TV.");
  if (state.view === "mytv") updateMyTVSearchSurface();
  else renderSearchPage();
}
function removeKeywordPass(id) {
  id = String(id || "");
  if (!state.app || !state.app.preferences) return;
  state.app.preferences.keywordPasses = keywordPasses().filter(function(pass) { return pass.id !== id; });
  savePrefs();
  if (state.view === "mytv") updateMyTVSearchSurface();
  else renderOnLaterPage();
}
function onLaterFilters() {
  return [
    { id: "all", label: "All" },
    { id: "live", label: "Live Now" },
    { id: "today", label: "Today" },
    { id: "sports", label: "Sports" },
    { id: "events", label: "Events" },
    { id: "movies", label: "Movies" },
    { id: "passes", label: "Passes" }
  ];
}
function onLaterPrograms() {
  const now = Math.floor(Date.now() / 1000);
  const endOfToday = new Date();
  endOfToday.setHours(23, 59, 59, 999);
  const todayEnd = Math.floor(endOfToday.getTime() / 1000);
  const time = state.onLaterTime || "all";
  const type = state.onLaterType || "all";
  return programsFor("").filter(function(program) {
    if (programIsGuidePlaceholder(program)) return false;
    if ((program.endUnix || 0) < now) return false;
    if (time === "live" && !programIsLive(program)) return false;
    if (time === "today" && ((program.startUnix || 0) < now - 3600 || (program.startUnix || 0) > todayEnd)) return false;
    if (type === "sports" && !programLooksSports(program)) return false;
    if (type === "events" && !programLooksEvent(program)) return false;
    if (type === "movies" && !programLooksMovie(program)) return false;
    if (type === "passes" && !keywordPasses().some(function(pass) { return lower(programSearchText(program)).indexOf(lower(pass.keyword)) !== -1; })) return false;
    return true;
  }).sort(function(left, right) {
    return (left.startUnix || 0) - (right.startUnix || 0);
  });
}
function renderProgramDiscoveryRow(program) {
  const channel = channelByID(program.channelId) || {};
  return renderSearchResultRow({
    attrs: "data-search-program-channel=\"" + escapeHTML(program.channelId || "") + "\" data-search-program=\"" + escapeHTML(program.id || "") + "\"",
    art: logoHTML(channel),
    title: program.title || "Untitled program",
    meta: [(programIsLive(program) ? "Live now" : dateTimeLabel(program.startUnix)), channel.name || ""].filter(Boolean).join(" - "),
    action: "Details",
    recordable: recordingSchedulingEnabled() && programIsUpcoming(program),
    channelId: program.channelId,
    programId: program.id || ""
  });
}
function renderProgramDiscoveryCard(program) {
  const channel = channelByID(program.channelId) || {};
  const artwork = safeSportsMediaURL(program && program.imageUrl);
  const fallback = logoHTML(channel);
  const media = artwork ? "<span class=\"on-later-card-media has-art\"><img src=\"" + escapeHTML(artwork) + "\" alt=\"\" data-img-error=\"card-art\"><span class=\"on-later-card-fallback\" hidden>" + fallback + "</span></span>" : "<span class=\"on-later-card-media\"><span class=\"on-later-card-fallback\">" + fallback + "</span></span>";
  const label = programIsLive(program) ? "Live now" : dateTimeLabel(program.startUnix);
  const categories = items(program && program.categories);
  const kind = programLooksSports(program) ? "Sports" : (programLooksMovie(program) ? "Movie" : (programLooksEvent(program) ? "Event" : (categories[0] || "TV")));
  const record = recordingSchedulingEnabled() && programIsUpcoming(program) ? "<button class=\"on-later-card-record\" type=\"button\" data-schedule-channel=\"" + escapeHTML(program.channelId || "") + "\" data-schedule-program=\"" + escapeHTML(program.id || "") + "\">" + icon("record") + "<span>Record</span></button>" : "";
  return "<article class=\"on-later-program-card" + (artwork ? " has-art" : " no-art") + "\"><button class=\"on-later-program-open\" type=\"button\" data-search-program-channel=\"" + escapeHTML(program.channelId || "") + "\" data-search-program=\"" + escapeHTML(program.id || "") + "\">" + media
    + "<span class=\"on-later-card-copy\"><span class=\"on-later-card-kicker\">" + escapeHTML(kind) + "</span><strong>" + escapeHTML(program.title || "Untitled program") + "</strong><small>" + escapeHTML([label, channel.name || ""].filter(Boolean).join(" · ")) + "</small></span></button>"
    + "<footer><button type=\"button\" data-search-program-channel=\"" + escapeHTML(program.channelId || "") + "\" data-search-program=\"" + escapeHTML(program.id || "") + "\">Details</button>" + record + "</footer></article>";
}
function onLaterProgramIdentity(program) {
  return String(program && program.id || "") || [program && program.title, program && program.channelId, program && program.startUnix, program && program.endUnix].join("|");
}
function distinctOnLaterPrograms(programs) {
  const seen = {};
  return items(programs).filter(function(program) {
    const key = onLaterProgramIdentity(program);
    if (!key || seen[key]) return false;
    seen[key] = true;
    return true;
  });
}
function onLaterShelfKey(title) {
  return lower(title).replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "") || "guide";
}
function onLaterShelfHTML(title, programs, renderer) {
  const all = distinctOnLaterPrograms(programs);
  if (!all.length) return "";
  const key = onLaterShelfKey(title);
  const limit = Math.max(18, Number(state.onLaterShelfLimits[key] || 18));
  const visible = all.slice(0, limit);
  const remaining = all.length - visible.length;
  const more = remaining > 0 ? "<button class=\"on-later-more\" type=\"button\" data-onlater-more=\"" + escapeHTML(key) + "\"><strong>Show more</strong><span>" + escapeHTML(String(Math.min(18, remaining))) + " of " + escapeHTML(String(remaining)) + " remaining</span></button>" : "";
  const renderCard = renderer || renderProgramDiscoveryCard;
  return "<section class=\"on-later-shelf\" data-onlater-shelf=\"" + escapeHTML(key) + "\"><header><h3>" + escapeHTML(title) + "</h3><span>" + escapeHTML(String(all.length)) + "</span></header><div class=\"on-later-shelf-rail\">" + visible.map(renderCard).join("") + more + "</div></section>";
}
function restoreOnLaterShelfPosition(key, previousLimit, previousScrollLeft) {
  const root = byId("view");
  if (!root || typeof root.querySelector !== "function") return false;
  const shelf = root.querySelector('[data-onlater-shelf="' + String(key || "guide").replace(/[^a-z0-9-]/gi, "") + '"]');
  const rail = shelf && shelf.querySelector(".on-later-shelf-rail");
  if (!rail) return false;
  const cards = typeof rail.querySelectorAll === "function" ? rail.querySelectorAll(".on-later-program-card") : [];
  const firstNewCard = cards && cards[previousLimit];
  rail.scrollLeft = Math.max(Number(previousScrollLeft || 0), firstNewCard ? Math.max(0, Number(firstNewCard.offsetLeft || 0) - 16) : 0);
  const focusTarget = firstNewCard && typeof firstNewCard.querySelector === "function" ? firstNewCard.querySelector("button") : null;
  if (focusTarget && typeof focusTarget.focus === "function") {
    try { focusTarget.focus({ preventScroll: true }); } catch (_) { focusTarget.focus(); }
  }
  return !!firstNewCard;
}
function renderOnLaterAiringCard(group) {
  const first = items(group && group.programs)[0] || {};
  const channel = channelByID(first.channelId) || {};
  const artwork = safeSportsMediaURL(first.imageUrl);
  const fallback = logoHTML(channel);
  const media = artwork ? "<span class=\"on-later-card-media has-art\"><img src=\"" + escapeHTML(artwork) + "\" alt=\"\" data-img-error=\"card-art\"><span class=\"on-later-card-fallback\" hidden>" + fallback + "</span></span>" : "<span class=\"on-later-card-media\"><span class=\"on-later-card-fallback\">" + fallback + "</span></span>";
  return "<article class=\"on-later-program-card" + (artwork ? " has-art" : " no-art") + "\"><button class=\"on-later-program-open\" type=\"button\" data-search-airing=\"" + escapeHTML(group.key || group.title || "") + "\" data-search-airing-channel=\"" + escapeHTML(first.channelId || "") + "\">" + media
    + "<span class=\"on-later-card-copy\"><span class=\"on-later-card-kicker\">" + (items(group.programs).length > 1 ? "More Airings" : "Upcoming Airing") + "</span><strong>" + escapeHTML(group.title || first.title || "Untitled program") + "</strong><small>" + escapeHTML([items(group.programs).length + (items(group.programs).length === 1 ? " airing" : " airings"), dateTimeLabel(first.startUnix), channel.name || ""].filter(Boolean).join(" · ")) + "</small></span></button>"
    + "<footer><button type=\"button\" data-search-airing=\"" + escapeHTML(group.key || group.title || "") + "\" data-search-airing-channel=\"" + escapeHTML(first.channelId || "") + "\">Show airings</button></footer></article>";
}
function onLaterShelves(programs, time, type) {
  const now = new Date();
  const nowUnix = Math.floor(now.getTime() / 1000);
  const todayEndDate = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1);
  const tomorrowEndDate = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 2);
  const weekEndDate = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 8);
  const todayEnd = Math.floor(todayEndDate.getTime() / 1000);
  const tomorrowEnd = Math.floor(tomorrowEndDate.getTime() / 1000);
  const weekEnd = Math.floor(weekEndDate.getTime() / 1000);
  if (time !== "all" || type !== "all") {
    const filter = onLaterFilters().find(function(item) { return item.id === (type !== "all" ? type : time); });
    return [{ title: filter ? filter.label : "Guide Picks", programs: programs }];
  }
  return [
    { title: "Live Now", programs: programs.filter(programIsLive) },
    { title: "On Today", programs: programs.filter(function(program) { return !programIsLive(program) && Number(program.startUnix || 0) >= nowUnix && Number(program.startUnix || 0) < todayEnd; }) },
    { title: "On Tomorrow", programs: programs.filter(function(program) { const start = Number(program.startUnix || 0); return start >= todayEnd && start < tomorrowEnd; }) },
    { title: "On This Week", programs: programs.filter(function(program) { const start = Number(program.startUnix || 0); return start >= tomorrowEnd && start < weekEnd; }) },
    { title: "Upcoming Movies", programs: programs.filter(programLooksMovie) },
    { title: "Sports", programs: programs.filter(programLooksSports) },
    { title: "Events & Specials", programs: programs.filter(programLooksEvent) },
    { title: "All Guide Picks", programs: programs }
  ];
}
function renderOnLaterPage() {
  const root = byId("view");
  const time = state.onLaterTime || "all";
  const type = state.onLaterType || "all";
  const filterButton = function(item, group, label) {
    const active = (group === "time" ? time : type) === item.id;
    return "<button class=\"search-chip" + (active ? " active" : "") + "\" type=\"button\" data-onlater-" + group + "=\"" + escapeHTML(item.id) + "\" aria-pressed=\"" + (active ? "true" : "false") + "\">" + escapeHTML(label || item.label) + "</button>";
  };
  const byID = function(id) { return onLaterFilters().find(function(item) { return item.id === id; }); };
  const filters = '<div class="filter-sections"><div class="on-later-filter-group filter-section" data-on-later-filter-group="time"><span class="filter-section-label">Time</span><div class="search-chip-row">' + ["all", "live", "today"].map(function(id) { return filterButton(byID(id), "time"); }).join("") + '</div></div><div class="on-later-filter-group filter-section" data-on-later-filter-group="type"><span class="filter-section-label">Type</span><div class="search-chip-row">' + ["all", "sports", "events", "movies", "passes"].map(function(id) { return filterButton(byID(id), "type", id === "all" ? "All Types" : ""); }).join("") + "</div></div></div>";
  const programs = onLaterPrograms();
  const repeatAirings = groupedUpcomingAirings(programs, "");
  const passes = keywordPasses();
  const passHTML = passes.length ? sectionHeader("Keyword Passes") + "<div class=\"keyword-pass-list\">" + passes.map(function(pass) {
    return "<div class=\"keyword-pass\"><button type=\"button\" data-search-recent=\"" + escapeHTML(pass.keyword) + "\"><strong>" + escapeHTML(pass.keyword) + "</strong><small>" + escapeHTML(String(onLaterPrograms().filter(function(program) { return lower(programSearchText(program)).indexOf(lower(pass.keyword)) !== -1; }).length)) + " matches</small></button><button type=\"button\" data-keyword-pass-remove=\"" + escapeHTML(pass.id) + "\">Remove</button></div>";
  }).join("") + "</div>" : "";
  root.innerHTML = "<div class=\"search-page on-later-page\"><div class=\"search-hero\"><h2>On Later</h2><p>Upcoming guide content organized for watching and recording.</p></div>" + filters
    + (passHTML && (type === "all" || type === "passes") ? passHTML : "")
    + (programs.length ? "<div class=\"on-later-shelves\">" + (repeatAirings.length && time !== "live" ? onLaterShelfHTML("Upcoming Airings", repeatAirings, renderOnLaterAiringCard) : "") + onLaterShelves(programs, time, type).map(function(shelf) { return onLaterShelfHTML(shelf.title, shelf.programs); }).join("") + "</div>" : "<div class=\"empty\">No matching guide entries.</div>")
    + "</div>";
}
function myTVGuidePrograms() {
  const passes = keywordPasses();
  const now = Math.floor(Date.now() / 1000);
  if (!passes.length) return [];
  return distinctOnLaterPrograms(programsFor("").filter(function(program) {
    if (programIsGuidePlaceholder(program) || Number(program.endUnix || 0) < now) return false;
    const text = lower(programSearchText(program));
    return passes.some(function(pass) { return text.indexOf(lower(pass.keyword)) !== -1; });
  })).sort(function(left, right) { return Number(left.startUnix || 0) - Number(right.startUnix || 0); });
}
function loadEvents(force) {
  if (state.eventsLoading) return Promise.resolve(state.events || { events: [], categories: [] });
  if (state.events && !force) return Promise.resolve(state.events);
  state.eventsLoading = true;
  return getJSON("/dispatcharr/api/events" + (force && siloUserIsAdmin() ? "?refresh=1" : "")).then(function(payload) {
    state.events = payload || { events: [], categories: [] };
    delete state.events.error;
    return state.events;
  }).catch(function(error) {
    if (!state.events) state.events = { events: [], categories: [], error: readableError(error) };
    else state.events.error = readableError(error);
    showAppToast("Could not refresh events.");
    return state.events;
  }).finally(function() {
    state.eventsLoading = false;
    renderRail();
    if (state.view === "events") renderEventsPage();
    if (state.view === "mytv") updateMyTVSearchSurface();
    if (state.view === "search") updateSearchPageResults();
  });
}
function eventTabLabel(tab) {
  return ({ featured: "Featured", live: "Live", upcoming: "Upcoming", all: "All" })[tab] || "Live";
}
function eventTabButtonsHTML() {
  return ["featured", "upcoming", "live", "all"].map(function(tab) {
    return "<button type=\"button\" data-event-tab=\"" + tab + "\" class=\"" + (state.eventsTab === tab ? "active" : "") + "\" aria-pressed=\"" + (state.eventsTab === tab ? "true" : "false") + "\">" + escapeHTML(eventTabLabel(tab)) + "</button>";
  }).join("");
}
function renderEventTabFilters() {
  return "<div class=\"sports-filter-row\"><div class=\"view-toggle\" aria-label=\"Events filter\">" + eventTabButtonsHTML() + "</div></div>";
}
function renderEventsPage() {
  const root = byId("view");
  const scrollTop = root ? root.scrollTop : 0;
  if (!state.events && !state.eventsLoading) loadEvents(false);
  renderSportsTopbarTabs();
  const payload = state.events || { events: [], categories: [] };
  const events = collapseRepeatAirings(filteredBroadcastEvents(payload));
  root.innerHTML = "<div class=\"sports-page\"><div class=\"sports-pinned\"><div class=\"event-filter-bar\">" + renderEventTabFilters() + renderEventCategoryFilters(payload) + "</div>"
    + recoveryPanelHTML(payload.error, "events")
    + (state.eventsLoading && !events.length ? "<div class=\"empty\">Loading events...</div>" : "")
    + "</div><div class=\"sports-score-scroll\">"
    + (events.length ? "<div class=\"event-shelves\">" + broadcastEventShelves(events).map(function(shelf) { return renderBroadcastEventShelf(shelf.title, shelf.events); }).join("") + "</div>" : (!state.eventsLoading ? emptyStateHTML("No events on your channels right now.", "Award shows, debates, parades, and other specials appear here when the guide lists them.") : ""))
    + "</div>"
    + "</div>";
  if (root) root.scrollTop = scrollTop;
}
function renderBroadcastEventShelf(title, events) {
  if (!events.length) return "";
  const countLabel = events.length + " event" + (events.length === 1 ? "" : "s");
  return "<section class=\"event-shelf\"><header><h3>" + escapeHTML(title) + "</h3><span>" + escapeHTML(countLabel) + "</span></header><div class=\"event-shelf-rail\">" + events.map(renderBroadcastEventCard).join("") + "</div></section>";
}
function broadcastEventShelves(events) {
  if (state.eventCategory) return [{ title: eventCategoryName(state.eventCategory) || "Events", events: events }];
  if (state.eventsTab === "featured") return [{ title: "Featured", events: events }];
  if (state.eventsTab === "live") return [{ title: "Live Now", events: events }];
  if (state.eventsTab === "upcoming") return [{ title: "Upcoming", events: events }];
  return [
    { title: "Live Now", events: events.filter(sportsEventIsLive) },
    { title: "Coming Up", events: events.filter(function(event) { return !sportsEventIsLive(event) && !event.completed; }) },
    { title: "Recently Ended", events: events.filter(function(event) { return !!event.completed; }) }
  ];
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
  const personal = featuredEventMap();
  const global = adminFeaturedEventMap();
  return items(payload && payload.events).filter(function(event) {
    if (!uniqueEventChannels(event.channels).length) return false;
    if (state.eventCategory && event.categoryId !== state.eventCategory) return false;
    const live = sportsEventIsLive(event);
    if (state.eventsTab === "featured" && !global[event.id] && !personal[event.id]) return false;
    if (state.eventsTab === "live" && !live) return false;
    const startUnix = Number(event.startUnix || 0);
    if (state.eventsTab === "upcoming" && (event.completed || live || (startUnix > 0 && startUnix < now - 3600))) return false;
    return true;
  }).sort(function(left, right) {
    const leftPriority = global[left.id] ? 4 : (personal[left.id] ? 3 : (sportsEventIsLive(left) ? 2 : 1));
    const rightPriority = global[right.id] ? 4 : (personal[right.id] ? 3 : (sportsEventIsLive(right) ? 2 : 1));
    if (leftPriority !== rightPriority) return rightPriority - leftPriority;
    return Number(left.startUnix || Number.MAX_SAFE_INTEGER) - Number(right.startUnix || Number.MAX_SAFE_INTEGER);
  });
}
function collapseRepeatAirings(events) {
  const byKey = {};
  const result = [];
  items(events).forEach(function(event) {
    const channel = uniqueEventChannels(event.channels)[0];
    const key = lower(event.name || event.shortName) + "|" + (channel ? channel.id : "") + "|" + (event.completed ? "ended" : "open");
    const first = byKey[key];
    if (first && !sportsEventIsLive(event)) {
      first.laterAirings.push(Number(event.startUnix || 0));
      return;
    }
    const copy = Object.assign({}, event, { laterAirings: [] });
    if (!first) byKey[key] = copy;
    result.push(copy);
  });
  return result;
}
function broadcastEventMatchesQuery(event) {
  const channels = items(event.channels).map(function(channel) { return [channel.name, channel.categoryName, channel.reason].join(" "); }).join(" ");
  const text = [event.name, event.shortName, event.categoryName, event.keyword, event.description, channels].join(" ");
  return lower(text).indexOf(lower(state.query)) !== -1;
}
function eventArtworkLoaded(image) {
  if (image && image.naturalWidth <= 2) eventArtworkFailed(image);
}
function eventArtworkFailed(image) {
  const media = image && image.parentElement;
  const card = media && media.closest ? media.closest(".event-card") : null;
  if (!media || !card) return;
  image.setAttribute("hidden", "true");
  const fallback = media.querySelector(".event-card-media-fallback");
  if (fallback) fallback.removeAttribute("hidden");
  media.classList.remove("has-art");
  media.classList.add("no-art");
  card.classList.add("no-art");
}
function eventCategoryMark(event) {
  const category = lower(event && (event.categoryId || event.categoryName));
  if (category.indexOf("award") !== -1) return icon("trophy");
  if (category.indexOf("parade") !== -1 || category.indexOf("civic") !== -1) return icon("calendar");
  return icon("calendar");
}
function renderBroadcastEventCard(event) {
  const status = eventStatusLabel(event);
  const title = cleanProviderText(event.shortName || event.name) || "Event";
  const artwork = safeSportsMediaURL(event.artworkUrl || event.imageUrl || event.posterUrl || event.thumbnailUrl || "");
  const cardClass = artwork ? 'class="event-card sports-card' : 'class="event-card no-art sports-card';
  const uniqueChannels = uniqueEventChannels(event.channels);
  const globallyFeatured = !!adminFeaturedEventMap()[event.id];
  const personallyFeatured = !!featuredEventMap()[event.id];
  const featured = globallyFeatured || personallyFeatured;
  const featureLabel = globallyFeatured ? "Featured by admin" : (personallyFeatured ? "Remove from featured" : "Feature event");
  const featureControl = "<button type=\"button\" class=\"event-feature-button" + (featured ? " active" : "") + "\" data-event-feature=\"" + escapeHTML(event.id || "") + "\"" + (globallyFeatured ? " disabled" : "") + " aria-label=\"" + escapeHTML(featureLabel) + "\" aria-pressed=\"" + (featured ? "true" : "false") + "\">" + icon(featured ? "heart-solid" : "heart") + "</button>";
  const windows = items(event.windows);
  const laterAirings = items(event.laterAirings).filter(function(startUnix) { return startUnix > 0; });
  const meta = [event.keyword || "", windows.length > 1 ? windows.length + " coverage windows" : "", uniqueChannels.length ? uniqueChannels.length + " channel" + (uniqueChannels.length === 1 ? "" : "s") : "", laterAirings.length ? pluralLabel(laterAirings.length, "more airing") : ""].filter(Boolean).map(function(value, index) { return "<span" + (index === 0 && event.keyword ? " class=\"event-keyword\"" : "") + ">" + escapeHTML(value) + "</span>"; }).join("");
  const laterHTML = laterAirings.length ? "<p class=\"event-later-airings\">Also airs " + escapeHTML(laterAirings.slice(0, 3).map(dateTimeLabel).join(" · ")) + (laterAirings.length > 3 ? " and more" : "") + "</p>" : "";
  const fallbackMedia = uniqueChannels[0] ? logoHTML(uniqueChannels[0]) : eventCategoryMark(event);
  const media = "<span class=\"event-card-media" + (artwork ? " has-art" : " no-art") + "\">"
    + (artwork ? "<img src=\"" + escapeHTML(artwork) + "\" alt=\"\" data-img-load=\"event-art\" data-img-error=\"event-art\">" : "")
    + "<span class=\"event-card-media-fallback\"" + (artwork ? " hidden" : "") + ">" + fallbackMedia + "</span></span>";
  const channelsOpen = uniqueChannels.length > 1 && !!state.expandedEvents[event.id];
  return "<article " + cardClass + (sportsEventIsLive(event) ? " live" : "") + (featured ? " featured" : "") + (channelsOpen ? " channels-expanded" : "") + '"><div class="event-card-visual">' + media + '<header class="event-card-head"><span class="event-card-category">' + escapeHTML(event.categoryName || "Events") + "</span><span class=\"event-card-status\">" + escapeHTML(status) + "</span>" + featureControl + "<strong class=\"event-card-title\" data-overflow-tooltip=\"" + escapeHTML(event.name || title) + "\">" + escapeHTML(title) + "</strong></header></div>"
    + "<div class=\"event-card-body" + (artwork ? "" : " no-art") + "\"><div class=\"event-details\">" + (event.description ? "<p data-overflow-description=\"true\">" + escapeHTML(event.description) + "</p>" : "") + "<div class=\"event-meta\">" + meta + "</div>" + laterHTML + renderEventBroadcastWindows(event) + "</div></div>"
    + renderBroadcastEventChannels(event)
    + "</article>";
}
function toggleFeaturedEvent(eventID) {
  eventID = String(eventID || "").trim();
  if (!eventID || adminFeaturedEventMap()[eventID]) return;
  const featured = Object.assign({}, featuredEventMap());
  if (featured[eventID]) delete featured[eventID];
  else featured[eventID] = true;
  state.app.preferences.featuredEvents = featured;
  savePrefs();
  if (state.view === "mytv") updateMyTVSearchSurface();
  else if (state.view === "search") updateSearchPageResults();
  else renderEventsPage();
}
function renderEventBroadcastWindows(event) {
  const windows = items(event && event.windows);
  if (windows.length < 2) return "";
  const visible = windows.slice(0, 4);
  const rows = visible.map(function(windowInfo, index) {
    const channels = uniqueEventChannels(windowInfo.channels);
    const end = windowInfo.endUnix ? " to " + timeLabel(windowInfo.endUnix) : "";
    return "<div class=\"event-broadcast-window\"><strong>" + escapeHTML(sportsDateLabel(windowInfo.startUnix)) + escapeHTML(end) + "</strong><small>" + escapeHTML(channels.length + " channel" + (channels.length === 1 ? "" : "s")) + "</small></div>";
  }).join("");
  const more = windows.length > visible.length ? "<div class=\"event-broadcast-more\">+" + (windows.length - visible.length) + " later</div>" : "";
  return "<div class=\"event-broadcast-windows\" aria-label=\"Broadcast coverage windows\">" + rows + more + "</div>";
}
function recoveryPanelHTML(error, kind) {
  if (!error) return "";
  const label = kind === "sports" ? "sports" : "events";
  return '<div class="recovery-panel"><span role="status" aria-live="polite">' + escapeHTML(error) + "</span><div class=\"recovery-actions\"><button type=\"button\" data-recovery-retry=\"" + label + "\" aria-label=\"Retry loading " + label + "\">Retry</button><button type=\"button\" data-recovery-reload=\"true\" aria-label=\"Reload Live TV\">Reload</button></div></div>";
}
function eventStatusLabel(event) {
  if (sportsEventIsLive(event)) return "Live";
  if (event.completed) return "Ended";
  if (event.startUnix) return sportsDateLabel(event.startUnix);
  return "Time TBD";
}
function eventChannelMenuID(eventID) {
  return "event-channels-" + String(eventID || "event").replace(/[^a-z0-9_-]+/gi, "-");
}
function renderBroadcastEventChannels(event) {
  const channels = uniqueEventChannels(event.channels);
  if (!channels.length) return "<div class=\"sports-channel-empty muted\">No matching channels.</div>";
  if (channels.length === 1) return "<div class=\"sports-channels event-card-channels\">" + renderSportsChannelChip(channels[0], "event-footer") + "</div>";
  const expanded = !!state.expandedEvents[event.id];
  const menuID = eventChannelMenuID(event.id);
  const title = cleanProviderText(event.shortName || event.name) || "Event";
  return "<div class=\"sports-channels event-card-channels event-channel-menu" + (expanded ? " expanded" : "") + "\">"
    + "<button type=\"button\" class=\"event-channel-menu-button\" data-event-expand=\"" + escapeHTML(event.id || "") + "\" aria-expanded=\"" + (expanded ? "true" : "false") + "\" aria-controls=\"" + escapeHTML(menuID) + "\" aria-label=\"" + escapeHTML((expanded ? "Hide" : "Show") + " channels for " + title) + "\"><span>Watch on " + channels.length + " channels</span>" + icon("chevron-down") + "</button>"
    + "<div class=\"event-channel-menu-list\" id=\"" + escapeHTML(menuID) + "\"" + (expanded ? "" : " hidden") + ">" + channels.map(function(channel) { return renderSportsChannelChip(channel, "event-footer"); }).join("") + "</div></div>";
}
function setEventTab(tab) {
  state.eventsTab = tab || "live";
  state.expandedEvents = {};
  renderEventsPage();
  commitAppRoute("push");
}
function setEventCategory(categoryID) {
  state.eventCategory = categoryID || "";
  state.expandedEvents = {};
  renderEventsPage();
  commitAppRoute("push");
}
function toggleBroadcastEventChannels(eventID) {
  eventID = String(eventID || "");
  if (!eventID) return;
  if (state.expandedEvents[eventID]) delete state.expandedEvents[eventID];
  else state.expandedEvents[eventID] = true;
  renderEventsPage();
}
function favoriteCards(channels, hasFavorites) {
  if (!channels.length) {
    const filtered = !!(hasFavorites && state.folderQuery);
    return "<div class=\"favorites-empty\"><div class=\"favorites-empty-icon\">" + icon(filtered ? "search" : "heart") + "</div><strong>" + (filtered ? "No matching favorites" : "No favorite channels yet") + "</strong><p>" + (filtered ? "Try a different channel or program name." : "Favorite a channel from Home or Guide and it will appear here.") + "</p>" + (filtered ? "" : "<button type=\"button\" class=\"favorites-empty-action\" data-favorites-action=\"guide\">" + icon("guide") + "<span>Open Guide</span></button>") + "</div>";
  }
  return "<div class=\"favorites-grid\">" + channels.map(function(channel, index) {
    const reorder = channels.length > 1 ? "<button type=\"button\" class=\"favorite-order-up\" data-favorite-move=\"up\" data-channel-id=\"" + escapeHTML(channel.id) + "\" aria-label=\"Move " + escapeHTML(channel.name || "channel") + " up\" title=\"Move up\"" + (index === 0 ? " disabled" : "") + ">" + icon("chevron-down") + "</button><button type=\"button\" data-favorite-move=\"down\" data-channel-id=\"" + escapeHTML(channel.id) + "\" aria-label=\"Move " + escapeHTML(channel.name || "channel") + " down\" title=\"Move down\"" + (index === channels.length - 1 ? " disabled" : "") + ">" + icon("chevron-down") + "</button>" : "";
    const controls = favoriteMap()[channel.id] ? "<div class=\"favorite-card-actions\" aria-label=\"Manage " + escapeHTML(channel.name || "channel") + "\"><button type=\"button\" class=\"favorite-remove\" data-favorite-remove=\"" + escapeHTML(channel.id) + "\" aria-label=\"Remove " + escapeHTML(channel.name || "channel") + " from favorites\" title=\"Remove from favorites\">" + icon("heart-solid") + "</button>" + reorder + "</div>" : "";
    return "<article class=\"favorite-card\"><button class=\"continue-card\" data-channel=\"" + escapeHTML(channel.id) + "\"><div class=\"poster-box\">" + (channel.logoUrl ? "<img src=\"" + escapeHTML(channel.logoUrl) + "\" alt=\"\">" : "<span>" + escapeHTML((channel.name || "TV").slice(0, 5)) + "</span>") + "</div><strong>" + escapeHTML(channel.name || "Untitled") + "</strong><div class=\"muted\">" + escapeHTML(channel.categoryName || "Live TV") + "</div></button>" + controls + "</article>";
  }).join("") + "</div>";
}
function favoritesPage(channels) {
  const filtered = channels.filter(channelMatchesFolderQuery).slice(0, 60);
  const count = channels.length;
  const countLabel = count + " " + (count === 1 ? "channel" : "channels");
  return "<section class=\"favorites-page\"><header class=\"favorites-header\"><h2>Favorite channels</h2><div class=\"favorites-count\">" + escapeHTML(countLabel) + "</div></header><label class=\"favorites-search\"><span>" + icon("search") + "</span><input id=\"folder-filter\" type=\"search\" placeholder=\"Filter favorites\" value=\"" + escapeHTML(state.folderQuery || "") + "\" autocomplete=\"off\" aria-label=\"Filter favorite channels\"></label>" + favoriteCards(filtered, count > 0) + "</section>";
}
function compareCategoryDisplayName(left, right) {
  const leftName = String((left && (left.name || left.id)) || "");
  const rightName = String((right && (right.name || right.id)) || "");
  return leftName.localeCompare(rightName, undefined, { numeric: true, sensitivity: "base" }) || leftName.localeCompare(rightName) || String((left && left.id) || "").localeCompare(String((right && right.id) || ""));
}
function categoryGrid(options) {
  options = options || {};
  const includeAllGuide = !!options.includeAllGuide;
  const hidden = hiddenMap();
  const sourceCategories = sourceCategoriesWithChannels(function(channel) {
    return !(channel.categoryId && hidden[channel.categoryId]);
  });
  const listing = adminListingCategories("");
  const featured = sourceCategories.filter(function(category) { return !!category.featured; }).sort(compareCategoryDisplayName);
  const featuredSourceIDs = {};
  featured.forEach(function(category) { featuredSourceIDs[category.sourceID] = true; });
  const regularListing = listing.filter(function(category) {
    return !(category.kind === "source" && featuredSourceIDs[category.sourceID]);
  });
  const sections = [];
  if (featured.length) sections.push(categoryGridSection(featuredGroupLabel(), featured));
  if (regularListing.length || (includeAllGuide && listing.length)) sections.push(categoryGridSection(adminListingTitle(), regularListing, includeAllGuide));
  if (!listing.length && (sourceCategories.length || includeAllGuide)) sections.push(categoryGridSection(adminListingTitle(), sourceCategories, includeAllGuide));
  return sections.length ? sections.join("") : "<div class=\"empty\">No groups yet.</div>";
}
function categoryGridSection(title, categories, includeAllGuide) {
  const allGuide = includeAllGuide ? allChannelsGuideTileHTML() : "";
  const grouped = categoryPrefixGroups(categories);
  if (!grouped.groups.length) return sectionHeader(title) + "<div class=\"category-grid\">" + allGuide + categories.map(function(category) { return categoryTileHTML(category); }).join("") + "</div>";
  const columns = Math.min(3, Math.max(grouped.groups.length, 1));
  const looseCount = (allGuide ? 1 : 0) + grouped.loose.length;
  const spanLead = columns > 1 && looseCount % columns === 1;
  const loose = allGuide + grouped.loose.map(function(category) { return categoryTileHTML(category); }).join("");
  return sectionHeader(title)
    + "<div class=\"category-board\" style=\"--category-columns:" + columns + "\">"
    + (loose ? "<div class=\"category-grid" + (spanLead ? " span-lead" : "") + "\">" + loose + "</div>" : "")
    + "<div class=\"category-groups\">" + grouped.groups.map(function(group) {
      const total = group.categories.reduce(function(sum, category) { return sum + (Number(category.count) || 0); }, 0);
      return "<section class=\"category-group\" aria-label=\"" + escapeHTML(group.prefix) + "\"><header><h3>" + escapeHTML(group.prefix) + "</h3>" + (total ? "<span>" + escapeHTML(total + " " + (total === 1 ? "channel" : "channels")) + "</span>" : "") + "</header><div class=\"category-group-list\">"
        + group.categories.map(function(category) { return categoryTileHTML(category, group.names[category.id]); }).join("") + "</div></section>";
    }).join("") + "</div></div>";
}
function categoryPrefixSplit(name) {
  const match = String(name || "").trim().match(/^([^\s|:/]{2,})\s*[|:/]?\s+(.+)$/);
  return match ? { prefix: match[1], rest: match[2].trim() } : null;
}
function categoryPrefixGroups(categories) {
  const byPrefix = {};
  const order = [];
  (categories || []).forEach(function(category) {
    const split = categoryPrefixSplit(category && (category.name || category.id));
    if (!split) return;
    const key = split.prefix.toLowerCase();
    if (!byPrefix[key]) { byPrefix[key] = { prefix: split.prefix, categories: [], names: {} }; order.push(key); }
    byPrefix[key].categories.push(category);
    byPrefix[key].names[category.id] = split.rest;
  });
  const groups = order.map(function(key) { return byPrefix[key]; }).filter(function(group) { return group.categories.length >= 2; });
  const groupedIDs = {};
  groups.forEach(function(group) { group.categories.forEach(function(category) { groupedIDs[category.id] = true; }); });
  return { groups: groups, loose: (categories || []).filter(function(category) { return !groupedIDs[category.id]; }) };
}
function allChannelsGuideTileHTML() {
  const count = visibleChannels(true).length;
  const meta = count + " " + (count === 1 ? "channel" : "channels");
  return "<button class=\"tile\" type=\"button\" data-all-channels-guide=\"true\" aria-label=\"All channels, " + escapeHTML(meta) + "\"><span class=\"tile-glyph\" aria-hidden=\"true\">" + icon("guide") + "</span><span class=\"tile-copy\"><strong>All</strong><span>" + escapeHTML(meta) + "</span></span><span class=\"tile-disclosure\" aria-hidden=\"true\">" + icon("chevron-right") + "</span></button>";
}
function categoryTileHTML(category, displayName) {
  const name = String((category && (category.name || category.id)) || "");
  const shown = String(displayName || name);
  const meta = String((category && category.count ? category.count + " channels" : (category && category.kind) || "") || "");
  return "<button class=\"tile" + (state.category === category.id ? " active" : "") + "\" data-category=\"" + escapeHTML(category.id) + "\" aria-label=\"" + escapeHTML(meta ? name + ", " + meta : name) + "\"><span class=\"tile-glyph\" aria-hidden=\"true\">" + icon(categoryGlyphName(category)) + "</span><span class=\"tile-copy\"><strong data-overflow-tooltip=\"" + escapeHTML(name) + "\">" + escapeHTML(shown) + "</strong><span>" + escapeHTML(meta) + "</span></span><span class=\"tile-disclosure\" aria-hidden=\"true\">" + icon("chevron-right") + "</span></button>";
}
function categoryGlyphName(category) {
  const name = lower(category && (category.name || category.id));
  if (/replay|catch.?up|archive/.test(name)) return "rewind";
  if (/event|special/.test(name)) return "clock";
  if (/kid|children|family|cartoon/.test(name)) return "kids";
  if (/news|weather/.test(name)) return "news";
  if (/sport|fight|racing/.test(name)) return "trophy";
  if (/\btv\b|movie|film|series|entertainment/.test(name)) return "tv";
  return "collection";
}
function activeVirtualCategoryID(path, featured) {
  return featured ? featuredCategoryID(path) : virtualCategoryID(path);
}
function virtualFolderHeader(path, featured, showViewToggle) {
  const categoryID = activeVirtualCategoryID(path, featured);
  return "<div class=\"section-title virtual-folder-title\">" + virtualFolderBreadcrumbs(path, featured) + "<div class=\"virtual-folder-actions\">" + guideFreshnessHTML() + (showViewToggle ? renderVirtualCategoryViewToggle() : "") + renderSaveChannelListButton(categoryID) + "</div></div>";
}
function virtualFolderBreadcrumbs(path, featured) {
  const parts = path.split(" / ").filter(Boolean);
  const rootLabel = featured ? featuredGroupLabel() : virtualGroupLabel();
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
    const name = renamedCategoryDisplayName(rawName);
    const featured = categoryStartsFeatured(rawName);
    const featuredPath = featured ? featuredPathForSourceName(rawName) : "";
    return { id: featuredPath ? featuredCategoryID(featuredPath) : sourceCategoryID(category.id), sourceID: category.id, name: name, featured: featured, kind: featuredPath ? "featured" : "source", count: categoryCounts[category.id] || 0 };
  });
}
function customGroupCategories() {
  return customGroups().map(function(group) {
    return { id: customCategoryID(group.id), name: group.name, kind: "custom", count: customMemberships(group.id).filter(function(id) { return channelMatchesProfileSelection(channelByID(id)); }).length };
  }).filter(function(group) {
    return group.count > 0;
  });
}
function virtualGroupCategories(includeChannel) {
  return categoriesFromChannelPaths("", includeChannel, virtualPathsForChannel, virtualCategoryID, "virtual", true);
}
function virtualCategoriesFromPaths(parentPath, includeChannel, includeAllDescendants) {
  return categoriesFromChannelPaths(parentPath, includeChannel, virtualPathsForChannel, virtualCategoryID, "virtual", includeAllDescendants);
}
function featuredCategoriesFromPaths(parentPath, includeChannel, includeAllDescendants) {
  return categoriesFromChannelPaths(parentPath, includeChannel, featuredPathsForChannel, featuredCategoryID, "featured", includeAllDescendants);
}
function categoriesFromChannelPaths(parentPath, includeChannel, pathsForChannel, categoryIDForPath, kind, includeAllDescendants) {
  parentPath = String(parentPath || "");
  const groups = {};
  effectiveChannels(false).forEach(function(channel) {
    if (includeChannel && !includeChannel(channel)) return;
    pathsForChannel(channel).forEach(function(path) {
      const parts = path.split(" / ").filter(Boolean);
      const parentParts = parentPath ? parentPath.split(" / ").filter(Boolean) : [];
      if (parts.length <= parentParts.length) return;
      for (let index = 0; index < parentParts.length; index++) {
        if (parts[index] !== parentParts[index]) return;
      }
      const limit = includeAllDescendants ? parts.length : parentParts.length + 1;
      for (let index = parentParts.length; index < limit; index++) {
        const childPath = parts.slice(0, index + 1).join(" / ");
        groups[childPath] = groups[childPath] || { id: categoryIDForPath(childPath), name: includeAllDescendants ? childPath : parts[index], kind: kind, count: 0, channelIDs: {} };
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
function guideFilterCategories() {
  const hidden = hiddenMap();
  const includeChannel = function(channel) { return !(channel.categoryId && hidden[channel.categoryId]); };
  const mode = adminSettings().mode || "normal";
  const custom = customGroupCategories();
  if (mode !== "delimiter") return custom.concat(sourceCategoriesWithChannels(includeChannel));
  return custom
    .concat(featuredCategoriesFromPaths("", includeChannel, true))
    .concat(virtualCategoriesFromPaths("", includeChannel, true))
    .sort(compareCategoryDisplayName);
}
function adminListingTitle() {
  const mode = adminSettings().mode || "normal";
  if (mode === "delimiter") return configuredGroupLabel();
  return "Channel Groups";
}
function organizationRootLabel() {
  return useProfileGroupVirtualPaths() ? "Channel Profiles" : "Channel Groups";
}
function configuredGroupLabel() {
  return virtualGroupLabelSuffix(adminSettings().virtualGroupLabel);
}
function virtualGroupLabel() {
  return configuredGroupLabel();
}
function featuredGroupLabel() {
  const mode = adminSettings().mode || "normal";
  return "Featured " + (mode === "delimiter" ? configuredGroupLabel() : "Channel Groups");
}
function allGroupLabel() {
  return "All " + adminListingTitle().toLowerCase();
}
function virtualGroupLabelSuffix(value) {
  value = String(value || "").trim().replace(/^virtual\s+/i, "").trim();
  return value || "Groups";
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
    if (!channel || !channelMatchesProfileSelection(channel)) return;
    seen[id] = true;
    channels.push(channel);
  });
  return channels.slice(0, limit || channels.length);
}
function channelHasCurrentGuide(channel) {
  if (!channel) return false;
  const now = Math.floor(Date.now() / 1000);
  return programsFor(channel.id).some(function(program) {
    const start = program.startUnix || 0;
    const end = program.endUnix || start + 1800;
    return start <= now + 600 && end >= now;
  });
}
function channelHasNearGuide(channel) {
  if (!channel) return false;
  const now = Math.floor(Date.now() / 1000);
  return programsFor(channel.id).some(function(program) {
    const start = program.startUnix || 0;
    const end = program.endUnix || start + 1800;
    return start <= now + 1800 && end >= now - 300;
  });
}
function maybeWarmGuideForChannels(channels, key) {
  if (!state.app || state.appLoadedFromCache || !items(channels).length) return;
  if (items(channels).every(channelHasNearGuide)) return;
  const channelIds = items(channels).map(function(channel) { return channel && channel.id; }).filter(Boolean);
  if (!channelIds.length) return;
  const warmKey = String(key || channelIds.slice(0, 20).join("|"));
  const now = Date.now();
  if (state.guideWarmPings[warmKey] && now - state.guideWarmPings[warmKey] < 5 * 60 * 1000) return;
  state.guideWarmPings[warmKey] = now;
  postJSON("/dispatcharr/api/guide/ping", { channelIds: channelIds }).then(function(result) {
    if (result && result.refreshing) {
      setTimeout(function() {
        refreshStatusData().then(function() { return refreshSupplementalData(); }).then(function() {
          if (state.view === "guide") refreshVisibleGuideBlock();
          else render();
        }).catch(function() {});
      }, 12000);
    }
  }).catch(function(error) {
    try { console.warn("Dispatcharr guide warm ping failed", error); } catch (_) {}
  });
}
function homeGuideChannels(watched) {
  const seen = {};
  const pool = [];
  items(watched).concat(visibleChannels(false).slice(0, 120)).forEach(function(channel) {
    if (!channel || seen[channel.id]) return;
    seen[channel.id] = true;
    pool.push(channel);
  });
  return pool.filter(channelHasCurrentGuide).slice(0, 5);
}
function renderHomeGuide(channels, emptyMessage, options) {
  const meta = options && options.hideFreshness ? "" : "<div class=\"guide-meta-row\">" + guideFreshnessHTML() + "</div>";
  const shelfClass = options && options.shelf ? " home-shelf-guide" : "";
  if (!channels.length) return meta + "<div class=\"empty\">" + escapeHTML(emptyMessage || "No recently watched channels yet.") + "</div>";
  const slots = guideSlots();
  return meta + "<div class=\"home-guide guide-scroll" + shelfClass + "\"><div class=\"guide-page guide-timeline\" style=\"" + guideTimelineStyle(slots) + "\">" + guideTimeHeadHTML(slots) + channels.map(function(channel, channelIndex) {
    return "<div class=\"epg-row\">" + renderGuideChannelButton(channel) + "<div class=\"epg-programs\">" + renderEPGCells(channel, channelIndex) + "</div></div>";
  }).join("") + "</div></div>";
}
function attachHomeGuidePageScroll() {
  document.querySelectorAll(".home-guide").forEach(function(guide) {
    guide.addEventListener("wheel", function(event) {
      if (Math.abs(event.deltaY) <= Math.abs(event.deltaX)) return;
      const main = document.querySelector(".main");
      if (!main || main.scrollHeight <= main.clientHeight) return;
      let delta = event.deltaY;
      if (event.deltaMode === 1) delta *= 16;
      if (event.deltaMode === 2) delta *= main.clientHeight;
      const maxScrollTop = main.scrollHeight - main.clientHeight;
      const nextScrollTop = Math.max(0, Math.min(maxScrollTop, main.scrollTop + delta));
      if (nextScrollTop === main.scrollTop) return;
      main.scrollTop = nextScrollTop;
      event.preventDefault();
    }, { passive: false });
  });
}
function renderVirtualCategoryGuide(channels) {
  return renderHomeGuide(channels, "No channels in this virtual group yet.", { hideFreshness: true });
}
function virtualCategoryView() {
  return state.virtualCategoryView === "list" ? "list" : "guide";
}
function renderVirtualCategoryViewToggle() {
  const active = virtualCategoryView();
  return "<div class=\"view-toggle\" aria-label=\"Virtual category view\"><button type=\"button\" data-virtual-category-view=\"guide\" class=\"" + (active === "guide" ? "active" : "") + "\" aria-pressed=\"" + (active === "guide" ? "true" : "false") + "\">Guide</button><button type=\"button\" data-virtual-category-view=\"list\" class=\"" + (active === "list" ? "active" : "") + "\" aria-pressed=\"" + (active === "list" ? "true" : "false") + "\">List</button></div>";
}
function renderVirtualCategoryChannelList(channels) {
  if (!channels.length) return sectionHeader("Channels") + "<div class=\"empty\">No channels in this virtual group yet.</div>";
  return sectionHeader("Channels") + "<div class=\"channel-button-list\">" + channels.map(function(channel) {
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
function folderSearchNeedle() {
  return lower(String(state.folderQuery || "").trim());
}
function channelMatchesFolderQuery(channel) {
  const query = folderSearchNeedle();
  if (!query) return true;
  const current = currentProgram(channel) || {};
  const next = nextProgram(channel) || {};
  return lower([
    channel && channel.name,
    channel && channel.categoryName,
    sourceCategoryLabel(channel || {}),
    channel && channel.number,
    channel && channel.id,
    current.title,
    current.description,
    next.title,
    next.description
  ].join(" ")).indexOf(query) !== -1;
}
function categoryMatchesFolderQuery(category) {
  const query = folderSearchNeedle();
  if (!query) return true;
  return lower([
    category && category.name,
    category && category.id,
    category && category.kind
  ].join(" ")).indexOf(query) !== -1;
}
function folderFilterHTML(placeholder, actionsHTML) {
  const label = placeholder || "Filter visible channels";
  return "<div class=\"folder-filter\">" + (actionsHTML ? "<div class=\"folder-filter-actions\">" + actionsHTML + "</div>" : "") + "<input id=\"folder-filter\" class=\"search\" type=\"search\" placeholder=\"" + escapeHTML(label) + "\" value=\"" + escapeHTML(state.folderQuery || "") + "\" autocomplete=\"off\" aria-label=\"" + escapeHTML(label) + "\"></div>";
}
function folderGroupOptionHTML(category, selectedID, pickerKind) {
  const id = String((category && category.id) || "");
  const name = String((category && (category.name || category.id)) || "All categories");
  const selected = id === String(selectedID || "");
  return "<button type=\"button\" class=\"folder-category-option" + (selected ? " selected" : "") + "\" data-folder-group-option=\"" + escapeHTML(pickerKind) + "\" data-folder-group-category=\"" + escapeHTML(id) + "\" data-folder-group-search=\"" + escapeHTML(lower(name)) + "\" role=\"option\" aria-selected=\"" + (selected ? "true" : "false") + "\"><span class=\"folder-category-check\">" + (selected ? icon("check") : "") + "</span><span><strong>" + escapeHTML(name) + "</strong></span></button>";
}
function renderFolderGroupPicker(children, selectedID, pickerKind) {
  const selected = items(children).find(function(category) { return category.id === selectedID; });
  const label = selected ? selected.name || selected.id : "All categories";
  const options = [folderGroupOptionHTML({ id: "", name: "All categories" }, selectedID, pickerKind)].concat(items(children).map(function(category) {
    return folderGroupOptionHTML(category, selectedID, pickerKind);
  })).join("");
  return "<div class=\"folder-category-picker" + (state.folderGroupPickerOpen ? " open" : "") + "\"><button type=\"button\" class=\"folder-category-trigger\" data-folder-group-toggle=\"true\" aria-haspopup=\"listbox\" aria-expanded=\"" + (state.folderGroupPickerOpen ? "true" : "false") + "\"><span class=\"folder-category-trigger-copy\"><small>Category</small><strong>" + escapeHTML(label) + "</strong></span><span class=\"folder-category-chevron\">" + icon("chevron-down") + "</span></button><div class=\"folder-category-popover\"><label class=\"folder-category-search\"><span>" + icon("search") + "</span><input id=\"folder-category-search\" placeholder=\"Find a category\" autocomplete=\"off\"></label><div class=\"folder-category-options\" role=\"listbox\">" + options + "</div></div></div>";
}
function renderSavedLineupGroupSelect(children) {
  return renderFolderGroupPicker(children, state.savedLineupGroupCategoryID, "saved");
}
function renderNestedFolderGroupSelect(children) {
  return renderFolderGroupPicker(children, state.folderGroupCategoryID, "folder");
}
function renderLivePage() {
  const channels = visibleChannels(false);
  if (state.view === "favorites") {
    byId("view").innerHTML = favoritesPage(channels);
    return;
  }
  if (state.category.indexOf("virtual:") === 0 || state.category.indexOf("featured:") === 0) {
    const featured = state.category.indexOf("featured:") === 0;
    const path = featured ? featuredCategoryPath(state.category) : virtualCategoryPath(state.category);
    const hidden = hiddenMap();
    const children = (featured ? featuredChildCategories : virtualChildCategories)(path, function(channel) {
      return !(channel.categoryId && hidden[channel.categoryId]);
    });
    const lineup = activeSavedLineup();
    const compactGroups = !!(lineup && lineup.hideFinalGroups && children.length);
    const nestedFolder = path.split(" / ").filter(Boolean).length > 1;
    const useGroupSelector = !!(children.length && (compactGroups || nestedFolder));
    if (state.savedLineupGroupCategoryID && !children.some(function(category) { return category.id === state.savedLineupGroupCategoryID; })) state.savedLineupGroupCategoryID = "";
    if (state.folderGroupCategoryID && !children.some(function(category) { return category.id === state.folderGroupCategoryID; })) state.folderGroupCategoryID = "";
    const filteredChildren = children.filter(categoryMatchesFolderQuery);
    const selectedGroupID = compactGroups ? state.savedLineupGroupCategoryID : (useGroupSelector ? state.folderGroupCategoryID : "");
    const filteredChannels = channels.filter(function(channel) {
      return (!selectedGroupID || channelInSelectedCategory(channel, selectedGroupID)) && channelMatchesFolderQuery(channel);
    });
    const browseOnly = !useGroupSelector && children.length > 0;
    const guideWorkspace = !browseOnly && virtualCategoryView() === "guide";
    const childFolders = browseOnly
      ? (filteredChildren.length
        ? "<div class=\"category-grid\" aria-label=\"Browse folders\">" + filteredChildren.map(function(category) { return categoryTileHTML(category); }).join("") + "</div>"
        : emptyStateHTML("No matching folders.", "Try a different filter."))
      : "";
    const filterActions = browseOnly ? "" : (compactGroups ? renderSavedLineupGroupSelect(children) : (useGroupSelector ? renderNestedFolderGroupSelect(children) : ""));
    const folderHeader = virtualFolderHeader(path, featured, !browseOnly)
      + folderFilterHTML(browseOnly ? "Filter this folder" : (guideWorkspace ? "Filter Guide" : "Filter this folder"), filterActions)
      + childFolders;
    if (browseOnly) {
      byId("view").innerHTML = folderHeader;
      return;
    }
    byId("view").innerHTML = guideWorkspace
      ? "<section class=\"virtual-folder-workspace is-guide\">" + folderHeader + "<div class=\"virtual-folder-guide\">" + renderVirtualCategoryGuide(filteredChannels) + "</div></section>"
      : folderHeader + renderVirtualCategoryContent(filteredChannels);
    maybeWarmGuideForChannels(filteredChannels, state.category);
    return;
  }
  const filteredChannels = channels.filter(channelMatchesFolderQuery);
  byId("view").innerHTML = (state.category ? "" : categoryGrid()) + sectionHeader(categoryName(state.category) || "Channels") + folderFilterHTML("Filter visible channels") + rowCards(filteredChannels.slice(0, 24));
  if (state.category) maybeWarmGuideForChannels(filteredChannels, state.category);
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
  const silo = recording && recording._silo ? recording._silo : {};
  const recordingID = String(silo.recording_id || recording.id || "");
  const actions = [];
  if (playbackURL) actions.push("<button class=\"recording-action\" data-recording-playback=\"" + escapeHTML(playbackURL) + "\">" + icon("play") + "<span>Playback</span></button>");
  const admin = siloUserIsAdmin();
  if (admin && recordingID && (silo.can_stop || status === "pending" || status === "scheduled" || status === "recording")) {
    actions.push("<button class=\"recording-action\" data-recording-stop=\"" + escapeHTML(recordingID) + "\">" + icon("stop") + "<span>Stop</span></button>");
  }
  if (admin && recordingID && silo.can_delete !== false) {
    actions.push("<button class=\"recording-action\" data-recording-delete=\"" + escapeHTML(recordingID) + "\">" + icon("trash") + "<span>Delete</span></button>");
  }
  return "<div class=\"recording-card\"><span><strong>" + escapeHTML(recordingTitle(recording)) + "</strong><span class=\"recording-meta\">" + escapeHTML(recordingChannelName(recording) + " - " + recordingWindow(recording)) + "</span></span><div class=\"recording-actions\">" + actions.join("") + "<span class=\"recording-badge " + escapeHTML(status) + "\">" + escapeHTML(status.split("_").join(" ")) + "</span></div></div>";
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
    if (state.view === "recordings" || state.view === "search" || state.view === "mytv") render();
  });
}
function programByID(channelID, programID) {
  return programsFor(channelID).find(function(program) { return String(program.id || "") === String(programID || ""); }) || null;
}
function scheduleProgram(channelID, programID, button) {
  if (!dvrEnabled()) {
    showAppToast(sourceMode() === "direct_login" ? "Recordings are turned off by Dispatcharr Admin." : "Recordings require Dispatcharr Direct Connect.");
    return;
  }
  if (!recordingSchedulingEnabled()) {
    showAppToast(recordingScheduleReason());
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
  }).catch(function(error) {
    const rawMessage = String(error && error.message ? error.message : error || "");
    const lowerMessage = rawMessage.toLowerCase();
    const status = Number(error && error.status || 0);
    if (status === 403
      || lowerMessage.indexOf("admin account or api key") >= 0
      || lowerMessage.indexOf("request failed (403)") >= 0
      || lowerMessage.indexOf("unexpected status 403") >= 0
      || lowerMessage.indexOf("forbidden") >= 0
      || lowerMessage.indexOf("permission") >= 0) {
      state.recordingCapability = { available: true, canSchedule: false, reason: "Scheduling requires a Dispatcharr admin account or Admin API Key." };
      render();
      renderProgramDetailsModal();
      showAppToast(state.recordingCapability.reason);
      return;
    }
    const message = readableError(error);
    if (status === 401 || message.toLowerCase().indexOf("session expired") >= 0) {
      showAppToast(message);
      return;
    }
    showAppToast("Dispatcharr could not schedule that recording.");
  }).finally(function() {
    if (button) button.disabled = false;
  });
}
// The /api/app bootstrap carries isAdmin (stamped from X-Silo-User-Role by the
// host). Anything other than an explicit true is treated as a regular user.
function siloUserIsAdmin() {
  return isAdminRoute || !!(state.app && state.app.isAdmin === true);
}
function adminOnlyDenied(error, action) {
  if (!error || Number(error.status || 0) !== 403) return false;
  showAppToast("Only Silo admins can " + action + ".");
  return true;
}
function stopRecording(recordingID, button) {
  if (!dvrEnabled() || !recordingID) return;
  if (button) button.disabled = true;
  postJSON("/dispatcharr/api/recordings/stop", { id: recordingID }).then(function() {
    state.recordings = null;
    loadRecordings(true);
    showAppToast("Recording stopped in Dispatcharr.");
  }).catch(function(error) {
    if (adminOnlyDenied(error, "stop recordings")) return;
    showAppToast(readableError(error) || "Dispatcharr could not stop that recording.");
  }).finally(function() {
    if (button) button.disabled = false;
  });
}
function deleteRecording(recordingID, button) {
  if (!dvrEnabled() || !recordingID) return;
  if (button) button.disabled = true;
  postJSON("/dispatcharr/api/recordings/delete", { id: recordingID }).then(function() {
    state.recordings = null;
    loadRecordings(true);
    showAppToast("Recording deleted in Dispatcharr.");
  }).catch(function(error) {
    if (adminOnlyDenied(error, "delete recordings")) return;
    showAppToast(readableError(error) || "Dispatcharr could not delete that recording.");
  }).finally(function() {
    if (button) button.disabled = false;
  });
}
function savedLineupID(name) {
  const baseID = lower(name).replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "") || "my-channels";
  let id = baseID;
  let index = 2;
  while (savedLineups().some(function(lineup) { return lineup.id === id; })) id = baseID + "-" + index++;
  return id;
}
function openSavedLineupEditor(categoryID, lineupID) {
  const existing = savedLineupByID(lineupID) || savedLineupForCategory(categoryID);
  const targetCategoryID = existing ? existing.categoryId : String(categoryID || state.category || "");
  if (!targetCategoryID) return;
  state.savedLineupEditor = existing
    ? { id: existing.id, categoryId: existing.categoryId, name: existing.name, hideFinalGroups: existing.hideFinalGroups === true, isNew: false }
    : { id: "", categoryId: targetCategoryID, name: savedLineupCategoryLabel(targetCategoryID), hideFinalGroups: false, isNew: true };
  renderSavedLineupModal();
}
function closeSavedLineupEditor() {
  state.savedLineupEditor = null;
  renderSavedLineupModal();
}
function saveSavedLineupEditor() {
  const editor = state.savedLineupEditor;
  if (!editor) return;
  const name = String(editor.name || "").trim();
  if (!name) {
    showAppToast("Name this channel list before saving.");
    return;
  }
  const lineups = savedLineups();
  const id = editor.id || savedLineupID(name);
  const saved = { id: id, name: name, categoryId: editor.categoryId, hideFinalGroups: editor.hideFinalGroups === true };
  const next = lineups.filter(function(lineup) { return lineup.id !== id && lineup.categoryId !== saved.categoryId; });
  next.push(saved);
  state.app.preferences.savedLineups = next;
  state.activeSavedLineupID = id;
  state.savedLineupGroupCategoryID = "";
  savePrefs();
  closeSavedLineupEditor();
  render();
  showAppToast("Saved lineup.");
}
function deleteSavedLineupEditor() {
  const editor = state.savedLineupEditor;
  if (!editor || editor.isNew) return;
  state.app.preferences.savedLineups = savedLineups().filter(function(lineup) { return lineup.id !== editor.id; });
  if (state.activeSavedLineupID === editor.id) {
    state.activeSavedLineupID = "";
    state.savedLineupGroupCategoryID = "";
  }
  savePrefs();
  closeSavedLineupEditor();
  render();
  showAppToast("Removed saved lineup.");
}
function savedLineupModalHTML(editor) {
  const title = editor.isNew ? "Save Channel List" : "Edit Channel List";
  return "<div class=\"program-modal-backdrop\" data-saved-lineup-close=\"true\"></div><section class=\"lineup-modal\" role=\"dialog\" aria-modal=\"true\" aria-labelledby=\"lineup-modal-title\">"
    + "<button class=\"program-modal-close\" type=\"button\" data-saved-lineup-close=\"true\" aria-label=\"Close\">" + icon("x") + "</button>"
    + "<div class=\"lineup-modal-head\"><h2 id=\"lineup-modal-title\">" + title + "</h2><p>Keep this profile or group available as a personal Live TV lineup.</p></div>"
    + "<div class=\"lineup-modal-body\"><label><span>Name</span><input type=\"text\" data-saved-lineup-field=\"name\" value=\"" + escapeHTML(editor.name) + "\" maxlength=\"80\" autocomplete=\"off\"></label><div class=\"lineup-modal-source\"><span>Source</span><strong>" + escapeHTML(savedLineupCategoryLabel(editor.categoryId)) + "</strong></div><label class=\"lineup-modal-toggle\"><span><strong>Hide final groups</strong><small>Move the remaining groups into a dropdown above the guide.</small></span><input type=\"checkbox\" data-saved-lineup-field=\"hideFinalGroups\"" + (editor.hideFinalGroups ? " checked" : "") + "></label></div>"
    + "<div class=\"lineup-modal-actions\">" + (!editor.isNew ? "<button type=\"button\" class=\"danger\" data-saved-lineup-action=\"delete\">Remove</button>" : "") + "<span></span><button type=\"button\" data-saved-lineup-close=\"true\">Cancel</button><button type=\"button\" class=\"primary\" data-saved-lineup-action=\"save\">Save</button></div>"
    + "</section>";
}
function renderSavedLineupModal() {
  let root = byId("saved-lineup-root");
  if (!root) {
    root = document.createElement("div");
    root.id = "saved-lineup-root";
    document.body.appendChild(root);
  }
  root.innerHTML = state.savedLineupEditor ? savedLineupModalHTML(state.savedLineupEditor) : "";
  const shell = document.querySelector(".shell");
  if (shell) {
    if (state.savedLineupEditor) shell.setAttribute("inert", "");
    else if (!state.programDetails) shell.removeAttribute("inert");
  }
  document.body.classList.toggle("program-modal-open", !!(state.savedLineupEditor || state.programDetails));
  if (state.savedLineupEditor) {
    const input = root.querySelector("[data-saved-lineup-field=\"name\"]");
    if (input) input.focus();
  }
}
function renderSaveChannelListButton(categoryID) {
  categoryID = String(categoryID || "");
  if (!categoryID) return "";
  const existing = savedLineupForCategory(categoryID);
  const label = existing ? "Edit Channel List" : "Save Channel List";
  return "<button type=\"button\" class=\"section-action save-channel-list\" data-saved-lineup-edit=\"" + escapeHTML(existing ? existing.id : "") + "\" data-saved-lineup-category=\"" + escapeHTML(categoryID) + "\">" + icon(existing ? "heart-solid" : "heart") + "<span>" + label + "</span></button>";
}
function programDetailsState() {
  if (!state.programDetails) return null;
  const channel = channelByID(state.programDetails.channelID);
  const program = programByID(state.programDetails.channelID, state.programDetails.programID) || state.programDetails.fallbackProgram;
  if (!channel || !program) return null;
  return { channel: channel, program: program };
}
function openProgramDetails(channelID, programID) {
  const program = programByID(channelID, programID);
  const channel = channelByID(channelID);
  if (!channel) return;
  programModalReturnFocus = document.activeElement;
  if (!program || programIsGuidePlaceholder(program)) {
    state.programDetails = { channelID: channelID, programID: programID, fallbackProgram: { id: "", title: "Program details unavailable", description: "Program details are not available for this guide entry. You can still watch the channel.", startUnix: 0, endUnix: 0 } };
    renderProgramDetailsModal();
    return;
  }
  state.programDetails = { channelID: channelID, programID: programID };
  renderProgramDetailsModal();
}
function closeProgramDetails() {
  state.programDetails = null;
  renderProgramDetailsModal();
  const returnFocus = programModalReturnFocus;
  programModalReturnFocus = null;
  if (returnFocus && typeof returnFocus.focus === "function" && document.contains(returnFocus)) returnFocus.focus();
}
function programDetailTags(program, channel) {
  const tags = [];
  if (programLooksSports(program)) tags.push("Sports");
  else if (programLooksMovie(program)) tags.push("Movie");
  if (program.rating) tags.push(program.rating);
  if (channel && channel.categoryName) tags.push(categoryDisplayName(channel.categoryName));
  if (programIsLive(program)) tags.push("Live now");
  return uniqueIDs(tags).slice(0, 5);
}
function programDurationLabel(seconds) {
  const minutes = Math.max(1, Math.round(Number(seconds || 0) / 60));
  const hours = Math.floor(minutes / 60);
  const remainder = minutes % 60;
  if (hours && remainder) return hours + " hr " + remainder + " min";
  if (hours) return hours + (hours === 1 ? " hr" : " hrs");
  return minutes + " min";
}
function programDetailsModalHTML(details) {
  const channel = details.channel;
  const program = details.program;
  const title = program.title || guideUnavailableLabel();
  const description = String(program.summary || program.description || "").trim();
  const start = program.startUnix || 0;
  const end = program.endUnix || 0;
  const duration = start && end ? Math.max(1, Math.round((end - start) / 60)) : 0;
  const timeText = [start ? dateTimeLabel(start) : "", duration ? programDurationLabel(duration * 60) : ""].filter(Boolean).join(", ");
  const channelName = channel.name || channel.categoryName || "Live TV";
  const tags = programDetailTags(program, channel);
  const canSchedule = recordingSchedulingEnabled() && end > Math.floor(Date.now() / 1000);
  return "<div class=\"program-modal-backdrop\" data-program-modal-close=\"true\"></div><section class=\"program-modal\" role=\"dialog\" aria-modal=\"true\" aria-labelledby=\"program-modal-title\" aria-describedby=\"program-modal-description\">"
    + "<button class=\"program-modal-close\" type=\"button\" data-program-modal-close=\"true\" aria-label=\"Close\">" + icon("x") + "</button>"
    + "<div class=\"program-modal-head\"><div class=\"program-modal-art\">" + logoHTML(channel) + "</div><div class=\"program-modal-title\"><h2 id=\"program-modal-title\">" + escapeHTML(title) + "</h2><p>" + escapeHTML(channelName) + "</p><div class=\"program-modal-time\">" + icon("clock") + "<span>" + escapeHTML(timeText || (programIsLive(program) ? "Live now" : "Guide airing")) + "</span></div><div class=\"program-modal-tags\">" + tags.map(function(tag) { return "<span" + (tag === "Live now" ? " class=\"is-live\"" : "") + ">" + escapeHTML(tag) + "</span>"; }).join("") + "</div></div></div>"
    + "<div id=\"program-modal-description\" class=\"program-modal-body\">" + (description ? escapeHTML(description) : "No additional details are available for this airing.") + "</div>"
    + "<div class=\"program-modal-actions\"><button type=\"button\" data-search-airing=\"" + escapeHTML(title) + "\">" + icon("search") + "<span>More Airings</span></button><button type=\"button\" data-program-detail-watch=\"" + escapeHTML(channel.id) + "\">" + icon("play") + "<span>Watch Now</span></button>" + (canSchedule ? "<button type=\"button\" data-program-detail-schedule=\"" + escapeHTML(channel.id) + "\" data-program-detail-program=\"" + escapeHTML(program.id || "") + "\">" + icon("record") + "<span>Record</span></button>" : "") + "</div>"
    + "</section>";
}
function renderProgramDetailsModal() {
  let root = byId("program-details-root");
  if (!root) {
    root = document.createElement("div");
    root.id = "program-details-root";
    document.body.appendChild(root);
  }
  const details = programDetailsState();
  root.innerHTML = details ? programDetailsModalHTML(details) : "";
  const shell = document.querySelector(".shell");
  if (shell) {
    if (details) shell.setAttribute("inert", "");
    else shell.removeAttribute("inert");
  }
  document.body.classList.toggle("program-modal-open", !!details);
  if (details) {
    const closeButton = root.querySelector(".program-modal-close");
    if (closeButton) closeButton.focus();
  }
}
function trapProgramModalFocus(event) {
  if (!state.programDetails || event.key !== "Tab") return false;
  const modal = document.querySelector(".program-modal");
  if (!modal) return false;
  const focusable = Array.prototype.slice.call(modal.querySelectorAll("button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex='-1'])"));
  if (!focusable.length) return false;
  const first = focusable[0];
  const last = focusable[focusable.length - 1];
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault();
    last.focus();
    return true;
  }
  if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault();
    first.focus();
    return true;
  }
  return false;
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
function liveProgram(channel) {
  if (!channel) return null;
  const now = Math.floor(Date.now() / 1000);
  return programsFor(channel.id).find(function(program) {
    return (!program.startUnix || program.startUnix <= now) && (!program.endUnix || program.endUnix >= now);
  }) || null;
}
function nextProgram(channel) {
  if (!channel) return null;
  const now = Math.floor(Date.now() / 1000);
  return programsFor(channel.id).find(function(program) {
    return (program.startUnix || 0) > now;
  }) || null;
}
function timeShiftSeek(delta) {
  const video = byId("player");
  if (!video || !video.seekable || !video.seekable.length) return;
  const start = video.seekable.start(0);
  const end = video.seekable.end(video.seekable.length - 1);
  video.currentTime = Math.max(start, Math.min(end - 0.25, video.currentTime + delta));
  if (state.timeShiftSession) state.timeShiftSession.rewound = true;
  updateTimeShiftUI();
}
function timeShiftGoLive() {
  const video = byId("player");
  if (!video || !video.seekable || !video.seekable.length) return;
  video.currentTime = timeShiftLivePosition(video);
  if (state.timeShiftSession) state.timeShiftSession.rewound = false;
  video.play().catch(function() {});
  updateTimeShiftUI();
}
function boundedHLSBufferSeconds(value) {
  return Math.max(5, Math.min(60, Math.round(Number(value) || 12)));
}
function liveHLSOptions(value) {
  const seconds = boundedHLSBufferSeconds(value);
  return {
    liveSyncDuration: seconds,
    liveMaxLatencyDuration: Math.max(seconds + 3, seconds * 2),
    maxBufferLength: seconds,
    maxMaxBufferLength: Math.max(seconds * 2, 30),
    backBufferLength: 30,
    startFragPrefetch: true,
    maxLiveSyncPlaybackRate: 1.15
  };
}
function playbackErrorStatus(error) {
  const values = [
    error && error.status,
    error && error.code,
    error && error.response && error.response.code,
    error && error.info && error.info.code,
    error && error.errorInfo && error.errorInfo.code
  ];
  for (let index = 0; index < values.length; index += 1) {
    const status = Number(values[index]);
    if (Number.isFinite(status) && status >= 100 && status <= 599) return status;
  }
  return 0;
}
function handlePlaybackFatalError(error) {
  const status = playbackErrorStatus(error);
  state.playerWaiting = false;
  const video = byId("player");
  if (video) video.pause();
  updateCenterPlayButton();
  if (status === 401 || status === 403) {
    showPlayerError("Playback authorization expired. Try again to reopen the channel.");
    return;
  }
  if (status === 503) {
    showPlayerError("This channel is temporarily unavailable.");
    return;
  }
  showPlayerError("The stream could not be loaded. The channel may be offline or out of connections.");
}
function isPluginStreamRoute(url) {
  return String(url || "").indexOf("/dispatcharr/stream?") !== -1;
}
async function resolvePlayableStreamURL(url) {
  const payload = await coreGetJSON(url + "&resolve=json");
  const resolved = payload && typeof payload.url === "string" ? payload.url.trim() : "";
  return /^https:\/\//i.test(resolved) || (/^http:\/\//i.test(resolved) && window.location.protocol === "http:") ? resolved : url;
}
function attachVideoSource(video, url, options) {
  const attachment = {
    hls: null,
    tsPlayer: null,
    cancelled: false,
    destroy: function() {
      attachment.cancelled = true;
      if (attachment.hls) { attachment.hls.destroy(); attachment.hls = null; }
      if (attachment.tsPlayer) { attachment.tsPlayer.destroy(); attachment.tsPlayer = null; }
      if (video) {
        video.pause();
        video.removeAttribute("src");
        video.load();
      }
    }
  };
  if (!isPluginStreamRoute(url)) return startVideoSource(video, url, options, attachment);
  resolvePlayableStreamURL(url).then(function(playableURL) {
    if (!attachment.cancelled) startVideoSource(video, playableURL, options, attachment);
  }, function(error) {
    if (attachment.cancelled) return;
    if (options && typeof options.onFatal === "function") options.onFatal(error);
    else startVideoSource(video, url, options, attachment);
  });
  return attachment;
}
function startVideoSource(video, url, options, attachment) {
  const rewindable = !!(options && options.rewindable);
  const managedTimeShift = !!(options && options.managedTimeShift);
  const isHLS = (options && options.format === "hls") || url.indexOf(".m3u8") !== -1;
  if (window.Hls && Hls.isSupported() && isHLS) {
    // Managed rewind must not jump back to live after a pause or seek.
    const hlsOptions = managedTimeShift ? { liveSyncDurationCount: 1, liveMaxLatencyDurationCount: Infinity, maxBufferLength: 60 } : liveHLSOptions(options && options.hlsBufferSeconds);
    hlsOptions.xhrSetup = applyCoreMediaRequest;
    attachment.hls = new Hls(hlsOptions);
    let recoveryAttempts = 0;
    let fatalHandled = false;
    attachment.hls.on(Hls.Events.FRAG_LOADED, function() { recoveryAttempts = 0; });
    attachment.hls.on(Hls.Events.ERROR, function(_, data) {
      if (!data || !data.fatal) return;
      if (!managedTimeShift && recoveryAttempts < 2) {
        recoveryAttempts += 1;
        if (data.type === Hls.ErrorTypes.NETWORK_ERROR) {
          attachment.hls.startLoad();
          return;
        }
        if (data.type === Hls.ErrorTypes.MEDIA_ERROR) {
          attachment.hls.recoverMediaError();
          return;
        }
      }
      if (!fatalHandled && options && typeof options.onFatal === "function") {
        fatalHandled = true;
        options.onFatal(data);
      }
    });
    attachment.hls.loadSource(url);
    attachment.hls.attachMedia(video);
  } else if (window.mpegts && mpegts.isSupported() && !isHLS) {
    const mediaHeaders = coreMediaRequestHeaders(url);
    const mediaSource = { type: "mpegts", isLive: !rewindable, url: url, headers: mediaHeaders, withCredentials: Object.keys(mediaHeaders).length > 0 };
    attachment.tsPlayer = mpegts.createPlayer(mediaSource, { headers: mediaHeaders });
    if (mpegts.Events && mpegts.Events.ERROR) {
      let fatalHandled = false;
      attachment.tsPlayer.on(mpegts.Events.ERROR, function(errorType, errorDetail, errorInfo) {
        if (fatalHandled) return;
        fatalHandled = true;
        if (options && typeof options.onFatal === "function") options.onFatal({ type: errorType, detail: errorDetail, info: errorInfo });
      });
    }
    attachment.tsPlayer.attachMediaElement(video);
    attachment.tsPlayer.load();
  } else {
    video.src = url;
  }
  return attachment;
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
  if (target.getAttribute("data-tooltip-always") === "true") return true;
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
function renderSettings() {
  ensureSelectedCustomGroup();
  const showSourceCategorySettings = !virtualCategoriesActive();
  byId("view").innerHTML = "<div class=\"settings-stack\">"
    + (isDispatcharrDirectSource() ? "<div class=\"settings-card profile-settings-card\"><div class=\"settings-card-head\"><div><h2>Live TV profiles</h2><p>Choose which Dispatcharr profile lineups appear in your Live TV experience.</p></div><span id=\"profile-selection-summary\" class=\"profile-selection-summary\"></span></div><div id=\"profile-settings\"></div></div>" : "")
    + "<div class=\"settings-card custom-groups-card\"><h2>Custom groups</h2><div id=\"custom-group-settings\"></div></div>"
    + (showSourceCategorySettings ? "<div class=\"settings-card\"><h2>Hidden channel groups</h2><div id=\"settings-list\" class=\"settings-list\"></div></div>" : "")
    + "</div>";
  renderProfileSettings();
  renderCustomGroupSettings();
  if (!showSourceCategorySettings) return;
  const root = byId("settings-list");
  const categories = sourceCategoriesWithChannels();
  root.innerHTML = categories.map(function(category) {
    return "<label><span>" + escapeHTML(category.name || category.sourceID) + "</span><input type=\"checkbox\" data-hide=\"" + escapeHTML(category.sourceID) + "\"" + (hiddenMap()[category.sourceID] ? " checked" : "") + "></label>";
  }).join("") || "<div class=\"empty\">No channel groups available for this connection.</div>";
}
function renderProfileSettings() {
  const root = byId("profile-settings");
  if (!root) return;
  const profiles = availableChannelProfiles();
  const selection = profileSelection();
  const allProfiles = selection.mode !== "selected";
  const selected = selectedProfileMap();
  const summary = byId("profile-selection-summary");
  if (summary) summary.textContent = allProfiles ? "All " + profiles.length + " profiles" : selection.profileIds.length + " of " + profiles.length + " profiles";
  if (!profiles.length) {
    root.innerHTML = profileSaveStatusHTML() + "<div class=\"empty\">No Dispatcharr channel profiles are available.</div>";
    return;
  }
  const query = lower(state.profileSettingsQuery);
  const visibleProfiles = profiles.filter(function(profile) {
    return !query || lower([profile.name, profile.id].join(" ")).indexOf(query) !== -1;
  });
  const rows = visibleProfiles.map(function(profile) {
    const checked = allProfiles || !!selected[profile.id];
    const count = Number(profile.channelCount || 0);
    return "<label class=\"profile-selection-row\"><span><strong>" + escapeHTML(profile.name || profile.id) + "</strong><small>" + escapeHTML(count + " channel" + (count === 1 ? "" : "s")) + "</small></span><input type=\"checkbox\" data-profile-selection-id=\"" + escapeHTML(profile.id) + "\"" + (checked ? " checked" : "") + "></label>";
  }).join("");
  root.innerHTML = profileSaveStatusHTML()
    + "<div class=\"profile-settings-toolbar\"><label class=\"profile-settings-search\"><span>" + icon("search") + "</span><input id=\"profile-settings-filter\" placeholder=\"Filter profiles\" value=\"" + escapeHTML(state.profileSettingsQuery) + "\" autocomplete=\"off\"></label><button type=\"button\" data-profile-selection-action=\"all\"" + (allProfiles ? " disabled" : "") + ">Use all profiles</button></div>"
    + "<div class=\"profile-selection-list\">" + (rows || "<div class=\"empty\">No profiles match that filter.</div>") + "</div>";
}
function applyProfileSelection(selection) {
  if (!state.app || !state.app.preferences) return;
  state.app.preferences.profileSelection = normalizeProfileSelection(selection);
  invalidateProfileSelectionCache();
  state.guideChannels = [];
  state.guideWindowStart = -1;
  state.guideWindowEnd = -1;
  state.sportsExpandedEvents = {};
  state.expandedEvents = {};
  if (state.category && !categoryName(state.category)) state.category = "";
  savePrefs();
  render();
}
function useAllProfiles() {
  applyProfileSelection({ mode: "all", profileIds: [] });
}
function updateSelectedProfile(profileID, enabled) {
  profileID = String(profileID || "");
  const profiles = availableChannelProfiles();
  const allIDs = profiles.map(function(profile) { return profile.id; });
  let selectedIDs = profileSelectionIsAll() ? allIDs.slice() : profileSelection().profileIds.slice();
  if (enabled && selectedIDs.indexOf(profileID) === -1) selectedIDs.push(profileID);
  if (!enabled) selectedIDs = selectedIDs.filter(function(id) { return id !== profileID; });
  selectedIDs = uniqueIDs(selectedIDs).filter(function(id) { return allIDs.indexOf(id) !== -1; });
  if (!selectedIDs.length) {
    showAppToast("Select at least one Live TV profile.");
    renderProfileSettings();
    return;
  }
  applyProfileSelection(selectedIDs.length === allIDs.length ? { mode: "all", profileIds: [] } : { mode: "selected", profileIds: selectedIDs });
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
  settings[field] = target.type === "checkbox" ? !!target.checked : target.value;
  if (!settings.delimiter) settings.delimiter = "pipe";
  state.adminCategorySettings = settings;
  if (state.category.indexOf("virtual:") === 0 && !categoryName(state.category)) state.category = "";
  normalizeAdminCategorySettings();
  markAdminSettingsDraft();
  renderAdminPage();
}
function updateOrganizationLayer(field, target) {
  const settings = state.adminCategorySettings || defaultAdminCategorySettings();
  const layers = organizationLayerState();
  layers[field] = !!target.checked;
  if (layers.profiles && layers.groups) settings.virtualGroupSource = "profile_group";
  else if (layers.profiles) settings.virtualGroupSource = "profile";
  else if (layers.groups) settings.virtualGroupSource = layers.channels ? "group_channel" : "group";
  else settings.virtualGroupSource = "channel";
  state.adminCategorySettings = settings;
  normalizeAdminCategorySettings();
  markAdminSettingsDraft();
  renderAdminPage();
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
function closePlayerPopovers(except) {
  if (except !== "audio") state.audioMenuOpen = false;
  if (except !== "volume") state.volumeMenuOpen = false;
  if (except !== "more") state.moreMenuOpen = false;
  updateAudioMenu();
  updateVolumeMenu();
  renderPlayerMoreMenu();
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
function handlePlayerAction(action, button) {
  const video = byId("player");
  wakePlayerChrome();
  if (action === "back") {
    returnFromPlayer();
    return;
  }
  if (action === "retry") {
    if (state.currentChannel) playChannel(state.currentChannel, { historyMode: "none" });
    return;
  }
  if (action === "guide") {
    state.playerGuideOpen = !state.playerGuideOpen;
    state.playerSportsOpen = false;
    stopPlayerSportsRefresh();
    closePlayerPopovers();
    renderPlayerGuidePanel();
    renderPlayerSportsDrawer();
    return;
  }
  if (action === "guide-close") {
    state.playerGuideOpen = false;
    renderPlayerGuidePanel();
    return;
  }
  if (action === "sports") {
    togglePlayerSports();
    return;
  }
  if (action === "sports-close") {
    togglePlayerSports(false);
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
  if (action === "rewind-30") {
    timeShiftSeek(-30);
    return;
  }
  if (action === "forward-30") {
    timeShiftSeek(30);
    return;
  }
  if (action === "go-live") {
    timeShiftGoLive();
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
    setView("search");
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
    const isFavorite = setChannelFavorite(id, !favoriteMap()[id]);
    if (button) {
      button.innerHTML = icon(isFavorite ? "heart-solid" : "heart");
      button.classList.toggle("active", isFavorite);
      button.setAttribute("aria-pressed", isFavorite ? "true" : "false");
      button.setAttribute("aria-label", isFavorite ? "Remove channel from favorites" : "Favorite channel");
    }
    renderRail();
  }
}
function returnFromPlayer() {
  if (window.history && window.history.state && window.history.state[appHistoryStateKey]) {
    window.history.back();
    return;
  }
  const context = state.playerReturnContext;
  if (!context) {
    setView("live");
    return;
  }
  state.playerReturnContext = null;
  state.category = context.category || "";
  state.query = context.query || "";
  state.folderQuery = context.folderQuery || "";
  setView(context.view || "live", { preserveBrowseState: true });
  requestAnimationFrame(function() {
    window.scrollTo(0, context.scrollY || 0);
    const main = document.querySelector(".main");
    if (main) main.scrollTop = context.mainScrollTop || 0;
    const guideScroll = byId("guide-scroll");
    if (guideScroll) {
      guideScroll.scrollLeft = context.guideScrollLeft || 0;
      guideScroll.scrollTop = context.guideScrollTop || 0;
      renderGuideWindow(true);
    }
  });
}
document.addEventListener("click", function(event) {
  const guideNow = event.target.closest("[data-guide-now]");
  if (guideNow) { event.preventDefault(); jumpGuideToNow(); return; }
  const guideResult = event.target.closest("[data-guide-search-channel]");
  if (guideResult) {
    event.preventDefault();
    jumpGuideToProgram(guideResult.getAttribute("data-guide-search-channel"), guideResult.getAttribute("data-guide-search-program"), Number(guideResult.getAttribute("data-guide-search-start")));
    return;
  }
  if (!event.target.closest(".guide-search-wrap")) dismissGuideProgramSearch();
  const guideCategory = event.target.closest("[data-guide-category]");
  if (guideCategory) {
    event.preventDefault();
    state.category = guideCategory.getAttribute("data-guide-category") || "";
    state.guideCategoryPickerOpen = false;
    state.guideCategoryQuery = "";
    renderGuidePage();
    return;
  }
  const guideCategoryToggle = event.target.closest("[data-guide-category-toggle]");
  if (guideCategoryToggle) {
    event.preventDefault();
    state.guideCategoryPickerOpen = !state.guideCategoryPickerOpen;
    renderGuidePage();
    if (state.guideCategoryPickerOpen) {
      const input = byId("guide-category-search");
      if (input) input.focus();
    }
    return;
  }
  if (state.guideCategoryPickerOpen && !event.target.closest(".guide-category-picker")) {
    state.guideCategoryPickerOpen = false;
    const picker = document.querySelector(".guide-category-picker");
    if (picker) picker.classList.remove("open");
  }
  const folderGroupOption = event.target.closest("[data-folder-group-option]");
  if (folderGroupOption) {
    event.preventDefault();
    const value = folderGroupOption.getAttribute("data-folder-group-category") || "";
    if (folderGroupOption.getAttribute("data-folder-group-option") === "saved") state.savedLineupGroupCategoryID = value;
    else state.folderGroupCategoryID = value;
    state.folderGroupPickerOpen = false;
    renderLivePage();
    return;
  }
  const folderGroupToggle = event.target.closest("[data-folder-group-toggle]");
  if (folderGroupToggle) {
    event.preventDefault();
    state.folderGroupPickerOpen = !state.folderGroupPickerOpen;
    renderLivePage();
    if (state.folderGroupPickerOpen) {
      const input = byId("folder-category-search");
      if (input) input.focus();
    }
    return;
  }
  if (state.folderGroupPickerOpen && !event.target.closest(".folder-category-picker")) {
    state.folderGroupPickerOpen = false;
    const picker = document.querySelector(".folder-category-picker");
    if (picker) picker.classList.remove("open");
  }
  const savedLineupOpen = event.target.closest("[data-saved-lineup-open]");
  if (savedLineupOpen) {
    event.preventDefault();
    openSavedLineup(savedLineupOpen.getAttribute("data-saved-lineup-open"));
    return;
  }
  const savedLineupEdit = event.target.closest("[data-saved-lineup-edit]");
  if (savedLineupEdit) {
    event.preventDefault();
    openSavedLineupEditor(savedLineupEdit.getAttribute("data-saved-lineup-category"), savedLineupEdit.getAttribute("data-saved-lineup-edit"));
    return;
  }
  const savedLineupClose = event.target.closest("[data-saved-lineup-close]");
  if (savedLineupClose) {
    event.preventDefault();
    closeSavedLineupEditor();
    return;
  }
  const savedLineupAction = event.target.closest("[data-saved-lineup-action]");
  if (savedLineupAction) {
    event.preventDefault();
    if (savedLineupAction.getAttribute("data-saved-lineup-action") === "delete") deleteSavedLineupEditor();
    else saveSavedLineupEditor();
    return;
  }
  const timeShiftAdminAction = event.target.closest("[data-timeshift-admin-action]");
  if (timeShiftAdminAction) {
    const action = timeShiftAdminAction.getAttribute("data-timeshift-admin-action");
    if (action === "refresh") refreshAdminTimeShiftStatus(false);
    if (action === "clear") clearAdminTimeShiftCache();
    return;
  }
  const rewindWindow = event.target.closest("[data-admin-rewind-window]");
  if (rewindWindow) {
    event.preventDefault();
    updateAdminTimeShiftField("window", { value: rewindWindow.getAttribute("data-admin-rewind-window") });
    return;
  }
  const adminSportsAction = event.target.closest("[data-admin-sports-action]");
  if (adminSportsAction) {
    event.preventDefault();
    const sportsAction = adminSportsAction.getAttribute("data-admin-sports-action");
    if (sportsAction === "refresh-libraries") loadSportsLibraries(true);
    if (sportsAction === "add-team-channel" || sportsAction === "remove-team-channel") updateAdminSportsTeamChannels(sportsAction, adminSportsAction);
    return;
  }
  const settingsMenuButton = event.target.closest("#settings-menu-button");
  if (settingsMenuButton) {
    event.preventDefault();
    setSettingsMenuOpen(!settingsMenuOpen());
    return;
  }
  if (!event.target.closest(".settings-menu")) setSettingsMenuOpen(false);
  const profileSelectionAction = event.target.closest("[data-profile-selection-action]");
  if (profileSelectionAction) {
    event.preventDefault();
    if (profileSelectionAction.getAttribute("data-profile-selection-action") === "all") useAllProfiles();
    return;
  }
  const playerSportsChannel = event.target.closest("[data-player-sports-channel]");
  if (playerSportsChannel) {
    event.preventDefault();
    const channel = channelByID(playerSportsChannel.getAttribute("data-player-sports-channel"));
    if (channel) {
      state.playerSportsOpen = false;
      stopPlayerSportsRefresh();
      playChannel(channel);
    }
    return;
  }
  const playerSportsMore = event.target.closest("[data-player-sports-more] > summary");
  if (playerSportsMore) {
    const details = playerSportsMore.closest("[data-player-sports-more]");
    requestAnimationFrame(function() {
      state.playerSportsMoreOpen = !!(details && details.open);
      keepPlayerSportsDisclosureVisible(details);
    });
    return;
  }
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
    setSettingsMenuOpen(false);
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
    refreshGuideBlockData();
    return;
  }
  const programDetailClose = event.target.closest("[data-program-modal-close]");
  if (programDetailClose) {
    event.preventDefault();
    closeProgramDetails();
    return;
  }
  const programDetailWatch = event.target.closest("[data-program-detail-watch]");
  if (programDetailWatch) {
    event.preventDefault();
    const channel = channelByID(programDetailWatch.getAttribute("data-program-detail-watch"));
    closeProgramDetails();
    if (channel) playChannel(channel);
    return;
  }
  const programDetailSchedule = event.target.closest("[data-program-detail-schedule]");
  if (programDetailSchedule) {
    event.preventDefault();
    scheduleProgram(programDetailSchedule.getAttribute("data-program-detail-schedule"), programDetailSchedule.getAttribute("data-program-detail-program"), programDetailSchedule);
    return;
  }
  const programDetailTarget = event.target.closest("[data-program-detail-channel]");
  if (programDetailTarget) {
    event.preventDefault();
    openProgramDetails(programDetailTarget.getAttribute("data-program-detail-channel"), programDetailTarget.getAttribute("data-program-detail"));
    return;
  }
  const searchCancel = event.target.closest("[data-search-cancel]");
  if (searchCancel) {
    event.preventDefault();
    setView(state.searchReturnView || "home");
    return;
  }
  const searchQueryClear = event.target.closest("[data-search-query-clear]");
  if (searchQueryClear) {
    event.preventDefault();
    state.searchQuery = "";
    state.searchAiringChannel = "";
    renderSearchPage();
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
    state.searchAiringChannel = "";
    rememberSearch(state.searchQuery);
    renderSearchPage();
    return;
  }
  const searchType = event.target.closest("[data-search-type]");
  if (searchType) {
    event.preventDefault();
    state.searchType = searchType.getAttribute("data-search-type") || "all";
    state.searchAiringChannel = "";
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
  const searchCategory = event.target.closest("[data-search-category]");
  if (searchCategory) {
    event.preventDefault();
    rememberSearch(state.searchQuery);
    setCategory(searchCategory.getAttribute("data-search-category"));
    return;
  }
  const searchProgram = event.target.closest("[data-search-program-channel]");
  if (searchProgram) {
    event.preventDefault();
    rememberSearch(state.searchQuery);
    const channelID = searchProgram.getAttribute("data-search-program-channel");
    const programID = searchProgram.getAttribute("data-search-program");
    openProgramDetails(channelID, programID);
    return;
  }
  const searchAiring = event.target.closest("[data-search-airing]");
  if (searchAiring) {
    event.preventDefault();
    rememberSearch(state.searchQuery);
    closeProgramDetails();
    state.searchQuery = searchAiring.getAttribute("data-search-airing") || state.searchQuery;
    state.searchAiringChannel = searchAiring.getAttribute("data-search-airing-channel") || "";
    state.searchType = "programs";
    setView("search");
    return;
  }
  const myTVSearchClear = event.target.closest("[data-my-tv-search-clear]");
  if (myTVSearchClear) {
    event.preventDefault();
    state.myTVQuery = "";
    const input = byId("my-tv-search");
    if (input) {
      input.value = "";
      input.focus();
    }
    updateMyTVSearchSurface();
    return;
  }
  const keywordPassAdd = event.target.closest("[data-keyword-pass-add]");
  if (keywordPassAdd) {
    event.preventDefault();
    addKeywordPass(keywordPassAdd.getAttribute("data-keyword-pass-add"));
    return;
  }
  const keywordPassRemove = event.target.closest("[data-keyword-pass-remove]");
  if (keywordPassRemove) {
    event.preventDefault();
    removeKeywordPass(keywordPassRemove.getAttribute("data-keyword-pass-remove"));
    return;
  }
  const onLaterMore = event.target.closest("[data-onlater-more]");
  if (onLaterMore) {
    event.preventDefault();
    const key = onLaterMore.getAttribute("data-onlater-more") || "guide";
    const previousLimit = Math.max(18, Number(state.onLaterShelfLimits[key] || 18));
    const previousRail = onLaterMore.closest(".on-later-shelf-rail");
    const previousScrollLeft = previousRail ? Number(previousRail.scrollLeft || 0) : 0;
    state.onLaterShelfLimits[key] = previousLimit + 18;
    if (state.view === "mytv") updateMyTVSearchSurface();
    else renderOnLaterPage();
    restoreOnLaterShelfPosition(key, previousLimit, previousScrollLeft);
    return;
  }
  const onLaterType = event.target.closest("[data-onlater-type]");
  if (onLaterType) {
    event.preventDefault();
    state.onLaterType = onLaterType.getAttribute("data-onlater-type") || "all";
    renderOnLaterPage();
    return;
  }
  const onLaterTime = event.target.closest("[data-onlater-time]");
  if (onLaterTime) {
    event.preventDefault();
    state.onLaterTime = onLaterTime.getAttribute("data-onlater-time") || "all";
    renderOnLaterPage();
    return;
  }
  const sportsTab = event.target.closest("[data-sports-tab]");
  if (sportsTab) {
    event.preventDefault();
    setSportsTab(sportsTab.getAttribute("data-sports-tab"));
    return;
  }
  const sportsOpenEvent = event.target.closest("[data-sports-open-event]");
  if (sportsOpenEvent) {
    event.preventDefault();
    openSportsEvent(sportsOpenEvent.getAttribute("data-sports-open-event"));
    return;
  }
  const sportsOpenLeague = event.target.closest("[data-sports-open-league]");
  if (sportsOpenLeague) {
    event.preventDefault();
    openSportsLeague(sportsOpenLeague.getAttribute("data-sports-open-league"));
    return;
  }
  const sportsBack = event.target.closest("[data-sports-back]");
  if (sportsBack) {
    event.preventDefault();
    backFromSportsDetail();
    return;
  }
  const sportsLeague = event.target.closest("[data-sports-league]");
  if (sportsLeague) {
    event.preventDefault();
    setSportsLeague(sportsLeague.getAttribute("data-sports-league"));
    return;
  }
  const sportsRefresh = event.target.closest("[data-sports-refresh]");
  if (sportsRefresh) {
    event.preventDefault();
    sportsGameStatsState.fetchedAt = 0;
    loadSports(true);
    renderSportsPage();
    return;
  }
  const recoveryRetry = event.target.closest("[data-recovery-retry]");
  if (recoveryRetry) {
    event.preventDefault();
    const kind = recoveryRetry.getAttribute("data-recovery-retry");
    if (kind === "sports") loadSports(true);
    else loadEvents(true);
    return;
  }
  const recoveryReload = event.target.closest("[data-recovery-reload]");
  if (recoveryReload) {
    event.preventDefault();
    window.location.reload();
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
    toggleSportsTeamFavorite(sportsFavorite.getAttribute("data-sports-favorite-team"), sportsFavorite.getAttribute("data-sports-favorite-enabled") === "true", sportsTeamLabelFromButton(sportsFavorite));
    return;
  }
  const sportsFavoriteLeague = event.target.closest("[data-sports-favorite-league]");
  if (sportsFavoriteLeague) {
    event.preventDefault();
    toggleSportsLeagueFavorite(sportsFavoriteLeague.getAttribute("data-sports-favorite-league"), sportsFavoriteLeague.getAttribute("data-sports-favorite-enabled") === "true");
    return;
  }
  const sportsPreferredChannel = event.target.closest("[data-sports-preferred-channel]");
  if (sportsPreferredChannel) {
    event.preventDefault();
    event.stopPropagation();
    setSportsPreferredChannel(sportsPreferredChannel.getAttribute("data-sports-preferred-league"), sportsPreferredChannel.getAttribute("data-sports-preferred-channel"));
    return;
  }
  const sportsSpoilers = event.target.closest("[data-sports-spoilers]");
  if (sportsSpoilers) {
    event.preventDefault();
    toggleSportsSpoilers(sportsSpoilers.getAttribute("data-sports-spoilers"));
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
  const eventFeature = event.target.closest("[data-event-feature]");
  if (eventFeature) {
    event.preventDefault();
    toggleFeaturedEvent(eventFeature.getAttribute("data-event-feature"));
    return;
  }
  const recordingPlayback = event.target.closest("[data-recording-playback]");
  if (recordingPlayback) {
    event.preventDefault();
    const url = recordingPlayback.getAttribute("data-recording-playback");
    if (url) window.open(url, "_blank", "noopener");
    return;
  }
  const recordingStop = event.target.closest("[data-recording-stop]");
  if (recordingStop) {
    event.preventDefault();
    stopRecording(recordingStop.getAttribute("data-recording-stop"), recordingStop);
    return;
  }
  const recordingDelete = event.target.closest("[data-recording-delete]");
  if (recordingDelete) {
    event.preventDefault();
    deleteRecording(recordingDelete.getAttribute("data-recording-delete"), recordingDelete);
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
  const customGroupAddChannel = event.target.closest("[data-custom-group-add-channel]");
  if (customGroupAddChannel) {
    event.preventDefault();
    addChannelToSelectedGroup(customGroupAddChannel.getAttribute("data-custom-group-add-channel"));
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
    if (action === "retry") loadAdminSourceGroups(true);
    return;
  }
  const adminProfileRefresh = event.target.closest("[data-admin-profile-refresh]");
  if (adminProfileRefresh) {
    event.preventDefault();
    refreshAdminProfiles();
    return;
  }
  const adminStatusRefresh = event.target.closest("[data-admin-status-refresh]");
  if (adminStatusRefresh) {
    event.preventDefault();
    refreshAdminStatus();
    return;
  }
  const adminSettingsAction = event.target.closest("[data-admin-settings-action]");
  if (adminSettingsAction) {
    event.preventDefault();
    const action = adminSettingsAction.getAttribute("data-admin-settings-action");
    if (action === "save") {
      if (state.adminTab === "organization") saveAdminOrganizationSettings();
      else saveAdminCategorySettings();
    }
    if (action === "discard") {
      if (state.adminTab === "organization") discardAdminOrganizationSettings();
      else discardAdminCategorySettings();
    }
    return;
  }
  const adminConnectionBackdrop = event.target.closest("[data-admin-connection-backdrop]");
  if (adminConnectionBackdrop && event.target === adminConnectionBackdrop) {
    event.preventDefault();
    discardAdminConnection();
    return;
  }
  const adminConnectionStep = event.target.closest("[data-admin-connection-step]");
  if (adminConnectionStep) {
    event.preventDefault();
    state.adminConnectionEditorStep = adminConnectionStep.getAttribute("data-admin-connection-step") || "connection";
    renderAdminPage();
    return;
  }
  const adminSourceMode = event.target.closest("[data-admin-source-mode]");
  if (adminSourceMode) {
    event.preventDefault();
    updateAdminConnectionField("sourceMode", { value: adminSourceMode.getAttribute("data-admin-source-mode") });
    return;
  }
  const adminLineupAll = event.target.closest("[data-admin-lineup-all]");
  if (adminLineupAll) {
    event.preventDefault();
    useAllAdminLineup(adminLineupAll.getAttribute("data-admin-lineup-all"));
    return;
  }
  const adminConnectionAction = event.target.closest("[data-admin-connection-action]");
  if (adminConnectionAction) {
    event.preventDefault();
    const action = adminConnectionAction.getAttribute("data-admin-connection-action");
    if (action === "add") openAdminConnectionEditor(true);
    else if (action === "edit") openAdminConnectionEditor(false);
    else if (action === "cancel" || action === "discard") discardAdminConnection();
    else if (action === "refresh") refreshAdminCatalog();
    else if (action === "retry-load") reloadAdminConnection();
    else if (action === "test-saved") {
      openAdminConnectionEditor(false);
      submitAdminConnection("test");
    } else submitAdminConnection(action);
    return;
  }
  const adminTab = event.target.closest("[data-admin-tab]");
  if (adminTab) {
    event.preventDefault();
    setAdminTab(adminTab.getAttribute("data-admin-tab"));
    return;
  }
  const adminOrganizationTab = event.target.closest("[data-admin-organization-tab]");
  if (adminOrganizationTab) {
    event.preventDefault();
    const organizationTab = adminOrganizationTab.getAttribute("data-admin-organization-tab");
    state.adminOrganizationTab = organizationTab === "groups" || organizationTab === "rules" ? organizationTab : "profiles";
    state.adminOrganizationQuery = "";
    renderAdminPage();
    return;
  }
  const virtualCategoryViewTarget = event.target.closest("[data-virtual-category-view]");
  if (virtualCategoryViewTarget) {
    event.preventDefault();
    setVirtualCategoryView(virtualCategoryViewTarget.getAttribute("data-virtual-category-view"));
    return;
  }
  const saveChannel = event.target.closest("[data-save-channel]");
  if (saveChannel) {
    event.preventDefault();
    const id = saveChannel.getAttribute("data-save-channel");
    const saved = setChannelFavorite(id, !favoriteMap()[id]);
    showAppToast(saved ? "Channel saved to My TV." : "Channel removed from My TV favorites.");
    if (state.view === "mytv") updateMyTVSearchSurface();
    else {
      const results = byId("search-page-results");
      if (results) results.innerHTML = renderSearchPageResults();
    }
    return;
  }
  const favoriteMove = event.target.closest("[data-favorite-move]");
  if (favoriteMove) {
    event.preventDefault();
    moveFavorite(favoriteMove.getAttribute("data-channel-id"), favoriteMove.getAttribute("data-favorite-move"));
    return;
  }
  const favoriteRemove = event.target.closest("[data-favorite-remove]");
  if (favoriteRemove) {
    event.preventDefault();
    setChannelFavorite(favoriteRemove.getAttribute("data-favorite-remove"), false);
    render();
    return;
  }
  const favoritesAction = event.target.closest("[data-favorites-action]");
  if (favoritesAction) {
    event.preventDefault();
    setView(favoritesAction.getAttribute("data-favorites-action") || "guide");
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
  const playerGuideMultiview = event.target.closest("[data-player-guide-multiview]");
  if (playerGuideMultiview) {
    event.preventDefault();
    event.stopPropagation();
    const channel = channelByID(playerGuideMultiview.getAttribute("data-player-guide-multiview"));
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
  const channelsGroup = event.target.closest("[data-channels-group]");
  if (channelsGroup) {
    event.preventDefault();
    selectChannelsBrowserGroup(channelsGroup.getAttribute("data-channels-group"));
    return;
  }
  const channelsChannel = event.target.closest("[data-channels-channel]");
  if (channelsChannel) {
    event.preventDefault();
    selectChannelsBrowserChannel(channelsChannel.getAttribute("data-channels-channel"));
    return;
  }
  const allChannelsGuide = event.target.closest("[data-all-channels-guide]");
  if (allChannelsGuide) {
    event.preventDefault();
    setView("guide");
    return;
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
  rememberGuideFocus(event.target);
  const target = overflowTooltipTarget(event);
  if (target) showOverflowTooltip(target, event);
  if (state.view === "player") wakePlayerChrome();
});
document.addEventListener("focusout", function(event) {
  const target = overflowTooltipTarget(event);
  if (target) hideOverflowTooltip();
});
document.addEventListener("keydown", function(event) {
  if (state.folderGroupPickerOpen && event.key === "Escape") {
    event.preventDefault();
    state.folderGroupPickerOpen = false;
    const picker = document.querySelector(".folder-category-picker");
    if (picker) picker.classList.remove("open");
    const trigger = picker ? picker.querySelector("[data-folder-group-toggle]") : null;
    if (trigger) {
      trigger.setAttribute("aria-expanded", "false");
      trigger.focus();
    }
    return;
  }
  if (state.adminConnectionEditorOpen && event.key === "Escape") {
    event.preventDefault();
    discardAdminConnection();
    return;
  }
  if (trapProgramModalFocus(event)) return;
  if (handleGuideKeyboard(event)) return;
  if (state.programDetails && event.key === "Escape") {
    event.preventDefault();
    closeProgramDetails();
    return;
  }
  if (event.target && event.target.id === "search-page-input" && event.key === "Enter") {
    event.preventDefault();
    clearSearchResultsTimer();
    updateSearchPageResults();
    rememberSearch(state.searchQuery);
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
  if (event.key === "Escape" && state.playerSportsOpen) {
    event.preventDefault();
    togglePlayerSports(false);
    return;
  }
  if (event.key === "ArrowDown" && sportsFirstPlayerActive() && !state.playerSportsOpen) {
    event.preventDefault();
    togglePlayerSports(true);
    return;
  }
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
  if (event.target && event.target.hasAttribute("data-folder-group")) {
    state.folderGroupCategoryID = event.target.value || "";
    renderLivePage();
    return;
  }
  if (event.target && event.target.hasAttribute("data-saved-lineup-group")) {
    state.savedLineupGroupCategoryID = event.target.value || "";
    renderLivePage();
    return;
  }
  const savedLineupField = event.target.getAttribute("data-saved-lineup-field");
  if (savedLineupField && state.savedLineupEditor) {
    state.savedLineupEditor[savedLineupField] = event.target.type === "checkbox" ? !!event.target.checked : event.target.value;
    return;
  }
  const profileID = event.target.getAttribute("data-profile-selection-id");
  if (profileID) {
    updateSelectedProfile(profileID, !!event.target.checked);
    return;
  }
  const adminField = event.target.getAttribute("data-admin-category-field");
  if (adminField) {
    updateCategoryParsingField(adminField, event.target);
    return;
  }
  const organizationLayer = event.target.getAttribute("data-admin-organization-layer");
  if (organizationLayer) {
    updateOrganizationLayer(organizationLayer, event.target);
    return;
  }
  const adminECMField = event.target.getAttribute("data-admin-ecm-field");
  if (adminECMField) {
    updateAdminECMField(adminECMField, event.target);
    return;
  }
  const adminRecordingField = event.target.getAttribute("data-admin-recording-field");
  if (adminRecordingField) {
    updateAdminRecordingField(adminRecordingField, event.target);
    return;
  }
  const adminIdentityField = event.target.getAttribute("data-admin-identity-field");
  if (adminIdentityField) {
    updateAdminIdentityField(adminIdentityField, event.target);
    return;
  }
  const adminPlayerField = event.target.getAttribute("data-admin-player-field");
  if (adminPlayerField) {
    updateAdminPlayerField(adminPlayerField, event.target);
    return;
  }
  const adminSportsField = event.target.getAttribute("data-admin-sports-field");
  if (adminSportsField) {
    updateAdminSportsField(adminSportsField, event.target);
    return;
  }
  const adminTimeShiftField = event.target.getAttribute("data-admin-timeshift-field");
  if (adminTimeShiftField) {
    updateAdminTimeShiftField(adminTimeShiftField, event.target);
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
  render();
});
document.addEventListener("input", function(event) {
  const adminPlayerField = event.target.getAttribute("data-admin-player-field");
  if (adminPlayerField === "hls-buffer") {
    updateAdminPlayerField(adminPlayerField, event.target, true);
    return;
  }
  const adminTimeShiftField = event.target.getAttribute("data-admin-timeshift-field");
  if (adminTimeShiftField && event.target.type === "number") {
    updateAdminTimeShiftField(adminTimeShiftField, event.target, true);
    return;
  }
  if (event.target.hasAttribute("data-admin-organization-filter")) {
    state.adminOrganizationQuery = event.target.value || "";
    const query = lower(state.adminOrganizationQuery).trim();
    let visibleCount = 0;
    document.querySelectorAll("[data-admin-organization-search]").forEach(function(card) {
      card.hidden = !!query && String(card.getAttribute("data-admin-organization-search") || "").indexOf(query) < 0;
      if (!card.hidden) visibleCount++;
    });
    const empty = document.querySelector(".organization-filter-empty");
    if (empty) empty.hidden = visibleCount > 0;
    return;
  }
  const lineupFilter = event.target.getAttribute("data-admin-lineup-filter");
  if (lineupFilter) {
    const query = lower(event.target.value || "").trim();
    document.querySelectorAll("[data-admin-lineup-options=\"" + lineupFilter + "\"] [data-admin-lineup-search-value]").forEach(function(option) {
      option.hidden = !!query && String(option.getAttribute("data-admin-lineup-search-value") || "").indexOf(query) < 0;
    });
    return;
  }
  const lineupKind = event.target.getAttribute("data-admin-lineup-kind");
  const lineupID = event.target.getAttribute("data-admin-lineup-id");
  if (lineupKind && lineupID) {
    updateAdminLineupSelection(lineupKind, lineupID, !!event.target.checked);
    return;
  }
  if (event.target && event.target.id === "guide-category-search") {
    state.guideCategoryQuery = event.target.value || "";
    const options = document.querySelector(".guide-category-options");
    if (options) options.innerHTML = guideCategoryOptionsHTML(guideFilterCategories());
    return;
  }
  if (event.target && event.target.id === "folder-category-search") {
    const query = lower(event.target.value || "").trim();
    document.querySelectorAll(".folder-category-option").forEach(function(option) {
      option.hidden = !!query && String(option.getAttribute("data-folder-group-search") || "").indexOf(query) === -1;
    });
    return;
  }
  const savedLineupField = event.target.getAttribute("data-saved-lineup-field");
  if (savedLineupField && state.savedLineupEditor) {
    state.savedLineupEditor[savedLineupField] = event.target.type === "checkbox" ? !!event.target.checked : event.target.value;
    return;
  }
  const adminConnectionField = event.target.getAttribute("data-admin-connection-field");
  if (adminConnectionField) {
    updateAdminConnectionField(adminConnectionField, event.target);
    return;
  }
  if (event.target && event.target.id === "profile-settings-filter") {
    state.profileSettingsQuery = event.target.value || "";
    renderProfileSettings();
    const input = byId("profile-settings-filter");
    if (input) {
      input.focus();
      input.setSelectionRange(input.value.length, input.value.length);
    }
    return;
  }
  if (event.target && event.target.id === "player-volume-slider") {
    state.volume = Number(event.target.value || 0) / 100;
    applyVolumeToVideo();
    syncMultiviewAudio();
  }
  if (event.target && event.target.id === "player-timeshift-range") {
    const video = byId("player");
    if (video && video.seekable && video.seekable.length) {
      video.currentTime = video.seekable.start(0) + Number(event.target.value || 0);
      if (state.timeShiftSession) state.timeShiftSession.rewound = true;
      updateTimeShiftUI();
    }
    return;
  }
  if (event.target && event.target.id === "search-page-input") {
    state.searchQuery = event.target.value || "";
    state.searchAiringChannel = "";
    scheduleSearchResultsUpdate();
    return;
  }
  if (event.target && event.target.id === "my-tv-search") {
    state.myTVQuery = event.target.value || "";
    updateMyTVSearchSurface();
    ensureMyTVTeamCatalog(state.myTVQuery);
    return;
  }
  if (event.target && event.target.id === "player-guide-search") {
    state.playerGuideQuery = event.target.value || "";
    renderPlayerGuidePanel();
    const input = byId("player-guide-search");
    if (input) {
      input.focus();
      input.setSelectionRange(input.value.length, input.value.length);
    }
    return;
  }
  if (event.target && event.target.id === "folder-filter") {
    state.folderQuery = event.target.value || "";
    renderLivePage();
    const input = byId("folder-filter");
    if (input) {
      input.focus();
      input.setSelectionRange(input.value.length, input.value.length);
    }
    return;
  }
  if (event.target && event.target.id === "multiview-search") {
    state.multiviewQuery = event.target.value || "";
    const picker = byId("multiview-picker");
    if (picker) picker.outerHTML = renderMultiviewPicker();
    const input = byId("multiview-search");
    if (input) {
      input.focus();
      input.setSelectionRange(input.value.length, input.value.length);
    }
    return;
  }
  const adminFeaturedEventID = event.target.getAttribute("data-admin-featured-event-id");
  if (adminFeaturedEventID) {
    const settings = state.adminCategorySettings || defaultAdminCategorySettings();
    const selected = {};
    items(settings.featuredEventIds).forEach(function(id) { selected[String(id || "")] = true; });
    if (event.target.checked) selected[adminFeaturedEventID] = true;
    else delete selected[adminFeaturedEventID];
    settings.featuredEventIds = Object.keys(selected);
    state.adminCategorySettings = settings;
    markAdminSettingsDraft();
    renderAdminTopbarActions();
    return;
  }
  const adminCategoryField = event.target.getAttribute("data-admin-category-field");
  if (adminCategoryField) {
    const settings = state.adminCategorySettings || defaultAdminCategorySettings();
    settings[adminCategoryField] = event.target.type === "checkbox" ? !!event.target.checked : event.target.value;
    state.adminCategorySettings = settings;
    normalizeAdminCategorySettings();
    const preview = byId("organization-preview");
    if (preview) preview.outerHTML = renderOrganizationPreview(adminSettings());
    return;
  }
  const adminEventKeywordIndex = event.target.getAttribute("data-admin-event-keyword-index");
  if (adminEventKeywordIndex !== null) {
    updateAdminEventKeywords(Number(adminEventKeywordIndex), event.target.getAttribute("data-admin-event-keyword-field") || "keywords", event.target.value || "");
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
document.addEventListener("keydown", function(event) {
  if (event.key === "Escape" && state.savedLineupEditor) {
    event.preventDefault();
    closeSavedLineupEditor();
    return;
  }
  if (event.key === "Escape") setSettingsMenuOpen(false);
});
document.querySelectorAll("[data-view]").forEach(function(button) {
  button.onclick = function() {
    setSettingsMenuOpen(false);
    setView(button.dataset.view);
  };
});
const appSearchButton = byId("app-search-button");
if (appSearchButton) appSearchButton.onclick = function() { setView("search"); };
const globalSearch = byId("global-search");
if (globalSearch) {
  globalSearch.oninput = function(event) { state.query = event.target.value; updateLiveSearchFilter(); };
  globalSearch.onkeydown = function(event) {
    if (event.key !== "Enter") return;
    state.searchQuery = event.target.value || "";
    state.searchAiringChannel = "";
    rememberSearch(state.searchQuery);
    setView("search");
  };
}
window.initialAppRouteSnapshot = readAppRouteHash();
if (window.initialAppRouteSnapshot.view !== "player") applyAppRouteSnapshot(window.initialAppRouteSnapshot);
window.addEventListener("popstate", function(event) {
  const snapshot = event.state && event.state[appHistoryStateKey] ? event.state[appHistoryStateKey] : readAppRouteHash();
  restoreAppRoute(snapshot);
});
window.addEventListener("resize", function() {
  if (state.view === "guide") scheduleGuideWindowRender();
  if (state.view === "player" && state.playerSportsOpen) syncPlayerSportsDrawerLayout();
});
function stopWatchOnUnload(sessionID) {
  if (!sessionID) return;
  // keepalive fetch (not sendBeacon) so the Silo auth/profile headers from
  // coreRequestOptions travel with the request.
  try {
    fetch(route("/dispatcharr/api/watch/stop"), coreRequestOptions({
      method: "POST",
      keepalive: true,
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ sessionId: sessionID, reason: "page_unload" })
    })).catch(function() {});
  } catch (_) {}
}
window.addEventListener("pagehide", function() {
  if (state.currentSession) stopWatchOnUnload(state.currentSession.id);
  items(state.multiviewTiles).forEach(function(tile) {
    if (tile.session) stopWatchOnUnload(tile.session.id);
  });
});
