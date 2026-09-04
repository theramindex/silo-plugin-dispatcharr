// Split from app.js so Guide and player views can change independently.
function playerGuideProgramLines(channel) {
  const current = liveProgram(channel);
  const next = nextProgram(channel);
  const currentTitle = current && current.title ? current.title : "";
  const nextTitle = next && next.title ? next.title : "";
  if (currentTitle) {
    return {
      primary: (timeLabel(current.startUnix) || "Live") + " - " + currentTitle,
      secondary: nextTitle ? "Next " + (timeLabel(next.startUnix) || "Soon") + " - " + nextTitle : ""
    };
  }
  if (nextTitle) {
    return { primary: "Next " + (timeLabel(next.startUnix) || "Soon") + " - " + nextTitle, secondary: "" };
  }
  return { primary: guideUnavailableLabel(), secondary: "" };
}

function playerGuideMatches(channel, query) {
  query = lower(query).trim();
  if (!query) return true;
  const current = liveProgram(channel) || {};
  const next = nextProgram(channel) || {};
  const lines = playerGuideProgramLines(channel);
  const haystack = [
    channel && channel.name,
    channel && channel.categoryName,
    sourceCategoryLabel(channel || {}),
    current.title,
    current.description,
    next.title,
    next.description,
    lines.primary,
    lines.secondary
  ].map(lower).join(" ");
  return haystack.indexOf(query) !== -1;
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
  const playbackShellClass = (replayMode ? "playback-shell is-replay" : "playback-shell") + (sportsFirstPlayerActive() ? " sports-enabled" : "");
  const videoAttributes = " autoplay playsinline";
  const modeTag = replayMode ? "Replay" : "AV";
  const liveProgramWindow = !replayMode ? "<div class=\"player-live-window\"><span>Started " + escapeHTML(start) + "</span><strong><span class=\"live-dot\"></span>Live</strong><span>Ends " + escapeHTML(end) + "</span></div>" : "";
  const timeShiftControls = "<div id=\"player-timeshift-controls\" class=\"player-timeshift-controls hidden\"><button class=\"player-icon\" data-player-action=\"rewind-30\" aria-label=\"Rewind 30 seconds\">" + icon("rewind") + "</button><button class=\"player-icon\" data-player-action=\"play-toggle\" aria-label=\"Play or pause\">" + icon("play") + "</button><button class=\"player-icon\" data-player-action=\"forward-30\" aria-label=\"Forward 30 seconds\">" + icon("forward") + "</button><input id=\"player-timeshift-range\" type=\"range\" min=\"0\" max=\"1\" step=\"0.25\" value=\"1\" aria-label=\"Live Rewind position\"><button class=\"player-live-button\" data-player-action=\"go-live\"><span class=\"live-dot\"></span><span id=\"player-timeshift-label\">LIVE</span></button></div>";
  const topActions = "<div class=\"player-top-actions\">"
    + "<div class=\"player-audio\"><button id=\"player-audio-button\" class=\"player-chip\" data-player-action=\"audio-menu\" aria-haspopup=\"true\" aria-expanded=\"false\"><span>Audio</span>" + icon("chevron-down") + "</button><div id=\"player-audio-menu\" class=\"player-menu\" role=\"menu\"></div></div>"
    + "<div class=\"player-volume\"><button id=\"player-volume-button\" class=\"player-icon\" data-player-action=\"volume-menu\" aria-label=\"Volume\" aria-haspopup=\"true\" aria-expanded=\"false\">" + icon("speaker") + "</button><div id=\"player-volume-popover\" class=\"volume-popover\"><span>VOL</span><input id=\"player-volume-slider\" type=\"range\" min=\"0\" max=\"100\" step=\"1\" value=\"" + Math.round(state.volume * 100) + "\" aria-label=\"Volume\"><span id=\"player-volume-value\" class=\"volume-value\"></span></div></div>"
    + "<button class=\"player-icon\" data-player-action=\"cast\" aria-label=\"AirPlay or Cast\">" + icon("airplay") + "</button>"
    + "<button id=\"player-guide-button\" class=\"player-icon player-guide-button\" data-player-action=\"guide\" aria-label=\"Guide\" aria-haspopup=\"true\" aria-expanded=\"false\">" + icon("guide") + "</button>"
    + "<div class=\"player-more\"><button id=\"player-more-button\" class=\"player-icon\" data-player-action=\"more\" aria-label=\"More\" aria-haspopup=\"true\" aria-expanded=\"false\">" + icon("ellipsis") + "</button><div id=\"player-more-menu\" class=\"player-more-menu\"></div></div></div>";
  const bottomActions = "<div class=\"player-bottom-actions\">" + playerFavoriteButtonHTML(channel)
    + "<button class=\"player-icon\" data-player-action=\"add-multiview\" aria-label=\"Add current channel to multiview\">" + icon("multiview") + "</button>"
    + "<button id=\"player-subtitles-button\" class=\"player-icon\" data-player-action=\"subtitles\" aria-label=\"Subtitles\" aria-pressed=\"false\">" + icon("captions") + "</button>"
    + "<button id=\"player-language-button\" class=\"player-icon\" data-player-action=\"language-menu\" aria-label=\"Audio language\" aria-haspopup=\"true\" aria-expanded=\"false\">" + icon("language") + "</button></div>";
  byId("view").innerHTML = "<section class=\"" + playbackShellClass + "\"><div class=\"playback-stage\">"
    + "<video id=\"player\" class=\"playback-video\"" + videoAttributes + "></video><div class=\"playback-scrim\"></div>"
    + "<button id=\"player-center-button\" class=\"player-center-button hidden\" data-player-action=\"play-toggle\" aria-label=\"Play\">" + icon("play") + "</button>"
    + "<div class=\"player-top\"><button class=\"player-exit player-icon\" data-player-action=\"back\" aria-label=\"Back to Live TV browse\">" + icon("arrow-left") + "</button>" + topActions + "</div>"
    + "<div id=\"player-toast\" class=\"player-toast\" role=\"status\"></div><div id=\"player-guide-panel\" class=\"player-guide-panel\"></div>"
    + "<div id=\"player-sports-status\" class=\"sr-only\" role=\"status\"></div><div id=\"player-sports-drawer\" class=\"player-sports-drawer\"></div>"
    + "<div class=\"player-bottom\"><div class=\"player-bottom-row\"><div class=\"player-meta\">" + playerLogoHTML(channel)
    + "<div class=\"player-kicker\">" + escapeHTML(channelName) + "</div><h2 class=\"player-title\">" + escapeHTML(title) + "</h2>"
    + "<p class=\"player-description\" data-overflow-description=\"true\">" + escapeHTML(description) + "</p><div class=\"player-tags\"><span class=\"player-tag\">" + escapeHTML(categoryNameText) + "</span><span id=\"player-mode-tag\" class=\"player-tag\">" + escapeHTML(modeTag) + "</span></div></div>"
    + bottomActions + "</div>" + timeShiftControls + liveProgramWindow + "</div></div></section>";
  updateAudioMenu();
  updateVolumeMenu();
  renderPlayerGuidePanel();
  renderPlayerSportsDrawer();
  if (sportsFirstPlayerActive()) {
    loadSports(false).then(renderPlayerSportsDrawer);
    startPlayerSportsRefresh();
  }
  renderPlayerMoreMenu();
  updateFullscreenButton();
  wakePlayerChrome(1800);
}

