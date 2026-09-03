// Split from app.js so Guide and player views can change independently.
function renderGuideChannelButton(channel) {
  const channelName = channel.name || "Untitled";
  return "<button class=\"epg-channel\" data-channel=\"" + escapeHTML(channel.id) + "\" data-channel-name=\"" + escapeHTML(channelName) + "\" aria-label=\"" + escapeHTML(channelName) + "\" title=\"" + escapeHTML(channelName) + "\">" + logoHTML(channel) + "<span class=\"epg-channel-title\">" + escapeHTML(channelName) + "</span></button>";
}

function refreshGuideRowsForQuery() {
  if (state.view !== "guide" || !byId("epg")) return false;
  resetGuideRows();
  renderEPG();
  return true;
}

function renderGuidePage() {
  const categories = guideFilterCategories();
  const slots = guideSlots();
  state.guideLastSlotStart = guideSlotStart();
  const saveAction = renderSaveChannelListButton(state.category);
  byId("view").innerHTML = '<div class="guide-page"><div class="guide-commandbar"><div class="guide-commandbar-title"><strong>TV Guide</strong>' + guideFreshnessHTML() + "</div>" + renderGuideCategoryPicker(categories) + "<label class=\"guide-search-field\"><span>" + icon("search") + "</span><input id=\"guide-search\" class=\"search\" placeholder=\"Search programs or channels\" value=\"" + escapeHTML(state.query) + "\" aria-label=\"Search programs or channels\"></label>" + (saveAction ? "<div class=\"guide-commandbar-actions\">" + saveAction + "</div>" : "") + "</div><div id=\"guide-scroll\" class=\"guide-scroll\"><div class=\"guide-timeline\" style=\"" + guideTimelineStyle(slots) + "\"><div class=\"time-head\"><span>Today</span>" + slots.map(function(slot) { return "<span>" + escapeHTML(timeLabel(slot)) + "</span>"; }).join("") + '</div><div id="epg" class="guide-window-spacer" style="height:0px"><div class="guide-window" style="transform:translateY(0px)"></div></div></div></div></div>';
  byId("guide-search").oninput = function(event) { state.query = event.target.value; resetGuideRows(); renderEPG(); };
  const guideScroll = byId("guide-scroll");
  if (guideScroll) guideScroll.onscroll = scheduleGuideWindowRender;
  resetGuideRows();
  maybeWarmGuideForChannels(state.guideChannels.slice(0, guideWindowOverscan() * 2), "guide:" + (state.category || "all"));
  renderEPG();
}

function guideCategoryOptionHTML(category) {
  const selected = String((category && category.id) || "") === String(state.category || "");
  const fullName = String((category && (category.name || category.id)) || allGroupLabel());
  const parts = fullName.split(" / ").filter(Boolean);
  const label = parts.length ? parts[parts.length - 1] : fullName;
  const parent = parts.length > 1 ? parts.slice(0, -1).join(" / ") : "";
  return "<button type=\"button\" class=\"guide-category-option" + (selected ? " selected" : "") + "\" data-guide-category=\"" + escapeHTML((category && category.id) || "") + "\" role=\"option\" aria-selected=\"" + (selected ? "true" : "false") + "\"><span class=\"guide-category-check\">" + (selected ? icon("check") : "") + "</span><span><strong>" + escapeHTML(label) + "</strong>" + (parent ? "<small>" + escapeHTML(parent) + "</small>" : "") + "</span></button>";
}

function guideCategoryOptionsHTML(categories) {
  const query = lower(state.guideCategoryQuery).trim();
  const filtered = items(categories).filter(function(category) { return !query || lower(category.name || category.id).indexOf(query) !== -1; });
  const all = guideCategoryOptionHTML({ id: "", name: allGroupLabel() });
  return all + (filtered.length ? filtered.map(guideCategoryOptionHTML).join("") : "<div class=\"guide-category-empty\">No matching categories</div>");
}

