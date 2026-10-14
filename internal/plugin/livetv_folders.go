package plugin

import (
	"regexp"
	"strings"

	"github.com/theramindex/silo-plugin-dispatcharr/internal/model"
)

// livetv/v1 channel folders: a Go port of the web app's virtual folder paths
// (virtualPathsForChannel and its helpers in ui/app.js, ui/prefs.js and
// ui/lineup.js), so Silo's Jellyfin bridge browses the same tree the web app
// shows. Keep the two in step. The contract lineup is the same for every
// caller, so a user's profile selection is applied by Silo: in the profile
// group sources each folder is also served with the channel profile it comes
// from (profileFolders).

const livetvFolderSeparator = " / "

type livetvFolderSettings struct {
	delimiter      bool // categoryParsing().enabled
	dash           bool // delimiter "dash" instead of "pipe"
	source         string
	collapse       bool // collapseDuplicateVirtualGroups
	flatten        bool // flattenRedundantGroupWrappers
	aliases        [][2]string
	profileNames   map[string]string
	activeProfile  string // source.channelProfile.id
	categoryLabels map[string]string
}

func (s *HTTPRoutesServer) livetvFolderSettings(source model.Source, categories []model.Category) livetvFolderSettings {
	settings := s.normalizedAdminSettings()
	text := func(key string) string { value, _ := settings[key].(string); return value }
	folder := livetvFolderSettings{
		delimiter:      text("mode") == "delimiter",
		dash:           text("delimiter") == "dash",
		source:         text("virtualGroupSource"),
		collapse:       boolSetting(settings, "collapseDuplicateVirtualGroups", true),
		flatten:        boolSetting(settings, "flattenRedundantGroupWrappers", true),
		profileNames:   map[string]string{},
		categoryLabels: map[string]string{},
	}
	if aliases, ok := settings["categoryAliases"].([]map[string]string); ok {
		for _, alias := range aliases {
			folder.aliases = append(folder.aliases, [2]string{alias["sourcePath"], alias["aliasPath"]})
		}
	}
	for _, profile := range source.Profiles {
		if profile.ID != "" {
			folder.profileNames[profile.ID] = profile.Name
		}
	}
	if source.ChannelProfile != nil {
		folder.activeProfile = source.ChannelProfile.ID
	}
	for _, category := range categories {
		folder.categoryLabels[category.ID] = category.Name
	}
	return folder
}

// livetvChannelFolders returns each channel's folders by channel id. Only the
// deepest paths are kept: a folder's parents are implied. A channel with no
// virtual path is listed in its source group's path.
func livetvChannelFolders(settings livetvFolderSettings, channels []model.Channel) map[string][]string {
	tree := livetvFolderTree{livetvFolderSettings: settings}
	tree.wrappers = tree.redundantProfileGroupWrappers(channels)
	out := make(map[string][]string, len(channels))
	for _, channel := range channels {
		paths := tree.virtualPaths(channel)
		if len(paths) == 0 {
			if path := tree.sourceGroupPath(channel); path != "" {
				paths = []string{path}
			}
		}
		out[channel.ID] = livetvLeafFolders(paths)
	}
	return out
}

// livetvProfileFolder is one of a channel's folders and the channel profile it
// comes from, or "" for a folder that comes from no profile and is shown
// whichever profiles a user selects.
type livetvProfileFolder struct {
	Path      string `json:"path"`
	ProfileID string `json:"profileId"`
}

// livetvChannelProfileFolders returns each channel's folders tagged with the
// profile they come from, by channel id, for the sources that build folders
// from channel profiles; nil for the others. Every path is listed, not only
// the deepest, and a path two profiles share is listed once for each, so Silo
// can drop the folders of profiles a user did not select and keep the deepest
// of the rest. With every profile selected the result is the folders
// livetvChannelFolders returns.
func livetvChannelProfileFolders(settings livetvFolderSettings, channels []model.Channel) map[string][]livetvProfileFolder {
	tree := livetvFolderTree{livetvFolderSettings: settings}
	if tree.source != "profile" && !tree.useProfileGroups() {
		return nil
	}
	tree.wrappers = tree.redundantProfileGroupWrappers(channels)
	out := make(map[string][]livetvProfileFolder, len(channels))
	for _, channel := range channels {
		ids := channel.ProfileIDs
		if tree.activeProfile != "" {
			ids = []string{tree.activeProfile}
		}
		// The paths that come from no profile are what is left with none.
		none := tree
		none.activeProfile = ""
		bare := channel
		bare.ProfileIDs = nil
		untagged := map[string]bool{}
		var folders []livetvProfileFolder
		for _, path := range none.virtualPaths(bare) {
			untagged[path] = true
			folders = append(folders, livetvProfileFolder{Path: path})
		}
		for _, id := range ids {
			one := tree
			one.activeProfile = id
			for _, path := range one.virtualPaths(channel) {
				if !untagged[path] {
					folders = append(folders, livetvProfileFolder{Path: path, ProfileID: id})
				}
			}
		}
		if len(folders) == 0 {
			if path := tree.sourceGroupPath(channel); path != "" {
				folders = []livetvProfileFolder{{Path: path}}
			}
		}
		out[channel.ID] = folders
	}
	return out
}

