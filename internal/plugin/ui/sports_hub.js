// Sports hub: Today, Scores, My Teams, News, and team pages.
// Games are organized by team and league; a channel match adds a Watch action.

const SPORTS_HUB_TABS = ["today", "scores", "teams", "news"];
const SPORTS_LEGACY_TABS = { live: "today", upcoming: "scores", all: "scores", favorites: "teams" };

function sportsHubTab(tab) {
  const value = String(tab || "");
  if (value === "replays" || SPORTS_HUB_TABS.indexOf(value) !== -1) return value;
  return SPORTS_LEGACY_TABS[value] || "today";
}

function sportsHubTabLabel(tab) {
  return ({ today: "Today", scores: "Scores", teams: "My Teams", news: "News", replays: "Replays" })[tab] || "Today";
}

function sportsHubTabs() {
  const tabs = SPORTS_HUB_TABS.slice();
  if (configuredSportsLibraryIDs().length) tabs.push("replays");
  return tabs;
}

function sportsEventWatchable(event) {
  return uniqueEventChannels(event && event.channels).length > 0;
}

function sportsEventIsGame(event) {
  return lower(event && event.status) !== "replay";
}

function sportsTeamKey(leagueID, team) {
  const name = sportsTeamName(team || {});
  if (!leagueID || !name) return "";
  return String(leagueID) + "~" + sportsGamePassSlug(name);
}

function sportsTeamKeyParts(key) {
  const index = String(key || "").indexOf("~");
  if (index < 1) return { leagueID: "", slug: "" };
  return { leagueID: key.slice(0, index), slug: key.slice(index + 1) };
}

function sportsHubTeams(payload) {
  const teams = {};
  items(payload && payload.events).forEach(function(event) {
    if (sportsEventIsRace(event) || sportsEventIsProgram(event) || !sportsEventIsGame(event)) return;
    [event.away, event.home].forEach(function(team) {
      const key = sportsTeamKey(event.leagueId, team);
      if (!key) return;
      if (!teams[key]) teams[key] = { key: key, leagueID: event.leagueId, leagueName: event.leagueName || event.leagueId, team: team, events: [] };
      teams[key].events.push(event);
    });
  });
  return teams;
}

function sportsFollowedHubTeams(payload) {
  const teams = sportsHubTeams(payload);
  const followed = Object.keys(teams).map(function(key) { return teams[key]; }).filter(function(entry) {
    return sportsFavoriteTeamMatches(entry.team);
  });
  const seen = {};
  followed.forEach(function(entry) { seen[sportsGamePassSlug(sportsTeamName(entry.team))] = true; });
  myTVFollowedPeople().forEach(function(person) {
    if (person.unresolved || !person.name || seen[sportsGamePassSlug(person.name)]) return;
    const leagueID = sportsLeagueIDForFollow(person);
    if (!leagueID) return;
    const key = sportsTeamKey(leagueID, person);
    followed.push({ key: key, leagueID: leagueID, leagueName: person.leagueName || leagueID, team: person, events: [] });
    seen[sportsGamePassSlug(person.name)] = true;
  });
  return followed.sort(function(left, right) {
    return sportsTeamName(left.team).localeCompare(sportsTeamName(right.team));
  });
}

function sportsLeagueIDForFollow(person) {
  const match = String(person.id || "").match(/^gamepass:([^:]+):/);
  if (match) return match[1];
  const leagueName = lower(person.leagueName);
  const league = items(state.sports && state.sports.leagues).find(function(item) { return lower(item.name) === leagueName; });
  return league ? league.id : "";
}

function sportsTeamEventsSorted(entry) {
  const now = Math.floor(Date.now() / 1000);
  const events = items(entry && entry.events);
  const live = events.filter(sportsEventIsLive);
  const upcoming = events.filter(function(event) { return !sportsEventIsLive(event) && !event.completed && Number(event.startUnix || 0) >= now - 3600; })
    .sort(function(left, right) { return Number(left.startUnix || 0) - Number(right.startUnix || 0); });
  const recent = events.filter(function(event) { return event.completed; })
    .sort(function(left, right) { return Number(right.startUnix || 0) - Number(left.startUnix || 0); });
  return { live: live, upcoming: upcoming, recent: recent };
}

function sportsTeamOpponentLine(entry, event) {
  const isHome = sportsGamePassSlug(sportsTeamName(event.home || {})) === sportsGamePassSlug(sportsTeamName(entry.team));
  const opponent = isHome ? event.away : event.home;
  return (isHome ? "vs " : "at ") + sportsTeamName(opponent || {});
}

function sportsTeamResultLine(entry, event) {
  const isHome = sportsGamePassSlug(sportsTeamName(event.home || {})) === sportsGamePassSlug(sportsTeamName(entry.team));
  const own = Number(isHome ? event.homeScore : event.awayScore);
  const other = Number(isHome ? event.awayScore : event.homeScore);
  if (!sportsEventHasScores(event) || sportsScoresHidden(false) || !Number.isFinite(own) || !Number.isFinite(other)) return "Final " + sportsTeamOpponentLine(entry, event);
  const outcome = own > other ? "W" : (own < other ? "L" : "T");
  return outcome + " " + own + "–" + other + " " + sportsTeamOpponentLine(entry, event);
}

function sportsTeamStatusLine(entry) {
  const games = sportsTeamEventsSorted(entry);
  if (games.live.length) {
    const event = games.live[0];
    const score = sportsEventHasScores(event) && !sportsScoresHidden(false) ? " · " + (event.awayScore || "0") + "–" + (event.homeScore || "0") : "";
    return { text: "Live " + sportsTeamOpponentLine(entry, event) + score, live: true, event: event };
  }
  if (games.upcoming.length) {
    const event = games.upcoming[0];
    return { text: sportsDateLabel(event.startUnix) + " " + sportsTeamOpponentLine(entry, event), live: false, event: event };
  }
  if (games.recent.length) return { text: sportsTeamResultLine(entry, games.recent[0]), live: false, event: games.recent[0] };
  const cached = state.sportsTeamSummaries && state.sportsTeamSummaries[entry.key];
  const next = cached && cached.loaded && cached.value && items(cached.value.upcoming)[0];
  if (next) return { text: sportsDateLabel(next.startUnix) + (next.home ? " vs " : " at ") + next.opponent, live: false, event: null };
  return { text: "No games this week", live: false, event: null };
}

