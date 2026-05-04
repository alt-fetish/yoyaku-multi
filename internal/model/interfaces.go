package model

import "github.com/google/uuid"

// EventReader は単一開催日の取得インターフェース
type EventReader interface {
	GetEvent(id uuid.UUID) (*Event, error)
}

// EventWriter は開催日の作成・更新インターフェース
type EventWriter interface {
	CreateEvent(e *Event) error
	UpdateEvent(e *Event) error
	DeleteEvent(id uuid.UUID) error
}

// EventLister は開催日一覧のインターフェース
type EventLister interface {
	ListEvents(status string) ([]*EventWithCount, error)
}

// ApplicantStore は登録者の操作インターフェース
type ApplicantStore interface {
	GetApplicant(id uuid.UUID) (*Applicant, error)
	GetApplicantByToken(token string) (*Applicant, error)
	GetApplicantByEmail(email string) (*Applicant, error)
	CreateApplicant(a *Applicant) error
	ListApplicants() ([]*ApplicantWithCount, error)
}

// EntryStore は参加表明の操作インターフェース
type EntryStore interface {
	GetEntry(eventID, applicantID uuid.UUID) (*EventEntry, error)
	CreateEntry(entry *EventEntry) error
	UpdateEntryStatus(id uuid.UUID, status string) error
	DeleteEntry(id uuid.UUID) error
	ListEntriesByEvent(eventID uuid.UUID) ([]*EntryWithApplicant, error)
	ListEntriesByApplicant(applicantID uuid.UUID) ([]*EntryWithEvent, error)
}

// SessionStore はセッション履歴の操作インターフェース
type SessionStore interface {
	ListSessions(statusFilter string) ([]*SessionWithParticipants, error)
	GetSession(id uuid.UUID) (*SessionWithParticipants, error)
	CreateSession(s *Session, applicantIDs []uuid.UUID) error
	UpdateSession(s *Session, applicantIDs []uuid.UUID) error
	DeleteSession(id uuid.UUID) error
}

// ChatStore はチャットメッセージの操作インターフェース
type ChatStore interface {
	GetMessages(applicantID uuid.UUID) ([]*ChatMessage, error)
	PostMessage(msg *ChatMessage) error
	MarkAsRead(applicantID uuid.UUID, sender string) error
	UnreadCount(applicantID uuid.UUID) (int, error)
}

// Mailer はメール送信インターフェース
type Mailer interface {
	Send(to, subject, body string) error
}
