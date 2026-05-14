package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"road-proxy-v3/internal/plugin"
)

type compatProfile struct {
	SchemaVersion          string            `json:"schema_version"`
	ID                     string            `json:"id"`
	DisplayName            string            `json:"display_name"`
	ROADScope              string            `json:"road_scope"`
	SteamLobbySupported    bool              `json:"steam_lobby_supported"`
	RequiresGameLaunchArgs bool              `json:"requires_game_launch_args"`
	Match                  []string          `json:"match"`
	ExeNames               []string          `json:"exe_names"`
	SteamAppIDs            []int             `json:"steam_app_ids"`
	ExeHashes              map[string]string `json:"exe_hashes"`
	PluginName             string            `json:"plugin_name"`
	Network                string            `json:"network"`
	TargetHost             string            `json:"target_host"`
	TargetPort             int               `json:"target_port"`
	ClientListenPort       int               `json:"client_listen_port"`
	UDPPeerBroadcast       bool              `json:"udp_peer_broadcast"`
	UDPReplyPolicy         string            `json:"udp_reply_policy"`
	KnownPorts             []compatKnownPort `json:"known_ports"`
	NoteKeys               []string          `json:"note_keys"`
	LaunchAdviceKeys       []string          `json:"launch_advice_keys"`
}

type compatKnownPort struct {
	Network string `json:"network"`
	Port    int    `json:"port"`
	Role    string `json:"role,omitempty"`
	Notes   string `json:"notes,omitempty"`
}

type compatMatch struct {
	Profile    *compatProfile `json:"-"`
	Confidence string         `json:"confidence"`
	Reasons    []string       `json:"reasons"`
	Score      int            `json:"score"`
}

var (
	compatProfilesCache  []compatProfile
	compatProfilesLoaded bool
)

func matchCompatProfile(processName string, summary *captureSummary) *compatProfile {
	return matchCompatProfileWithReasons(processName, summary).Profile
}

func matchCompatProfileWithReasons(processName string, summary *captureSummary) compatMatch {
	name := normalizeProfileText(processName)

	best := compatMatch{Confidence: "unknown"}
	profiles := getCompatProfiles()
	for i := range profiles {
		profile := &profiles[i]
		match := evaluateCompatProfile(profile, name, summary)
		if match.Profile == nil {
			continue
		}
		if match.Score > best.Score || (match.Score == best.Score && profile.ID < best.ProfileID()) {
			best = match
		}
	}
	if best.Profile == nil {
		return compatMatch{Confidence: "unknown"}
	}
	best.Confidence = confidenceForScore(best.Score)
	return best
}

func (m compatMatch) ProfileID() string {
	if m.Profile == nil {
		return ""
	}
	return m.Profile.ID
}

func evaluateCompatProfile(profile *compatProfile, processName string, summary *captureSummary) compatMatch {
	score := 0
	reasons := []string{}

	if token, ok := profileMatchesProcess(profile, processName, summary); ok {
		score += 25
		reasons = append(reasons, "exe_name_match:"+token)
	}
	if profilePortSeen(profile, summary) {
		score += 20
		reasons = append(reasons, fmt.Sprintf("port_match:%s/%d", profile.Network, profile.TargetPort))
	}
	if summary != nil && profile.Network != "" && profile.Network == summary.RecommendedNet {
		score += 10
		reasons = append(reasons, "protocol_match:"+profile.Network)
	}
	if appID, ok := profileSteamAppIDMatches(profile, summary); ok {
		score += 30
		reasons = append(reasons, fmt.Sprintf("steam_app_id_match:%d", appID))
	}
	if algorithm, ok := profileExeHashMatches(profile, summary); ok {
		score += 40
		reasons = append(reasons, "exe_hash_match:"+algorithm)
	}
	if profile.ROADScope != "" {
		reasons = append(reasons, "road_scope:"+profile.ROADScope)
	}
	if !profile.SteamLobbySupported {
		reasons = append(reasons, "steam_lobby_not_supported")
	}
	if profile.RequiresGameLaunchArgs {
		reasons = append(reasons, "launch_args_required")
	}

	if score == 0 {
		return compatMatch{Confidence: "unknown"}
	}
	return compatMatch{
		Profile: profile,
		Reasons: reasons,
		Score:   score,
	}
}

func profileMatchesProcess(profile *compatProfile, processName string, summary *captureSummary) (string, bool) {
	for _, token := range profile.matchTokens() {
		token = normalizeProfileText(token)
		if token == "" || !strings.Contains(processName, token) {
			continue
		}
		if profile.ID == "minecraft-java" && (token == "java" || token == "javaw") && !profilePortSeen(profile, summary) {
			continue
		}
		return token, true
	}
	return "", false
}

func profilePortSeen(profile *compatProfile, summary *captureSummary) bool {
	if profile == nil || summary == nil || profile.TargetPort <= 0 || profile.Network == "" {
		return false
	}
	return portHitCount(summary.LocalPortHits[profile.Network], profile.TargetPort) > 0 ||
		portHitCount(summary.RemotePortHits[profile.Network], profile.TargetPort) > 0
}

func profileSteamAppIDMatches(profile *compatProfile, summary *captureSummary) (int, bool) {
	if profile == nil || summary == nil || summary.SteamAppID <= 0 {
		return 0, false
	}
	for _, appID := range profile.SteamAppIDs {
		if appID == summary.SteamAppID {
			return appID, true
		}
	}
	return 0, false
}