function renderGuideCategoryPicker(categories) {
  const open = !!state.guideCategoryPickerOpen;
  return "<div class=\"guide-category-picker" + (open ? " open" : "") + "\"><button type=\"button\" class=\"guide-category-trigger\" data-guide-category-toggle=\"true\" aria-haspopup=\"listbox\" aria-expanded=\"" + (open ? "true" : "false") + "\"><span class=\"guide-category-trigger-copy\"><small>Category</small><strong>" + escapeHTML(guideCategoryInputValue(categories)) + "</strong></span><span class=\"guide-category-chevron\">" + icon("chevron-down") + "</span></button><div class=\"guide-category-popover\"><label class=\"guide-category-search\"><span>" + icon("search") + "</span><input id=\"guide-category-search\" value=\"" + escapeHTML(state.guideCategoryQuery) + "\" placeholder=\"Find a category\" autocomplete=\"off\"></label><div class=\"guide-category-options\" role=\"listbox\">" + guideCategoryOptionsHTML(categories) + "</div></div></div>";
}

function guideCategoryInputValue(categories) {
  if (!state.category) return allGroupLabel();
  const category = items(categories).find(function(item) { return item.id === state.category; });
  return category ? category.name || category.id : "";
}

function guideWindowOverscan() { return 8; }

function guideRowHeight() {
  const scroll = byId("guide-scroll");
  const value = scroll ? getComputedStyle(scroll).getPropertyValue("--epg-row-h").trim() : "";
  const number = parseFloat(value);
  if (!number) return 70;
  return value.indexOf("rem") !== -1 ? number * parseFloat(getComputedStyle(document.documentElement).fontSize || "16") : number;
}

function resetGuideRows() {
  state.guideChannels = visibleChannels(true).filter(guideChannelMatchesQuery);
  state.guideRendered = 0;
  state.guideLoading = false;
  state.guideWindowStart = -1;
  state.guideWindowEnd = -1;
  if (state.guideRenderFrame) cancelAnimationFrame(state.guideRenderFrame);
  state.guideRenderFrame = 0;
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
  programs.forEach(function(entry) {
    const program = entry.program;
    const start = Math.max(entry.start, cursor);
    const end = entry.end;
    if (end <= start) return;
    if (start > cursor) cells.push(renderEPGGapCell(channel, cursor, start, windowInfo));
    const canSchedule = recordingSchedulingEnabled() && (program.endUnix || 0) > now;
    const isLive = start <= now && end > now;
    const programTitle = programIsGuidePlaceholder(program) ? guideUnavailableLabel() : program.title || guideUnavailableLabel();
    const titleParts = epgProgramTitleParts(programTitle);
    const accessibleTitle = titleParts.live ? titleParts.title + " Live" : titleParts.title;
    const programTime = epgVisibleTime(start, windowStart);
    cells.push("<div class=\"epg-cell program" + (isLive ? " live" : "") + "\" style=\"" + epgCellStyle(start, end, windowInfo) + "\"><button class=\"epg-play\" data-program-detail-channel=\"" + escapeHTML(channel.id) + "\" data-program-detail=\"" + escapeHTML(program.id || "") + "\" aria-label=\"" + escapeHTML(programTime + " " + accessibleTitle) + "\"><time>" + escapeHTML(programTime) + "</time><strong>" + escapeHTML(titleParts.title) + (titleParts.live ? "<span class=\"epg-live-marker\" aria-hidden=\"true\">" + escapeHTML(titleParts.marker) + "</span>" : "") + "</strong></button>" + (canSchedule ? "<button class=\"epg-schedule\" data-schedule-channel=\"" + escapeHTML(channel.id) + "\" data-schedule-program=\"" + escapeHTML(program.id || "") + "\" aria-label=\"Schedule recording\">" + icon("record") + "</button>" : "") + "</div>");
    cursor = end;
  });
  if (cursor < windowEnd) cells.push(renderEPGGapCell(channel, cursor, windowEnd, windowInfo));
  return cells.join("");
}

