package plugin

import (
	"sort"
	"strings"
	"time"
)

func mergeSportsGuideEvents(events, guideEvents []SportsEvent) []SportsEvent {
	merged := cloneSportsEvents(events)
	keys := make([]sportsMatchupKey, 0, len(merged)+len(guideEvents))
	index := newSportsEventStartIndex()
	for position, event := range merged {
		index.add(position, event.StartUnix)
		keys = append(keys, newSportsMatchupKey(event))
	}
	var candidates []int
	for _, guideEvent := range guideEvents {
		matched := false
		key := newSportsMatchupKey(guideEvent)
		candidates = index.candidates(guideEvent.StartUnix, candidates[:0])
		for _, position := range candidates {
			if !keys[position].mayMatch(key) || !sportsEventsSameMatchup(merged[position], guideEvent) {
				continue
			}
			before := merged[position].StartUnix
			overlaySportsGuideEvent(&merged[position], guideEvent)
			index.move(position, before, merged[position].StartUnix)
			keys[position] = newSportsMatchupKey(merged[position])
			matched = true
			break
		}
		if !matched {
			if !guideEvent.Completed {
				replayCandidates := candidates
				if guideEvent.Live || guideEvent.StartUnix <= 0 {
					// A live listing may replay a provider game of any age.
					replayCandidates = index.all(candidates[:0])
				}
				for _, position := range replayCandidates {
					providerEvent := merged[position]
					recent := guideEvent.Live || guideEvent.StartUnix-providerEvent.StartUnix < 36*3600
					if providerEvent.Completed && providerEvent.StartUnix < guideEvent.StartUnix && recent && sportsEventsSameIdentity(providerEvent, guideEvent) {
						guideEvent.Status = "replay"
						guideEvent.StatusText = "Replay"
						break
					}
				}
			}
			index.add(len(merged), guideEvent.StartUnix)
			merged = append(merged, guideEvent)
			keys = append(keys, newSportsMatchupKey(guideEvent))
		}
	}
	return collapseEquivalentSportsEvents(merged)
}

func collapseEquivalentSportsEvents(events []SportsEvent) []SportsEvent {
	collapsed := make([]SportsEvent, 0, len(events))
	keys := make([]sportsMatchupKey, 0, len(events))
	index := newSportsEventStartIndex()
	var candidates []int
	for _, event := range events {
		merged := false
		key := newSportsMatchupKey(event)
		candidates = index.candidates(event.StartUnix, candidates[:0])
		for _, position := range candidates {
			if !keys[position].mayMatch(key) || !sportsEventsSameMatchup(collapsed[position], event) {
				continue
			}
			before := collapsed[position].StartUnix
			overlaySportsGuideEvent(&collapsed[position], event)
			index.move(position, before, collapsed[position].StartUnix)
			keys[position] = newSportsMatchupKey(collapsed[position])
			merged = true
			break
		}
		if !merged {
			index.add(len(collapsed), event.StartUnix)
			collapsed = append(collapsed, event)
			keys = append(keys, key)
		}
	}
	return collapsed
}

// sportsMatchupKey caches the normalized team words sportsEventsSameMatchup
// needs to rule out a swapped-copy match without renormalizing names.
type sportsMatchupKey struct {
	start               int64
	phased              bool
	homeFirst, homeLast string
	awayFirst, awayLast string
}

func newSportsMatchupKey(event SportsEvent) sportsMatchupKey {
	key := sportsMatchupKey{start: event.StartUnix, phased: event.guidePhase != ""}
	key.homeFirst, key.homeLast = sportsFirstLastWords(event.Home.Name)
	key.awayFirst, key.awayLast = sportsFirstLastWords(event.Away.Name)
	return key
}

func sportsFirstLastWords(name string) (string, string) {
	words := strings.Fields(normalizeMatchText(name))
	if len(words) == 0 {
		return "", ""
	}
	return words[0], words[len(words)-1]
}