function hasOpenPlayerOverlay() {
  return state.audioMenuOpen || state.volumeMenuOpen || state.moreMenuOpen || state.playerGuideOpen;
}

function playerChromeHasFocus() {
  const active = document.activeElement;
  return !!(active && active.closest && active.closest(".player-top, .player-bottom, .player-guide-panel"));
}

function updatePlayerChrome() {
  const shell = document.querySelector(".playback-shell");
  if (!shell) return;
  shell.classList.toggle("is-idle", state.playerChromeIdle && !hasOpenPlayerOverlay() && !playerChromeHasFocus());
}

function wakePlayerChrome(delay) {
  if (state.view !== "player") return;
  state.playerChromeIdle = false;
  updatePlayerChrome();
  if (state.playerChromeTimer) clearTimeout(state.playerChromeTimer);
  state.playerChromeTimer = setTimeout(function() {
    if (playerChromeHasFocus()) {
      wakePlayerChrome();
      return;
    }
    state.playerChromeIdle = true;
    updatePlayerChrome();
  }, delay || 2400);
}

function renderPlayerGuidePanel() {
  const panel = byId("player-guide-panel");
  const button = byId("player-guide-button");
  if (!panel) return;
  const query = state.playerGuideQuery || "";
  const channels = visibleChannels(true).filter(function(channel) { return playerGuideMatches(channel, query); }).slice(0, 60);
  panel.classList.toggle("open", state.playerGuideOpen);
  if (button) {
    button.classList.toggle("active", state.playerGuideOpen);
    button.setAttribute("aria-expanded", state.playerGuideOpen ? "true" : "false");
  }
  updatePlayerChrome();
  if (!state.playerGuideOpen) return;
  panel.innerHTML = "<div class=\"player-guide-head\"><div class=\"player-guide-title\"><strong>Channel Guide</strong><span>" + escapeHTML(categoryName(state.category) || "Live TV") + "</span></div><button class=\"player-icon\" data-player-action=\"guide-close\" aria-label=\"Close guide\">" + icon("x") + "</button><label class=\"player-guide-search\"><span>" + icon("search") + "</span><input id=\"player-guide-search\" value=\"" + escapeHTML(query) + "\" placeholder=\"Search channels or programs\" autocomplete=\"off\" aria-label=\"Search channel guide\"></label></div><div class=\"player-guide-list\">" + (channels.length ? channels.map(function(channel) {
    const lines = playerGuideProgramLines(channel);
    return "<div class=\"player-guide-row" + (state.currentChannel && state.currentChannel.id === channel.id ? " active" : "") + "\"><button class=\"player-guide-select\" type=\"button\" data-channel=\"" + escapeHTML(channel.id) + "\">" + logoHTML(channel) + "<span><strong>" + escapeHTML(channel.name || "Untitled") + "</strong><small>" + escapeHTML(lines.primary) + "</small>" + (lines.secondary ? "<small>" + escapeHTML(lines.secondary) + "</small>" : "") + "</span></button><button class=\"player-guide-add\" type=\"button\" data-player-guide-multiview=\"" + escapeHTML(channel.id) + "\" aria-label=\"Add " + escapeHTML(channel.name || "channel") + " to multiview\">" + icon("multiview") + "</button></div>";
  }).join("") : "<div class=\"player-guide-empty\">No matching channels.</div>") + "</div>";
}