function renderSportsHub(payload) {
  const tab = sportsHubTab(state.sportsTab);
  let body = "";
  if (tab === "scores") body = renderSportsScoresTab(payload);
  else if (tab === "teams") body = renderSportsTeamsTab(payload);
  else if (tab === "news") body = renderSportsNewsTab(payload);
  else body = renderSportsTodayTab(payload);
  const filters = renderSportsTabFilters(payload);
  return (filters ? "<div class=\"sports-pinned\">" + filters + "</div>" : "") + "<div class=\"sports-score-scroll sports-browse sports-hub\">" + body + "</div>";
}

function renderSportsTodayTab(payload) {
  const events = items(payload && payload.events);
  const watchable = events.filter(sportsEventWatchable);
  const featured = sportsFeaturedEvent(watchable);
  const onTV = watchable.filter(function(event) {
    return sportsEventIsOnNow(event) && (!featured || sportsEventStateID(event) !== sportsEventStateID(featured));
  }).sort(compareSportsEventsForTab).slice(0, 8);
  const loading = state.sportsLoading && !events.length;
  if (loading) return "<div class=\"empty\">Loading sports...</div>";
  return renderSportsYourTeams(payload)
    + (featured ? renderSportsFeature(featured) : "")
    + sportsSectionHTML(onTV.length ? "On your TV now" : "", "", onTV.length ? "<div class=\"sports-event-grid\">" + onTV.map(renderSportsEventTile).join("") + "</div>" : "", "sports-on-tv-section")
    + sportsSectionHTML("Scores", "<button type=\"button\" class=\"section-action\" data-sports-tab=\"scores\">All scores</button>", renderSportsScoreboard(sportsTodayEvents(events), { perLeague: 4, maxLeagues: 5 }), "sports-scores-section")
    + sportsSectionHTML("Top stories", "<button type=\"button\" class=\"section-action\" data-sports-tab=\"news\">More news</button>", renderSportsNewsList(sportsNewsForYou(payload), 6), "sports-news-section");
}

function sportsTodayEvents(events) {
  const now = new Date();
  const dayStart = Math.floor(new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime() / 1000);
  const dayEnd = dayStart + 86400;
  return events.filter(function(event) {
    if (!sportsEventIsGame(event)) return false;
    if (sportsEventIsLive(event)) return true;
    const start = Number(event.startUnix || 0);
    return start >= dayStart - 6 * 3600 && start < dayEnd;
  });
}

function renderSportsYourTeams(payload) {
  const followed = sportsFollowedHubTeams(payload);
  if (!followed.length) {
    return "<section class=\"sports-section sports-your-teams-empty\"><div><strong>Follow your teams</strong><p>Get their scores, schedules, and news here, with a Watch button whenever your channels carry the game.</p></div><button type=\"button\" class=\"sports-secondary-action\" data-sports-tab=\"teams\">Find teams</button></section>";
  }
  const cards = followed.map(function(entry) {
    const status = sportsTeamStatusLine(entry);
    const watch = status.event && sportsEventWatchable(status.event) && (status.live || sportsEventIsOnNow(status.event))
      ? "<button type=\"button\" class=\"sports-team-card-watch\" data-channel=\"" + escapeHTML(uniqueEventChannels(status.event.channels)[0].id || "") + "\">" + icon("play") + "<span>Watch</span></button>" : "";
    return "<article class=\"sports-team-card" + (status.live ? " live" : "") + "\"><button type=\"button\" class=\"sports-team-card-main\" data-sports-team-open=\"" + escapeHTML(entry.key) + "\">"
      + renderSportsTeamLogo(entry.team, "sports-team-card-logo")
      + "<span><strong>" + escapeHTML(sportsTeamName(entry.team)) + "</strong><small>" + escapeHTML(status.text) + "</small></span></button>" + watch + "</article>";
  }).join("");
  return sportsSectionHTML("Your teams", "<button type=\"button\" class=\"section-action\" data-sports-tab=\"teams\">Manage</button>", "<div class=\"sports-team-strip\">" + cards + "</div>", "sports-your-teams");
}

function sportsScoreboardLeagueOrder(groups) {
  const favorites = sportsFavoriteLeagueMap();
  return Object.keys(groups).sort(function(left, right) {
    const leftGroup = groups[left];
    const rightGroup = groups[right];
    const leftScore = (favorites[left] ? 100 : 0) + (leftGroup.followed ? 50 : 0) + (leftGroup.live ? 20 : 0) + Math.min(leftGroup.events.length, 10) - (left === "sports" ? 1000 : 0);
    const rightScore = (favorites[right] ? 100 : 0) + (rightGroup.followed ? 50 : 0) + (rightGroup.live ? 20 : 0) + Math.min(rightGroup.events.length, 10) - (right === "sports" ? 1000 : 0);
    return rightScore - leftScore || String(leftGroup.name).localeCompare(String(rightGroup.name));
  });
}

function sportsScoreboardEventOrder(left, right) {
  const rank = function(event) { return sportsEventIsLive(event) ? 0 : (event.completed ? 2 : 1); };
  const leftRank = rank(left);
  const rightRank = rank(right);
  if (leftRank !== rightRank) return leftRank - rightRank;
  const leftFollowed = sportsEventIsFollowed(left);
  if (leftFollowed !== sportsEventIsFollowed(right)) return leftFollowed ? -1 : 1;
  const leftStart = Number(left.startUnix || 0);
  const rightStart = Number(right.startUnix || 0);
  return leftRank === 2 ? rightStart - leftStart : leftStart - rightStart;
}

function sportsScoreboardGroups(events) {
  const groups = {};
  events.forEach(function(event) {
    if (sportsEventIsProgram(event)) return;
    const id = String(event.leagueId || "sports");
    if (!groups[id]) groups[id] = { name: id === "sports" ? "Other sports" : (event.leagueName || id), events: [], live: false, followed: false };
    groups[id].events.push(event);
    if (sportsEventIsLive(event)) groups[id].live = true;
    if (sportsEventIsFollowed(event)) groups[id].followed = true;
  });
  return groups;
}