func livetvLeafFolders(paths []string) []string {
	var leaves []string
	for _, path := range paths {
		parent := false
		for _, other := range paths {
			if strings.HasPrefix(other, path+livetvFolderSeparator) {
				parent = true
				break
			}
		}
		if !parent {
			leaves = append(leaves, path)
		}
	}
	return leaves
}

type livetvFolderTree struct {
	livetvFolderSettings
	wrappers map[string]string
}

func (t livetvFolderTree) useSourceGroups() bool {
	return t.source == "group" || t.source == "group_channel" || t.source == "profile_group"
}

func (t livetvFolderTree) useChannelNames() bool {
	return t.source == "channel" || t.source == "group_channel"
}

func (t livetvFolderTree) useProfileGroups() bool { return t.source == "profile_group" }

// virtualPaths ports virtualPathsForChannel.
func (t livetvFolderTree) virtualPaths(channel model.Channel) []string {
	if t.source == "profile" {
		return livetvUnique(t.profilePaths(channel))
	}
	if t.useProfileGroups() {
		return livetvUnique(t.profileVirtualPaths(channel))
	}
	var paths []string
	sourcePath := t.sourceVirtualPath(channel)
	if sourcePath != "" && t.useSourceGroups() {
		paths = append(paths, sourcePath)
		paths = append(paths, t.aliasPaths(sourcePath)...)
		if t.source == "group_channel" {
			if combined := t.appendVirtualPathParts(sourcePath, livetvChannelNameFolderParts(channel)); combined != "" {
				paths = append(paths, combined)
				paths = append(paths, t.aliasPaths(combined)...)
			}
		}
	}
	if t.useChannelNames() {
		paths = append(paths, t.inferredChannelNamePaths(channel)...)
	}
	return livetvUnique(paths)
}

func (t livetvFolderTree) profileVirtualPaths(channel model.Channel) []string {
	sourcePath := t.sourceGroupPath(channel)
	if sourcePath == "" {
		return nil
	}
	sourceParts := livetvSplitPath(sourcePath)
	aliasPaths := t.aliasPaths(sourcePath)
	groupPaths := append([]string{sourcePath}, aliasPaths...)
	marketParts := livetvLocalMarketParts(channel, sourceParts)
	paths := append([]string(nil), aliasPaths...)
	for _, profilePath := range t.profilePaths(channel) {
		paths = append(paths, profilePath)
		for _, groupPath := range groupPaths {
			groupParts := livetvSplitPath(groupPath)
			if wrapper := t.wrappers[strings.ToLower(profilePath)]; wrapper != "" && len(groupParts) > 1 && strings.ToLower(groupParts[0]) == wrapper {
				groupParts = groupParts[1:]
			}
			combined := t.appendVirtualPathParts(profilePath, groupParts)
			if combined != "" {
				paths = append(paths, combined)
			}
			base := combined
			if base == "" {
				base = profilePath + livetvFolderSeparator + groupPath
			}
			if market := t.appendVirtualPathParts(base, marketParts); market != "" {
				paths = append(paths, market)
			}
		}
	}
	return livetvUnique(paths)
}

// redundantProfileGroupWrappers ports the web app function of that name: a
// profile whose channels all sit in groups under one top-level name, with more
// than one group below it, drops that name from its folders.
func (t livetvFolderTree) redundantProfileGroupWrappers(channels []model.Channel) map[string]string {
	if !t.flatten || !t.useProfileGroups() {
		return nil
	}
	type candidate struct {
		wrapper  string
		eligible bool
		tails    map[string]bool
	}
	candidates := map[string]*candidate{}
	for _, channel := range channels {
		groupParts := livetvSplitPath(t.sourceGroupPath(channel))
		if len(groupParts) == 0 {
			continue
		}
		wrapper := strings.ToLower(groupParts[0])
		tail := strings.ToLower(strings.Join(groupParts[1:], livetvFolderSeparator))
		for _, profilePath := range t.profilePaths(channel) {
			key := strings.ToLower(profilePath)
			c := candidates[key]
			if c == nil {
				c = &candidate{wrapper: wrapper, eligible: len(groupParts) > 1, tails: map[string]bool{}}
				if len(groupParts) > 1 {
					c.tails[tail] = true
				}
				candidates[key] = c
				continue
			}
			if c.wrapper != wrapper || len(groupParts) < 2 {
				c.eligible = false
				continue
			}
			c.tails[tail] = true
		}
	}
	wrappers := map[string]string{}
	for key, c := range candidates {
		if c.eligible && len(c.tails) > 1 {
			wrappers[key] = c.wrapper
		}
	}
	return wrappers
}

