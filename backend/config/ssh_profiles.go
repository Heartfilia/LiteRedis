package config

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
)

func normalizeSSH(cfg SSHConfig) SSHConfig {
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.User = strings.TrimSpace(cfg.User)
	cfg.PrivateKeyPath = strings.TrimSpace(cfg.PrivateKeyPath)
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	// Passwords and private-key passphrases are intentionally not trimmed.
	return cfg
}

func ValidateSSHConfig(cfg SSHConfig) error {
	if strings.TrimSpace(cfg.Host) == "" || strings.TrimSpace(cfg.User) == "" {
		return fmt.Errorf("SSH host and user are required")
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("SSH port must be between 1 and 65535")
	}
	return nil
}

// Migrate legacy inline SSH settings in memory. Stable IDs keep reads idempotent;
// migration is persisted only with the next explicit configuration save. Identical
// credentials share a profile; different credentials on the same host stay separate.
func ensureSSHProfiles(store *ConfigStore) {
	for i := range store.Connections {
		conn := &store.Connections[i]
		if conn.SSHProfileID == "" && conn.SSH != nil && conn.SSH.Host != "" {
			ssh := normalizeSSH(*conn.SSH)
			for _, profile := range store.SSHProfiles {
				if reflect.DeepEqual(profile.SSH, ssh) {
					conn.SSHProfileID = profile.ID
					break
				}
			}
			if conn.SSHProfileID == "" {
				id := uuid.NewSHA1(uuid.NameSpaceURL, []byte("LiteRedis/ssh/"+conn.ID)).String()
				for n := 2; sshIDUsed(store, id); n++ {
					id = uuid.NewSHA1(uuid.NameSpaceURL, []byte(fmt.Sprintf("LiteRedis/ssh/%s/%d", conn.ID, n))).String()
				}
				name := strings.TrimSpace(conn.Name) + " SSH"
				for n := 2; sshNameUsed(store, name, ""); n++ {
					name = fmt.Sprintf("%s SSH (%d)", strings.TrimSpace(conn.Name), n)
				}
				store.SSHProfiles = append(store.SSHProfiles, SSHProfile{ID: id, Name: name, SSH: ssh, CreatedAt: conn.CreatedAt, UpdatedAt: conn.UpdatedAt})
				conn.SSHProfileID = id
			}
		}
		// Keep an inline snapshot for old exports and existing transport code.
		// A dangling reference must never silently fall back to stale credentials.
		_ = resolveConnectionSSH(store, conn)
	}
}

func sshIDUsed(store *ConfigStore, id string) bool {
	for _, profile := range store.SSHProfiles {
		if profile.ID == id {
			return true
		}
	}
	return false
}

func sshNameUsed(store *ConfigStore, name, exceptID string) bool {
	for _, profile := range store.SSHProfiles {
		if profile.ID != exceptID && strings.EqualFold(profile.Name, name) {
			return true
		}
	}
	return false
}

func resolveConnectionSSH(store *ConfigStore, cfg *ConnectionConfig) error {
	if cfg.SSHProfileID == "" {
		return nil // Legacy inline configurations remain valid.
	}
	for _, profile := range store.SSHProfiles {
		if profile.ID == cfg.SSHProfileID {
			ssh := profile.SSH
			cfg.SSH = &ssh
			return nil
		}
	}
	if cfg.SSHEnabled {
		return fmt.Errorf("SSH account no longer exists; choose an SSH account again")
	}
	return nil
}

func ResolveConnectionSSH(cfg ConnectionConfig) (ConnectionConfig, error) {
	if cfg.SSHProfileID == "" {
		return cfg, nil
	}
	storeMu.RLock()
	defer storeMu.RUnlock()
	store, err := loadStore()
	if err != nil {
		return cfg, err
	}
	err = resolveConnectionSSH(store, &cfg)
	return cfg, err
}

func ListSSHProfiles() ([]SSHProfile, error) {
	storeMu.RLock()
	defer storeMu.RUnlock()
	store, err := loadStore()
	if err != nil {
		return nil, err
	}
	if store.SSHProfiles == nil {
		return []SSHProfile{}, nil
	}
	return store.SSHProfiles, nil
}

func SaveSSHProfile(profile SSHProfile) (SSHProfile, error) {
	profile.Name = strings.TrimSpace(profile.Name)
	profile.SSH = normalizeSSH(profile.SSH)
	if profile.Name == "" {
		return profile, fmt.Errorf("SSH account name is required")
	}
	if err := ValidateSSHConfig(profile.SSH); err != nil {
		return profile, err
	}
	storeMu.Lock()
	defer storeMu.Unlock()
	store, err := loadStore()
	if err != nil {
		return profile, err
	}
	if sshNameUsed(store, profile.Name, profile.ID) {
		return profile, fmt.Errorf("SSH account name already exists")
	}
	now := time.Now()
	profile.UpdatedAt = now
	for i, existing := range store.SSHProfiles {
		if existing.ID == profile.ID {
			profile.CreatedAt = existing.CreatedAt
			store.SSHProfiles[i] = profile
			return profile, saveStore(store)
		}
	}
	if profile.ID != "" {
		return profile, fmt.Errorf("SSH account no longer exists")
	}
	profile.ID = uuid.NewString()
	profile.CreatedAt = now
	store.SSHProfiles = append(store.SSHProfiles, profile)
	return profile, saveStore(store)
}

func DeleteSSHProfile(id string) error {
	storeMu.Lock()
	defer storeMu.Unlock()
	store, err := loadStore()
	if err != nil {
		return err
	}
	for _, conn := range store.Connections {
		if conn.SSHProfileID == id {
			return fmt.Errorf("SSH account is used by Redis connection %q; unlink it first", conn.Name)
		}
	}
	for i, profile := range store.SSHProfiles {
		if profile.ID == id {
			store.SSHProfiles = append(store.SSHProfiles[:i], store.SSHProfiles[i+1:]...)
			return saveStore(store)
		}
	}
	return fmt.Errorf("SSH account no longer exists")
}