function renderSportsScoreboard(events, options) {
  options = options || {};
  const groups = sportsScoreboardGroups(events);
  let order = sportsScoreboardLeagueOrder(groups);
  if (options.leagueID) order = order.filter(function(id) { return id === options.leagueID; });
  if (options.maxLeagues) order = order.slice(0, options.maxLeagues);
  if (!order.length) return emptyStateHTML("No games right now.", "Scores show up here as games start.");
  return "<div class=\"sports-scoreboard\">" + order.map(function(id) {
    const group = groups[id];
    const rows = group.events.slice().sort(sportsScoreboardEventOrder);
    const visible = options.perLeague ? rows.slice(0, options.perLeague) : rows;
    const collapsed = !options.leagueID && !!(state.sportsCollapsedLeagues && state.sportsCollapsedLeagues[id]);
    const toggleLabel = (collapsed ? "Expand " : "Collapse ") + group.name;
    const toggle = options.leagueID ? "" : "<button type=\"button\" class=\"sports-scoreboard-toggle\" data-sports-league-toggle=\"" + escapeHTML(id) + "\" aria-expanded=\"" + (collapsed ? "false" : "true") + "\" aria-label=\"" + escapeHTML(toggleLabel) + "\" title=\"" + escapeHTML(toggleLabel) + "\">" + icon("chevron-down") + "</button>";
    return "<section class=\"sports-scoreboard-league" + (collapsed ? " collapsed" : "") + "\"><header>" + toggle + "<button type=\"button\" data-sports-open-league=\"" + escapeHTML(id) + "\">" + escapeHTML(group.name) + icon("chevron-right") + "</button><span>" + escapeHTML(pluralLabel(rows.length, "game")) + "</span></header>"
      + (collapsed ? "" : "<div class=\"sports-scoreboard-rows\">" + visible.map(renderSportsScoreRow).join("") + "</div>") + "</section>";
  }).join("") + "</div>";
}

function renderSportsScoreRow(event) {
  const live = sportsEventIsLive(event);
  const showScore = sportsEventHasScores(event) && !sportsScoresHidden(false);
  const channel = uniqueEventChannels(event.channels)[0];
  const status = live ? sportsStatusLabel(event) : (event.completed ? (event.statusText || "Final") : sportsDateLabel(event.startUnix));
  const team = function(side, score) {
    const value = side || {};
    return "<span class=\"sports-score-team" + (value.favorite ? " followed" : "") + "\">" + renderSportsTeamLogo(value, "sports-score-logo") + "<span>" + escapeHTML(sportsTeamName(value) || "TBD") + "</span>" + (showScore ? "<b>" + escapeHTML(score || "0") + "</b>" : "") + "</span>";
  };
  const hasTeams = !!(sportsTeamName(event.away || {}) || sportsTeamName(event.home || {}));
  const matchup = sportsEventIsRace(event) || sportsEventIsProgram(event) || !hasTeams
    ? "<span class=\"sports-score-program\">" + escapeHTML(sportsEventTitle(event)) + "</span>"
    : team(event.away, event.awayScore) + team(event.home, event.homeScore);
  const watchLabel = "Watch " + sportsEventTitle(event) + " on " + ((channel && channel.name) || "channel");
  const onNow = sportsEventIsOnNow(event);
  const watch = channel && !onNow
    ? (event.completed ? "" : "<span class=\"sports-score-channel\" title=\"" + escapeHTML(channel.name || "") + "\">" + escapeHTML(channel.name || "") + "</span>")
    : channel
    ? "<button type=\"button\" class=\"sports-score-watch\" data-channel=\"" + escapeHTML(channel.id || "") + "\" aria-label=\"" + escapeHTML(watchLabel) + "\" title=\"" + escapeHTML(channel.name || "Watch") + "\">" + icon("play") + "<span>Watch</span></button>"
    : "<span class=\"sports-score-unavailable\">Not on your channels</span>";
  const recording = !live && !event.completed ? sportsEventRecordingTarget(event) : null;
  const recordLabel = "Record " + sportsEventTitle(event);
  const record = recording ? "<button type=\"button\" class=\"sports-score-record\" data-schedule-channel=\"" + escapeHTML(recording.channelID) + "\" data-schedule-program=\"" + escapeHTML(recording.programID) + "\" aria-label=\"" + escapeHTML(recordLabel) + "\" title=\"" + escapeHTML(recordLabel) + "\">" + icon("record") + "</button>" : "";
  return "<div class=\"sports-score-row" + (live ? " live" : "") + (event.completed ? " final" : "") + "\"><button type=\"button\" class=\"sports-score-main\" data-sports-open-event=\"" + escapeHTML(sportsEventStateID(event)) + "\"><span class=\"sports-score-status\">" + escapeHTML(status) + "</span><span class=\"sports-score-teams\">" + matchup + "</span></button>" + record + watch + "</div>";
}

// A future game can be recorded when a matched channel lists a guide program
// starting near the game's start time.
function sportsEventRecordingTarget(event) {
  if (typeof recordingSchedulingEnabled !== "function" || !recordingSchedulingEnabled()) return null;
  const start = Number(event && event.startUnix || 0);
  if (!start) return null;
  const channels = uniqueEventChannels(event.channels);
  for (let index = 0; index < channels.length; index++) {
    const program = programsFor(channels[index].id).find(function(item) {
      return item.id && Math.abs(Number(item.startUnix || 0) - start) <= 45 * 60 && Number(item.endUnix || 0) > Math.floor(Date.now() / 1000);
    });
    if (program) return { channelID: channels[index].id, programID: program.id };
  }
  return null;
}