// mayMatch is a necessary condition for sportsEventsSameMatchup. Outside the
// 6h (or 30h phased) window only a swapped home/away copy can match, and two
// team names can only match when their first or last words agree.
func (key sportsMatchupKey) mayMatch(other sportsMatchupKey) bool {
	if key.start <= 0 || other.start <= 0 {
		return true
	}
	difference := key.start - other.start
	if difference < 0 {
		difference = -difference
	}
	if difference <= int64(6*time.Hour/time.Second) || ((key.phased || other.phased) && difference <= int64(30*time.Hour/time.Second)) {
		return true
	}
	if difference > sportsMatchupMaxWindowSeconds {
		return false
	}
	related := func(firstA, lastA, firstB, lastB string) bool {
		return firstA != "" && firstB != "" && (firstA == firstB || lastA == lastB)
	}
	return related(key.homeFirst, key.homeLast, other.awayFirst, other.awayLast) && related(key.awayFirst, key.awayLast, other.homeFirst, other.homeLast)
}

const (
	sportsEventBucketSeconds = int64(15 * 60)
	// The widest window sportsEventsSameMatchup accepts (swapped home/away).
	sportsMatchupMaxWindowSeconds = int64(36 * 3600)
)

// sportsEventStartIndex buckets event positions by 15-minute start time, so
// a merge only compares events whose starts can fall in the same matchup
// window. Undated events stay candidates for everything, as before.
type sportsEventStartIndex struct {
	buckets map[int64][]int
	undated []int
	count   int
}

func newSportsEventStartIndex() *sportsEventStartIndex {
	return &sportsEventStartIndex{buckets: map[int64][]int{}}
}

func sportsEventBucket(start int64) int64 {
	bucket := start / sportsEventBucketSeconds
	if start < 0 && start%sportsEventBucketSeconds != 0 {
		bucket--
	}
	return bucket
}

func (index *sportsEventStartIndex) add(position int, start int64) {
	if position >= index.count {
		index.count = position + 1
	}
	if start <= 0 {
		index.undated = append(index.undated, position)
		return
	}
	bucket := sportsEventBucket(start)
	index.buckets[bucket] = append(index.buckets[bucket], position)
}

func (index *sportsEventStartIndex) move(position int, before, after int64) {
	if before == after || (before <= 0 && after <= 0) || (before > 0 && after > 0 && sportsEventBucket(before) == sportsEventBucket(after)) {
		return
	}
	remove := func(list []int) []int {
		for offset, value := range list {
			if value == position {
				return append(list[:offset], list[offset+1:]...)
			}
		}
		return list
	}
	if before <= 0 {
		index.undated = remove(index.undated)
	} else {
		bucket := sportsEventBucket(before)
		index.buckets[bucket] = remove(index.buckets[bucket])
	}
	index.add(position, after)
}

func (index *sportsEventStartIndex) all(buffer []int) []int {
	for position := 0; position < index.count; position++ {
		buffer = append(buffer, position)
	}
	return buffer
}

// candidates returns, in insertion order, every position that could share a
// matchup with an event starting at start.
func (index *sportsEventStartIndex) candidates(start int64, buffer []int) []int {
	if start <= 0 {
		return index.all(buffer)
	}
	buffer = append(buffer, index.undated...)
	first := sportsEventBucket(start - sportsMatchupMaxWindowSeconds)
	last := sportsEventBucket(start + sportsMatchupMaxWindowSeconds)
	for bucket := first; bucket <= last; bucket++ {
		buffer = append(buffer, index.buckets[bucket]...)
	}
	sort.Ints(buffer)
	return buffer
}

