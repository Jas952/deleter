package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const sessionFileName = ".session.json"

// Session хранит данные авторизации
type Session struct {
	UserID               string                 `json:"user_id"`
	Cookies              map[string]interface{} `json:"cookies"`
	Headers              map[string]interface{} `json:"headers"`
	QueryIDUserTweets    string                 `json:"query_id_user_tweets"`
	QueryIDDeleteRetweet string                 `json:"query_id_delete_retweet"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
}

// SessionManager управляет сохранением и загрузкой сессии
type SessionManager struct {
	dataDir string
}

// NewSessionManager создает менеджер сессии
func NewSessionManager(dataDir string) *SessionManager {
	if dataDir == "" {
		dataDir = "."
	}
	return &SessionManager{dataDir: dataDir}
}

// Save сохраняет сессию в файл
func (sm *SessionManager) Save(session *Session) error {
	session.UpdatedAt = time.Now()
	if session.CreatedAt.IsZero() {
		session.CreatedAt = session.UpdatedAt
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	path := filepath.Join(sm.dataDir, sessionFileName)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}

	return nil
}

// Load загружает сессию из файла
func (sm *SessionManager) Load() (*Session, error) {
	path := filepath.Join(sm.dataDir, sessionFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("parse session: %w", err)
	}

	return &session, nil
}

// Exists проверяет существование файла сессии
func (sm *SessionManager) Exists() bool {
	path := filepath.Join(sm.dataDir, sessionFileName)
	_, err := os.Stat(path)
	return err == nil
}

// Delete удаляет файл сессии
func (sm *SessionManager) Delete() error {
	path := filepath.Join(sm.dataDir, sessionFileName)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove session file: %w", err)
	}
	return nil
}

// IsValid проверяет не протухла ли сессия (старше 14 дней)
func (s *Session) IsValid() bool {
	if s.UpdatedAt.IsZero() {
		return false
	}
	return time.Since(s.UpdatedAt) < 14*24*time.Hour
}

// DaysUntilExpiry возвращает количество дней до истечения срока
func (s *Session) DaysUntilExpiry() int {
	if s.UpdatedAt.IsZero() {
		return 0
	}
	expiry := s.UpdatedAt.Add(14 * 24 * time.Hour)
	days := int(time.Until(expiry).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}