function sportsStandingsFor(leagueID, divisions) {
  if (!espnNewsLeague(leagueID)) return null;
  state.sportsStandings = state.sportsStandings || {};
  const key = leagueID + (divisions ? "|division" : "");
  const cached = state.sportsStandings[key];
  if (cached) return cached;
  const entry = { loaded: false, value: null };
  state.sportsStandings[key] = entry;
  getJSONWithin("/dispatcharr/api/sports/standings?league=" + encodeURIComponent(leagueID) + (divisions ? "&level=division" : ""), 15000, "Standings took too long.").catch(function() {
    return { columns: [], groups: [], message: "Standings are unavailable right now." };
  }).then(function(value) {
    entry.loaded = true;
    entry.value = value || { columns: [], groups: [] };
    if (state.view === "sports" && !state.sportsSelectedEventID) renderSportsPage();
  });
  return entry;
}

function renderSportsStandings(leagueID, options) {
  options = options || {};
  const entry = sportsStandingsFor(leagueID, !!options.groupOnly);
  if (!entry) return "";
  if (!entry.loaded) return sportsSectionHTML("Standings", "", "<div class=\"empty\">Loading standings...</div>", "sports-standings-section");
  const payload = entry.value || {};
  const highlight = sportsGamePassSlug(options.team || "");
  let groups = items(payload.groups);
  if (highlight && options.groupOnly) {
    const own = groups.filter(function(group) { return items(group.rows).some(function(row) { return sportsGamePassSlug(row.team) === highlight; }); });
    if (own.length) groups = own.slice(0, 1);
  }
  if (!groups.length) return payload.message ? sportsSectionHTML("Standings", "", "<p class=\"sports-team-note\">" + escapeHTML(payload.message) + "</p>", "sports-standings-section") : "";
  const columns = items(payload.columns);
  const tables = groups.map(function(group) {
    return "<div class=\"sports-standings-group\"><table class=\"sports-standings\"><caption>" + escapeHTML(group.name) + "</caption><thead><tr><th scope=\"col\">Team</th>" + columns.map(function(column) { return "<th scope=\"col\">" + escapeHTML(column) + "</th>"; }).join("") + "</tr></thead><tbody>"
      + items(group.rows).map(function(row, index) {
        const own = highlight && sportsGamePassSlug(row.team) === highlight;
        return "<tr class=\"" + (own ? "own" : "") + "\"><th scope=\"row\"><span class=\"sports-standings-rank\">" + (index + 1) + "</span>" + (row.logoUrl ? "<img src=\"" + escapeHTML(row.logoUrl) + "\" alt=\"\" loading=\"lazy\" onerror=\"this.remove()\">" : "") + "<span>" + escapeHTML(row.team) + "</span>" + (row.clinch ? "<em title=\"" + escapeHTML(row.clinch === "e" ? "Eliminated" : "Clinched a playoff spot") + "\">" + escapeHTML(row.clinch) + "</em>" : "") + "</th>"
          + items(row.values).map(function(value) { return "<td>" + escapeHTML(value || "–") + "</td>"; }).join("") + "</tr>";
      }).join("") + "</tbody></table></div>";
  }).join("");
  return sportsSectionHTML("Standings", "", "<div class=\"sports-standings-grid\">" + tables + "</div>", "sports-standings-section");
}

function sportsScheduleRowsHTML(entry, summary, payloadGames) {
  const covered = payloadGames.map(function(event) { return Number(event.startUnix || 0); });
  const rows = payloadGames.map(renderSportsScoreRow);
  items(summary && summary.upcoming).forEach(function(game) {
    if (covered.some(function(start) { return Math.abs(start - game.startUnix) <= 3 * 3600; })) return;
    rows.push("<div class=\"sports-score-row\"><span class=\"sports-score-main static\"><span class=\"sports-score-status\">" + escapeHTML(sportsDateLabel(game.startUnix)) + "</span><span class=\"sports-score-teams\"><span class=\"sports-score-program\">" + escapeHTML((game.home ? "vs " : "at ") + game.opponent) + "</span></span></span><span class=\"sports-score-unavailable\">" + escapeHTML(game.broadcast || "") + "</span></div>");
  });
  return rows.slice(0, 10).join("");
}

function renderSportsUpcomingForTeams(payload) {
  const followed = sportsFollowedHubTeams(payload).filter(function(entry) { return espnNewsLeague(entry.leagueID); });
  if (!followed.length) return "";
  const games = [];
  let loading = false;
  followed.forEach(function(entry) {
    const summary = sportsTeamSummaryFor(entry);
    if (!summary) {
      loading = true;
      return;
    }
    items(summary.upcoming).slice(0, 4).forEach(function(game) { games.push({ entry: entry, game: game }); });
  });
  games.sort(function(left, right) { return left.game.startUnix - right.game.startUnix; });
  const body = games.length ? "<div class=\"sports-scoreboard-rows sports-upcoming-list\">" + games.slice(0, 12).map(function(item) {
    return "<div class=\"sports-score-row\"><button type=\"button\" class=\"sports-score-main\" data-sports-team-open=\"" + escapeHTML(item.entry.key) + "\"><span class=\"sports-score-status\">" + escapeHTML(sportsDateLabel(item.game.startUnix)) + "</span><span class=\"sports-score-teams\"><span class=\"sports-score-team\">" + renderSportsTeamLogo(item.entry.team, "sports-score-logo") + "<span>" + escapeHTML(sportsTeamName(item.entry.team) + (item.game.home ? " vs " : " at ") + item.game.opponent) + "</span></span></span></button><span class=\"sports-score-unavailable\">" + escapeHTML(item.game.broadcast || "") + "</span></div>";
  }).join("") + "</div>" : (loading ? "<div class=\"empty\">Loading schedules...</div>" : "");
  return sportsSectionHTML(body ? "Upcoming for your teams" : "", "", body, "sports-upcoming-section");
}

