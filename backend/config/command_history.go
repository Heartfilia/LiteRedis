package config

import "strings"

const commandHistoryLimit = 100

// GetCommandHistory 读取持久化的命令历史（最近执行的在前）
func GetCommandHistory() []string {
	storeMu.RLock()
	defer storeMu.RUnlock()

	store, err := loadStore()
	if err != nil {
		return []string{}
	}
	if len(store.Settings.CommandHistory) == 0 {
		return []string{}
	}
	out := make([]string, len(store.Settings.CommandHistory))
	copy(out, store.Settings.CommandHistory)
	return out
}

// AppendCommandHistory 记录一条命令：去重后放到最前，最多保留 100 条
func AppendCommandHistory(command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}

	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}
	history := make([]string, 0, commandHistoryLimit)
	history = append(history, command)
	for _, item := range store.Settings.CommandHistory {
		if item == command {
			continue
		}
		if len(history) >= commandHistoryLimit {
			break
		}
		history = append(history, item)
	}
	store.Settings.CommandHistory = history
	return saveStore(store)
}

// ClearCommandHistory 清空命令历史
func ClearCommandHistory() error {
	storeMu.Lock()
	defer storeMu.Unlock()

	store, err := loadStore()
	if err != nil {
		return err
	}
	if len(store.Settings.CommandHistory) == 0 {
		return nil
	}
	store.Settings.CommandHistory = nil
	return saveStore(store)
}
