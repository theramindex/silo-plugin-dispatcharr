(function (root, factory) {
  var matcher = factory();
  if (typeof module === "object" && module.exports) module.exports = { SportsReplayMatcher: matcher };
  if (root) root.SportsReplayMatcher = matcher;
})(typeof globalThis !== "undefined" ? globalThis : this, function () {
  "use strict";

  var DEFAULTS = {
    threshold: 92,
    maxDateDays: 7,
    maxMatches: 10,
    replayLookbackDays: 45,
    replayFutureHours: 12,
    maxCatalogWindows: 4
  };
  var STOP_WORDS = new Set(["a", "an", "and", "at", "away", "championship", "game", "in", "live", "of", "on", "the", "to", "vs", "versus"]);
  var SPORTS_WORDS = new Set(["baseball", "basketball", "football", "hockey", "league", "match", "mlb", "mls", "nba", "nfl", "nhl", "soccer", "sports", "wnba"]);

  function asText(value) {
    return value === null || value === undefined ? "" : String(value);
  }

  function normalized(value) {
    return asText(value)
      .normalize("NFKD")
      .replace(/[\u0300-\u036f]/g, "")
      .toLowerCase()
      .replace(/&/g, " and ")
      .replace(/[^a-z0-9]+/g, " ")
      .trim()
      .replace(/\s+/g, " ");
  }

  function tokens(value) {
    return normalized(value).split(" ").filter(Boolean);
  }

  function tokenSet(value) {
    return new Set(tokens(value));
  }

  function hasPhrase(haystack, phrase) {
    var text = " " + normalized(haystack) + " ";
    var needle = " " + normalized(phrase) + " ";
    return needle.length > 2 && text.indexOf(needle) !== -1;
  }

  function unique(values) {
    var seen = new Set();
    return values.filter(function (value) {
      var key = normalized(value);
      if (!key || seen.has(key)) return false;
      seen.add(key);
      return true;
    });
  }

  function firstValue() {
    for (var index = 0; index < arguments.length; index += 1) {
      var value = arguments[index];
      if (value !== null && value !== undefined && asText(value).trim()) return value;
    }
    return "";
  }

  function teamAliases(team) {
    team = team || {};
    var name = firstValue(team.name, team.teamName, team.displayName);
    var abbreviation = firstValue(team.abbreviation, team.abbr, team.shortName);
    return unique([name, abbreviation]);
  }

  function eventTeams(event) {
    event = event || {};
    var home = event.home || event.homeTeam || {};
    var away = event.away || event.awayTeam || {};
    return [home, away].map(teamAliases).filter(function (aliases) { return aliases.length > 0; });
  }

  function eventTitle(event) {
    return firstValue(event && event.name, event && event.title, event && event.shortName);
  }

  function candidateTitle(item) {
    item = item || {};
    return firstValue(item.name, item.title, item.display_name, item.displayName);
  }

  function candidateText(item) {
    item = item || {};
    return [
      candidateTitle(item), item.description, item.plot, item.summary,
      item.league, item.league_name, item.leagueName, item.category,
      item.category_name, item.categoryName, item.genre, item.genres
    ].map(asText).join(" ");
  }

  function eventLeague(event) {
    return firstValue(event && event.leagueName, event && event.league, event && event.leagueId);
  }

  function leagueAliases(value) {
    var key = normalized(value);
    var aliases = {
      wnba: ["wnba", "womens national basketball association"],
      nba: ["nba", "national basketball association"],
      mlb: ["mlb", "major league baseball"],
      nfl: ["nfl", "national football league"],
      nhl: ["nhl", "national hockey league"],
      mls: ["mls", "major league soccer"]
    };
    return unique([value].concat(aliases[key] || []));
  }

  function dateValue(value) {
    if (value === null || value === undefined || value === "") return 0;
    if (typeof value === "number" || /^\d+(?:\.\d+)?$/.test(asText(value).trim())) {
      var numeric = Number(value);
      if (!isFinite(numeric)) return 0;
      return numeric > 100000000000 ? Math.round(numeric / 1000) : Math.round(numeric);
    }
    var parsed = Date.parse(asText(value));
    return isNaN(parsed) ? 0 : Math.round(parsed / 1000);
  }

  function catalogDates(item) {
    item = item || {};
    var metrics = item.sort_metrics || item.sortMetrics || {};
    return unique([
      item.release_date, item.releaseDate,
      item.first_air_date, item.firstAirDate,
      metrics.release_date, metrics.releaseDate
    ]).map(dateValue).filter(Boolean);
  }

  function eventDate(event) {
    return dateValue(firstValue(event && event.startUnix, event && event.start_unix, event && event.startDate));
  }

  function isoDate(unix) {
    return new Date(unix * 1000).toISOString().slice(0, 10);
  }

  function catalogWindows(events, options) {
    options = Object.assign({}, DEFAULTS, options || {});
    var nowUnix = dateValue(options.nowUnix) || Math.round(Date.now() / 1000);
    var earliest = nowUnix - Math.max(1, Number(options.replayLookbackDays) || DEFAULTS.replayLookbackDays) * 86400;
    var latest = nowUnix + Math.max(0, Number(options.replayFutureHours) || DEFAULTS.replayFutureHours) * 3600;
    var padding = Math.max(0, Number(options.maxDateDays) || DEFAULTS.maxDateDays) * 86400;
    var windows = (Array.isArray(events) ? events : []).map(eventDate).filter(function (unix) {
      return unix >= earliest && unix <= latest;
    }).sort(function (left, right) { return left - right; }).map(function (unix) {
      return { start: unix - padding, end: unix + padding };
    });
    var merged = [];
    windows.forEach(function (window) {
      var previous = merged[merged.length - 1];
      if (previous && window.start <= previous.end + 86400) previous.end = Math.max(previous.end, window.end);
      else merged.push(window);
    });
    return merged.slice(-Math.max(1, Math.floor(Number(options.maxCatalogWindows) || DEFAULTS.maxCatalogWindows))).map(function (window) {
      return [isoDate(window.start), isoDate(window.end)];
    });
  }

  function dateScore(eventUnix, dates, maxDateDays) {
    if (!eventUnix || !dates.length) return { usable: false, score: 0, days: null };
    var nearest = Math.min.apply(null, dates.map(function (date) { return Math.abs(eventUnix - date) / 86400; }));
    if (nearest > maxDateDays) return { usable: false, score: 0, days: nearest };
    if (nearest <= 1) return { usable: true, score: 15, days: nearest };
    if (nearest <= 3) return { usable: true, score: 12, days: nearest };
    if (nearest <= 7) return { usable: true, score: 8, days: nearest };
    return { usable: true, score: 3, days: nearest };
  }

  function sportsContext(text, league) {
    var value = tokenSet(text);
    if (leagueAliases(league).some(function (alias) { return hasPhrase(text, alias); })) return true;
    return Array.from(value).some(function (token) { return SPORTS_WORDS.has(token); });
  }

  function aliasMatch(text, aliases, allowAbbreviation) {
    var full = aliases[0];
    if (full && full.split(" ").length > 1 && hasPhrase(text, full)) return "full";
    if (full && full.split(" ").length === 1 && full.length > 3 && hasPhrase(text, full)) return "full";
    var abbreviation = aliases.slice(1).find(function (alias) {
      return alias.length >= 3 && alias.length <= 5 && alias.split(" ").length === 1 && hasPhrase(text, alias);
    });
    return allowAbbreviation && abbreviation ? "abbreviation" : "";
  }

  function pairMatch(event, item) {
    var teams = eventTeams(event);
    if (teams.length < 2) return { matched: false, kind: "none" };
    var text = candidateTitle(item);
    if (!text) return { matched: false, kind: "none" };
    var context = sportsContext(candidateText(item), eventLeague(event));
    var matches = teams.map(function (aliases) { return aliasMatch(text, aliases, context); });
    if (!matches[0] || !matches[1]) return { matched: false, kind: "none" };
    if (matches[0] === "full" && matches[1] === "full") return { matched: true, kind: "full" };
    if (context && (matches[0] === "abbreviation" || matches[1] === "abbreviation")) {
      return { matched: true, kind: "mixed" };
    }
    return { matched: false, kind: "none" };
  }

  function overlapScore(event, item) {
    var eventWords = tokenSet(eventTitle(event));
    var itemWords = tokenSet(candidateTitle(item));
    var teamWords = new Set();
    eventTeams(event).forEach(function (aliases) {
      tokens(aliases[0]).forEach(function (word) { teamWords.add(word); });
    });
    var useful = Array.from(eventWords).filter(function (word) {
      return !STOP_WORDS.has(word) && !teamWords.has(word) && word.length > 2;
    });
    if (!useful.length) return 0;
    var overlap = useful.filter(function (word) { return itemWords.has(word); }).length / useful.length;
    return Math.round(overlap * 10);
  }

  function contentID(item) {
    item = item || {};
    return asText(firstValue(item.content_id, item.contentId, item.id)).trim();
  }

  function accessibleLibraryIDs(configuredIDs, libraries) {
    var allowed = new Set((Array.isArray(libraries) ? libraries : []).map(function (library) {
      return Number(library && library.id);
    }).filter(function (id) { return Number.isInteger(id) && id > 0; }));
    var seen = new Set();
    return (Array.isArray(configuredIDs) ? configuredIDs : []).map(Number).filter(function (id) {
      if (!Number.isInteger(id) || id <= 0 || seen.has(id) || !allowed.has(id)) return false;
      seen.add(id);
      return true;
    });
  }

  async function collectCatalogPages(fetchPage, maxPages) {
    var limit = Math.max(1, Math.floor(Number(maxPages) || 5));
    var collected = [];
    var offset = 0;
    var truncated = false;
    for (var page = 0; page < limit; page += 1) {
      var payload = await fetchPage(offset);
      var pageItems = Array.isArray(payload && payload.items) ? payload.items : [];
      collected.push.apply(collected, pageItems);
      if (!payload || !payload.has_more || !pageItems.length) return { items: collected, truncated: false };
      offset += pageItems.length;
      truncated = page === limit - 1;
    }
    return { items: collected, truncated: truncated };
  }

  function scoreCandidate(event, item, options) {
    var pair = pairMatch(event, item);
    if (!pair.matched) return null;
    var dates = catalogDates(item);
    var datesMatch = dateScore(eventDate(event), dates, options.maxDateDays);
    if (!datesMatch.usable) return null;
    var league = eventLeague(event);
    var leagueMatched = leagueAliases(league).some(function (alias) {
      return hasPhrase(candidateText(item), alias);
    });
    var score = pair.kind === "full" ? 62 : 52;
    var reasons = [pair.kind === "full" ? "both team names match" : "both teams match in sports context"];
    if (leagueMatched) {
      score += 18;
      reasons.push("league matches");
    }
    if (dates.length) {
      score += datesMatch.score;
      reasons.push(datesMatch.days <= 1 ? "date is within one day" : "date is nearby");
    }
    var overlap = overlapScore(event, item);
    if (overlap) {
      score += overlap;
      reasons.push("title tokens overlap");
    }
    if (!leagueMatched && pair.kind !== "full") return null;
    if (score < options.threshold) return null;
    return { item: item, score: score, confidence: "high", reasons: reasons };
  }

  function matchEvents(events, catalogItems, options) {
    options = Object.assign({}, DEFAULTS, options || {});
    options.threshold = Number.isFinite(Number(options.threshold)) ? Number(options.threshold) : DEFAULTS.threshold;
    options.maxDateDays = Number.isFinite(Number(options.maxDateDays)) ? Math.max(0, Number(options.maxDateDays)) : DEFAULTS.maxDateDays;
    options.maxMatches = Number.isFinite(Number(options.maxMatches)) ? Math.max(1, Math.floor(Number(options.maxMatches))) : DEFAULTS.maxMatches;
    var result = {};
    (Array.isArray(events) ? events : []).forEach(function (event) {
      var id = asText(firstValue(event && event.id, event && event.eventId)).trim();
      if (!id) return;
      var byContent = new Map();
      (Array.isArray(catalogItems) ? catalogItems : []).forEach(function (item, index) {
        var match = scoreCandidate(event, item, options);
        if (!match) return;
        var key = contentID(item) || "index:" + index;
        var existing = byContent.get(key);
        if (!existing || match.score > existing.score) byContent.set(key, match);
      });
      result[id] = Array.from(byContent.values()).sort(function (left, right) {
        if (right.score !== left.score) return right.score - left.score;
        return candidateTitle(left.item).localeCompare(candidateTitle(right.item));
      }).slice(0, options.maxMatches);
    });
    return result;
  }

  return { matchEvents: matchEvents, accessibleLibraryIDs: accessibleLibraryIDs, catalogWindows: catalogWindows, collectCatalogPages: collectCatalogPages };
});