function renderSportsScoresTab(payload) {
  const filter = state.sportsScoresFilter === "tv" ? "tv" : "all";
  const events = items(payload && payload.events).filter(function(event) { return sportsEventIsGame(event) && (filter === "all" || sportsEventWatchable(event)); });
  const toggle = "<div class=\"view-toggle sports-scores-filter\" aria-label=\"Which games\">" + [["all", "All games"], ["tv", "On my channels"]].map(function(option) {
    return "<button type=\"button\" data-sports-scores-filter=\"" + option[0] + "\" class=\"" + (filter === option[0] ? "active" : "") + "\" aria-pressed=\"" + (filter === option[0] ? "true" : "false") + "\">" + option[1] + "</button>";
  }).join("") + "</div>";
  if (state.sportsLoading && !events.length) return toggle + "<div class=\"empty\">Loading scores...</div>";
  const groups = sportsScoreboardGroups(events);
  const order = sportsScoreboardLeagueOrder(groups);
  if (!order.length) return toggle + emptyStateHTML("No games right now.", "Scores show up here as games start.");
  const selected = groups[state.sportsScoresLeague] ? state.sportsScoresLeague : order[0];
  const picker = "<div class=\"sports-league-picker\" role=\"group\" aria-label=\"Leagues\">" + order.map(function(id) {
    const group = groups[id];
    const active = id === selected;
    return "<button type=\"button\" class=\"chip" + (active ? " active" : "") + "\" data-sports-scores-league=\"" + escapeHTML(id) + "\" aria-pressed=\"" + (active ? "true" : "false") + "\">" + (group.live ? "<span class=\"sports-league-live\" aria-label=\"Live\"></span>" : "") + escapeHTML(group.name) + "<small>" + escapeHTML(String(group.events.length)) + "</small></button>";
  }).join("") + "</div>";
  return "<div class=\"sports-scores-controls\">" + toggle + picker + "</div>" + "<div class=\"sports-scores-league\">" + renderSportsScoreboard(events, { leagueID: selected }) + "</div>";
}

function renderSportsTeamsTab(payload) {
  const followed = sportsFollowedHubTeams(payload);
  const query = String(state.sportsTeamQuery || "");
  const followedHTML = followed.length ? "<div class=\"sports-team-grid\">" + followed.map(function(entry) {
    const status = sportsTeamStatusLine(entry);
    return "<button type=\"button\" class=\"sports-team-tile" + (status.live ? " live" : "") + "\" data-sports-team-open=\"" + escapeHTML(entry.key) + "\">" + renderSportsTeamLogo(entry.team, "sports-team-tile-logo") + "<strong>" + escapeHTML(sportsTeamName(entry.team)) + "</strong><small>" + escapeHTML(entry.leagueName) + "</small><span>" + escapeHTML(status.text) + "</span></button>";
  }).join("") + "</div>" : emptyStateHTML("You aren't following any teams yet.", "Search above to follow a team.");
  return "<section class=\"sports-section sports-team-search\"><label class=\"sports-team-search-field\"><span>" + icon("search") + "</span><input id=\"sports-team-search\" type=\"search\" value=\"" + escapeHTML(query) + "\" placeholder=\"Find a team\" autocomplete=\"off\" aria-label=\"Find a team\"></label><div id=\"sports-team-search-results\">" + renderSportsTeamSearchResults(payload, query) + "</div></section>"
    + sportsSectionHTML("Following", "", followedHTML, "sports-following-section")
    + renderSportsUpcomingForTeams(payload);
}

function renderSportsTeamSearchResults(payload, query) {
  const needle = lower(query).trim();
  const teams = sportsHubTeams(payload);
  const entries = Object.keys(teams).map(function(key) { return teams[key]; }).filter(function(entry) {
    if (!needle) return false;
    const team = entry.team || {};
    return lower([team.name, team.abbreviation, entry.leagueName].join(" ")).indexOf(needle) !== -1;
  }).sort(function(left, right) { return sportsTeamName(left.team).localeCompare(sportsTeamName(right.team)); }).slice(0, 24);
  if (!needle) return "<p class=\"sports-team-search-note\">Search teams playing this week, or browse a league from Scores.</p>";
  if (!entries.length) return "<p class=\"sports-team-search-note\">No teams match &ldquo;" + escapeHTML(query) + "&rdquo; this week.</p>";
  return "<div class=\"sports-team-results\">" + entries.map(function(entry) {
    const team = Object.assign({ leagueName: entry.leagueName }, entry.team);
    const followed = sportsFavoriteTeamMatches(team);
    return "<div class=\"sports-team-result\"><button type=\"button\" class=\"sports-team-result-main\" data-sports-team-open=\"" + escapeHTML(entry.key) + "\">" + renderSportsTeamLogo(team, "sports-team-result-logo") + "<span><strong>" + escapeHTML(sportsTeamName(team)) + "</strong><small>" + escapeHTML(entry.leagueName) + "</small></span></button>"
      + "<button type=\"button\" class=\"sports-team-follow" + (followed ? " active" : "") + "\" data-sports-favorite-team=\"" + escapeHTML(sportsFollowIDForTeam(entry.leagueID, team)) + "\"" + sportsTeamLabelAttrs(team) + " data-sports-favorite-enabled=\"" + (followed ? "false" : "true") + "\" aria-pressed=\"" + (followed ? "true" : "false") + "\">" + icon(followed ? "heart-solid" : "heart") + "<span>" + (followed ? "Following" : "Follow") + "</span></button></div>";
  }).join("") + "</div>";
}

function sportsFollowIDForTeam(leagueID, team) {
  const slug = sportsGamePassSlug(sportsTeamName(team));
  const saved = sportsSavedTeamID(team, slug);
  if (saved) return saved;
  return team.id ? String(team.id) : "gamepass:" + leagueID + ":" + slug;
}