function currentStreamURL() {
  return state.currentChannel ? route("/dispatcharr/stream?channel_id=" + encodeURIComponent(state.currentChannel.id)) : "";
}

function browserStreamURL(channel) {
  const query = "channel_id=" + encodeURIComponent(channel.id);
  return route("/dispatcharr/stream?" + query + (sourceMode() === "xtream" ? "&output_profile=2" : ""));
}

function stopTimeShiftSession() {
  state.timeShiftAttempt += 1;
  const session = state.timeShiftSession;
  state.timeShiftSession = null;
  if (state.timeShiftHeartbeat) {
    clearInterval(state.timeShiftHeartbeat);
    state.timeShiftHeartbeat = null;
  }
  if (state.timeShiftTimelineTimer) {
    clearInterval(state.timeShiftTimelineTimer);
    state.timeShiftTimelineTimer = null;
  }
  if (session && session.leaseId) postJSON("/dispatcharr/api/timeshift/stop", { leaseId: session.leaseId }).catch(function() {});
  updateTimeShiftUI();
}

async function prepareTimeShift(channel) {
  if (!liveRewindEnabled() || !channel || channel.streamFormat === "hls") return null;
  const attemptID = state.timeShiftAttempt;
  const session = await postJSON("/dispatcharr/api/timeshift/start", { channelId: channel.id });
  if (attemptID !== state.timeShiftAttempt || state.view !== "player" || !state.currentChannel || state.currentChannel.id !== channel.id) {
    postJSON("/dispatcharr/api/timeshift/stop", { leaseId: session.leaseId }).catch(function() {});
    const stale = new Error("rewind attempt superseded");
    stale.superseded = true;
    throw stale;
  }
  state.timeShiftSession = session;
  state.timeShiftHeartbeat = setInterval(function() {
    if (state.timeShiftSession && state.timeShiftSession.leaseId) postJSON("/dispatcharr/api/timeshift/heartbeat", { leaseId: state.timeShiftSession.leaseId }).catch(function() {});
  }, 30000);
  for (let pollAttempt = 0; pollAttempt < 30; pollAttempt++) {
    await new Promise(function(resolve) { setTimeout(resolve, 500); });
    if (attemptID !== state.timeShiftAttempt || state.view !== "player" || !state.timeShiftSession || state.timeShiftSession.leaseId !== session.leaseId) {
      const stale = new Error("rewind attempt superseded");
      stale.superseded = true;
      throw stale;
    }
    const status = await getJSON("/dispatcharr/api/timeshift/status?lease_id=" + encodeURIComponent(session.leaseId));
    if (status.state === "failed") throw new Error(status.error || "rewind buffer failed");
    if (status.segmentCount >= 2) {
      session.status = status;
      session.ready = true;
      return route(session.manifestPath);
    }
  }
  throw new Error("rewind buffer startup timed out");
}

