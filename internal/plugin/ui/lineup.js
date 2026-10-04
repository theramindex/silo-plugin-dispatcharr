// Theme bootstrap. This file is loaded synchronously in <head> so the Silo
// theme applies before first paint; CSP (script-src 'self') forbids the
// inline script that used to do this.
(function() {
  var root = typeof document !== "undefined" ? document.documentElement : null, key = "dispatcharr-silo-theme";
  if (!root || typeof window === "undefined" || !window.location) return;
  try {
    var url = new URL(window.location.href);
    var theme = String(root.getAttribute("data-silo-theme") || "").replace(/[^a-z0-9_-]/g, "");
    if (theme) window.localStorage.setItem(key, theme);
    else {
      var saved = String(window.localStorage.getItem(key) || "").replace(/[^a-z0-9_-]/g, "");
      if (saved) root.setAttribute("data-silo-theme", saved);
    }
    if (url.searchParams.has("theme")) {
      url.searchParams.delete("theme");
      window.history.replaceState(window.history.state, "", url.pathname + url.search + url.hash);
    }
  } catch (_) {}
})();

(function(global) {
  "use strict";

  function items(value) {
    return Array.isArray(value) ? value : [];
  }

  function normalizedNameToken(value) {
    return String(value || "").trim().replace(/\s+/g, " ");
  }

  function parsedDelimitedPath(name, delimiter) {
    name = normalizedNameToken(name);
    if (!name) return [];
    const pattern = delimiter === "dash" ? /\s+-\s*/ : /\s*\|\s*/;
    return name.split(pattern).map(normalizedNameToken).filter(Boolean);
  }

  function channelNamePathParts(channel) {
    return String((channel && channel.name) || "").split("|").map(normalizedNameToken).filter(Boolean);
  }

  function appendUnduplicatedPathParts(basePath, extraParts) {
    const baseParts = String(basePath || "").split(" / ").map(normalizedNameToken).filter(Boolean);
    let additions = items(extraParts).map(normalizedNameToken).filter(Boolean);
    let sharedPrefixLength = 0;
    while (
      sharedPrefixLength < baseParts.length &&
      sharedPrefixLength < additions.length &&
      baseParts[sharedPrefixLength].toLowerCase() === additions[sharedPrefixLength].toLowerCase()
    ) {
      sharedPrefixLength += 1;
    }
    if (sharedPrefixLength) additions = additions.slice(sharedPrefixLength);
    while (additions.length && baseParts.length && baseParts[baseParts.length - 1].toLowerCase() === additions[0].toLowerCase()) {
      additions = additions.slice(1);
    }
    return additions.length ? baseParts.concat(additions).join(" / ") : "";
  }

  function appendVirtualPathParts(basePath, extraParts, collapseDuplicates) {
    const baseParts = String(basePath || "").split(" / ").map(normalizedNameToken).filter(Boolean);
    const additions = items(extraParts).map(normalizedNameToken).filter(Boolean);
    if (!baseParts.length || !additions.length) return "";
    if (collapseDuplicates === false) return baseParts.concat(additions).join(" / ");
    return appendUnduplicatedPathParts(basePath, additions);
  }

  global.DispatcharrLineup = Object.freeze({
    appendUnduplicatedPathParts: appendUnduplicatedPathParts,
    appendVirtualPathParts: appendVirtualPathParts,
    channelNamePathParts: channelNamePathParts,
    normalizedNameToken: normalizedNameToken,
    parsedDelimitedPath: parsedDelimitedPath
  });
})(window);