function renderSportsTeamPage(payload, key) {
  const parts = sportsTeamKeyParts(key);
  const teams = sportsHubTeams(payload);
  const followedEntry = sportsFollowedHubTeams(payload).find(function(entry) { return entry.key === key; });
  const entry = teams[key] || followedEntry || { key: key, leagueID: parts.leagueID, leagueName: parts.leagueID, team: { name: parts.slug.replace(/-/g, " ") }, events: [] };
  const summary = sportsTeamSummaryFor(entry);
  const team = Object.assign({ leagueName: entry.leagueName }, entry.team, summary && summary.logoUrl && !(entry.team || {}).logoUrl ? { logoUrl: summary.logoUrl } : {});
  const followed = sportsFavoriteTeamMatches(team);
  const games = sportsTeamEventsSorted(entry);
  const next = games.live[0] || games.upcoming[0];
  const meta = [entry.leagueName, summary && summary.record, summary && summary.standing].filter(Boolean).join(" · ");
  const color = summary && /^[0-9a-f]{6}$/i.test(summary.color || "") ? " style=\"--team-color:#" + summary.color + "\"" : "";
  const header = "<header class=\"sports-team-hero\"" + color + ">" + renderSportsTeamLogo(team, "sports-team-hero-logo") + "<div><span class=\"sports-eyebrow\">Team</span><h1>" + escapeHTML(sportsTeamName(team)) + "</h1><p>" + escapeHTML(meta) + "</p></div>"
    + "<button type=\"button\" class=\"sports-team-follow" + (followed ? " active" : "") + "\" data-sports-favorite-team=\"" + escapeHTML(sportsFollowIDForTeam(entry.leagueID, team)) + "\"" + sportsTeamLabelAttrs(team) + " data-sports-favorite-enabled=\"" + (followed ? "false" : "true") + "\" aria-pressed=\"" + (followed ? "true" : "false") + "\">" + icon(followed ? "heart-solid" : "heart") + "<span>" + (followed ? "Following" : "Follow") + "</span></button></header>";
  const nextBody = next ? "<div class=\"sports-event-grid sports-team-next\">" + renderSportsEventTile(next) + "</div>"
    : (summary && summary.nextEventName ? "<p class=\"sports-team-note\">" + escapeHTML(summary.nextEventName + (summary.nextEventUnix ? " · " + sportsDateLabel(summary.nextEventUnix) : "")) + "</p>" : "");
  const schedule = games.upcoming.slice(next === games.upcoming[0] ? 1 : 0, 8);
  const scheduleRows = sportsScheduleRowsHTML(entry, summary, schedule);
  const newsBody = !summary ? "<div class=\"empty\">Loading news...</div>" : (summary.articles && summary.articles.length ? renderSportsNewsList(summary.articles, 8) : emptyStateHTML("No recent news.", summary.message || ""));
  return "<div class=\"sports-pinned sports-detail-toolbar\"><button type=\"button\" class=\"sports-back\" data-sports-back=\"browse\">" + icon("arrow-left") + "<span>" + escapeHTML(sportsTabLabel(state.sportsTab)) + "</span></button></div>"
    + "<div class=\"sports-score-scroll sports-team-page\">" + header
    + "<div class=\"sports-split\"><div class=\"sports-split-main\">"
    + sportsSectionHTML(next && sportsEventIsLive(next) ? "Live now" : "Next game", "", nextBody, "sports-team-next-section")
    + sportsSectionHTML("Coming up", "", scheduleRows ? "<div class=\"sports-scoreboard-rows\">" + scheduleRows + "</div>" : "", "sports-team-schedule-section")
    + sportsSectionHTML("Recent results", "", games.recent.length ? "<div class=\"sports-scoreboard-rows\">" + games.recent.slice(0, 5).map(renderSportsScoreRow).join("") + "</div>" : "", "sports-team-results-section")
    + "</div><aside class=\"sports-split-side\">"
    + renderSportsStandings(entry.leagueID, { team: sportsTeamName(team), groupOnly: true })
    + sportsSectionHTML("News", "", newsBody, "sports-news-section compact")
    + "</aside></div></div>";
}

function sportsTeamSummaryFor(entry) {
  const key = entry && entry.key;
  if (!key) return null;
  state.sportsTeamSummaries = state.sportsTeamSummaries || {};
  const cached = state.sportsTeamSummaries[key];
  if (cached && cached.loaded) return cached.value;
  if (!cached) {
    state.sportsTeamSummaries[key] = { loaded: false };
    const query = "?league=" + encodeURIComponent(entry.leagueID) + "&name=" + encodeURIComponent(sportsTeamName(entry.team));
    getJSONWithin("/dispatcharr/api/sports/team" + query, 15000, "Team details took too long.").catch(function() {
      return { available: false, articles: [], message: "Team details are unavailable right now." };
    }).then(function(value) {
      state.sportsTeamSummaries[key] = { loaded: true, value: value || { articles: [] } };
      if (state.view === "sports" && !state.sportsSelectedEventID) renderSportsPage();
    });
  }
  return null;
}

function sportsNewsLeaguesForYou(payload) {
  const ids = [];
  const add = function(id) { if (id && espnNewsLeague(id) && ids.indexOf(id) === -1) ids.push(id); };
  Object.keys(sportsFavoriteLeagueMap()).forEach(add);
  sportsFollowedHubTeams(payload).forEach(function(entry) { add(entry.leagueID); });
  items(payload && payload.leagues).slice().sort(function(left, right) {
    return Number(right.liveCount || 0) + Number(right.upcomingCount || 0) - Number(left.liveCount || 0) - Number(left.upcomingCount || 0);
  }).forEach(function(league) { add(league.id); });
  return ids;
}

const SPORTS_NEWS_LEAGUE_NAMES = { mlb: "MLB", nfl: "NFL", nba: "NBA", wnba: "WNBA", nhl: "NHL", mls: "MLS", "premier-league": "Premier League", "uefa-champions-league": "Champions League", "college-football": "College Football", "mens-college-basketball": "Men's College Basketball", "womens-college-basketball": "Women's College Basketball", "formula-1": "Formula 1", golf: "PGA Tour", mma: "UFC" };

function espnNewsLeague(leagueID) {
  return Object.prototype.hasOwnProperty.call(SPORTS_NEWS_LEAGUE_NAMES, String(leagueID || ""));
}

function sportsNewsRequest(leagueID, teamName) {
  const key = leagueID + "|" + (teamName || "");
  state.sportsNews = state.sportsNews || {};
  const cached = state.sportsNews[key];
  if (cached) return cached;
  const entry = { loaded: false, articles: [], message: "" };
  state.sportsNews[key] = entry;
  const query = "?league=" + encodeURIComponent(leagueID) + (teamName ? "&team=" + encodeURIComponent(teamName) : "");
  getJSONWithin("/dispatcharr/api/sports/news" + query, 15000, "News took too long.").catch(function() {
    return { articles: [], message: "News is unavailable right now." };
  }).then(function(payload) {
    entry.loaded = true;
    entry.articles = items(payload && payload.articles);
    entry.message = (payload && payload.message) || "";
    if (state.view === "sports" && !state.sportsSelectedEventID) renderSportsPage();
  });
  return entry;
}