func (t livetvFolderTree) profilePaths(channel model.Channel) []string {
	ids := channel.ProfileIDs
	if t.activeProfile != "" {
		ids = []string{t.activeProfile}
	}
	var paths []string
	for _, id := range ids {
		name := t.profileNames[id]
		if name == "" {
			name = id
		}
		if path := livetvProfilePath(name); path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}

func livetvProfilePath(name string) string {
	var parts []string
	for _, part := range strings.Split(name, "|") {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, livetvFolderSeparator)
}

// sourceLabel ports sourceCategoryLabel: the category's name with a leading
// "*" (featured) removed.
func (t livetvFolderTree) sourceLabel(channel model.Channel) string {
	raw := t.categoryLabels[channel.CategoryID]
	if raw == "" {
		raw = channel.CategoryName
	}
	return livetvCategoryDisplayName(raw)
}

func livetvCategoryDisplayName(name string) string {
	name = strings.TrimSpace(name)
	if strings.HasPrefix(name, "*") {
		return strings.TrimSpace(name[1:])
	}
	return name
}

func (t livetvFolderTree) sourceVirtualPath(channel model.Channel) string {
	return strings.Join(t.parsedCategoryPath(t.sourceLabel(channel)), livetvFolderSeparator)
}

func (t livetvFolderTree) sourceGroupPath(channel model.Channel) string {
	if path := t.sourceVirtualPath(channel); path != "" {
		return path
	}
	return livetvCategoryDisplayName(t.sourceLabel(channel))
}

var (
	livetvPipeSplit  = regexp.MustCompile(`\s*\|\s*`)
	livetvDashSplit  = regexp.MustCompile(`\s+-\s*`)
	livetvSlashSplit = regexp.MustCompile(`\s*/\s*`)
	livetvSpaces     = regexp.MustCompile(`\s+`)
)

func livetvNameToken(value string) string {
	return livetvSpaces.ReplaceAllString(strings.TrimSpace(value), " ")
}

func livetvNonEmptyTokens(parts []string) []string {
	var out []string
	for _, part := range parts {
		if part = livetvNameToken(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// parsedCategoryPath ports parsedCategoryPath: a name splits into a path only
// when delimiter parsing is on and it has more than one part.
func (t livetvFolderTree) parsedCategoryPath(name string) []string {
	name = livetvNameToken(name)
	if !t.delimiter || name == "" {
		return nil
	}
	pattern := livetvPipeSplit
	if t.dash {
		pattern = livetvDashSplit
	}
	parts := livetvNonEmptyTokens(pattern.Split(name, -1))
	if len(parts) > 1 {
		return parts
	}
	return nil
}

func (t livetvFolderTree) configuredPath(value string) string {
	display := livetvCategoryDisplayName(value)
	if parts := t.parsedCategoryPath(display); len(parts) > 1 {
		return strings.Join(parts, livetvFolderSeparator)
	}
	if parts := livetvNonEmptyTokens(livetvSlashSplit.Split(display, -1)); len(parts) > 1 {
		return strings.Join(parts, livetvFolderSeparator)
	}
	return ""
}

func (t livetvFolderTree) configuredPathOrLabel(value string) string {
	if path := t.configuredPath(value); path != "" {
		return path
	}
	return strings.TrimSpace(livetvCategoryDisplayName(value))
}

// aliasPaths ports aliasVirtualPathsForSourcePath.
func (t livetvFolderTree) aliasPaths(sourcePath string) []string {
	normalized := t.configuredPathOrLabel(sourcePath)
	if normalized == "" {
		return nil
	}
	var paths []string
	for _, alias := range t.aliases {
		from, to := t.configuredPathOrLabel(alias[0]), t.configuredPathOrLabel(alias[1])
		if from == "" || to == "" {
			continue
		}
		if normalized != from && !strings.HasPrefix(normalized, from+livetvFolderSeparator) {
			continue
		}
		remapped := to
		if normalized != from {
			remapped = to + livetvFolderSeparator + normalized[len(from)+len(livetvFolderSeparator):]
		}
		paths = append(paths, remapped)
	}
	return livetvUnique(paths)
}

func livetvChannelNameParts(channel model.Channel) []string {
	return livetvNonEmptyTokens(strings.Split(channel.Name, "|"))
}

func livetvChannelNameFolderParts(channel model.Channel) []string {
	parts := livetvChannelNameParts(channel)
	if len(parts) < 2 {
		return nil
	}
	return parts[:len(parts)-1]
}

func livetvSplitPath(path string) []string {
	return livetvNonEmptyTokens(strings.Split(path, livetvFolderSeparator))
}

// appendVirtualPathParts ports DispatcharrLineup.appendVirtualPathParts.
func (t livetvFolderTree) appendVirtualPathParts(basePath string, extra []string) string {
	baseParts := livetvSplitPath(basePath)
	additions := livetvNonEmptyTokens(extra)
	if len(baseParts) == 0 || len(additions) == 0 {
		return ""
	}
	if !t.collapse {
		return strings.Join(append(baseParts, additions...), livetvFolderSeparator)
	}
	return livetvAppendUnduplicated(baseParts, additions)
}

func livetvAppendUnduplicated(baseParts, additions []string) string {
	shared := 0
	for shared < len(baseParts) && shared < len(additions) && strings.EqualFold(baseParts[shared], additions[shared]) {
		shared++
	}
	additions = additions[shared:]
	for len(additions) > 0 && len(baseParts) > 0 && strings.EqualFold(baseParts[len(baseParts)-1], additions[0]) {
		additions = additions[1:]
	}
	if len(additions) == 0 {
		return ""
	}
	return strings.Join(append(append([]string(nil), baseParts...), additions...), livetvFolderSeparator)
}

var livetvUSStates = map[string]bool{
	"AL": true, "AK": true, "AZ": true, "AR": true, "CA": true, "CO": true, "CT": true, "DE": true, "FL": true, "GA": true,
	"HI": true, "ID": true, "IL": true, "IN": true, "IA": true, "KS": true, "KY": true, "LA": true, "ME": true, "MD": true,
	"MA": true, "MI": true, "MN": true, "MS": true, "MO": true, "MT": true, "NE": true, "NV": true, "NH": true, "NJ": true,
	"NM": true, "NY": true, "NC": true, "ND": true, "OH": true, "OK": true, "OR": true, "PA": true, "RI": true, "SC": true,
	"SD": true, "TN": true, "TX": true, "UT": true, "VT": true, "VA": true, "WA": true, "WV": true, "WI": true, "WY": true,
	"DC": true,
}

var (
	livetvInternationalSports = regexp.MustCompile(`\b(sport|sports|team|teams|league|football|soccer|futbol|fútbol|mlb|nba|nfl|nhl|mls|f1)\b`)
	livetvInternationalNews   = regexp.MustCompile(`\b(news|noticias|nouvelles)\b`)
	livetvInternationalKids   = regexp.MustCompile(`\b(kids|children|cartoon|junior)\b`)
)

// inferredChannelNamePaths ports inferredChannelNameGroupPaths: "NY | WABC"
// files under US TV / Locals / NY, anything else under an International root.
func (t livetvFolderTree) inferredChannelNamePaths(channel model.Channel) []string {
	parts := livetvChannelNameParts(channel)
	if len(parts) < 2 {
		return nil
	}
	first, second := parts[0], parts[1]
	if livetvUSStates[strings.ToUpper(first)] {
		state := strings.ToUpper(first)
		return livetvUnique([]string{"US TV / Locals / " + state, "US TV / Locals / " + state + " / " + second})
	}
	label := t.sourceLabel(channel)
	text := strings.ToLower(strings.Join(parts, " ") + " " + label + " " + label)
	root := "International TV"
	switch {
	case livetvInternationalSports.MatchString(text):
		root = "International Sports"
	case livetvInternationalNews.MatchString(text):
		root = "International News"
	case livetvInternationalKids.MatchString(text):
		root = "International Kids"
	}
	return livetvUnique([]string{root + " / " + first, root + " / " + first + " / " + second})
}

func livetvLocalMarketParts(channel model.Channel, sourceParts []string) []string {
	if len(sourceParts) == 0 {
		return nil
	}
	tail := strings.ToLower(sourceParts[len(sourceParts)-1])
	if tail != "local" && tail != "locals" {
		return nil
	}
	parts := livetvChannelNameParts(channel)
	if len(parts) < 2 {
		return nil
	}
	if livetvUSStates[strings.ToUpper(parts[0])] {
		return parts[1:2]
	}
	return parts[:1]
}

func livetvUnique(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