function fallbackFromTimeShift(channel, message) {
  if (state.view !== "player" || !channel || !state.currentChannel || channel.id !== state.currentChannel.id) return;
  stopTimeShiftSession();
  setVideoSource(browserStreamURL(channel), { rewindable: isRewindableChannel(channel), format: channel.streamFormat, hlsBufferSeconds: channel.hlsBufferSeconds });
  if (message) showPlayerToast(message);
}

function updateTimeShiftUI() {
  const controls = byId("player-timeshift-controls");
  const range = byId("player-timeshift-range");
  const label = byId("player-timeshift-label");
  const tag = byId("player-mode-tag");
  const video = byId("player");
  const active = !!(state.timeShiftSession && state.timeShiftSession.ready && video && video.seekable && video.seekable.length);
  if (controls) controls.classList.toggle("hidden", !active);
  if (tag) tag.textContent = active ? "Live Rewind" : (isRewindableChannel(state.currentChannel) ? "Replay" : "AV");
  if (!active) return;
  const start = video.seekable.start(0);
  const end = video.seekable.end(video.seekable.length - 1);
  const position = Math.max(start, Math.min(end, video.currentTime || end));
  const windowSeconds = Math.max(0, end - start);
  const behind = Math.max(0, end - position);
  if (range) {
    range.max = String(windowSeconds);
    range.value = String(Math.max(0, position - start));
  }
  if (label) label.textContent = behind < 3 ? "LIVE" : "-" + Math.floor(behind / 60) + ":" + String(Math.floor(behind % 60)).padStart(2, "0");
}