func overlaySportsGuideEvent(base *SportsEvent, extra SportsEvent) {
	swapped := sportsTeamPairsSwapped(*base, extra)
	// A reversed broadcast copy of a finished game must not replace the upcoming listing.
	preferUpcoming := swapped && base.Completed && !extra.Completed
	sameCompletion := !swapped || base.Completed == extra.Completed
	if preferUpcoming || (sameCompletion && sportsEventListingRank(extra) > sportsEventListingRank(*base)) {
		kept := *base
		*base = extra
		extra = kept
	}
	base.Channels = mergeSportsChannelMatches(extra.Channels, base.Channels)
	if (base.LeagueID == "" || base.LeagueID == "sports") && extra.LeagueID != "" && extra.LeagueID != "sports" {
		base.LeagueID = extra.LeagueID
		base.LeagueName = extra.LeagueName
		base.SportName = firstNonEmpty(extra.SportName, base.SportName)
		base.LeagueLogoURL = firstNonEmpty(extra.LeagueLogoURL, base.LeagueLogoURL)
	} else if extra.SportName != "" && extra.SportName != "Sports" {
		base.LeagueID = extra.LeagueID
		base.LeagueName = extra.LeagueName
		base.SportName = extra.SportName
		base.LeagueLogoURL = firstNonEmpty(extra.LeagueLogoURL, base.LeagueLogoURL)
	}
	home, away := extra.Home, extra.Away
	sameSides := sportsTeamNameMatch(base.Home.Name, extra.Home.Name).ok && sportsTeamNameMatch(base.Away.Name, extra.Away.Name).ok
	swappedSides := sportsTeamNameMatch(base.Home.Name, extra.Away.Name).ok && sportsTeamNameMatch(base.Away.Name, extra.Home.Name).ok
	if swappedSides && !sameSides {
		home, away = extra.Away, extra.Home
		extra.HomeScore, extra.AwayScore = extra.AwayScore, extra.HomeScore
	}
	if sportsTeamNameMatch(base.Home.Name, home.Name).ok {
		primary, supplemental := preferSportsTeamIdentity(base.Home, home)
		base.Home = mergeSportsTeamIdentity(primary, supplemental)
	}
	if sportsTeamNameMatch(base.Away.Name, away.Name).ok {
		primary, supplemental := preferSportsTeamIdentity(base.Away, away)
		base.Away = mergeSportsTeamIdentity(primary, supplemental)
	}
	if extra.Live && base.Status == "scheduled" {
		base.Live = true
		base.Status = "airing"
		base.StatusText = "On now"
	}
	if base.HomeScore == "" {
		base.HomeScore = extra.HomeScore
	}
	if base.AwayScore == "" {
		base.AwayScore = extra.AwayScore
	}
	if base.Period == "" {
		base.Period = extra.Period
	}
	if base.Clock == "" {
		base.Clock = extra.Clock
	}
	if base.Venue == "" {
		base.Venue = extra.Venue
	}
	if base.Description == "" {
		base.Description = extra.Description
	}
	if base.guidePhase != "" && extra.guidePhase == "" {
		base.guidePhase = ""
	}
}

func sportsEventsSameMatchup(left, right SportsEvent) bool {
	if left.StartUnix > 0 && right.StartUnix > 0 {
		difference := left.StartUnix - right.StartUnix
		if difference < 0 {
			difference = -difference
		}
		// Windows: 6h by default, 30h when either side is a pregame/postgame
		// listing (pregame can start at midnight, postgame runs after the
		// game), 36h for swapped home/away copies. The swap check is costly, so
		// it only runs when the shorter windows do not already decide.
		phased := left.guidePhase != "" || right.guidePhase != ""
		switch {
		case difference <= int64(6*time.Hour/time.Second):
		case phased && difference <= int64(30*time.Hour/time.Second):
		case difference > sportsMatchupMaxWindowSeconds || !sportsTeamPairsSwapped(left, right):
			return false
		}
	}
	return sportsEventsSameIdentity(left, right)
}