function epgProgramTitleParts(title) {
  const marker = "\u1d38\u1da6\u1d5b\u1d49";
  const value = String(title || "");
  const trimmed = value.trimEnd();
  if (!trimmed.endsWith(marker)) return { title: value, live: false, marker: "" };
  return { title: trimmed.slice(0, -marker.length).trimEnd(), live: true, marker: marker };
}

function epgVisibleTime(startUnix, windowStart) {
  return timeLabel(Math.max(startUnix || windowStart, windowStart));
}

function renderEPGGapCell(channel, startUnix, endUnix, windowInfo) {
  if (endUnix <= startUnix) return "";
  const emptyTitle = guideUnavailableLabel();
  const emptyTime = timeLabel(startUnix);
  return "<button class=\"epg-cell program epg-gap\" data-channel=\"" + escapeHTML(channel.id) + "\" aria-label=\"" + escapeHTML(emptyTime + " " + emptyTitle) + "\" style=\"" + epgCellStyle(startUnix, endUnix, windowInfo) + "\"><time>" + escapeHTML(emptyTime) + "</time><strong>" + escapeHTML(emptyTitle) + "</strong></button>";
}

function renderEPGRow(channel, channelIndex) {
  return "<div class=\"epg-row\">" + renderGuideChannelButton(channel) + "<div class=\"epg-programs\">" + renderEPGCells(channel, channelIndex) + "</div></div>";
}

function renderEPG() {
  renderGuideWindow(true);
}

function scheduleGuideWindowRender() {
  if (state.guideRenderFrame) return;
  state.guideRenderFrame = requestAnimationFrame(function() {
    state.guideRenderFrame = 0;
    renderGuideWindow(false);
  });
}

function guideVisibleRange(totalRows, scrollTop, viewportHeight, rowHeight, headerHeight) {
  const visibleRows = Math.max(1, Math.ceil(Math.max(0, viewportHeight) / rowHeight));
  const overscan = guideWindowOverscan();
  const rowsScrollTop = Math.max(0, scrollTop - headerHeight);
  const start = Math.max(0, Math.floor(rowsScrollTop / rowHeight) - overscan);
  const end = Math.min(totalRows, start + Math.min(40, visibleRows + overscan * 2));
  return { start: start, end: end };
}

function renderGuideWindow(force) {
  if (state.view !== "guide" || state.guideLoading) return;
  const root = byId("epg");
  if (!root) return;
  if (!state.guideChannels.length) {
    state.guideWindowStart = -1;
    state.guideWindowEnd = -1;
    root.style.height = "auto";
    root.innerHTML = "<div class=\"guide-window\" style=\"transform:translateY(0px)\"><div class=\"empty\">No guide matches.</div></div>";
    return;
  }
  const guideScroll = byId("guide-scroll");
  const rowHeight = guideRowHeight();
  const timeHead = guideScroll ? guideScroll.querySelector(".time-head") : null;
  const range = guideVisibleRange(state.guideChannels.length, guideScroll ? guideScroll.scrollTop : 0, guideScroll ? guideScroll.clientHeight : window.innerHeight, rowHeight, timeHead ? timeHead.offsetHeight : 0);
  const start = range.start;
  const end = range.end;
  if (!force && start === state.guideWindowStart && end === state.guideWindowEnd) return;
  state.guideLoading = true;
  const rows = state.guideChannels.slice(start, end).map(function(channel, offset) {
    return renderEPGRow(channel, start + offset);
  }).join("");
  state.guideRendered = end;
  state.guideWindowStart = start;
  state.guideWindowEnd = end;
  root.style.height = (state.guideChannels.length * rowHeight) + "px";
  root.innerHTML = "<div class=\"guide-window\" style=\"transform:translateY(" + (start * rowHeight) + "px)\">" + rows + "</div>";
  state.guideLoading = false;
}