function mergeSportsNews(entries) {
  const seen = {};
  const articles = [];
  entries.forEach(function(entry) {
    items(entry && entry.articles).forEach(function(article) {
      if (!article || seen[article.id]) return;
      seen[article.id] = true;
      articles.push(article);
    });
  });
  return articles.sort(function(left, right) { return String(right.published || "").localeCompare(String(left.published || "")); });
}

function sportsNewsForYou(payload) {
  const followed = sportsFollowedHubTeams(payload).filter(function(entry) { return espnNewsLeague(entry.leagueID); }).slice(0, 4);
  const requests = followed.map(function(entry) { return sportsNewsRequest(entry.leagueID, sportsTeamName(entry.team)); })
    .concat(sportsNewsLeaguesForYou(payload).slice(0, 3).map(function(id) { return sportsNewsRequest(id, ""); }));
  return { loading: requests.some(function(entry) { return !entry.loaded; }), articles: mergeSportsNews(requests) };
}

function renderSportsNewsTab(payload) {
  const leagues = sportsNewsLeaguesForYou(payload);
  const selected = state.sportsNewsLeague && leagues.indexOf(state.sportsNewsLeague) !== -1 ? state.sportsNewsLeague : "";
  const chips = "<div class=\"sports-leagues\"><button type=\"button\" class=\"chip" + (!selected ? " active" : "") + "\" data-sports-news-league=\"\">For you</button>" + leagues.map(function(id) {
    return "<button type=\"button\" class=\"chip" + (selected === id ? " active" : "") + "\" data-sports-news-league=\"" + escapeHTML(id) + "\">" + escapeHTML(SPORTS_NEWS_LEAGUE_NAMES[id] || id) + "</button>";
  }).join("") + "</div>";
  let news;
  if (selected) {
    const entry = sportsNewsRequest(selected, "");
    news = { loading: !entry.loaded, articles: entry.articles, message: entry.message };
  } else {
    news = sportsNewsForYou(payload);
  }
  return chips + renderSportsNewsList(news, 30);
}

function sportsNewsAge(published) {
  const time = Date.parse(published || "");
  if (!Number.isFinite(time)) return "";
  const minutes = Math.max(0, Math.round((Date.now() - time) / 60000));
  if (minutes < 60) return minutes <= 1 ? "Just now" : minutes + "m ago";
  const hours = Math.round(minutes / 60);
  if (hours < 24) return hours + "h ago";
  return Math.round(hours / 24) + "d ago";
}

function renderSportsNewsList(news, limit) {
  const list = Array.isArray(news) ? { loading: false, articles: news } : (news || { articles: [] });
  const articles = items(list.articles).slice(0, limit || 10);
  if (!articles.length) {
    if (list.loading) return "<div class=\"empty\">Loading news...</div>";
    return emptyStateHTML("No news right now.", list.message || "Follow teams or leagues to see their headlines here.");
  }
  return "<div class=\"sports-news-list\" aria-label=\"Headlines from ESPN\">" + articles.map(function(article) {
    const meta = [article.leagueName, sportsNewsAge(article.published), article.premium ? "ESPN+" : ""].filter(Boolean).join(" · ");
    const image = article.imageUrl ? "<img src=\"" + escapeHTML(article.imageUrl) + "\" alt=\"\" loading=\"lazy\" onerror=\"this.remove()\">" : "";
    return "<a class=\"sports-news-item\" href=\"" + escapeHTML(article.url) + "\" target=\"_blank\" rel=\"noopener noreferrer\">" + image + "<span><small>" + escapeHTML(meta) + "</small><strong>" + escapeHTML(article.headline) + "</strong>" + (article.description ? "<p>" + escapeHTML(article.description) + "</p>" : "") + "</span></a>";
  }).join("") + "</div>";
}

function sportsLeagueNewsHTML(leagueID) {
  if (!espnNewsLeague(leagueID)) return "";
  const entry = sportsNewsRequest(leagueID, "");
  return sportsSectionHTML("News", "", renderSportsNewsList({ loading: !entry.loaded, articles: entry.articles, message: entry.message }, 8), "sports-news-section");
}

// In the standalone Sports app the header carries the Sports sections and the
// viewer's own teams instead of Live TV navigation.
function renderSportsAppHeader() {
  const current = state.sportsTeam || state.sportsLeague || state.sportsSelectedEventID ? "" : sportsHubTab(state.sportsTab);
  const tabs = sportsHubTabs().map(function(tab) {
    const active = current === tab;
    return "<button type=\"button\" data-sports-tab=\"" + tab + "\" class=\"" + (active ? "active" : "") + "\"" + (active ? " aria-current=\"page\"" : "") + ">" + escapeHTML(sportsHubTabLabel(tab)) + "</button>";
  }).join("");
  const teams = sportsFollowedHubTeams(state.sports || {}).slice(0, 8).map(function(entry) {
    const status = sportsTeamStatusLine(entry);
    const active = state.sportsTeam === entry.key;
    const label = sportsTeamName(entry.team) + " · " + status.text;
    return "<button type=\"button\" class=\"sports-header-team" + (status.live ? " live" : "") + (active ? " active" : "") + "\" data-sports-team-open=\"" + escapeHTML(entry.key) + "\" aria-label=\"" + escapeHTML(label) + "\" title=\"" + escapeHTML(label) + "\"" + (active ? " aria-current=\"page\"" : "") + ">" + renderSportsTeamLogo(entry.team, "sports-header-team-logo") + "</button>";
  }).join("");
  const add = "<button type=\"button\" class=\"sports-header-team sports-header-add\" data-sports-tab=\"teams\" aria-label=\"Follow teams\" title=\"Follow teams\">" + icon("plus") + "</button>";
  const payload = state.sports || {};
  const refreshing = state.sportsLoading || !!payload.refreshing || state.sportsReplaysLoading;
  const replayStatus = sportsReplayStatusLabel();
  const refreshLabel = (refreshing ? "Refreshing scores" : "Refresh scores") + " · Data by " + sportsDataSourceLabel(payload) + (replayStatus ? " · " + replayStatus : "");
  const refresh = "<button type=\"button\" class=\"topbar-icon sports-header-refresh" + (refreshing ? " is-loading" : "") + "\" data-sports-refresh=\"true\" aria-label=\"" + escapeHTML(refreshLabel) + "\" title=\"" + escapeHTML(refreshLabel) + "\"" + (refreshing ? " disabled aria-busy=\"true\"" : "") + ">" + icon(refreshing ? "loader" : "refresh") + "</button>";
  return "<nav class=\"nav topnav sports-app-nav\" aria-label=\"Sports sections\">" + tabs + "</nav><div class=\"sports-header-teams\" role=\"group\" aria-label=\"Your teams\">" + teams + add + "</div>" + refresh;
}