function applyAspectMode() {
  const video = byId("player");
  if (video) video.style.objectFit = state.aspectMode === "fit" ? "contain" : "cover";
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
  const sportsControl = sportsFirstPlayerActive()
    ? "<button data-player-action=\"sports\">" + menuIcon("trophy") + "<span>Sports center<small>Scores, matchups, and related channels</small></span></button>"
    : "";
  menu.innerHTML = "<div class=\"player-more-kicker\">Video settings & controls</div>"
    + "<button data-player-action=\"aspect\">" + menuIcon("aspect") + "<span>Aspect ratio<small>" + (state.aspectMode === "fit" ? "Fit to screen" : "Fill screen") + "</small></span></button>"
    + "<button data-player-action=\"fullscreen\">" + menuIcon(document.fullscreenElement ? "fullscreen-exit" : "fullscreen") + "<span>Fullscreen<small>" + (document.fullscreenElement ? "Exit player fullscreen" : "Fill the display") + "</small></span></button>"
    + "<button data-player-action=\"pip\">" + menuIcon("pip") + "<span>Picture in Picture<small>Keep watching over other windows</small></span></button>"
    + "<button data-player-action=\"guide\">" + menuIcon("guide") + "<span>Channel guide<small>Browse channels without leaving playback</small></span></button>"
    + sportsControl
    + "<button data-player-action=\"add-multiview\">" + menuIcon("multiview") + "<span>Add to multiview<small>Tile this channel with up to three more</small></span></button>"
    + "<button data-player-action=\"search-channel\">" + menuIcon("search") + "<span>Search channel<small>Jump to the channel list search</small></span></button>"
    + (recent.length ? "<div class=\"player-more-separator\"></div><div class=\"player-more-kicker\">Channels history</div>" + recent.map(function(channel) { return "<button data-channel=\"" + escapeHTML(channel.id) + "\">" + logoHTML(channel) + "<span>" + escapeHTML(channel.name || "Untitled") + "<small>" + escapeHTML(channel.categoryName || "Live TV") + "</small></span></button>"; }).join("") : "")
    + "<div class=\"player-more-separator\"></div><div class=\"player-more-kicker\">Video & audio casting</div>"
    + "<button data-player-action=\"cast\">" + menuIcon("airplay") + "<span>AirPlay or Cast<small>Use browser playback target picker</small></span></button>"
    + "<button data-player-action=\"copy-stream\">" + menuIcon("copy") + "<span>Copy stream URL<small>For an external player</small></span></button>"
    + "<button data-player-action=\"open-stream\">" + menuIcon("external") + "<span>Use external video player<small>Open the stream route in a new tab</small></span></button>";
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

function showPlayerToast(message) {
  const toast = byId("player-toast");
  if (!toast) return;
  toast.textContent = message;
  toast.classList.add("show");
  clearTimeout(state.toastTimer);
  state.toastTimer = setTimeout(function() { toast.classList.remove("show"); }, 2400);
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

function setVideoSource(url, options) {
  const video = byId("player");
  if (!video) return;
  const rewindable = !!(options && options.rewindable);
  video.controls = false;
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
  const attachment = attachVideoSource(video, url, { rewindable: rewindable, managedTimeShift: !!(options && options.managedTimeShift), format: options && options.format, hlsBufferSeconds: options && options.hlsBufferSeconds, onFatal: options && options.onFatal ? options.onFatal : handlePlaybackFatalError });
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

async function playChannel(channel, options) {
  options = options || {};
  const keepSportsPlayer = state.view === "player" && state.playerSportsMode;
  const useSportsPlayer = sportsFirstPlayerEnabled() && (state.view === "sports" || keepSportsPlayer);
  if (state.view !== "player") {
    const main = document.querySelector(".main");
    const guideScroll = byId("guide-scroll");
    state.playerReturnContext = {
      view: state.view,
      category: state.category,
      query: state.query,
      folderQuery: state.folderQuery,
      scrollY: window.scrollY,
      mainScrollTop: main ? main.scrollTop : 0,
      guideScrollTop: guideScroll ? guideScroll.scrollTop : 0,
      guideScrollLeft: guideScroll ? guideScroll.scrollLeft : 0
    };
  }
  stopTimeShiftSession();
  const timeShiftAttempt = state.timeShiftAttempt;
  state.currentChannel = channel;
  state.playerSportsMode = useSportsPlayer;
  state.playerSportsOpen = useSportsPlayer;
  state.playerSportsMoreOpen = false;
  state.view = "player";
  render();
  if (options.historyMode !== "none") commitAppRoute("push");
  try {
    await ensurePlayerLibraries(liveRewindEnabled() && channel.streamFormat !== "hls" ? "" : channel.streamFormat);
  } catch (_) {
    showPlayerToast("Playback components could not be loaded.");
    return;
  }
  if (timeShiftAttempt !== state.timeShiftAttempt || !state.currentChannel || state.currentChannel.id !== channel.id) return;
  startWatch(channel);
  if (liveRewindEnabled() && channel.streamFormat !== "hls") {
    showPlayerToast("Preparing Live Rewind...");
    try {
      const manifestURL = await prepareTimeShift(channel);
      setVideoSource(manifestURL, { rewindable: true, managedTimeShift: true, format: "hls", onFatal: function() { fallbackFromTimeShift(channel, "Live Rewind stopped. Continuing live."); } });
      state.timeShiftTimelineTimer = setInterval(updateTimeShiftUI, 1000);
      const video = byId("player");
      if (video) {
        video.addEventListener("timeupdate", updateTimeShiftUI);
        video.addEventListener("progress", updateTimeShiftUI);
      }
      updateTimeShiftUI();
      showPlayerToast("Live Rewind ready.");
    } catch (error) {
      if (timeShiftAttempt === state.timeShiftAttempt && !(error && error.superseded)) fallbackFromTimeShift(channel, "Live Rewind unavailable. Playing live.");
    }
  } else {
    setVideoSource(browserStreamURL(channel), { rewindable: isRewindableChannel(channel), format: channel.streamFormat, hlsBufferSeconds: channel.hlsBufferSeconds });
  }
  if (timeShiftAttempt !== state.timeShiftAttempt || !state.currentChannel || state.currentChannel.id !== channel.id) return;
  const guide = await getJSON("/dispatcharr/api/guide?channel_id=" + encodeURIComponent(channel.id)).catch(function() { return { programs: [] }; });
  if (!state.currentChannel || state.currentChannel.id !== channel.id) return;
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

startGuideAutoRefresh();
const initialAppLoad = isAdminRoute ? loadAdminApp() : loadApp();
initialAppLoad.then(function() {
  const snapshot = window.initialAppRouteSnapshot || readAppRouteHash();
  if (snapshot.view === "player" && snapshot.channelID) restoreAppRoute(snapshot);
  else commitAppRoute("replace");
}).catch(handleAppBootFailure);