func profileExeHashMatches(profile *compatProfile, summary *captureSummary) (string, bool) {
	if profile == nil || summary == nil || summary.ProcessSHA256 == "" || len(profile.ExeHashes) == 0 {
		return "", false
	}
	got := normalizeHash(summary.ProcessSHA256)
	for algorithm, want := range profile.ExeHashes {
		if strings.EqualFold(strings.TrimSpace(algorithm), "sha256") && got == normalizeHash(want) {
			return "sha256", true
		}
	}
	return "", false
}

func confidenceForScore(score int) string {
	switch {
	case score >= 80:
		return "platinum"
	case score >= 45:
		return "gold"
	case score >= 30:
		return "silver"
	case score >= 10:
		return "bronze"
	default:
		return "unknown"
	}
}

func profileNotes(profile *compatProfile) []string {
	if profile == nil {
		return nil
	}
	notes := make([]string, 0, len(profile.NoteKeys)+len(profile.LaunchAdviceKeys))
	for _, key := range profile.NoteKeys {
		notes = append(notes, sm(key))
	}
	for _, key := range profile.LaunchAdviceKeys {
		notes = append(notes, fmt.Sprintf(sm("studio.profile.launch_prefix"), sm(key)))
	}
	return notes
}

func getCompatProfiles() []compatProfile {
	if compatProfilesLoaded {
		return compatProfilesCache
	}
	profiles, err := loadCompatProfiles()
	if err != nil {
		compatProfilesCache = []compatProfile{}
		compatProfilesLoaded = true
		return compatProfilesCache
	}
	compatProfilesCache = profiles
	compatProfilesLoaded = true
	return compatProfilesCache
}

func loadCompatProfiles() ([]compatProfile, error) {
	dir := resolveStudioExistingPath("compat-profiles")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	profiles := make([]compatProfile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") || strings.HasPrefix(entry.Name(), "_") {
			continue
		}
		profile, err := loadCompatProfileFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].ID < profiles[j].ID
	})
	return profiles, nil
}

func loadCompatProfileFile(path string) (compatProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return compatProfile{}, err
	}
	var profile compatProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return compatProfile{}, err
	}
	profile.normalize()
	if err := profile.validate(); err != nil {
		return compatProfile{}, fmt.Errorf("%s: %w", path, err)
	}
	return profile, nil
}

func (p *compatProfile) normalize() {
	p.ID = strings.TrimSpace(p.ID)
	p.DisplayName = strings.TrimSpace(p.DisplayName)
	p.Network = strings.ToLower(strings.TrimSpace(p.Network))
	p.TargetHost = strings.TrimSpace(p.TargetHost)
	if p.TargetHost == "" {
		p.TargetHost = "127.0.0.1"
	}
	p.ROADScope = strings.TrimSpace(p.ROADScope)
	if p.ROADScope == "" {
		p.ROADScope = "direct_lan_local_port_only"
	}
	p.UDPReplyPolicy = strings.ToLower(strings.TrimSpace(p.UDPReplyPolicy))
	if p.Network == "udp" && p.UDPReplyPolicy == "" {
		p.UDPReplyPolicy = plugin.UDPReplyPolicyAny
	}
}

func (p compatProfile) validate() error {
	if p.SchemaVersion != "compat_profile.v1" {
		return fmt.Errorf("schema_version must be compat_profile.v1")
	}
	if p.ID == "" {
		return fmt.Errorf("id is required")
	}
	if p.DisplayName == "" {
		return fmt.Errorf("display_name is required")
	}
	if len(p.matchTokens()) == 0 {
		return fmt.Errorf("match or exe_names is required")
	}
	if p.PluginName == "" {
		return fmt.Errorf("plugin_name is required")
	}
	switch p.Network {
	case "tcp", "udp":
	default:
		return fmt.Errorf("network must be tcp or udp")
	}
	if p.TargetPort <= 0 || p.TargetPort > 65535 {
		return fmt.Errorf("target_port must be in 1-65535")
	}
	if p.ClientListenPort < 0 || p.ClientListenPort > 65535 {
		return fmt.Errorf("client_listen_port must be in 0-65535")
	}
	if p.ROADScope != "direct_lan_local_port_only" {
		return fmt.Errorf("road_scope must be direct_lan_local_port_only")
	}
	switch p.UDPReplyPolicy {
	case "", plugin.UDPReplyPolicyAny, plugin.UDPReplyPolicySameIP, plugin.UDPReplyPolicyStrict:
	default:
		return fmt.Errorf("udp_reply_policy must be any, same_ip, or strict")
	}
	for i, knownPort := range p.KnownPorts {
		if err := knownPort.validate(); err != nil {
			return fmt.Errorf("known_ports[%d]: %w", i, err)
		}
	}
	return nil
}

func (p compatProfile) matchTokens() []string {
	out := make([]string, 0, len(p.Match)+len(p.ExeNames))
	out = append(out, p.Match...)
	out = append(out, p.ExeNames...)
	return out
}

func (p compatKnownPort) validate() error {
	switch p.Network {
	case "tcp", "udp":
	default:
		return fmt.Errorf("network must be tcp or udp")
	}
	if p.Port <= 0 || p.Port > 65535 {
		return fmt.Errorf("port must be in 1-65535")
	}
	return nil
}

func mergeNotes(groups ...[]string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, group := range groups {
		for _, note := range group {
			note = strings.TrimSpace(note)
			if note == "" {
				continue
			}
			if _, ok := seen[note]; ok {
				continue
			}
			seen[note] = struct{}{}
			out = append(out, note)
		}
	}
	return out
}

func normalizeProfileText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimSuffix(s, ".exe")
	return s
}

func normalizeHash(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	replacer := strings.NewReplacer(" ", "", ":", "", "-", "")
	return replacer.Replace(s)
}

func portHitCount(src map[int]int, port int) int {
	if src == nil {
		return 0
	}
	return src[port]
}