function openSportsTeam(key) {
  state.view = "sports";
  state.sportsTeam = String(key || "");
  state.sportsSelectedEventID = "";
  state.sportsLeague = "";
  renderSportsPage();
  commitAppRoute("push");
}

let sportsHighlightReturnFocus = null;

function openSportsHighlight(trigger) {
  closeSportsHighlight();
  const src = trigger.getAttribute("data-sports-highlight-src") || "";
  if (!/^https:\/\//i.test(src)) return;
  const title = trigger.getAttribute("data-sports-highlight-title") || "Highlight";
  const page = trigger.getAttribute("data-sports-highlight-page") || "";
  sportsHighlightReturnFocus = trigger;
  const root = document.createElement("div");
  root.id = "sports-highlight-modal";
  root.className = "sports-highlight-modal";
  root.innerHTML = "<div class=\"sports-highlight-backdrop\" data-sports-highlight-close=\"true\"></div>"
    + "<section class=\"sports-highlight-dialog\" role=\"dialog\" aria-modal=\"true\" aria-labelledby=\"sports-highlight-title\">"
    + "<header><h2 id=\"sports-highlight-title\">" + escapeHTML(title) + "</h2>"
    + (/^https:\/\//i.test(page) && page !== src ? "<a href=\"" + escapeHTML(page) + "\" target=\"_blank\" rel=\"noopener noreferrer\">Open on ESPN</a>" : "")
    + "<button type=\"button\" data-sports-highlight-close=\"true\" aria-label=\"Close highlight\">" + icon("x") + "</button></header>"
    + "<video src=\"" + escapeHTML(src) + "\" controls autoplay playsinline></video></section>";
  document.body.appendChild(root);
  document.body.classList.add("program-modal-open");
  const close = root.querySelector("header button");
  if (close) close.focus();
}

function closeSportsHighlight() {
  const root = byId("sports-highlight-modal");
  if (!root) return;
  const video = root.querySelector("video");
  if (video) { video.pause(); video.removeAttribute("src"); video.load(); }
  root.remove();
  document.body.classList.remove("program-modal-open");
  const target = sportsHighlightReturnFocus;
  sportsHighlightReturnFocus = null;
  if (target && document.contains(target)) target.focus();
}

document.addEventListener("keydown", function(event) {
  if (event.key === "Escape" && byId("sports-highlight-modal")) {
    event.preventDefault();
    event.stopPropagation();
    closeSportsHighlight();
  }
}, true);

document.addEventListener("click", function(event) {
  const highlightClose = event.target.closest && event.target.closest("[data-sports-highlight-close]");
  if (highlightClose) {
    event.preventDefault();
    closeSportsHighlight();
    return;
  }
  const scoresLeague = event.target.closest && event.target.closest("[data-sports-scores-league]");
  if (scoresLeague) {
    event.preventDefault();
    state.sportsScoresLeague = scoresLeague.getAttribute("data-sports-scores-league");
    renderSportsPage();
    const scroller = document.querySelector(".sports-score-scroll");
    if (scroller) scroller.scrollTop = 0;
    return;
  }
  const leagueToggle = event.target.closest && event.target.closest("[data-sports-league-toggle]");
  if (leagueToggle) {
    event.preventDefault();
    const id = leagueToggle.getAttribute("data-sports-league-toggle");
    state.sportsCollapsedLeagues = Object.assign({}, state.sportsCollapsedLeagues);
    if (state.sportsCollapsedLeagues[id]) delete state.sportsCollapsedLeagues[id];
    else state.sportsCollapsedLeagues[id] = true;
    renderSportsPage();
    const restored = document.querySelector("[data-sports-league-toggle=\"" + cssEscape(id) + "\"]");
    if (restored) restored.focus();
    return;
  }
  const highlight = event.target.closest && event.target.closest("[data-sports-highlight-src]");
  if (highlight) {
    event.preventDefault();
    openSportsHighlight(highlight);
    return;
  }
  const teamOpen = event.target.closest && event.target.closest("[data-sports-team-open]");
  if (teamOpen) {
    event.preventDefault();
    openSportsTeam(teamOpen.getAttribute("data-sports-team-open"));
    return;
  }
  const newsLeague = event.target.closest && event.target.closest("[data-sports-news-league]");
  if (newsLeague) {
    event.preventDefault();
    state.sportsNewsLeague = newsLeague.getAttribute("data-sports-news-league") || "";
    renderSportsPage();
    return;
  }
  const scoresFilter = event.target.closest && event.target.closest("[data-sports-scores-filter]");
  if (scoresFilter) {
    event.preventDefault();
    state.sportsScoresFilter = scoresFilter.getAttribute("data-sports-scores-filter") || "all";
    renderSportsPage();
  }
});

document.addEventListener("input", function(event) {
  if (!event.target || event.target.id !== "sports-team-search") return;
  state.sportsTeamQuery = event.target.value;
  const results = byId("sports-team-search-results");
  if (results) results.innerHTML = renderSportsTeamSearchResults(state.sports || {}, state.sportsTeamQuery);
});