func sportsEventListingRank(event SportsEvent) int {
	rank := 0
	switch event.guidePhase {
	case "live":
		rank += 9
	case "":
		rank += 8
	}
	if event.Venue != "" {
		rank += 3
	}
	if event.HomeScore != "" && event.AwayScore != "" {
		rank += 2
	}
	if event.LeagueID != "" && event.LeagueID != "sports" {
		rank += 4
	}
	if event.Home.LogoURL != "" {
		rank += 2
	}
	if event.Away.LogoURL != "" {
		rank += 2
	}
	rank += len(normalizeMatchText(event.Home.Name)) + len(normalizeMatchText(event.Away.Name))
	if event.ImageURL != "" || event.Artwork != nil {
		rank += 3
	}
	return rank
}

func sportsTeamPairsSwapped(left, right SportsEvent) bool {
	sameSides := sportsTeamNameMatch(left.Home.Name, right.Home.Name).ok && sportsTeamNameMatch(left.Away.Name, right.Away.Name).ok
	if sameSides {
		return false
	}
	homeAway := sportsTeamNameMatch(left.Home.Name, right.Away.Name)
	awayHome := sportsTeamNameMatch(left.Away.Name, right.Home.Name)
	return homeAway.ok && awayHome.ok && (homeAway.strong || awayHome.strong)
}

func sportsEventsSameIdentity(left, right SportsEvent) bool {
	if sportsTeamPairsEquivalent(left, right) {
		return true
	}
	leftText := normalizeMatchText(strings.Join([]string{left.Name, left.ShortName, left.Away.Name, left.Home.Name}, " "))
	rightText := normalizeMatchText(strings.Join([]string{right.Name, right.ShortName, right.Away.Name, right.Home.Name}, " "))
	return strongSportsGuideMatch(leftText, right) && strongSportsGuideMatch(rightText, left)
}

type sportsTeamNameRelation struct {
	ok     bool
	strong bool
}

func sportsTeamNameMatch(left, right string) sportsTeamNameRelation {
	leftNorm := normalizeMatchText(left)
	rightNorm := normalizeMatchText(right)
	if leftNorm == "" || rightNorm == "" {
		return sportsTeamNameRelation{}
	}
	if leftNorm == rightNorm {
		return sportsTeamNameRelation{ok: true, strong: true}
	}
	leftWords := strings.Fields(leftNorm)
	rightWords := strings.Fields(rightNorm)
	if sportsWordListsPrefix(leftWords, rightWords) || sportsWordListsPrefix(rightWords, leftWords) {
		shorter := leftWords
		if len(rightWords) < len(leftWords) {
			shorter = rightWords
		}
		if len(shorter) == 1 && len([]rune(shorter[0])) >= 4 {
			return sportsTeamNameRelation{ok: true, strong: true}
		}
		if len(shorter) >= 2 {
			return sportsTeamNameRelation{ok: true, strong: false}
		}
	}
	leftNick := leftWords[len(leftWords)-1]
	rightNick := rightWords[len(rightWords)-1]
	if len([]rune(leftNick)) >= 4 && leftNick == rightNick {
		return sportsTeamNameRelation{ok: true, strong: true}
	}
	return sportsTeamNameRelation{}
}

func sportsWordListsPrefix(short, long []string) bool {
	if len(short) == 0 || len(short) >= len(long) {
		return false
	}
	for index, word := range short {
		if word != long[index] {
			return false
		}
	}
	return true
}

func sportsTeamPairsEquivalent(left, right SportsEvent) bool {
	pairs := [][2]sportsTeamNameRelation{
		{sportsTeamNameMatch(left.Home.Name, right.Home.Name), sportsTeamNameMatch(left.Away.Name, right.Away.Name)},
		{sportsTeamNameMatch(left.Home.Name, right.Away.Name), sportsTeamNameMatch(left.Away.Name, right.Home.Name)},
	}
	for _, pair := range pairs {
		if !pair[0].ok || !pair[1].ok {
			continue
		}
		if pair[0].strong || pair[1].strong {
			return true
		}
	}
	return false
}
